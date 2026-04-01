package pyramidize

// PyramidizeRequest is the RPC request for the main Pyramidize call.
type PyramidizeRequest struct {
	Text               string `json:"text"`
	DocumentType       string `json:"documentType"`       // "auto"|"email"|"wiki"|"powerpoint"|"memo"
	CommunicationStyle string `json:"communicationStyle"` // "professional"|"casual"|"concise"|"detailed"|"persuasive"|"neutral"|"diplomatic"|"direct"
	RelationshipLevel  string `json:"relationshipLevel"`  // "close"|"professional"|"authority"|"public"
	CustomInstructions string `json:"customInstructions"` // optional, not persisted
	Provider           string `json:"provider"`           // optional override: "claude"|"openai"|"ollama" — falls back to settings.ActiveProvider
	Model              string `json:"model"`              // optional override, e.g. "claude-sonnet-4-6" — falls back to provider default
	PromptVariant      int    `json:"promptVariant"`      // 0 = latest (default), 1 = v1, 2 = v2, etc.
}

// PyramidizeResult is the RPC response from the main Pyramidize call.
type PyramidizeResult struct {
	DocumentType       string   `json:"documentType"`
	Language           string   `json:"language"`
	FullDocument       string   `json:"fullDocument"`      // first line = subject/title
	Headers            []string `json:"headers"`
	QualityScore       float64  `json:"qualityScore"`
	QualityFlags       []string `json:"qualityFlags"`
	AppliedRefinement  bool     `json:"appliedRefinement"`
	RefinementWarning  string   `json:"refinementWarning"` // non-empty if still below threshold after retry
	DetectedType       string   `json:"detectedType"`      // only set when AUTO was used
	DetectedLang       string   `json:"detectedLang"`
	DetectedConfidence float64  `json:"detectedConfidence"`
}

// RefineGlobalRequest is the RPC request for a full-canvas AI revision.
type RefineGlobalRequest struct {
	FullCanvas         string `json:"fullCanvas"`
	OriginalText       string `json:"originalText"`
	Instruction        string `json:"instruction"`
	DocumentType       string `json:"documentType"`
	CommunicationStyle string `json:"communicationStyle"`
	RelationshipLevel  string `json:"relationshipLevel"`
	Provider           string `json:"provider"`
	Model              string `json:"model"`
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

// detectResult is the internal struct for parsing the detection response.
type detectResult struct {
	Type       string  `json:"type"`
	Language   string  `json:"language"`
	Confidence float64 `json:"confidence"`
}

// foundationResult is the internal struct for parsing the foundation call response.
type foundationResult struct {
	FullDocument string   `json:"fullDocument"`
	Headers      []string `json:"headers"`
	Language     string   `json:"language"`
	QualityScore float64  `json:"qualityScore"`
	QualityFlags []string `json:"qualityFlags"`
}

// refineResult is for parsing the refinement call response.
type refineResult struct {
	FullDocument string   `json:"fullDocument"`
	Headers      []string `json:"headers"`
	Language     string   `json:"language"`
	QualityScore float64  `json:"qualityScore"`
	QualityFlags []string `json:"qualityFlags"`
}

// canvasResult is for the global canvas instruction response.
type canvasResult struct {
	NewCanvas string `json:"newCanvas"`
}

// spliceResult is for the selection splice response.
type spliceResult struct {
	RewrittenSection string `json:"rewrittenSection"`
}
