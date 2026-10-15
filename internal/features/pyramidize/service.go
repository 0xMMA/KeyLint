package pyramidize

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"keylint/internal/features/clipboard"
	"keylint/internal/features/settings"
	"keylint/internal/features/silentfix"
	"keylint/internal/llm"
	"keylint/internal/logger"
)

// maxTokens caps a Pyramidize reply, thinking included. Model IDs live in
// settings (#33 step 4). It is a ceiling, not a spend: a reply is billed for
// what it generates, and the model does not see the limit.
//
// It was 4096. Sonnet 5 and Opus 5 think by default when a request says
// nothing about thinking, and every thinking token counts against max_tokens.
// Measured with Sonnet 5 on the two largest eval samples: 2352–3822 output
// tokens at the old limit for a visible document of roughly 700–1250, and at
// 2048 v1 was cut off before a single visible token. Thinking and effort are
// left at the model default; changing them is an eval-gated quality decision
// (E3, #34).
//
// 16000 is Anthropic's recommendation for a non-streaming request. Here the
// tighter bound is time: each attempt has a 90 s HTTP timeout, roughly
// 8000–11000 tokens at the 90–125 tokens/s measured, so a generation that runs
// longer ends as a timeout (retried once by the SDK within callTimeout) rather
// than as an output-limit error. The timeout is also what lets 16000 go out
// non-streaming at all: the SDK refuses a non-streaming request whose
// max_tokens implies a very long generation unless the client sets a timeout,
// and every client here does. Pyramidize is a visible wait in the app, which
// is why it gets the room and the silent Fix hotkey does not (see enhance).
//
// Only the Anthropic client sends max_tokens. OpenAI, the OpenAI-compatible
// Ollama endpoint and the Claude Code CLI are sent no output limit, so no model
// with a small output limit can reject it.
const maxTokens = 16000

// logFeature tags this feature's provider calls in the log.
const logFeature = "pyramidize"

// callTimeout bounds a single AI call. The HTTP client carries its own timeout,
// but a local CLI provider has none — and the -pyramidize CLI has no cancel
// button to fall back on.
const callTimeout = 120 * time.Second

// Service implements the Pyramidize RPC methods exposed to the frontend.
type Service struct {
	settings *settings.Service
	client   *http.Client

	// clipboard and desktop are nil in CLI mode, which has no Send Back.
	clipboard clipboardWriter
	desktop   sendBackDesktop
	sleep     func(time.Duration)
	now       func() time.Time
	selfPID   uint32

	mu         sync.Mutex
	cancelFunc context.CancelFunc

	// Captured source app from hotkey trigger (set before clipboard grab)
	sourceAppName string
	sourceWindow  silentfix.Window

	// sendingBack is held while a Send Back runs.
	sendingBack sync.Mutex

	// newClient builds the provider client. Tests replace it with a fake.
	newClient func(provider string, cfg llm.Config) (llm.Client, error)
}

// NewService creates a new PyramidizeService. c is nil in CLI mode.
func NewService(s *settings.Service, c *clipboard.Service) *Service {
	svc := &Service{
		settings:  s,
		client:    &http.Client{Timeout: 90 * time.Second},
		sleep:     time.Sleep,
		now:       time.Now,
		selfPID:   uint32(os.Getpid()),
		newClient: llm.New,
	}
	// Assigned only when set: a nil *clipboard.Service in an interface
	// field is not a nil interface.
	if c != nil {
		svc.clipboard = c
		svc.desktop = newDesktop(c)
	}
	return svc
}

// CaptureSourceApp captures the current foreground window before the clipboard grab.
// Called from main.go when the global hotkey fires, before any clipboard operation.
func (svc *Service) CaptureSourceApp() {
	if svc.desktop == nil {
		return
	}
	w := svc.desktop.Foreground()
	if w.Handle != 0 && w.PID == svc.selfPID {
		// The shortcut was pressed in KeyLint itself: Send Back must not
		// paste into KeyLint's own window, so the last source stays.
		logger.Info("pyramidize: shortcut pressed in KeyLint, source app kept")
		return
	}
	name := ""
	if w.Handle != 0 {
		name = svc.desktop.Title(w)
	}
	svc.mu.Lock()
	svc.sourceAppName = name
	svc.sourceWindow = w
	svc.mu.Unlock()
	logger.Info("pyramidize: captured source app", "name", logger.Redact(name), "window", w.Handle, "pid", w.PID)
}

