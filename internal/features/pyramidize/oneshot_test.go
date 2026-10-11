package pyramidize

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"keylint/internal/features/settings"
	"keylint/internal/llm"
)

// ADR-003: the user picks the form, one call writes it. These tests pin the
// run path through the real Pyramidize entry point with a counting client —
// restating a condition in a test passes happily while the real one is edited.

// countingClient answers every call with the same reply and records what it
// was asked.
type countingClient struct {
	calls int
	// reply is what every call answers with.
	reply    string
	requests []llm.Request
}

func (c *countingClient) Complete(_ context.Context, req llm.Request) (llm.Response, error) {
	c.calls++
	c.requests = append(c.requests, req)
	return llm.Response{Text: c.reply}, nil
}

// A v3-shaped reply: the document and its language.
const replyV3 = `{"fullDocument":"doc","language":"de"}`

// The same document with the old self-QA fields volunteered — a low score and
// a flag, which is what the refine pass used to act on.
const replyWithSelfQA = `{"fullDocument":"doc","headers":["h"],"language":"de",` +
	`"qualityScore":0.1,"qualityFlags":["MECE_VIOLATION"]}`

// newCountingService builds a Service over cfg whose every provider call goes
// to the returned counter.
func newCountingService(cfg settings.Settings, reply string) (*Service, *countingClient) {
	counter := &countingClient{reply: reply}
	settingsSvc := settings.NewServiceFrom(cfg, func(string) string { return "key" })
	svc := NewService(settingsSvc, nil)
	svc.newClient = func(string, llm.Config) (llm.Client, error) { return counter, nil }
	return svc, counter
}

// pyramidizeWithCounter runs Pyramidize once on default settings.
func pyramidizeWithCounter(t *testing.T, reply string, req PyramidizeRequest) (*countingClient, PyramidizeResult) {
	t.Helper()
	svc, counter := newCountingService(settings.Default(), reply)
	if req.Provider == "" {
		req.Provider = "claude"
	}
	result, err := svc.Pyramidize(req)
	if err != nil {
		t.Fatalf("Pyramidize: %v", err)
	}
	return counter, result
}

// TestEveryFormCostsOneCall: one call per run, whatever the form, the prompt
// variant or "auto". There is no detect step and no refine pass left to add one.
func TestEveryFormCostsOneCall(t *testing.T) {
	cases := []PyramidizeRequest{
		{DocumentType: "email"},
		{DocumentType: "chat"},
		{DocumentType: "memo"},
		{DocumentType: "wiki"},
		{DocumentType: "powerpoint"},
		{DocumentType: "presentation"},
		{DocumentType: "auto"},
		{DocumentType: ""},
		{DocumentType: "email", PromptVariant: 2},
		{DocumentType: "chat", PromptVariant: 3},
	}
	for _, req := range cases {
		t.Run(fmt.Sprintf("%s/v%d", req.DocumentType, req.PromptVariant), func(t *testing.T) {
			req.Text = "some draft"
			counter, result := pyramidizeWithCounter(t, replyV3, req)
			if counter.calls != 1 {
				t.Errorf("calls = %d, want 1 — Pyramidize is one call per run (ADR-003)", counter.calls)
			}
			if result.FullDocument != "doc" {
				t.Errorf("fullDocument = %q, want the model's document", result.FullDocument)
			}
		})
	}
}

// TestVolunteeredQualityFieldsBuyNoSecondCall replaces ADR-002's caveat
// (TestTheOneShotPropertyDependsOnPromptCompliance): schema enforcement is off,
// so nothing stops a model from volunteering qualityScore and qualityFlags.
// Under v2 that bought a refine call. Now nothing reads them, for any variant.
func TestVolunteeredQualityFieldsBuyNoSecondCall(t *testing.T) {
	for _, variant := range []int{0, 2, 3} {
		counter, result := pyramidizeWithCounter(t, replyWithSelfQA, PyramidizeRequest{
			Text: "some draft", DocumentType: "email", PromptVariant: variant,
		})
		if counter.calls != 1 {
			t.Errorf("variant %d: calls = %d, want 1 — a volunteered low score must not buy a refine", variant, counter.calls)
		}
		if result.FullDocument != "doc" {
			t.Errorf("variant %d: fullDocument = %q, want the first call's document", variant, result.FullDocument)
		}
		raw, _ := json.Marshal(result)
		for _, gone := range []string{"qualityScore", "qualityFlags", "appliedRefinement", "refinementWarning", "detectedType"} {
			if strings.Contains(string(raw), gone) {
				t.Errorf("variant %d: the result still carries %q: %s", variant, gone, raw)
			}
		}
	}
}

