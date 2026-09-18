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
	if got.Source != llm.ModelSourceUnreachable {
		t.Errorf("source = %q, want %q after a failed refetch", got.Source, llm.ModelSourceUnreachable)
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
			llm.ProviderClaude: {list: llm.CuratedModels(llm.ProviderClaude, llm.ModelSourceUnreachable), fetched: time.Now()},
			llm.ProviderOpenAI: {list: llm.CuratedModels(llm.ProviderOpenAI, llm.ModelSourceLive), fetched: time.Now()},
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
			llm.ProviderOllama: {list: llm.CuratedModels(llm.ProviderOllama, llm.ModelSourceLive), fetched: time.Now()},
			llm.ProviderClaude: {list: llm.CuratedModels(llm.ProviderClaude, llm.ModelSourceLive), fetched: time.Now()},
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

// TestTheTTLFollowsTheSituationTheUserIsIn covers the whole point of splitting
// the sources: a provider that was down and one with nothing pulled are both
// states the user is about to change — a daemon started, a model pulled — and
// ten minutes of the old answer after that is ten minutes of a wrong picker.
// A live listing has no such event coming, so it is kept.
func TestTheTTLFollowsTheSituationTheUserIsIn(t *testing.T) {
	for _, tc := range []struct {
		source string
		want   time.Duration
		why    string
	}{
		{llm.ModelSourceUnreachable, modelListFailureTTL, "the daemon or the key is about to be fixed"},
		{llm.ModelSourceEmpty, modelListFailureTTL, "a model is about to be pulled"},
		{llm.ModelSourceLive, modelListTTL, "the provider answered; nothing is pending"},
		{llm.ModelSourceFixed, modelListTTL, "there is no endpoint, so nothing can change"},
	} {
		entry := cachedModelList{list: llm.ModelList{Source: tc.source}}
		if got := entry.ttl(); got != tc.want {
			t.Errorf("ttl(%s) = %v, want %v — %s", tc.source, got, tc.want, tc.why)
		}
	}
}

// TestNoCredentialYieldsItsOwnSource: "add a key" and "the daemon is down" are
// different sentences for the user, so they must not arrive as one source.
//
// Ollama is used because it needs no key and would therefore be asked; the
// provider under test here is one that does need a key. Reading the keyring is
// what modelListConfig does, so this asserts the config decision rather than
// calling ListModels, which would hang on a machine with no secret service.
func TestNoCredentialIsNotTheSameAsUnreachable(t *testing.T) {
	if llm.ModelSourceNoCredentials == llm.ModelSourceUnreachable {
		t.Fatal("the two states collapsed into one value")
	}
	list := llm.CuratedModels(llm.ProviderOpenAI, llm.ModelSourceNoCredentials)
	if list.Source != llm.ModelSourceNoCredentials {
		t.Errorf("source = %q, want %q", list.Source, llm.ModelSourceNoCredentials)
	}
	if len(list.Models) == 0 {
		t.Error("a user without a key still gets a picker to look at")
	}
}

// TestAnEmptyListIsServedAsEmpty: the built-in list must not be substituted for
// a provider that answered with nothing — see the Ollama case in llm.ListModels.
func TestAnEmptyListIsNotRefilledFromTheCuratedOne(t *testing.T) {
	svc := serviceWithUnreachableOllama(t, cachedModelList{
		list:    llm.ModelList{Source: llm.ModelSourceEmpty},
		fetched: time.Now(),
	})

	got := svc.ListModels(llm.ProviderOllama)
	if got.Source != llm.ModelSourceEmpty {
		t.Errorf("source = %q, want %q", got.Source, llm.ModelSourceEmpty)
	}
	if len(got.Models) != 0 {
		t.Errorf("models = %v, want none", got.Models)
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