// GetSourceApp returns the captured source app name so the frontend can display
// the detection indicator and look up a saved app preset.
func (svc *Service) GetSourceApp() string {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	return svc.sourceAppName
}

// beginCall starts a cancellable call and cancels the one still in flight, if
// any: Pyramidize, RefineGlobal and Splice share one cancel, and a call left
// running behind a newer one would keep running (and billing) where no Cancel
// can reach it.
func (svc *Service) beginCall() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	svc.mu.Lock()
	previous := svc.cancelFunc
	svc.cancelFunc = cancel
	svc.mu.Unlock()
	if previous != nil {
		previous()
	}
	return ctx, cancel
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

// Pyramidize is the main RPC: it writes the text in the form the user picked,
// in exactly one model call (ADR-003). "auto" picks no form by model: it is
// the app preset for the captured source app, else email.
func (svc *Service) Pyramidize(req PyramidizeRequest) (PyramidizeResult, error) {
	ctx, cancel := svc.beginCall()
	defer cancel()

	cfg := svc.settings.Get()
	docType, err := resolveDocType(req.DocumentType, cfg.AppPresets, svc.GetSourceApp())
	if err != nil {
		return PyramidizeResult{}, err
	}
	systemPrompt, userMessage, schema, err := buildDocTypePrompt(docType, req.PromptVariant,
		req.CustomInstructions, req.Text)
	if err != nil {
		return PyramidizeResult{}, err
	}

	opts := aiOpts{provider: req.Provider, model: req.Model, effort: req.Effort}
	logger.Info("pyramidize: start", "docType", docType, "requestedDocType", req.DocumentType,
		"variant", req.PromptVariant, "provider", cfg.ActiveProvider,
		"overrideProvider", req.Provider, "overrideModel", req.Model, "overrideEffort", req.Effort)

	raw, err := svc.callAIWithContext(ctx, cfg, opts, systemPrompt, userMessage, schema)
	if err != nil {
		if ctx.Err() != nil {
			return PyramidizeResult{}, fmt.Errorf("cancelled")
		}
		return PyramidizeResult{}, fmt.Errorf("AI call failed: %w", err)
	}
	var r documentResult
	if err := unmarshalRobust(raw, &r); err != nil {
		return PyramidizeResult{}, unreadableReply("pyramidize", raw, err)
	}
	// Pasted over the user's text by Send back, an empty result would erase it.
	if strings.TrimSpace(r.FullDocument) == "" {
		logger.Warn("pyramidize: the reply has no document", "raw", logger.Redact(raw))
		return PyramidizeResult{}, fmt.Errorf("the model returned no document")
	}

	logger.Info("pyramidize: done", "docType", docType)
	return PyramidizeResult{
		DocumentType: strings.ToUpper(docType),
		Language:     r.Language,
		FullDocument: r.FullDocument,
	}, nil
}

// unreadableReply logs a reply that is not the JSON asked for — redacted, since
// it may hold the user's text — and returns an error the UI can show. The
// error used to carry the whole reply, and with it the user's draft.
func unreadableReply(step, raw string, err error) error {
	logger.Warn("pyramidize: unreadable reply", "step", step, "err", err, "raw", logger.Redact(raw))
	return fmt.Errorf("%s: the model's reply could not be read", step)
}

// SourcePreset is what the choice page shows about the captured source app.
type SourcePreset struct {
	// App is the name a form is remembered under: the matched preset's own
	// name, else the app segment of the window title ("Outlook" for "Inbox -
	// Outlook"), which is what presetDocType searches first. "" when nothing
	// was captured.
	App string `json:"app"`
	// DocumentType is the form to preselect: the matched preset's, else email
	// — what "auto" would write, so the UI shows the choice instead of making
	// it out of sight.
	DocumentType string `json:"documentType"`
	// Remembered is whether a preset matched; App is then its name.
	Remembered bool `json:"remembered"`
}

// GetSourcePreset tells the choice page which form to preselect for a
// captured window title, whether that is a remembered preset, and under which
// name "Remember <form> for <app>" stores one. The page passes its session's
// title: the backend's own capture may be newer (a replace the user declined).
func (svc *Service) GetSourcePreset(title string) SourcePreset {
	if name, docType, ok := matchPreset(svc.settings.Get().AppPresets, title); ok {
		return SourcePreset{App: name, DocumentType: docType, Remembered: true}
	}
	return SourcePreset{App: rememberName(title), DocumentType: "email"}
}

