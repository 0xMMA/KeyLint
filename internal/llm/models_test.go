package llm

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestDefaultModelPerFeature(t *testing.T) {
	// The fix flow wants something fast; Pyramidize restructures a document and
	// wants something stronger. They must not collapse into one value.
	//
	// Ollama is absent on purpose and covered by its own test below — see
	// TestOllamaDeliberatelyUsesOneModelForBothFeatures.
	for _, provider := range []string{ProviderClaude, ProviderOpenAI, ProviderClaudeCode} {
		fix := DefaultModel(provider, FeatureFix)
		pyramidize := DefaultModel(provider, FeaturePyramidize)
		if fix == "" || pyramidize == "" {
			t.Errorf("%s has an empty default: fix=%q pyramidize=%q", provider, fix, pyramidize)
		}
		if fix == pyramidize {
			t.Errorf("%s uses %q for both features", provider, fix)
		}
	}
	if DefaultModel("nonexistent", FeatureFix) != "" {
		t.Error("an unknown provider must have no default rather than a wrong one")
	}
}

// TestOllamaDeliberatelyUsesOneModelForBothFeatures states the one exception to
// the rule above, rather than leaving Ollama quietly out of the loop.
//
// Every other provider splits the two features, and for Ollama that split would
// have to name a second local model. Which one is a quality question, and there
// is no Ollama eval to answer it — the eval suite runs against Anthropic and
// OpenAI. Picking a heavier default on a guess would slow down every fix on a
// machine that may not even have that model pulled. So both features stay on
// llama3.2 until an eval says otherwise, and this test holds that decision
// still: changing it should take an argument, not a typo.
func TestOllamaDeliberatelyUsesOneModelForBothFeatures(t *testing.T) {
	fix := DefaultModel(ProviderOllama, FeatureFix)
	pyramidize := DefaultModel(ProviderOllama, FeaturePyramidize)

	if fix == "" {
		t.Fatal("Ollama has no default at all")
	}
	if fix != pyramidize {
		t.Errorf("fix=%q pyramidize=%q — if this split is now wanted, it needs an eval behind it", fix, pyramidize)
	}
}

func TestListModelsOllamaReadsTheDaemonsTags(t *testing.T) {
	var path, ua string
	srv := newRawServer(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		ua = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[
			{"name":"llama3.2:latest","model":"llama3.2:latest"},
			{"name":"mistral:7b","model":"mistral:7b"}
		]}`))
	})

	list, err := ListModels(context.Background(), ProviderOllama, Config{BaseURL: srv})
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	// The native endpoint, not the OpenAI-compatible one: it reports what is
	// actually pulled locally.
	if path != "/api/tags" {
		t.Errorf("path = %q, want /api/tags", path)
	}
	if list.Source != ModelSourceLive {
		t.Errorf("source = %q, want %q", list.Source, ModelSourceLive)
	}
	if len(list.Models) != 2 || list.Models[0].ID != "llama3.2:latest" {
		t.Errorf("models = %v, want the daemon's tags", list.Models)
	}
	// This request is hand-rolled rather than made through an SDK, so it has to
	// set the header itself — otherwise net/http sends "Go-http-client/1.1".
	if ua != userAgent {
		t.Errorf("User-Agent = %q, want %q", ua, userAgent)
	}
}

// TestTransportErrorsDropCredentialsFromTheURL: a self-hosted Ollama or an
// OpenAI-compatible gateway is reached through a URL the user typed, and Go
// puts that whole URL into the error. These errors are logged and shown, so a
// password in the URL must not travel with them.
func TestTransportErrorsDropCredentialsFromTheURL(t *testing.T) {
	// Nothing listens on port 1, so the request fails inside net/http and the
	// error carries the URL it was given.
	_, err := ListModels(context.Background(), ProviderOllama,
		Config{BaseURL: "http://user:hunter2@127.0.0.1:1"})
	if err == nil {
		t.Fatal("want an error from an unreachable host")
	}
	if strings.Contains(err.Error(), "hunter2") {
		t.Errorf("the error repeats the credential back: %v", err)
	}
	if !strings.Contains(err.Error(), "127.0.0.1:1") {
		t.Errorf("the host is gone too, which leaves nothing to debug: %v", err)
	}
}

// TestListModelsFallsBackWhenTheProviderCannotBeReached: the daemon is not
// running, or the URL is wrong. An empty picker is useless; the built-in list
// with a source that names the problem is not.
func TestListModelsFallsBackWhenTheProviderCannotBeReached(t *testing.T) {
	srv := newRawServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	list, err := ListModels(context.Background(), ProviderOllama, Config{BaseURL: srv})
	if err == nil {
		t.Error("expected the failure to be reported alongside the fallback")
	}
	if list.Source != ModelSourceUnreachable {
		t.Errorf("source = %q, want %q", list.Source, ModelSourceUnreachable)
	}
	if len(list.Models) == 0 {
		t.Error("the fallback list is empty, which is worse than no picker at all")
	}
}

// TestListModelsSeparatesAnEmptyAnswerFromAFailure is the fresh-`ollama serve`
// case: the daemon is up and has nothing pulled.
//
// Folding this into the unreachable case got two things wrong at once — it told
// the user a running daemon could not be reached, and it offered five models
// that machine cannot serve, each of which 404s at call time.
func TestListModelsSeparatesAnEmptyAnswerFromAFailure(t *testing.T) {
	srv := newRawServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[]}`))
	})

	list, err := ListModels(context.Background(), ProviderOllama, Config{BaseURL: srv})
	if err != nil {
		t.Errorf("err = %v, want none: the daemon answered", err)
	}
	if list.Source != ModelSourceEmpty {
		t.Errorf("source = %q, want %q", list.Source, ModelSourceEmpty)
	}
	if len(list.Models) != 0 {
		t.Errorf("models = %v, want none — the built-in list would be models this machine cannot serve", list.Models)
	}
}