// TestAutoFollowsTheAppPresetElseEmail: "auto" is no longer a model call. It
// is the app preset for the captured source app, else email.
func TestAutoFollowsTheAppPresetElseEmail(t *testing.T) {
	presets := []settings.AppPreset{
		{SourceApp: "Slack", DocumentType: "chat"},
		{SourceApp: "Outlook", DocumentType: "memo"},
		// More specific than "Outlook": the longest match wins.
		{SourceApp: "Outlook Web", DocumentType: "wiki"},
		{SourceApp: "Confluence", DocumentType: "nonsense"},
	}
	tests := []struct {
		name      string
		sourceApp string
		want      string
	}{
		{"no source app", "", "EMAIL"},
		{"no preset for the app", "Notepad", "EMAIL"},
		{"preset matches part of the window title", "general | Acme - Slack", "CHAT"},
		{"case-insensitive", "Inbox - OUTLOOK", "MEMO"},
		{"the longest matching preset wins", "Inbox - outlook web - Firefox", "WIKI"},
		{"a preset naming no form falls back to email", "Space - Confluence", "EMAIL"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := settings.Default()
			cfg.AppPresets = presets
			svc, counter := newCountingService(cfg, replyV3)
			svc.sourceAppName = tc.sourceApp

			result, err := svc.Pyramidize(PyramidizeRequest{Text: "draft", DocumentType: "auto", Provider: "claude"})
			if err != nil {
				t.Fatalf("Pyramidize: %v", err)
			}
			if result.DocumentType != tc.want {
				t.Errorf("documentType = %q, want %q", result.DocumentType, tc.want)
			}
			if counter.calls != 1 {
				t.Errorf("calls = %d, want 1 — auto is a lookup, not a model call", counter.calls)
			}
			// The prompt that went out is the resolved form's.
			form := strings.ToLower(tc.want)
			if !strings.Contains(counter.requests[0].System, promptV3Forms[form]) {
				t.Errorf("the system prompt is not the %s form's", form)
			}
			if got := svc.GetDefaultDocType(); got != form {
				t.Errorf("GetDefaultDocType = %q, want %q — the UI preselects what auto would write", got, form)
			}
		})
	}
}

// TestAnExplicitFormBeatsThePreset: the user's pick is never overridden.
func TestAnExplicitFormBeatsThePreset(t *testing.T) {
	cfg := settings.Default()
	cfg.AppPresets = []settings.AppPreset{{SourceApp: "Slack", DocumentType: "chat"}}
	svc, _ := newCountingService(cfg, replyV3)
	svc.sourceAppName = "Slack"

	result, err := svc.Pyramidize(PyramidizeRequest{Text: "draft", DocumentType: "wiki", Provider: "claude"})
	if err != nil {
		t.Fatalf("Pyramidize: %v", err)
	}
	if result.DocumentType != "WIKI" {
		t.Errorf("documentType = %q, want WIKI", result.DocumentType)
	}
}

// TestChatIsAForm: chat is accepted and gets its own section.
func TestChatIsAForm(t *testing.T) {
	counter, result := pyramidizeWithCounter(t, replyV3, PyramidizeRequest{Text: "draft", DocumentType: "chat"})
	if result.DocumentType != "CHAT" {
		t.Errorf("documentType = %q, want CHAT", result.DocumentType)
	}
	if !strings.Contains(counter.requests[0].System, `<form name="chat">`) {
		t.Error("a chat run did not send the chat section")
	}
}

// TestPresentationIsPowerpoint: "presentation" names the same form as
// "powerpoint", the value presets and the CLI already store.
func TestPresentationIsPowerpoint(t *testing.T) {
	_, result := pyramidizeWithCounter(t, replyV3, PyramidizeRequest{Text: "draft", DocumentType: "Presentation"})
	if result.DocumentType != "POWERPOINT" {
		t.Errorf("documentType = %q, want POWERPOINT", result.DocumentType)
	}
}

// TestRefusedRequestsSpendNothing: an unknown form or a variant that does not
// exist for it is an error before any call — it used to fall back to the email
// prompt (and variant 1) without a word.
func TestRefusedRequestsSpendNothing(t *testing.T) {
	cases := []struct {
		name string
		req  PyramidizeRequest
		want string
	}{
		{"unknown form", PyramidizeRequest{DocumentType: "blog"}, "unknown document type"},
		{"v1 was removed", PyramidizeRequest{DocumentType: "email", PromptVariant: 1}, "removed"},
		{"v2 is email only", PyramidizeRequest{DocumentType: "memo", PromptVariant: 2}, "email only"},
		{"no such variant", PyramidizeRequest{DocumentType: "email", PromptVariant: 9}, "unknown prompt variant"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, counter := newCountingService(settings.Default(), replyV3)
			tc.req.Text, tc.req.Provider = "draft", "claude"
			_, err := svc.Pyramidize(tc.req)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want one mentioning %q", err, tc.want)
			}
			if counter.calls != 0 {
				t.Errorf("calls = %d, want 0", counter.calls)
			}
		})
	}
}

// TestAnEmptyDocumentIsAnError: a reply without a document must not reach the
// editor (or Send back) as an empty result.
func TestAnEmptyDocumentIsAnError(t *testing.T) {
	svc, _ := newCountingService(settings.Default(), `{"language":"de"}`)
	_, err := svc.Pyramidize(PyramidizeRequest{Text: "draft", DocumentType: "email", Provider: "claude"})
	if err == nil || !strings.Contains(err.Error(), "no document") {
		t.Errorf("err = %v, want a no-document error", err)
	}
}

