package pyramidize

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"keylint/internal/features/clipboard"
	"keylint/internal/features/settings"
	"keylint/internal/logger"
)

// Service implements the Pyramidize RPC methods exposed to the frontend.
type Service struct {
	settings  *settings.Service
	clipboard *clipboard.Service
	client    *http.Client

	mu         sync.Mutex
	cancelFunc context.CancelFunc

	// Captured source app from hotkey trigger (set before clipboard grab)
	sourceAppName  string
	sourceWindowID string
}

// NewService creates a new PyramidizeService.
func NewService(s *settings.Service, c *clipboard.Service) *Service {
	return &Service{
		settings:  s,
		clipboard: c,
		client:    &http.Client{Timeout: 90 * time.Second},
	}
}

// CaptureSourceApp captures the current foreground window before the clipboard grab.
// Called from main.go when the global hotkey fires, before any clipboard operation.
func (svc *Service) CaptureSourceApp() {
	name, id := captureSourceApp()
	svc.mu.Lock()
	svc.sourceAppName = name
	svc.sourceWindowID = id
	svc.mu.Unlock()
	logger.Info("pyramidize: captured source app", "name", name, "id", id)
}

// GetSourceApp returns the captured source app name so the frontend can display
// the detection indicator and look up a saved app preset.
func (svc *Service) GetSourceApp() string {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	return svc.sourceAppName
}

// CancelOperation cancels the current in-flight Pyramidize, RefineGlobal, or Splice
// call. Safe to call when nothing is running (no-op).
func (svc *Service) CancelOperation() {
	svc.mu.Lock()
	fn := svc.cancelFunc
	svc.mu.Unlock()
	if fn != nil {
		fn()
		logger.Info("pyramidize: operation cancelled by user")
	}
}

// SendBack writes text to the system clipboard and pastes it back into the
// captured source application window.
func (svc *Service) SendBack(text string) error {
	if svc.clipboard == nil {
		return fmt.Errorf("SendBack is not available in CLI mode")
	}
	if err := svc.clipboard.Write(text); err != nil {
		return fmt.Errorf("clipboard write failed: %w", err)
	}
	svc.mu.Lock()
	windowID := svc.sourceWindowID
	svc.mu.Unlock()
	return sendBackToWindow(windowID)
}