// TestAProviderThatListsOnlyUncallableModelsIsNotCalledEmpty: an OpenAI project
// key scoped to Responses-API-only models lists plenty — this app can call none
// of it. Deciding "empty" on what survived the filter told that user their
// account lists no models, which is false, and left the picker with nothing in
// it. The Pyramidize panel's model select is not editable, so there was no way
// out of that screen.
func TestAProviderThatListsOnlyUncallableModelsIsNotCalledEmpty(t *testing.T) {
	srv := newRawServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[
			{"id":"o3-pro"},{"id":"gpt-5.2-pro"},{"id":"o3-deep-research"}
		]}`))
	})

	list, err := ListModels(context.Background(), ProviderOpenAI, Config{APIKey: "k", BaseURL: srv})
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if list.Source != ModelSourceUnusable {
		t.Errorf("source = %q, want %q — the account listed three models", list.Source, ModelSourceUnusable)
	}
	if len(list.Models) == 0 {
		t.Error("no fallback offered, so the picker is a dead end")
	}
}

// TestAnEmptyPayloadIsStillEmpty: the check above must not swallow the case it
// was split from — a provider that really listed nothing.
func TestAnEmptyPayloadIsStillEmpty(t *testing.T) {
	srv := newRawServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	})

	list, err := ListModels(context.Background(), ProviderOpenAI, Config{APIKey: "k", BaseURL: srv})
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if list.Source != ModelSourceEmpty {
		t.Errorf("source = %q, want %q", list.Source, ModelSourceEmpty)
	}
}

// TestEveryDefaultModelSurvivesTheFilter: a default that the filter drops is a
// model KeyLint uses but no user can see or re-pick in the list.
func TestEveryDefaultModelSurvivesTheFilter(t *testing.T) {
	for _, feature := range []string{FeatureFix, FeaturePyramidize} {
		id := DefaultModel(ProviderOpenAI, feature)
		if id == "" {
			t.Fatalf("no OpenAI default for %s", feature)
		}
		if !isChatModel(id) {
			t.Errorf("the %s default %q is filtered out of live listings", feature, id)
		}
	}
}

// TestCuratedModelsCannotBeMutatedByACaller: the list is package state handed
// out on every fallback, so a caller sorting or appending to it would change
// what every later fallback shows.
func TestCuratedModelsCannotBeMutatedByACaller(t *testing.T) {
	first := CuratedModels(ProviderOpenAI, ModelSourceUnreachable)
	if len(first.Models) == 0 {
		t.Fatal("no curated OpenAI models to test with")
	}
	first.Models[0] = ModelInfo{ID: "clobbered", Label: "clobbered"}

	if second := CuratedModels(ProviderOpenAI, ModelSourceUnreachable); second.Models[0].ID == "clobbered" {
		t.Error("the curated list is handed out by reference")
	}
}

// TestClaudeCodeAliasesAreCopiedToo: same reason, different list.
func TestClaudeCodeAliasesAreCopiedToo(t *testing.T) {
	first, _ := ListModels(context.Background(), ProviderClaudeCode, Config{})
	first.Models[0] = ModelInfo{ID: "clobbered"}

	second, _ := ListModels(context.Background(), ProviderClaudeCode, Config{})
	if second.Models[0].ID == "clobbered" {
		t.Error("the alias list is handed out by reference")
	}
}

// TestClaudeCodeAliasesMatchThePicker: the message naming the aliases is built
// from the list the picker offers, so the two cannot drift apart.
func TestClaudeCodeAliasesMatchThePicker(t *testing.T) {
	names := ClaudeCodeAliases()
	if len(names) != len(claudeCodeAliases) {
		t.Fatalf("ClaudeCodeAliases() = %v, want one entry per alias", names)
	}
	for i, name := range names {
		if name != claudeCodeAliases[i].ID {
			t.Errorf("alias %d = %q, want %q", i, name, claudeCodeAliases[i].ID)
		}
		if !IsClaudeCodeAlias(name) {
			t.Errorf("%q is named in messages but not accepted as an alias", name)
		}
	}
}

// TestListModelsClaudeCodeOffersOnlyAliases: the CLI resolves these to the
// current generation, which a pinned ID would freeze.
func TestListModelsClaudeCodeOffersOnlyAliases(t *testing.T) {
	list, err := ListModels(context.Background(), ProviderClaudeCode, Config{})
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	var ids []string
	for _, m := range list.Models {
		ids = append(ids, m.ID)
		if !IsClaudeCodeAlias(m.ID) {
			t.Errorf("%q is offered but is not an alias", m.ID)
		}
	}
	if strings.Join(ids, ",") != "opus,sonnet,haiku" {
		t.Errorf("aliases = %v, want opus, sonnet, haiku", ids)
	}
	if IsClaudeCodeAlias("claude-sonnet-4-6") {
		t.Error("a pinned API model ID must not count as an alias")
	}
}

// TestNotFoundNamesTheModel: a configured model that no longer exists upstream
// is the most likely 404 here, and an error that does not name it leaves the
// user hunting.
func TestNotFoundNamesTheModel(t *testing.T) {
	var got capture
	srv := newServer(t, &got, http.StatusNotFound, `{"error":{"message":"no such model"}}`)

	client := newOpenAI(Config{APIKey: "sk-test", BaseURL: srv.URL})
	_, err := client.Complete(context.Background(), Request{Model: "gpt-does-not-exist", User: "x"})
	if err == nil {
		t.Fatal("expected an error for status 404")
	}
	if !strings.Contains(err.Error(), "gpt-does-not-exist") {
		t.Errorf("error = %v, want it to name the model", err)
	}
	if !strings.Contains(err.Error(), "Settings") {
		t.Errorf("error = %v, want it to say where to change the model", err)
	}
}

// TestIsChatModelKeepsOnlyWhatCanAnswerACompletion: an OpenAI account lists
// embeddings, speech, image and realtime models next to the chat ones, and
// offering one that 400s at call time is worse than omitting it.
//
// The IDs below come from OpenAI's documentation, not from a listing this
// project has seen — there is no OPENAI_API_KEY here to check against.
func TestIsChatModelKeepsOnlyWhatCanAnswerACompletion(t *testing.T) {
	keep := []string{"gpt-5.2", "gpt-4o-mini", "gpt-4.1", "o3", "o4-mini", "chatgpt-4o-latest"}
	drop := []string{
		"text-embedding-3-large", "whisper-1", "dall-e-3", "tts-1",
		"gpt-image-1", "gpt-4o-transcribe", "gpt-4o-mini-tts",
		"gpt-4o-audio-preview", "gpt-4o-realtime-preview",
		"gpt-4o-mini-search-preview", "gpt-3.5-turbo-instruct", "omni-moderation-latest",
		// Reachable through the Responses API, not through the
		// /chat/completions call this app makes — so they 400, which is the
		// same outcome as an embedding model and deserves the same filter.
		"codex-mini-latest", "gpt-5.2-pro", "o3-deep-research",
		// The Codex family moved into the gpt-* namespace, where a "codex-"
		// prefix test cannot see it any more.
		"gpt-5-codex", "gpt-5.1-codex", "gpt-5.1-codex-mini", "gpt-5.1-codex-max",
	}
	for _, id := range keep {
		if !isChatModel(id) {
			t.Errorf("%q was dropped but can answer a completion", id)
		}
	}
	for _, id := range drop {
		if isChatModel(id) {
			t.Errorf("%q was kept but cannot answer a completion", id)
		}
	}
}

// TestListModelsOpenAIFiltersAndKeepsProviderOrder pins both halves: the
// account's non-chat models do not reach the picker, and the order the provider
// returned is preserved rather than re-sorted.
// TestCuratedOpenAIModelsSurviveTheFilter: the curated list is what a user sees
// when the account cannot be reached, so shipping an ID there that the filter
// would drop as uncallable would be offering a model that 400s.
func TestCuratedOpenAIModelsSurviveTheFilter(t *testing.T) {
	for _, model := range curatedModels[ProviderOpenAI] {
		if !isChatModel(model.ID) {
			t.Errorf("curated %q is filtered out of live listings as uncallable", model.ID)
		}
	}
}

func TestListModelsOpenAIFiltersAndKeepsProviderOrder(t *testing.T) {
	srv := newRawServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[
			{"id":"gpt-5.2","object":"model","created":3,"owned_by":"openai"},
			{"id":"text-embedding-3-large","object":"model","created":2,"owned_by":"openai"},
			{"id":"gpt-4o-mini","object":"model","created":1,"owned_by":"openai"}
		]}`))
	})

	list, err := ListModels(context.Background(), ProviderOpenAI, Config{APIKey: "sk-test", BaseURL: srv})
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if list.Source != ModelSourceLive {
		t.Errorf("source = %q, want %q", list.Source, ModelSourceLive)
	}
	var ids []string
	for _, m := range list.Models {
		ids = append(ids, m.ID)
	}
	if strings.Join(ids, ",") != "gpt-5.2,gpt-4o-mini" {
		t.Errorf("models = %v, want the chat models in the provider's order", ids)
	}
}

// TestListModelsAnthropicAsksForEveryPage: the SDK defaults to 20 per page and
// this code does not follow a cursor, so a workspace with more models would
// silently lose the oldest entries — which are the pinned IDs people configure.
func TestListModelsAnthropicAsksForEveryPage(t *testing.T) {
	var limit string
	srv := newRawServer(t, func(w http.ResponseWriter, r *http.Request) {
		limit = r.URL.Query().Get("limit")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[
			{"id":"claude-sonnet-4-6","display_name":"Sonnet 4.6","type":"model","created_at":"2026-01-01T00:00:00Z"}
		],"has_more":false}`))
	})

	list, err := ListModels(context.Background(), ProviderClaude, Config{APIKey: "sk-ant-test", BaseURL: srv})
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if limit == "" || limit == "20" {
		t.Errorf("limit = %q, want an explicit high limit rather than the SDK default", limit)
	}
	if len(list.Models) != 1 || list.Models[0].Label != "Sonnet 4.6" {
		t.Errorf("models = %v, want the display name as the label", list.Models)
	}
}
