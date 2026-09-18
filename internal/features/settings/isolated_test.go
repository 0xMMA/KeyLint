package settings

import (
	"os"
	"path/filepath"
	"testing"
)

// A service built with NewServiceFrom is what an eval run measures against. It
// must answer only from what it was handed: a developer's settings file and the
// machine's keyring cannot be allowed to move a number in quality-status.md.

func TestAnIsolatedServiceIgnoresTheSettingsFile(t *testing.T) {
	// A settings file that would break a service built the normal way.
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, appName), 0700); err != nil {
		t.Fatal(err)
	}
	broken := filepath.Join(dir, appName, "settings.json")
	if err := os.WriteFile(broken, []byte(`{not json at all`), 0600); err != nil {
		t.Fatal(err)
	}

	cfg := Default()
	cfg.ActiveProvider = "claude"
	svc := NewServiceFrom(cfg, func(string) string { return "" })

	if got := svc.Get().ActiveProvider; got != "claude" {
		t.Errorf("ActiveProvider = %q, want the one it was constructed with", got)
	}
	// The acceptance criterion from #59: the broken file changes nothing.
	if _, err := NewService(); err == nil {
		t.Log("note: NewService tolerated the broken file; the isolation claim is unaffected")
	}
}

func TestAnIsolatedServiceTakesKeysOnlyFromItsLookup(t *testing.T) {
	// An environment variable the normal lookup would find.
	t.Setenv("ANTHROPIC_API_KEY", "from-the-environment")

	svc := NewServiceFrom(Default(), func(provider string) string {
		if provider == "claude" {
			return "from-the-lookup"
		}
		return ""
	})

	if got := svc.GetKey("claude"); got != "from-the-lookup" {
		t.Errorf("GetKey = %q, want the lookup's answer", got)
	}
	if got := svc.GetKey("openai"); got != "" {
		t.Errorf("GetKey(openai) = %q, want empty — the lookup is the only source", got)
	}
	if status := svc.GetKeyStatus("claude"); !status.IsSet {
		t.Error("GetKeyStatus disagrees with GetKey")
	}
	if status := svc.GetKeyStatus("openai"); status.IsSet {
		t.Error("GetKeyStatus reports a key the lookup does not have")
	}
}

// TestEnvOnlyKeysReadsTheEnvironmentAndNothingElse: the eval's key source. `.env`
// is loaded into the environment before the run, so this is where its keys
// arrive — and the keyring is not consulted, which is the point.
func TestEnvOnlyKeysReadsTheEnvironmentAndNothingElse(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-test")
	t.Setenv("OPENAI_API_KEY", "")

	if got := EnvOnlyKeys("claude"); got != "sk-test" {
		t.Errorf("EnvOnlyKeys(claude) = %q, want the environment's value", got)
	}
	if got := EnvOnlyKeys("openai"); got != "" {
		t.Errorf("EnvOnlyKeys(openai) = %q, want empty", got)
	}
	if got := EnvOnlyKeys("ollama"); got != "" {
		t.Errorf("EnvOnlyKeys(ollama) = %q, want empty — it has no key", got)
	}
}

// TestAnIsolatedServiceRefusesToTouchTheKeyring: an eval must not be able to
// write into the developer's credential store by accident.
func TestAnIsolatedServiceRefusesToTouchTheKeyring(t *testing.T) {
	svc := NewServiceFrom(Default(), func(string) string { return "" })

	if err := svc.SetKey("claude", "whatever"); err == nil {
		t.Error("SetKey was allowed to reach the keyring")
	}
	if err := svc.DeleteKey("claude"); err == nil {
		t.Error("DeleteKey was allowed to reach the keyring")
	}
}

// TestAnIsolatedServiceSavesInMemoryOnly: Save must not write to the user's
// real settings file, and must not fail either — the pipeline calls it.
func TestAnIsolatedServiceSavesInMemoryOnly(t *testing.T) {
	svc := NewServiceFrom(Default(), func(string) string { return "" })

	updated := Default()
	updated.ActiveProvider = "openai"
	if err := svc.Save(updated); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if got := svc.Get().ActiveProvider; got != "openai" {
		t.Errorf("ActiveProvider = %q, want the saved value held in memory", got)
	}
}