// Pyramidize is the main RPC: detects the document type (if AUTO), generates the
// foundation document, and applies a refinement pass if the quality score is below
// the configured threshold.
func (svc *Service) Pyramidize(req PyramidizeRequest) (PyramidizeResult, error) {
	ctx, cancel := context.WithCancel(context.Background())
	svc.mu.Lock()
	svc.cancelFunc = cancel
	svc.mu.Unlock()
	defer cancel()

	cfg := svc.settings.Get()
	opts := aiOpts{provider: req.Provider, model: req.Model}
	logger.Info("pyramidize: start", "docType", req.DocumentType, "provider", cfg.ActiveProvider,
		"overrideProvider", req.Provider, "overrideModel", req.Model)

	var result PyramidizeResult
	docType := strings.ToLower(req.DocumentType)

	// Step 1: Auto-detect if needed
	if docType == "auto" {
		detected, err := svc.detect(ctx, cfg, opts, req.Text)
		if err != nil {
			if ctx.Err() != nil {
				return PyramidizeResult{}, fmt.Errorf("cancelled")
			}
			logger.Warn("pyramidize: detection failed, defaulting to email", "err", err)
			docType = "email"
		} else {
			docType = strings.ToLower(detected.Type)
			if docType == "" || !isValidDocType(docType) {
				docType = "email"
			}
			result.DetectedType = strings.ToUpper(docType)
			result.DetectedLang = detected.Language
			result.DetectedConfidence = detected.Confidence
		}
	}

	// Step 2: Foundation generation
	foundation, err := svc.foundation(ctx, cfg, opts, req, docType)
	if err != nil {
		if ctx.Err() != nil {
			return PyramidizeResult{}, fmt.Errorf("cancelled")
		}
		return PyramidizeResult{}, fmt.Errorf("foundation step failed: %w", err)
	}

	result.DocumentType = strings.ToUpper(docType)
	result.Language = foundation.Language
	result.FullDocument = foundation.FullDocument
	result.Headers = foundation.Headers
	result.QualityScore = foundation.QualityScore
	result.QualityFlags = foundation.QualityFlags

	// Ensure QualityFlags is never nil in JSON response
	if result.QualityFlags == nil {
		result.QualityFlags = []string{}
	}

	// Step 3: Optional refinement pass
	threshold := cfg.PyramidizeQualityThreshold
	if threshold == 0 {
		threshold = 0.65
	}
	if foundation.QualityScore < threshold && len(foundation.QualityFlags) > 0 {
		logger.Info("pyramidize: quality below threshold, refining",
			"score", foundation.QualityScore, "flags", foundation.QualityFlags, "threshold", threshold)

		refined, err := svc.refine(ctx, cfg, opts, req.Text, foundation.FullDocument, foundation.QualityFlags)
		if err != nil {
			if ctx.Err() != nil {
				return PyramidizeResult{}, fmt.Errorf("cancelled")
			}
			// Refinement failure is non-fatal — return the foundation result with a warning
			logger.Warn("pyramidize: refinement failed, using foundation result", "err", err)
		} else {
			result.FullDocument = refined.FullDocument
			result.Headers = refined.Headers
			result.QualityScore = refined.QualityScore
			result.QualityFlags = refined.QualityFlags
			result.AppliedRefinement = true
			if result.QualityFlags == nil {
				result.QualityFlags = []string{}
			}
			if refined.QualityScore < threshold {
				result.RefinementWarning = fmt.Sprintf(
					"Quality score %.2f is still below threshold after refinement.", refined.QualityScore)
			}
		}
	}

	logger.Info("pyramidize: done",
		"docType", result.DocumentType, "score", result.QualityScore,
		"refined", result.AppliedRefinement)
	return result, nil
}

// RefineGlobal applies a user instruction to the full canvas document.
func (svc *Service) RefineGlobal(req RefineGlobalRequest) (RefineGlobalResult, error) {
	ctx, cancel := context.WithCancel(context.Background())
	svc.mu.Lock()
	svc.cancelFunc = cancel
	svc.mu.Unlock()
	defer cancel()

	cfg := svc.settings.Get()
	opts := aiOpts{provider: req.Provider, model: req.Model}
	systemPrompt, userMessage := buildGlobalRefinePrompt(
		req.FullCanvas, req.OriginalText, req.Instruction,
		req.DocumentType, req.CommunicationStyle, req.RelationshipLevel,
	)

	raw, err := svc.callAIWithContext(ctx, cfg, opts, systemPrompt, userMessage)
	if err != nil {
		if ctx.Err() != nil {
			return RefineGlobalResult{}, fmt.Errorf("cancelled")
		}
		return RefineGlobalResult{}, fmt.Errorf("RefineGlobal AI call failed: %w", err)
	}

	var r canvasResult
	if err := unmarshalRobust(raw, &r); err != nil {
		return RefineGlobalResult{}, fmt.Errorf("RefineGlobal parse error: %w (raw: %s)", err, raw)
	}
	return RefineGlobalResult{NewCanvas: r.NewCanvas}, nil
}

