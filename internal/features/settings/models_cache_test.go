package settings

import (
	"path/filepath"
	"testing"
	"time"

	"keylint/internal/llm"
)

// The settings screen and the Pyramidize selector both ask for model lists, and
// a listing is a round trip to the provider — or a spawn against a daemon that
// may be down. These cover the cache around that.

// TestListModelsServesAnUnexpiredEntryWithoutAsking: a fresh entry must not cost
// a round trip, even for a provider that would fail if asked.
func TestListModelsServesAnUnexpiredEntryWithoutAsking(t *testing.T) {
	svc := serviceWithUnreachableOllama(t, cachedModelList{
		list:    llm.ModelList{Models: []llm.ModelInfo{{ID: "cached-model"}}, Source: llm.ModelSourceLive},
		fetched: time.Now(),
	})

	got := svc.ListModels(llm.ProviderOllama)
	if len(got.Models) != 1 || got.Models[0].ID != "cached-model" {
		t.Errorf("models = %v, want the cached entry", got.Models)
	}
}

// TestListModelsRefetchesAfterTheTTL: a user who starts Ollama, or pastes a key,
// must not have to restart the app to see their models.
//
// Ollama is used rather than claude-code because claude-code answers from its
// alias list before any cache or fetch logic runs, so it would prove nothing.
func TestListModelsRefetchesAfterTheTTL(t *testing.T) {
	svc := serviceWithUnreachableOllama(t, cachedModelList{
		list:    llm.ModelList{Models: []llm.ModelInfo{{ID: "stale"}}, Source: llm.ModelSourceLive},
		fetched: time.Now().Add(-modelListTTL - time.Minute),
	})

	// Nothing is listening, so the refetch falls back to the built-in list —
	// which is the point: the stale entry must be gone either way.
	got := svc.ListModels(llm.ProviderOllama)
	for _, model := range got.Models {
		if model.ID == "stale" {
			t.Fatal("an expired entry was served instead of being refreshed")
		}
	}
	if got.Source != llm.ModelSourceStatic {
		t.Errorf("source = %q, want %q after a failed refetch", got.Source, llm.ModelSourceStatic)
	}
}

// TestForgettingAModelListCoversTheFlowThisFeatureExistsFor: open Settings with
// no key, get the built-in list, paste a key — the picker must not keep showing
// the built-in list for the rest of the TTL.
//
// SetKey and DeleteKey call forgetModelList; they are not called here because
// they need an OS keyring, which a machine without a secret-service daemon
// leaves hanging rather than failing.
func TestForgettingAModelListCoversTheFlowThisFeatureExistsFor(t *testing.T) {
	svc := &Service{
		models: map[string]cachedModelList{
			llm.ProviderClaude: {list: llm.CuratedModels(llm.ProviderClaude), fetched: time.Now(), failed: true},
			llm.ProviderOpenAI: {list: llm.CuratedModels(llm.ProviderOpenAI), fetched: time.Now()},
		},
	}

	svc.forgetModelList(llm.ProviderClaude)

	if _, cached := svc.models[llm.ProviderClaude]; cached {
		t.Error("the model list survived, so a new key would not be picked up")
	}
	if _, cached := svc.models[llm.ProviderOpenAI]; !cached {
		t.Error("an unrelated provider's list was dropped")
	}
}

// TestSaveForgetsTheOllamaListWhenTheURLChanges: the daemon at the new address
// has different models pulled, so keeping the old list would not be stale — it
// would be wrong.
func TestSaveForgetsTheOllamaListWhenTheURLChanges(t *testing.T) {
	svc := &Service{
		filePath: filepath.Join(t.TempDir(), "settings.json"),
		current:  Default(),
		models: map[string]cachedModelList{
			llm.ProviderOllama: {list: llm.CuratedModels(llm.ProviderOllama), fetched: time.Now()},
			llm.ProviderClaude: {list: llm.CuratedModels(llm.ProviderClaude), fetched: time.Now()},
		},
	}

	updated := Default()
	updated.Providers.OllamaURL = "http://elsewhere:11434"
	if err := svc.Save(updated); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if _, cached := svc.models[llm.ProviderOllama]; cached {
		t.Error("the Ollama list survived a URL change")
	}
	if _, cached := svc.models[llm.ProviderClaude]; !cached {
		t.Error("an unrelated provider's list was dropped")
	}
}

// TestFailedListingsExpireSooner: the usual cause is something the user is about
// to fix.
func TestFailedListingsExpireSooner(t *testing.T) {
	if (cachedModelList{failed: true}).ttl() >= (cachedModelList{}).ttl() {
		t.Error("a failed listing must expire sooner than a good one")
	}
}

// TestOllamaIsAskedWithoutACredential: Ollama needs no key, so the "no
// credential yet" short-circuit must not skip it.
//
// The opposite case — an API provider with no key — is deliberately not tested:
// GetKey consults the OS keyring, which can hang on a machine without one.
func TestOllamaIsAskedWithoutACredential(t *testing.T) {
	svc := &Service{current: Default()}
	if _, ok := svc.modelListConfig(llm.ProviderOllama); !ok {
		t.Error("Ollama needs no key and must still be asked")
	}
}

// serviceWithUnreachableOllama builds a service whose Ollama URL points at a
// port nothing listens on, so a fetch fails immediately.
func serviceWithUnreachableOllama(t *testing.T, cached cachedModelList) *Service {
	t.Helper()
	svc := &Service{
		current: Default(),
		models:  map[string]cachedModelList{llm.ProviderOllama: cached},
	}
	svc.current.Providers.OllamaURL = "http://127.0.0.1:1"
	return svc
}