// maxRememberName is the longest name offered for remembering. An app is
// called "Outlook" or "Microsoft Teams"; anything longer is more likely a
// document or a subject, which is not stored.
const maxRememberName = 40

// rememberName is the name a new preset for this title is stored under: the
// segment the matcher searches first (titleParts), or "" when nothing safe
// can be stored — a title without a separator (it may be a person's name or a
// document), a name under three characters (the matcher ignores those), or
// one longer than maxRememberName.
func rememberName(title string) string {
	if len(titleSegments(title)) < 2 {
		return ""
	}
	name, _ := titleParts(title)
	if n := utf8.RuneCountInString(name); n < 3 || n > maxRememberName {
		return ""
	}
	return name
}

// RefineGlobal applies a user instruction to the full canvas document.
func (svc *Service) RefineGlobal(req RefineGlobalRequest) (RefineGlobalResult, error) {
	ctx, cancel := svc.beginCall()
	defer cancel()

	cfg := svc.settings.Get()
	opts := aiOpts{provider: req.Provider, model: req.Model, effort: req.Effort}
	systemPrompt, userMessage := buildGlobalRefinePrompt(
		req.FullCanvas, req.OriginalText, req.Instruction, req.DocumentType,
	)

	raw, err := svc.callAIWithContext(ctx, cfg, opts, systemPrompt, userMessage, canvasSchema)
	if err != nil {
		if ctx.Err() != nil {
			return RefineGlobalResult{}, fmt.Errorf("cancelled")
		}
		return RefineGlobalResult{}, fmt.Errorf("RefineGlobal AI call failed: %w", err)
	}

	var r canvasResult
	if err := unmarshalRobust(raw, &r); err != nil {
		return RefineGlobalResult{}, unreadableReply("RefineGlobal", raw, err)
	}
	return RefineGlobalResult{NewCanvas: r.NewCanvas}, nil
}