// Splice rewrites a selected section of the canvas according to a user instruction.
func (svc *Service) Splice(req SpliceRequest) (SpliceResult, error) {
	ctx, cancel := context.WithCancel(context.Background())
	svc.mu.Lock()
	svc.cancelFunc = cancel
	svc.mu.Unlock()
	defer cancel()

	cfg := svc.settings.Get()
	opts := aiOpts{provider: req.Provider, model: req.Model}
	systemPrompt, userMessage := buildSplicePrompt(
		req.FullCanvas, req.OriginalText, req.SelectedText, req.Instruction,
	)

	raw, err := svc.callAIWithContext(ctx, cfg, opts, systemPrompt, userMessage)
	if err != nil {
		if ctx.Err() != nil {
			return SpliceResult{}, fmt.Errorf("cancelled")
		}
		return SpliceResult{}, fmt.Errorf("Splice AI call failed: %w", err)
	}

	var r spliceResult
	if err := unmarshalRobust(raw, &r); err != nil {
		return SpliceResult{}, fmt.Errorf("Splice parse error: %w (raw: %s)", err, raw)
	}
	return SpliceResult{RewrittenSection: r.RewrittenSection}, nil
}

// GetAppPresets returns all saved source-app → doc-type presets from settings.
func (svc *Service) GetAppPresets() []AppPreset {
	cfg := svc.settings.Get()
	if len(cfg.AppPresets) == 0 {
		return []AppPreset{}
	}
	// Convert settings.AppPreset → pyramidize.AppPreset
	out := make([]AppPreset, len(cfg.AppPresets))
	for i, p := range cfg.AppPresets {
		out[i] = AppPreset{SourceApp: p.SourceApp, DocumentType: p.DocumentType}
	}
	return out
}

// SetAppPreset saves or updates an app preset matched by SourceApp name (case-insensitive).
func (svc *Service) SetAppPreset(preset AppPreset) error {
	cfg := svc.settings.Get()
	found := false
	for i, p := range cfg.AppPresets {
		if strings.EqualFold(p.SourceApp, preset.SourceApp) {
			cfg.AppPresets[i] = settings.AppPreset{
				SourceApp:    preset.SourceApp,
				DocumentType: preset.DocumentType,
			}
			found = true
			break
		}
	}
	if !found {
		cfg.AppPresets = append(cfg.AppPresets, settings.AppPreset{
			SourceApp:    preset.SourceApp,
			DocumentType: preset.DocumentType,
		})
	}
	return svc.settings.Save(cfg)
}

// DeleteAppPreset removes an app preset by source app name (case-insensitive).
func (svc *Service) DeleteAppPreset(sourceApp string) error {
	cfg := svc.settings.Get()
	filtered := cfg.AppPresets[:0]
	for _, p := range cfg.AppPresets {
		if !strings.EqualFold(p.SourceApp, sourceApp) {
			filtered = append(filtered, p)
		}
	}
	cfg.AppPresets = filtered
	return svc.settings.Save(cfg)
}

// GetQualityThreshold returns the configured quality threshold, defaulting to 0.65.
func (svc *Service) GetQualityThreshold() float64 {
	cfg := svc.settings.Get()
	if cfg.PyramidizeQualityThreshold == 0 {
		return 0.65
	}
	return cfg.PyramidizeQualityThreshold
}

// SetQualityThreshold saves the quality threshold. Value must be in [0, 1].
func (svc *Service) SetQualityThreshold(v float64) error {
	if v < 0 || v > 1 {
		return fmt.Errorf("threshold must be between 0 and 1, got %.2f", v)
	}
	cfg := svc.settings.Get()
	cfg.PyramidizeQualityThreshold = v
	return svc.settings.Save(cfg)
}

// aiOpts carries optional provider/model overrides for a single pipeline run.
// Empty strings fall back to the configured defaults.
type aiOpts struct {
	provider string // if empty, uses cfg.ActiveProvider
	model    string // if empty, uses provider built-in default
}

// --- internal pipeline helpers ---

func (svc *Service) detect(ctx context.Context, cfg settings.Settings, opts aiOpts, text string) (detectResult, error) {
	raw, err := svc.callAIWithContext(ctx, cfg, opts, detectPromptTemplate, text)
	if err != nil {
		return detectResult{}, err
	}
	var r detectResult
	if err := unmarshalRobust(raw, &r); err != nil {
		return detectResult{}, fmt.Errorf("detect parse error: %w (raw: %s)", err, raw)
	}
	return r, nil
}

