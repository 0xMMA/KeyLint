package pyramidize

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

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
			if got := svc.GetSourcePreset(tc.sourceApp).DocumentType; got != form {
				t.Errorf("GetSourcePreset().DocumentType = %q, want %q — the UI preselects what auto would write", got, form)
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

// TestInstructionsReachTheModel: the choice page's one field goes out with the
// run — in v3's instructions block, and in v2's style block next to the
// professional/professional it was measured with. Nothing else of the user's
// is a choice any more.
func TestInstructionsReachTheModel(t *testing.T) {
	counter, _ := pyramidizeWithCounter(t, replyV3, PyramidizeRequest{
		Text: "draft", DocumentType: "memo", CustomInstructions: "in English, please",
	})
	if !strings.Contains(counter.requests[0].System, "<user_instructions>\nin English, please\n</user_instructions>") {
		t.Error("v3: the instructions did not reach the system prompt")
	}
	if counter.requests[0].User != "draft" {
		t.Errorf("v3: user message = %q, want the text alone", counter.requests[0].User)
	}

	counter, _ = pyramidizeWithCounter(t, replyV3, PyramidizeRequest{
		Text: "draft", DocumentType: "email", PromptVariant: 2, CustomInstructions: "in English, please",
	})
	want := "Communication style: professional\nRelationship level: professional\nAdditional instructions: in English, please"
	if !strings.Contains(counter.requests[0].System, want) {
		t.Errorf("v2: the style block lacks %q", want)
	}
}

// TestSourcePresetNamesWhatTheChoicePageShows: the choice page preselects the
// form, tags it "Remembered for <app>", offers "Remember <form> for <app>" and
// Forget. So it needs the form, whether a preset matched, and the name a new
// preset is stored under — the matched preset's own name (so remembering
// another form updates it, and Forget deletes it), else the app the title
// names. App is "" where nothing safe can be remembered: no separator (the
// whole title might be a document or a person), a name the matcher could never
// find again (under three characters), or one too long to be an app's.
// Titles are what Windows shows; the roadmap's Windows check stays open.
func TestSourcePresetNamesWhatTheChoicePageShows(t *testing.T) {
	presets := []settings.AppPreset{
		{SourceApp: "Slack", DocumentType: "chat"},
		{SourceApp: " Confluence ", DocumentType: "wiki"}, // hand-edited, padded
	}
	none := func(app string) SourcePreset { return SourcePreset{App: app, DocumentType: "email"} }
	tests := []struct {
		name  string
		title string
		want  SourcePreset
	}{
		{"nothing captured", "", none("")},
		{"classic Outlook", "Inbox - max.mustermann@firma.de - Outlook", none("Outlook")},
		{"new Outlook", "Mail - Max Mustermann - Outlook", none("Outlook")},
		{"Word, en dash", "Bericht_Q3_final.docx – Word", none("Word")},
		{"new Teams separates with a bar", "Chat | Max Mustermann | Microsoft Teams", none("Microsoft Teams")},
		{"classic Teams", "Max Mustermann | Microsoft Teams", none("Microsoft Teams")},
		{"a web app in Chrome", "Inbox (3) - max@gmail.com - Gmail - Google Chrome", none("Gmail")},
		{"a web app in Firefox, em dash", "Posteingang - Outlook — Mozilla Firefox", none("Outlook")},
		{"a web app in Edge (zero-width space in its name)", "Inbox - Gmail - Microsoft\u200b Edge", none("Gmail")},
		{"no separator: nothing to remember", "Max Mustermann", none("")},
		{"a separator with nothing after it", "Draft - ", none("")},
		{"under three characters", "Notes - ab", none("")},
		{"too long to be an app", "Short - Quarterly planning notes for the whole department team", none("")},
		{"a preset: its own name", "general (Channel) - Acme - SLACK", SourcePreset{App: "Slack", DocumentType: "chat", Remembered: true}},
		{"a padded preset: its trimmed name", "Team space - Confluence", SourcePreset{App: "Confluence", DocumentType: "wiki", Remembered: true}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := settings.Default()
			cfg.AppPresets = presets
			svc, _ := newCountingService(cfg, replyV3)
			if got := svc.GetSourcePreset(tc.title); got != tc.want {
				t.Errorf("GetSourcePreset(%q) = %+v, want %+v", tc.title, got, tc.want)
			}
		})
	}
}

