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

func TestListModelsOllamaReadsTheDaemonsTags(t *testing.T) {
	var path string
	srv := newRawServer(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
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
}

// TestListModelsFallsBackToTheCuratedList covers the case a user actually hits:
// the daemon is not running, or the key is not set yet. An empty picker is
// useless; the built-in list with a source of "static" is not.
func TestListModelsFallsBackToTheCuratedList(t *testing.T) {
	srv := newRawServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	list, err := ListModels(context.Background(), ProviderOllama, Config{BaseURL: srv})
	if err == nil {
		t.Error("expected the failure to be reported alongside the fallback")
	}
	if list.Source != ModelSourceStatic {
		t.Errorf("source = %q, want %q", list.Source, ModelSourceStatic)
	}
	if len(list.Models) == 0 {
		t.Error("the fallback list is empty, which is worse than no picker at all")
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
func TestIsChatModelKeepsOnlyWhatCanAnswerACompletion(t *testing.T) {
	keep := []string{"gpt-5.2", "gpt-4o-mini", "gpt-4.1", "o3", "o4-mini", "chatgpt-4o-latest", "codex-mini-latest"}
	drop := []string{
		"text-embedding-3-large", "whisper-1", "dall-e-3", "tts-1",
		"gpt-image-1", "gpt-4o-transcribe", "gpt-4o-mini-tts",
		"gpt-4o-audio-preview", "gpt-4o-realtime-preview",
		"gpt-4o-mini-search-preview", "gpt-3.5-turbo-instruct", "omni-moderation-latest",
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
