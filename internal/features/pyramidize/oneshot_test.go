package pyramidize

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"keylint/internal/features/settings"
	"keylint/internal/llm"
)

// ADR-002 rests on one property of the shipped default: the email path makes a
// single call. That is not a comment, it follows from v2's schema — no quality
// fields means the refine trigger can never see a flag — and nothing pinned it.
// If someone puts qualityScore back into documentSchemaV2, or points email at
// documentSchema, email silently becomes a two-call pipeline again and the
// baseline in quality-status.md stops describing what ships.

// TestTheShippedEmailPromptCannotTriggerRefine: refine fires on
// `QualityScore < threshold && len(QualityFlags) > 0`. v2 asks the model for
// neither field, so the flags are always empty and the condition is unreachable.
func TestTheShippedEmailPromptCannotTriggerRefine(t *testing.T) {
	_, _, schema := buildDocTypePrompt("email", LatestEmailVariant, "professional", "professional", "", "some text")

	var parsed struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	if err := json.Unmarshal(schema, &parsed); err != nil {
		t.Fatalf("email schema is not valid JSON: %v", err)
	}
	for _, field := range []string{"qualityScore", "qualityFlags"} {
		if _, present := parsed.Properties[field]; present {
			t.Errorf("the shipped email schema asks for %q — email is a two-call pipeline again, "+
				"and ADR-002's premise no longer holds", field)
		}
	}

}

// countingClient answers every call with a valid v2-shaped document and counts
// how many it was asked for.
type countingClient struct {
	calls int
	// reply is what every call answers with. The shape decides how many calls
	// the pipeline makes, which is the whole point of these tests.
	reply string
}

func (c *countingClient) Complete(_ context.Context, _ llm.Request) (llm.Response, error) {
	c.calls++
	return llm.Response{Text: c.reply}, nil
}

// A v2-shaped reply: the three fields the shipped email prompt asks for.
const replyWithoutSelfQA = `{"fullDocument":"doc","headers":["h"],"language":"de","type":"email","confidence":0.9}`

// The same document with the self-QA fields volunteered — a low score and a
// flag, which is what refine exists to act on.
const replyWithSelfQA = `{"fullDocument":"doc","headers":["h"],"language":"de","type":"email","confidence":0.9,` +
	`"qualityScore":0.1,"qualityFlags":["thin"]}`

// pyramidizeWithCounter runs the real Pyramidize entry point against a counting
// client, which is the only way to pin how many model calls a document costs.
func pyramidizeWithCounter(t *testing.T, reply string, req PyramidizeRequest) (*countingClient, PyramidizeResult) {
	t.Helper()
	counter := &countingClient{reply: reply}
	settingsSvc := settings.NewServiceFrom(settings.Default(), func(string) string { return "key" })
	svc := NewService(settingsSvc, nil)
	svc.newClient = func(string, llm.Config) (llm.Client, error) { return counter, nil }

	if req.Provider == "" {
		req.Provider = "claude"
	}
	result, err := svc.Pyramidize(req)
	if err != nil {
		t.Fatalf("Pyramidize: %v", err)
	}
	return counter, result
}

// TestAnExplicitlyTypedEmailCostsOneCall is the measurement ADR-002 rests on.
//
// Asserted through Pyramidize itself rather than by restating the refine
// condition in the test: a copy of that condition passes happily while the real
// one is edited. The counting client makes the claim falsifiable — drop the
// `len(QualityFlags) > 0` term from service.go, or add "email" to the detect
// branch, and this fails.
//
// It holds only while the model answers in the shape v2 asks for; see
// TestTheOneShotPropertyDependsOnPromptCompliance for the other half.
func TestAnExplicitlyTypedEmailCostsOneCall(t *testing.T) {
	counter, _ := pyramidizeWithCounter(t, replyWithoutSelfQA, PyramidizeRequest{
		Text: "some draft", DocumentType: "email", PromptVariant: LatestEmailVariant,
	})

	if counter.calls != 1 {
		t.Errorf("calls = %d, want 1 — an explicitly typed email is one model call; "+
			"see ADR-002, whose whole comparison assumes this", counter.calls)
	}
}