func (svc *Service) foundation(ctx context.Context, cfg settings.Settings, opts aiOpts, req PyramidizeRequest, docType string) (foundationResult, error) {
	systemPrompt, userMessage := buildDocTypePrompt(docType, req.CommunicationStyle, req.RelationshipLevel, req.CustomInstructions, req.Text)
	raw, err := svc.callAIWithContext(ctx, cfg, opts, systemPrompt, userMessage)
	if err != nil {
		return foundationResult{}, err
	}
	var r foundationResult
	if err := unmarshalRobust(raw, &r); err != nil {
		return foundationResult{}, fmt.Errorf("foundation parse error: %w (raw: %s)", err, raw)
	}
	return r, nil
}

func (svc *Service) refine(ctx context.Context, cfg settings.Settings, opts aiOpts, originalText, failedOutput string, flags []string) (refineResult, error) {
	systemPrompt, userMessage := buildRefinePrompt(originalText, failedOutput, flags)
	raw, err := svc.callAIWithContext(ctx, cfg, opts, systemPrompt, userMessage)
	if err != nil {
		return refineResult{}, err
	}
	var r refineResult
	if err := unmarshalRobust(raw, &r); err != nil {
		return refineResult{}, fmt.Errorf("refine parse error: %w (raw: %s)", err, raw)
	}
	return r, nil
}

// buildDocTypePrompt dispatches to the correct prompt builder based on document type.
func buildDocTypePrompt(docType, style, relationship, customInstructions, text string) (systemPrompt, userMessage string) {
	switch docType {
	case "wiki":
		return buildWikiPrompt(style, relationship, customInstructions, text)
	case "memo":
		return buildMemoPrompt(style, relationship, customInstructions, text)
	case "powerpoint":
		return buildPPTPrompt(style, relationship, customInstructions, text)
	default: // "email" and any unrecognised type
		return buildEmailPrompt(style, relationship, customInstructions, text)
	}
}

// callAIWithContext runs an AI call in a goroutine and returns when the call
// completes or the context is cancelled (whichever comes first).
func (svc *Service) callAIWithContext(ctx context.Context, cfg settings.Settings, opts aiOpts, systemPrompt, userMessage string) (string, error) {
	type result struct {
		out string
		err error
	}
	ch := make(chan result, 1)

	go func() {
		out, err := svc.callAISync(cfg, opts, systemPrompt, userMessage)
		ch <- result{out, err}
	}()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case r := <-ch:
		return r.out, r.err
	}
}

// callAISync dispatches to the configured (or overridden) provider synchronously.
func (svc *Service) callAISync(cfg settings.Settings, opts aiOpts, systemPrompt, userMessage string) (string, error) {
	provider := opts.provider
	if provider == "" {
		provider = cfg.ActiveProvider
	}
	model := opts.model

	switch provider {
	case "openai":
		key := svc.settings.GetKey("openai")
		if key == "" {
			return "", fmt.Errorf("OpenAI API key is not configured — go to Settings → AI Providers")
		}
		return callOpenAI(svc.client, systemPrompt, userMessage, key, model)
	case "claude":
		key := svc.settings.GetKey("claude")
		if key == "" {
			return "", fmt.Errorf("Anthropic API key is not configured — go to Settings → AI Providers")
		}
		return callClaude(svc.client, systemPrompt, userMessage, key, model)
	case "ollama":
		return callOllama(svc.client, systemPrompt, userMessage, cfg.Providers.OllamaURL, model)
	default:
		return "", fmt.Errorf("unsupported provider: %q", provider)
	}
}

// isValidDocType reports whether s is a recognised Pyramidize document type.
func isValidDocType(s string) bool {
	switch s {
	case "email", "wiki", "memo", "powerpoint":
		return true
	}
	return false
}