// TestRememberingUnderTheSourceNameMatchesNextTime: what the choice page
// stores under GetSourcePreset(title).App is found again for the next capture
// from the same app, with another document in its title — the matcher and
// what is stored agree.
func TestRememberingUnderTheSourceNameMatchesNextTime(t *testing.T) {
	for _, tc := range []struct{ first, next, app string }{
		{"Inbox - max@firma.de - Outlook", "Re: Budget Q3 - Message (HTML) - Outlook", "Outlook"},
		{"Chat | Max Mustermann | Microsoft Teams", "Kalender | Microsoft Teams", "Microsoft Teams"},
		{"Inbox (3) - max@gmail.com - Gmail - Google Chrome", "Re: Angebot - max@gmail.com - Gmail - Google Chrome", "Gmail"},
		{"general (Channel) - Acme - Slack", "Max Mustermann (DM) - Acme - Slack", "Slack"},
		{"Bericht.docx - Word", "Angebot_v2.docx - Word", "Word"},
	} {
		t.Run(tc.app, func(t *testing.T) {
			svc, _ := newCountingService(settings.Default(), replyV3)
			app := svc.GetSourcePreset(tc.first).App
			if app != tc.app {
				t.Fatalf("first capture names %q, want %q", app, tc.app)
			}
			if err := svc.SetAppPreset(AppPreset{SourceApp: app, DocumentType: "chat"}); err != nil {
				t.Fatalf("SetAppPreset: %v", err)
			}
			if got := svc.GetSourcePreset(tc.next); got != (SourcePreset{App: tc.app, DocumentType: "chat", Remembered: true}) {
				t.Errorf("next capture %q: GetSourcePreset = %+v, want chat remembered for %s", tc.next, got, tc.app)
			}
		})
	}
}

// TestAWebAppPresetStaysInItsTab: in a browser the other tabs' page titles are
// not the app. A Gmail preset must not catch a page that only mentions Gmail.
func TestAWebAppPresetStaysInItsTab(t *testing.T) {
	presets := []settings.AppPreset{{SourceApp: "Gmail", DocumentType: "chat"}}
	if got := presetDocType(presets, "How to use Gmail filters - Stack Overflow - Google Chrome"); got != "email" {
		t.Errorf("a page about Gmail got the Gmail preset (%s)", got)
	}
	if got := presetDocType(presets, "Inbox - Gmail - Google Chrome"); got != "chat" {
		t.Errorf("Gmail itself lost its preset (%s)", got)
	}
}

// TestPresetNamesCompareTrimmedAndCaseFolded: names are one preset whatever
// their padding or case, on save, match and delete — a padded name used to be
// added twice and could not be forgotten.
func TestPresetNamesCompareTrimmedAndCaseFolded(t *testing.T) {
	cfg := settings.Default()
	cfg.AppPresets = []settings.AppPreset{{SourceApp: " Outlook ", DocumentType: "memo"}}
	svc, _ := newCountingService(cfg, replyV3)

	if err := svc.SetAppPreset(AppPreset{SourceApp: "outlook  ", DocumentType: "chat"}); err != nil {
		t.Fatalf("SetAppPreset: %v", err)
	}
	got := svc.GetAppPresets()
	if len(got) != 1 || got[0] != (AppPreset{SourceApp: "outlook", DocumentType: "chat"}) {
		t.Fatalf("presets = %+v, want one, trimmed and updated", got)
	}
	if err := svc.SetAppPreset(AppPreset{SourceApp: "   ", DocumentType: "chat"}); err == nil {
		t.Error("a blank name was stored")
	}
	if err := svc.DeleteAppPreset("  OUTLOOK "); err != nil {
		t.Fatalf("DeleteAppPreset: %v", err)
	}
	if got := svc.GetAppPresets(); len(got) != 0 {
		t.Errorf("presets after delete = %+v, want none", got)
	}
}