// Splice rewrites a selected section of the canvas according to a user instruction.
func (svc *Service) Splice(req SpliceRequest) (SpliceResult, error) {
	ctx, cancel := svc.beginCall()
	defer cancel()

	cfg := svc.settings.Get()
	opts := aiOpts{provider: req.Provider, model: req.Model, effort: req.Effort}
	systemPrompt, userMessage := buildSplicePrompt(
		req.FullCanvas, req.OriginalText, req.SelectedText, req.Instruction,
	)

	raw, err := svc.callAIWithContext(ctx, cfg, opts, systemPrompt, userMessage, spliceSchema)
	if err != nil {
		if ctx.Err() != nil {
			return SpliceResult{}, fmt.Errorf("cancelled")
		}
		return SpliceResult{}, fmt.Errorf("Splice AI call failed: %w", err)
	}

	var r spliceResult
	if err := unmarshalRobust(raw, &r); err != nil {
		return SpliceResult{}, unreadableReply("Splice", raw, err)
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

// samePresetName reports whether two preset names are one preset: compared
// trimmed and case-folded, on save, match and delete alike.
func samePresetName(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

// SetAppPreset saves or updates an app preset matched by SourceApp name
// (trimmed, case-insensitive). The name is stored trimmed.
func (svc *Service) SetAppPreset(preset AppPreset) error {
	preset.SourceApp = strings.TrimSpace(preset.SourceApp)
	if preset.SourceApp == "" {
		return fmt.Errorf("an app preset needs an app name")
	}
	// Through settings.Update, so a save landing meanwhile is not overwritten.
	return settings.Update(svc.settings, func(cfg *settings.Settings) {
		for i, p := range cfg.AppPresets {
			if samePresetName(p.SourceApp, preset.SourceApp) {
				cfg.AppPresets[i] = settings.AppPreset{
					SourceApp:    preset.SourceApp,
					DocumentType: preset.DocumentType,
				}
				return
			}
		}
		cfg.AppPresets = append(cfg.AppPresets, settings.AppPreset{
			SourceApp:    preset.SourceApp,
			DocumentType: preset.DocumentType,
		})
	})
}

// DeleteAppPreset removes an app preset by source app name (trimmed, case-insensitive).
func (svc *Service) DeleteAppPreset(sourceApp string) error {
	return settings.Update(svc.settings, func(cfg *settings.Settings) {
		filtered := make([]settings.AppPreset, 0, len(cfg.AppPresets))
		for _, p := range cfg.AppPresets {
			if !samePresetName(p.SourceApp, sourceApp) {
				filtered = append(filtered, p)
			}
		}
		cfg.AppPresets = filtered
	})
}

// aiOpts carries optional provider/model overrides for a single call.
// Empty strings fall back to the configured defaults.
type aiOpts struct {
	provider string // if empty, uses cfg.ActiveProvider
	model    string // if empty, uses provider built-in default
	// effort overrides the provider's Pyramidize effort from settings for one
	// run: "" follows settings, EffortNone sends none, a level is sent as is.
	effort string
	// temperature pins sampling. nil leaves it to the provider, which is what
	// the product does; the eval judge sets it so its scores are repeatable.
	temperature *float64
	// maxTokens overrides Pyramidize's output limit for one call. 0 means
	// its own. Only the eval judge sets it: an instrument must not
	// move when the product's limit does.
	maxTokens int
	// noEffort sends no effort whatever settings say. The eval judge sets it
	// for the same reason as maxTokens: a pinned instrument must not pick up
	// the effort a user (or a later eval configuration) chose for Pyramidize.
	noEffort bool
}

// callAIWithContext resolves the API key and runs an AI call. The context is
// passed through to the HTTP request, so cancelling it aborts the call in flight.
func (svc *Service) callAIWithContext(ctx context.Context, cfg settings.Settings, opts aiOpts, systemPrompt, userMessage string, schema json.RawMessage) (string, error) {
	provider := opts.provider
	if provider == "" {
		provider = cfg.ActiveProvider
	}
	apiKey, err := svc.resolveAPIKey(provider)
	if err != nil {
		return "", err
	}

	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	// One gate for every step: the callers still choose the right schema, so the
	// mapping stays tested even while enforcement is off.
	return svc.callAISync(callCtx, cfg, opts, apiKey, systemPrompt, userMessage, enforcedSchema(schema))
}

// resolveAPIKey fetches the API key for the given provider from the keyring.
// Returns an empty string (no error) for providers that don't require a key.
func (svc *Service) resolveAPIKey(provider string) (string, error) {
	switch provider {
	case "openai":
		key := svc.settings.GetKey("openai")
		if key == "" {
			return "", fmt.Errorf("OpenAI API key is not configured — go to Settings → AI Providers")
		}
		return key, nil
	case "claude":
		key := svc.settings.GetKey("claude")
		if key == "" {
			return "", fmt.Errorf("Anthropic API key is not configured — go to Settings → AI Providers")
		}
		return key, nil
	case llm.ProviderOpenRouter:
		key := svc.settings.GetKey(llm.ProviderOpenRouter)
		if key == "" {
			return "", fmt.Errorf("OpenRouter API key is not configured — go to Settings → AI Providers")
		}
		return key, nil
	default:
		return "", nil
	}
}

// callAISync dispatches to the configured (or overridden) provider.
// The apiKey is resolved by the caller — this function has no keyring dependency.
func (svc *Service) callAISync(ctx context.Context, cfg settings.Settings, opts aiOpts, apiKey, systemPrompt, userMessage string, schema json.RawMessage) (string, error) {
	provider := opts.provider
	if provider == "" {
		provider = cfg.ActiveProvider
	}

	clientCfg, model, err := svc.providerConfig(provider, cfg, apiKey, opts.model)
	if err != nil {
		return "", err
	}

	newClient := svc.newClient
	if newClient == nil {
		newClient = llm.New
	}
	client, err := newClient(provider, clientCfg)
	if err != nil {
		return "", err
	}

	limit := maxTokens
	if opts.maxTokens > 0 {
		limit = opts.maxTokens
	}
	resp, err := client.Complete(ctx, llm.Request{
		System:    systemPrompt,
		User:      userMessage,
		Model:     model,
		MaxTokens: limit,
		// Every step here parses JSON, so ask for an object even when schema
		// enforcement is off — that is what this pipeline did before schemas
		// existed, and losing it would leave OpenAI and Ollama unconstrained.
		JSONMode:    true,
		JSONSchema:  schema,
		Temperature: opts.temperature,
		// The provider's effort for Pyramidize; "" leaves the model on its
		// default. No NoThinking here: Pyramidize has a 16000-token limit and
		// is where reasoning is worth its time.
		Effort: pyramidizeEffort(cfg, provider, opts),
	})
	if err != nil {
		return "", err
	}
	return resp.Text, nil
}

// providerConfig resolves endpoint and model ID for a provider. A request-level
// override wins over the model in settings, which in turn wins over KeyLint's
// default — see Settings.ModelFor.
func (svc *Service) providerConfig(provider string, cfg settings.Settings, apiKey, modelOverride string) (llm.Config, string, error) {
	// A request-level override wins: the CLI's -pyramidize --model and the
	// eval set one. The app sends none since the choice page (E3 phase 3).
	model := strings.TrimSpace(modelOverride)
	if model == "" {
		model = cfg.ModelFor(provider, llm.FeaturePyramidize)
	}

	switch provider {
	case llm.ProviderOpenAI:
		return llm.Config{APIKey: apiKey, HTTPClient: svc.client, Feature: logFeature}, model, nil
	case llm.ProviderClaude, llm.ProviderOpenRouter:
		return llm.Config{APIKey: apiKey, HTTPClient: svc.client, Feature: logFeature}, model, nil
	case llm.ProviderClaudeCode:
		// The user signed in to the CLI themselves; KeyLint needs no key and
		// never touches their credentials.
		return llm.Config{Feature: logFeature}, model, nil
	case llm.ProviderOllama:
		return llm.Config{
			BaseURL:    cfg.Providers.OllamaURL,
			HTTPClient: svc.client,
			Feature:    logFeature,
		}, model, nil
	default:
		return llm.Config{}, "", fmt.Errorf("unsupported provider: %q", provider)
	}
}

// isValidDocType reports whether s is a form Pyramidize writes. "powerpoint"
// keeps its old name because app presets and the CLI already store it.
func isValidDocType(s string) bool {
	switch s {
	case "email", "chat", "memo", "wiki", "powerpoint":
		return true
	}
	return false
}

// normalizeDocType lower-cases a document type and maps "presentation", the
// form's name in ADR-003, onto "powerpoint".
func normalizeDocType(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "presentation" {
		return "powerpoint"
	}
	return s
}

// resolveDocType turns a request's document type into the form to write.
// "auto" and "" are a lookup, never a model call: the app preset for
// sourceApp, else email. Anything else must name a form — an unknown type used
// to run the email prompt without a word.
func resolveDocType(requested string, presets []settings.AppPreset, sourceApp string) (string, error) {
	docType := normalizeDocType(requested)
	if docType == "" || docType == "auto" {
		return presetDocType(presets, sourceApp), nil
	}
	if !isValidDocType(docType) {
		return "", fmt.Errorf("unknown document type %q (want email, chat, memo, wiki or powerpoint)", requested)
	}
	return docType, nil
}

// presetDocType is the form an app preset names for sourceApp, else email.
//
// The captured source app is a window title, and on Windows a title ends in
// the app: "Inbox - Outlook", "Report.docx – Word". So the app segment (after
// the last " - ", " – " or " — ") is searched first and the document part
// only when the app segment matches no preset — a browser tab ("Inbox - Gmail
// - Google Chrome") names its web app there. A preset name matches as whole
// words, ignoring case, and needs at least three characters: "Mail" must not
// match "Gmail" or "Email draft.docx". Within a part the longest name wins, so
// "Outlook Web" beats "Outlook". A preset naming no form (a hand-edited
// settings file) is skipped rather than trusted.
func presetDocType(presets []settings.AppPreset, sourceApp string) string {
	if _, docType, ok := matchPreset(presets, sourceApp); ok {
		return docType
	}
	return "email"
}

// matchPreset finds the preset presetDocType applies to sourceApp, with the
// name it is stored under (trimmed).
func matchPreset(presets []settings.AppPreset, sourceApp string) (name, docType string, ok bool) {
	primary, rest := titleParts(sourceApp)
	for _, part := range []string{primary, rest} {
		if p, found := bestPreset(presets, part); found {
			return strings.TrimSpace(p.SourceApp), normalizeDocType(p.DocumentType), true
		}
	}
	return "", "", false
}

// titleSeparators split a window title into segments. Windows titles end in
// the app ("Inbox - Outlook", "Report.docx – Word", "Inbox — Mozilla
// Firefox"); new Teams separates with a bar ("Chat | Max Mustermann |
// Microsoft Teams").
var titleSeparators = []string{" - ", " – ", " — ", " | "}

// titleSegments splits a title at every separator, trimmed, empty ones dropped.
func titleSegments(title string) []string {
	parts := []string{title}
	for _, sep := range titleSeparators {
		var next []string
		for _, p := range parts {
			next = append(next, strings.Split(p, sep)...)
		}
		parts = next
	}
	var out []string
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// browsers are app segments that name a browser rather than the app the user
// is in: there the web app is the segment before it ("… - Gmail - Google
// Chrome"). Kept small on purpose; compared ignoring case and zero-width
// spaces (Edge writes "Microsoft​ Edge"). A profile segment between the
// web app and the browser (Edge: "… - Gmail - Work - Microsoft Edge") is not
// recognised — the roadmap's Windows check covers real titles.
var browsers = map[string]bool{
	"google chrome": true, "chromium": true, "microsoft edge": true,
	"mozilla firefox": true, "firefox": true, "brave": true,
	"opera": true, "vivaldi": true, "safari": true,
}

func isBrowser(segment string) bool {
	plain := strings.Join(strings.Fields(strings.ReplaceAll(segment, "​", "")), " ")
	return browsers[strings.ToLower(plain)]
}

// titleParts says where a title names its app. primary is the app segment
// (the last one), or for a browser the web app before it: it is searched
// first, and it is what a new preset is stored under (rememberName). rest is
// searched only when primary matches nothing — for an app, its document part
// (a Teams-only preset can then still match an Outlook mail titled "Microsoft
// Teams rollout"); for a browser, only the browser's own name, since another
// page's title is not the app. A title without a separator is all primary.
func titleParts(title string) (primary, rest string) {
	segments := titleSegments(title)
	switch len(segments) {
	case 0:
		return "", ""
	case 1:
		return segments[0], ""
	}
	last := segments[len(segments)-1]
	if isBrowser(last) {
		return segments[len(segments)-2], last
	}
	return last, strings.Join(segments[:len(segments)-1], " - ")
}

// bestPreset is the preset with the longest name found in text, if any.
func bestPreset(presets []settings.AppPreset, text string) (settings.AppPreset, bool) {
	text = strings.ToLower(text)
	var best settings.AppPreset
	bestLen := 0
	for _, p := range presets {
		name := strings.ToLower(strings.TrimSpace(p.SourceApp))
		docType := normalizeDocType(p.DocumentType)
		if utf8.RuneCountInString(name) < 3 || !isValidDocType(docType) || !containsWord(text, name) {
			continue
		}
		if len(name) > bestLen {
			best, bestLen = p, len(name)
		}
	}
	return best, bestLen > 0
}

// containsWord reports whether word occurs in text with no letter or digit
// directly before or after it. Both are already lower case.
func containsWord(text, word string) bool {
	for from := 0; from <= len(text); {
		i := strings.Index(text[from:], word)
		if i < 0 {
			return false
		}
		start, end := from+i, from+i+len(word)
		before, _ := utf8.DecodeLastRuneInString(text[:start])
		after, _ := utf8.DecodeRuneInString(text[end:])
		if !isWordRune(before) && !isWordRune(after) {
			return true
		}
		from = start + 1
	}
	return false
}

func isWordRune(r rune) bool {
	return r != utf8.RuneError && (unicode.IsLetter(r) || unicode.IsDigit(r))
}

// pyramidizeEffort is the effort a call sends: the request's own for this run
// (no caller sets one today: the app sends none since the choice page, and
// the CLI and the eval have no effort flag), else the provider's Pyramidize effort from settings, or none for a call
// that must not take it. An override that is no level is ignored, as an
// unknown level in settings is.
func pyramidizeEffort(cfg settings.Settings, provider string, opts aiOpts) string {
	if opts.noEffort {
		return ""
	}
	switch effort := strings.TrimSpace(opts.effort); {
	case effort == EffortNone:
		return ""
	case llm.IsEffortLevel(effort):
		return effort
	}
	return cfg.EffortFor(provider, llm.FeaturePyramidize)
}