// TestEachVariantSendsItsOwnSchema: the schema a run carries matches the prompt
// it sends — v3's two fields, v2's three. Swapping them at the call site would
// otherwise leave the suite green while enforcement is off.
func TestEachVariantSendsItsOwnSchema(t *testing.T) {
	original := schemaEnforcement
	t.Cleanup(func() { schemaEnforcement = original })
	schemaEnforcement = true

	for _, tc := range []struct {
		variant int
		want    json.RawMessage
	}{{0, documentSchemaV3}, {3, documentSchemaV3}, {2, documentSchemaV2}} {
		counter, _ := pyramidizeWithCounter(t, replyV3, PyramidizeRequest{Text: "d", DocumentType: "email", PromptVariant: tc.variant})
		if string(counter.requests[0].JSONSchema) != string(tc.want) {
			t.Errorf("variant %d sent schema %s, want %s", tc.variant, counter.requests[0].JSONSchema, tc.want)
		}
	}
}

// TestPresetMatchesTheAppNotTheDocument: a preset name matches whole words of
// at least three characters, and the app segment of a window title (Windows
// ends titles in " - App" / " – App") is tried before the document part. Each
// row is a false hit the first matcher produced, or what must still work.
func TestPresetMatchesTheAppNotTheDocument(t *testing.T) {
	tests := []struct {
		name    string
		presets []settings.AppPreset
		title   string
		want    string
	}{
		{"Mail is not part of Gmail", []settings.AppPreset{{SourceApp: "Mail", DocumentType: "chat"}},
			"Gmail – Google Chrome", "email"},
		{"Mail is not part of Email", []settings.AppPreset{{SourceApp: "Mail", DocumentType: "chat"}},
			"Email draft.docx – Word", "email"},
		{"the app segment beats a name in the document", []settings.AppPreset{
			{SourceApp: "Microsoft Teams", DocumentType: "chat"}, {SourceApp: "Outlook", DocumentType: "memo"}},
			"Microsoft Teams rollout - Message - Outlook", "memo"},
		{"a whole word in the app segment", []settings.AppPreset{{SourceApp: "Mail", DocumentType: "chat"}},
			"Inbox – Mail", "chat"},
		{"a browser tab still matches in the document part", []settings.AppPreset{{SourceApp: "Gmail", DocumentType: "chat"}},
			"Inbox (3) - someone@example.com - Gmail - Google Chrome", "chat"},
		{"names shorter than three characters are ignored", []settings.AppPreset{{SourceApp: "ab", DocumentType: "chat"}},
			"ab - Notepad", "email"},
		{"an em dash separates too", []settings.AppPreset{{SourceApp: "Word", DocumentType: "memo"}, {SourceApp: "Notes", DocumentType: "chat"}},
			"Notes for Monday — Word", "memo"},
		{"a title without a separator is all app", []settings.AppPreset{{SourceApp: "Slack", DocumentType: "chat"}},
			"Slack", "chat"},
		{"punctuation is a word boundary", []settings.AppPreset{{SourceApp: "Slack", DocumentType: "chat"}},
			"general | Acme - Slack", "chat"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := presetDocType(tc.presets, tc.title); got != tc.want {
				t.Errorf("presetDocType(%q) = %q, want %q", tc.title, got, tc.want)
			}
		})
	}
}

// TestErrorsDoNotCarryTheReply: a reply that cannot be used goes to the log
// (redacted), not into the message the UI shows. It may hold the user's text.
func TestErrorsDoNotCarryTheReply(t *testing.T) {
	const secret = "PRIVATE-DRAFT-TEXT"
	for name, reply := range map[string]string{
		"unparseable": "here is your text: " + secret,
		"no document": `{"language":"de","note":"` + secret + `"}`,
	} {
		t.Run(name, func(t *testing.T) {
			svc, _ := newCountingService(settings.Default(), reply)
			_, err := svc.Pyramidize(PyramidizeRequest{Text: secret, DocumentType: "email", Provider: "claude"})
			if err == nil {
				t.Fatal("want an error")
			}
			if strings.Contains(err.Error(), secret) {
				t.Errorf("the error shows the reply: %v", err)
			}
		})
	}
	svc, _ := newCountingService(settings.Default(), "not json "+secret)
	if _, err := svc.RefineGlobal(RefineGlobalRequest{FullCanvas: "c", OriginalText: secret, Instruction: "i", Provider: "claude"}); err == nil || strings.Contains(err.Error(), secret) {
		t.Errorf("RefineGlobal error = %v, want one without the reply", err)
	}
	if _, err := svc.Splice(SpliceRequest{FullCanvas: "c", OriginalText: secret, SelectedText: "c", Instruction: "i", Provider: "claude"}); err == nil || strings.Contains(err.Error(), secret) {
		t.Errorf("Splice error = %v, want one without the reply", err)
	}
}