// blockingClient answers only once its context ends, and says so on started.
type blockingClient struct{ started chan struct{} }

func (b *blockingClient) Complete(ctx context.Context, _ llm.Request) (llm.Response, error) {
	b.started <- struct{}{}
	<-ctx.Done()
	return llm.Response{}, ctx.Err()
}

// TestANewCallCancelsTheOneInFlight: Pyramidize, RefineGlobal and Splice
// share one cancel. A call that starts while another is in flight used to
// orphan it — still running, still billed, and no Cancel could reach it.
func TestANewCallCancelsTheOneInFlight(t *testing.T) {
	calls := map[string]func(*Service) error{
		"Pyramidize": func(s *Service) error {
			_, err := s.Pyramidize(PyramidizeRequest{Text: "t", DocumentType: "email", Provider: "claude"})
			return err
		},
		"RefineGlobal": func(s *Service) error {
			_, err := s.RefineGlobal(RefineGlobalRequest{FullCanvas: "c", Instruction: "i", Provider: "claude"})
			return err
		},
		"Splice": func(s *Service) error {
			_, err := s.Splice(SpliceRequest{FullCanvas: "c", SelectedText: "c", Instruction: "i", Provider: "claude"})
			return err
		},
	}
	for first, call := range calls {
		t.Run(first, func(t *testing.T) {
			client := &blockingClient{started: make(chan struct{}, 2)}
			svc := NewService(settings.NewServiceFrom(settings.Default(), func(string) string { return "key" }), nil)
			svc.newClient = func(string, llm.Config) (llm.Client, error) { return client, nil }

			done := make(chan error, 1)
			go func() { done <- call(svc) }()
			<-client.started
			go func() { _ = calls["Pyramidize"](svc) }()
			<-client.started

			select {
			case err := <-done:
				if err == nil || err.Error() != "cancelled" {
					t.Errorf("the first call ended with %v, want \"cancelled\"", err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("the first call is still running after a second one started")
			}
			svc.CancelOperation()
		})
	}
}

// TestRefineCarriesNoStyleOrRelationship: "Change the whole text" kept the
// style and relationship of a page that no longer has them; it keeps the
// form and the canvas's own voice instead.
func TestRefineCarriesNoStyleOrRelationship(t *testing.T) {
	svc, counter := newCountingService(settings.Default(), `{"newCanvas":"c2"}`)
	if _, err := svc.RefineGlobal(RefineGlobalRequest{FullCanvas: "c", OriginalText: "o", Instruction: "shorter", DocumentType: "chat", Provider: "claude"}); err != nil {
		t.Fatalf("RefineGlobal: %v", err)
	}
	system := strings.ToLower(counter.requests[0].System)
	for _, gone := range []string{"communication style", "relationship level"} {
		if strings.Contains(system, gone) {
			t.Errorf("the refine prompt still names %q", gone)
		}
	}
	if !strings.Contains(system, "document type (chat)") {
		t.Error("the refine prompt no longer keeps the document type")
	}
}

// TestV2TrimsTheInstructions: v2 gets the instructions trimmed, as v3 does;
// blank ones add no "Additional instructions" line.
func TestV2TrimsTheInstructions(t *testing.T) {
	system, _, _, err := buildDocTypePrompt("email", 2, "   ", "draft")
	if err != nil {
		t.Fatalf("buildDocTypePrompt: %v", err)
	}
	if strings.Contains(system, "Additional instructions") {
		t.Error("blank instructions added an Additional instructions line to v2")
	}
	system, _, _, _ = buildDocTypePrompt("email", 2, "  in English \n", "draft")
	if !strings.Contains(system, "Additional instructions: in English\n</style_injection>") {
		t.Error("v2 did not get the instructions trimmed")
	}
}