// TestTheOneShotPropertyDependsOnPromptCompliance is the limit of the claim.
//
// v2 is one-shot because its prompt does not ask for qualityScore or
// qualityFlags — not because anything stops them arriving. Schema enforcement is
// off by default (schemas.go), so the model sees no schema at all, and
// foundationResult has json tags for both fields: a model that volunteers them
// fills the refine gate and buys a second call.
//
// This is not hypothetical plumbing — it is the one way the shipped default
// silently becomes two calls, and the fix is either to enforce the schema or to
// ignore those fields when the prompt did not ask for them.
func TestTheOneShotPropertyDependsOnPromptCompliance(t *testing.T) {
	counter, result := pyramidizeWithCounter(t, replyWithSelfQA, PyramidizeRequest{
		Text: "some draft", DocumentType: "email", PromptVariant: LatestEmailVariant,
	})

	if counter.calls != 2 || !result.AppliedRefinement {
		t.Errorf("calls = %d refined = %v — expected the volunteered self-QA fields to reach "+
			"the refine gate. If this now passes at one call, the gate was hardened and "+
			"ADR-002's caveat can be retired.", counter.calls, result.AppliedRefinement)
	}
}

// TestTheAutoDocTypeCostsTwoCalls pins the shipped default, which is NOT
// one-shot: both the GUI dropdown and the CLI -type flag default to "auto", and
// the UI says "Step 1/2: Detecting…" while it happens.
//
// ADR-002 originally claimed the email path was one-shot outright. It is not —
// only the explicitly typed path is. This test exists so the distinction cannot
// quietly stop being true in either direction.
func TestTheAutoDocTypeCostsTwoCalls(t *testing.T) {
	counter, result := pyramidizeWithCounter(t, replyWithoutSelfQA, PyramidizeRequest{
		Text: "some draft", DocumentType: "auto", PromptVariant: LatestEmailVariant,
	})

	if counter.calls != 2 {
		t.Errorf("calls = %d, want 2 (detect + foundation) for the shipped AUTO default", counter.calls)
	}
	if result.DetectedType == "" {
		t.Error("AUTO did not report a detected type, so detect did not run")
	}
}

// TestV1CanStillReachRefine: the pipeline arm ADR-002 measured has to be able to
// make a second call, or the comparison was one-shot against one-shot.
func TestV1CanStillReachRefine(t *testing.T) {
	counter, result := pyramidizeWithCounter(t, replyWithSelfQA, PyramidizeRequest{
		Text: "some draft", DocumentType: "email", PromptVariant: 1,
	})

	if counter.calls != 2 {
		t.Errorf("calls = %d, want 2 — v1 asks for qualityScore/qualityFlags, and a low "+
			"score with a flag is what refine is for", counter.calls)
	}
	if !result.AppliedRefinement {
		t.Error("refine did not run, so v1 is not the pipeline arm ADR-002 measured")
	}
}

// TestTheShippedEmailPromptAsksForNoSelfQA: the prompt text has to agree with
// the schema, or the model is told to produce fields the schema forbids.
func TestTheShippedEmailPromptAsksForNoSelfQA(t *testing.T) {
	system, _, _ := buildDocTypePrompt("email", LatestEmailVariant, "professional", "professional", "", "some text")

	for _, marker := range []string{"qualityScore", "qualityFlags"} {
		if strings.Contains(system, marker) {
			t.Errorf("the shipped email prompt mentions %q while its schema forbids it", marker)
		}
	}
}

// TestTheOtherDocTypesAreStillPipelineShaped documents the scope ADR-002 leaves
// open, and fails when that stops being true — which is the moment the refine
// stage and the quality-threshold setting become removable.
func TestTheOtherDocTypesAreStillPipelineShaped(t *testing.T) {
	for _, docType := range []string{"memo", "wiki", "powerpoint"} {
		_, _, schema := buildDocTypePrompt(docType, 0, "professional", "professional", "", "some text")

		var parsed struct {
			Properties map[string]json.RawMessage `json:"properties"`
		}
		if err := json.Unmarshal(schema, &parsed); err != nil {
			t.Fatalf("%s schema is not valid JSON: %v", docType, err)
		}
		if _, present := parsed.Properties["qualityFlags"]; !present {
			t.Errorf("%s no longer asks for qualityFlags — if every doc type is one-shot now, "+
				"the refine stage and PyramidizeQualityThreshold have no caller left; see ADR-002", docType)
		}
	}
}

// TestV1IsStillThePipelineVariant: ADR-002's comparison used --variant 1 as the
// pipeline arm. If v1 stops emitting the quality fields, that measurement no
// longer refers to anything in the code.
func TestV1IsStillThePipelineVariant(t *testing.T) {
	_, _, schema := buildDocTypePrompt("email", 1, "professional", "professional", "", "some text")

	var parsed struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(schema, &parsed); err != nil {
		t.Fatalf("v1 schema is not valid JSON: %v", err)
	}
	for _, field := range []string{"qualityScore", "qualityFlags"} {
		if _, present := parsed.Properties[field]; !present {
			t.Errorf("v1 no longer asks for %q, so it is not the pipeline arm ADR-002 measured", field)
		}
	}
}
