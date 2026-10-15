package pyramidize

// PyramidizeRequest is the RPC request for the main Pyramidize call.
type PyramidizeRequest struct {
	Text         string `json:"text"`
	DocumentType string `json:"documentType"` // "email"|"chat"|"memo"|"wiki"|"powerpoint" ("presentation" too); "auto"/"" = the app preset, else email
	// CustomInstructions are the user's own words ("more formal", "in
	// English"), optional and not persisted: with the form, the only choice
	// the user makes. Communication style and relationship level went with
	// the options column (roadmap E3 phase 3); v2 runs on the
	// professional/professional it was measured with.
	CustomInstructions string `json:"customInstructions"`
	Provider           string `json:"provider"`      // optional override: "claude"|"claude-code"|"openai"|"openrouter"|"bedrock"|"ollama" — falls back to settings.ActiveProvider
	Model              string `json:"model"`         // optional override, e.g. "claude-sonnet-4-6" — falls back to provider default
	Effort             string `json:"effort"`        // optional override: "" follows settings, EffortNone sends none, else a level (llm.Effort*)
	PromptVariant      int    `json:"promptVariant"` // 0 = latest (v3, every form); 2 = the v2 email prompt, email only
}

// EffortNone, as a request's Effort, sends no effort for that run even when
// settings name one for Pyramidize. An empty Effort follows settings instead,
// as an empty Model does. Nothing sends either since the choice page removed
// the page's effort picker (roadmap housekeeping: drop the dead overrides).
const EffortNone = "none"

// PyramidizeResult is the RPC response from the main Pyramidize call — one
// model call, so there is nothing to report beyond what it wrote. The quality
// score, flags, refinement and detection fields went with the steps that
// produced them (ADR-003).
type PyramidizeResult struct {
	// DocumentType is the form that was written, upper case ("EMAIL"): the
	// requested one, or what "auto" resolved to.
	DocumentType string `json:"documentType"`
	Language     string `json:"language"`
	FullDocument string `json:"fullDocument"` // first line = subject/title where the form has one
}

// RefineGlobalRequest is the RPC request for a full-canvas AI revision.
type RefineGlobalRequest struct {
	FullCanvas   string `json:"fullCanvas"`
	OriginalText string `json:"originalText"`
	Instruction  string `json:"instruction"`
	DocumentType string `json:"documentType"`
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	Effort       string `json:"effort"` // see PyramidizeRequest.Effort
}

// RefineGlobalResult is the RPC response for a full-canvas AI revision.
type RefineGlobalResult struct {
	NewCanvas string `json:"newCanvas"`
}

// SpliceRequest is the RPC request for rewriting a selected canvas section.
type SpliceRequest struct {
	FullCanvas   string `json:"fullCanvas"`
	OriginalText string `json:"originalText"`
	SelectedText string `json:"selectedText"`
	Instruction  string `json:"instruction"`
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	Effort       string `json:"effort"` // see PyramidizeRequest.Effort
}

// SpliceResult is the RPC response for rewriting a selected canvas section.
type SpliceResult struct {
	RewrittenSection string `json:"rewrittenSection"`
}

// AppPreset maps a source application name to a preferred document type.
// Mirrors settings.AppPreset — kept separate to avoid circular imports.
type AppPreset struct {
	SourceApp    string `json:"sourceApp"`
	DocumentType string `json:"documentType"`
}

// documentResult is what a Pyramidize reply is parsed into. v3 asks for
// exactly these fields; v2 also returns headers, which nothing reads.
type documentResult struct {
	FullDocument string `json:"fullDocument"`
	Language     string `json:"language"`
}

// canvasResult is for the global canvas instruction response.
type canvasResult struct {
	NewCanvas string `json:"newCanvas"`
}

// spliceResult is for the selection splice response.
type spliceResult struct {
	RewrittenSection string `json:"rewrittenSection"`
}
