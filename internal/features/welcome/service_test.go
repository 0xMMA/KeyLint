package welcome

import (
	"sync/atomic"
	"testing"
	"time"

	"keylint/internal/features/settings"
	"keylint/internal/llm"
)

// The first-run decision is tested against an isolated settings service: keys
// come from the lookup given here and nowhere else, and the CLI probe is
// swapped out, so the developer's own environment, keyring and signed-in CLI
// cannot decide whether a test passes.

var (
	cliSignedInSt  = llm.ClaudeCodeStatus{Installed: true, LoggedIn: true}
	cliSignedOutSt = llm.ClaudeCodeStatus{Installed: true}
	cliMissingSt   = llm.ClaudeCodeStatus{}
)

type fixture struct {
	settings *settings.Service
	welcome  *Service
	probes   *atomic.Int32
}

// newFixture builds the services. keys maps provider → key; cli is what the
// probe answers, after delay.
func newFixture(t *testing.T, cfg settings.Settings, keys map[string]string, cli llm.ClaudeCodeStatus, delay time.Duration) fixture {
	t.Helper()
	sSvc := settings.NewServiceFrom(cfg, func(p string) string { return keys[p] })
	probes := &atomic.Int32{}
	w := NewService(sSvc)
	w.probeWait = 50 * time.Millisecond
	w.cliStatus = func() llm.ClaudeCodeStatus {
		probes.Add(1)
		time.Sleep(delay)
		return cli
	}
	return fixture{settings: sSvc, welcome: w, probes: probes}
}

func cfgWith(active string, completed bool) settings.Settings {
	c := settings.Default()
	c.ActiveProvider = active
	c.CompletedSetup = completed
	return c
}

func TestIsFirstRun_Matrix(t *testing.T) {
	cases := []struct {
		name       string
		cfg        settings.Settings
		keys       map[string]string
		cli        llm.ClaudeCodeStatus
		slowProbe  bool
		wantWizard bool
		// wantActive is the active provider afterwards.
		wantActive string
		// wantProbe says whether the decision had to ask the CLI.
		wantProbe bool
	}{
		{
			name: "nothing usable: no key, no CLI", cfg: cfgWith("openai", false),
			cli: cliMissingSt, wantWizard: true, wantActive: "openai", wantProbe: true,
		},
		{
			name: "nothing usable: CLI installed but signed out", cfg: cfgWith("openai", false),
			cli: cliSignedOutSt, wantWizard: true, wantActive: "openai", wantProbe: true,
		},
		{
			name: "probe timeout and no key: unknown alone is not usable", cfg: cfgWith("openai", false),
			cli: cliSignedInSt, slowProbe: true, wantWizard: true, wantActive: "openai", wantProbe: true,
		},
		{
			name: "key for the active provider: no probe needed", cfg: cfgWith("claude", false),
			keys: map[string]string{"claude": "sk-ant-x"}, cli: cliMissingSt,
			wantActive: "claude", wantProbe: false,
		},
		{
			name: "key for another provider: active moves to it", cfg: cfgWith("openai", false),
			keys: map[string]string{"claude": "sk-ant-x"}, cli: cliMissingSt,
			wantActive: "claude", wantProbe: true,
		},
		{
			name: "CLI only: active moves to the CLI", cfg: cfgWith("openai", false),
			cli: cliSignedInSt, wantActive: "claude-code", wantProbe: true,
		},
		{
			name: "CLI and a key on another provider: CLI first, as in the wizard", cfg: cfgWith("openai", false),
			keys: map[string]string{"claude": "sk-ant-x"}, cli: cliSignedInSt,
			wantActive: "claude-code", wantProbe: true,
		},
		{
			name: "probe timeout plus a key: usable", cfg: cfgWith("openai", false),
			keys: map[string]string{"claude": "sk-ant-x"}, cli: cliSignedInSt, slowProbe: true,
			wantActive: "claude", wantProbe: true,
		},
		{
			name: "active CLI, signed in", cfg: cfgWith("claude-code", false),
			cli: cliSignedInSt, wantActive: "claude-code", wantProbe: true,
		},
		{
			name: "active CLI, probe timeout: unknown is not unusable", cfg: cfgWith("claude-code", false),
			cli: cliSignedInSt, slowProbe: true, wantActive: "claude-code", wantProbe: true,
		},
		{
			name: "active CLI signed out, key stored: usable, choice kept", cfg: cfgWith("claude-code", false),
			keys: map[string]string{"openai": "sk-x"}, cli: cliSignedOutSt,
			wantActive: "claude-code", wantProbe: true,
		},
		{
			name: "active CLI signed out, nothing else", cfg: cfgWith("claude-code", false),
			cli: cliSignedOutSt, wantWizard: true, wantActive: "claude-code", wantProbe: true,
		},
		{
			name: "active Ollama: chosen, needs no key", cfg: cfgWith("ollama", false),
			cli: cliMissingSt, wantActive: "ollama", wantProbe: false,
		},
		{
			name: "completed flag set: nothing asked", cfg: cfgWith("openai", true),
			cli: cliMissingSt, wantActive: "openai", wantProbe: false,
		},
		{
			name: "unavailable saved provider is explained, never switched", cfg: cfgWith("bedrock", false),
			keys: map[string]string{"openai": "sk-x"}, cli: cliMissingSt,
			wantActive: "bedrock", wantProbe: true,
		},
		{
			name: "a provider this build does not know moves to what works", cfg: cfgWith("some-future-provider", false),
			keys: map[string]string{"claude": "sk-ant-x"}, cli: cliMissingSt,
			wantActive: "claude", wantProbe: true,
		},
		{
			name: "an empty saved provider moves to the CLI", cfg: cfgWith("", false),
			cli: cliSignedInSt, wantActive: "claude-code", wantProbe: true,
		},
		{
			name: "a Bedrock env key alone does not count", cfg: cfgWith("openai", false),
			keys: map[string]string{"bedrock": "aws-secret"}, cli: cliMissingSt,
			wantWizard: true, wantActive: "openai", wantProbe: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			delay := time.Duration(0)
			if tc.slowProbe {
				delay = 2 * time.Second
			}
			f := newFixture(t, tc.cfg, tc.keys, tc.cli, delay)

			if got := f.welcome.IsFirstRun(); got != tc.wantWizard {
				t.Fatalf("IsFirstRun = %v, want %v", got, tc.wantWizard)
			}
			after := f.settings.Get()
			if after.ActiveProvider != tc.wantActive {
				t.Errorf("active provider = %q, want %q", after.ActiveProvider, tc.wantActive)
			}
			// Anything usable completes the setup silently; a wizard leaves it
			// for the wizard to finish.
			if after.CompletedSetup == tc.wantWizard {
				t.Errorf("completed_setup = %v with wizard = %v", after.CompletedSetup, tc.wantWizard)
			}
			if probed := f.probes.Load() > 0; probed != tc.wantProbe {
				t.Errorf("probed the CLI = %v, want %v", probed, tc.wantProbe)
			}
		})
	}
}

// A first start — no settings file — always opens the wizard, which preselects
// whatever works. Nothing is probed, so it never waits on the CLI.
func TestIsFirstRun_FirstStartShowsTheWizard(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	sSvc, err := settings.NewService()
	if err != nil {
		t.Fatal(err)
	}
	w := NewService(sSvc)
	var probed atomic.Bool
	w.cliStatus = func() llm.ClaudeCodeStatus { probed.Store(true); return cliSignedInSt }

	if !w.IsFirstRun() {
		t.Fatal("no wizard on a first start")
	}
	if probed.Load() {
		t.Error("probed the CLI on a first start")
	}
	if sSvc.Get().CompletedSetup {
		t.Error("first start marked setup complete without the user")
	}
}

// The wait for a slow probe is bounded: start-up must not sit out the probe's
// own 25-second budget.
func TestIsFirstRun_DoesNotWaitOutASlowProbe(t *testing.T) {
	f := newFixture(t, cfgWith("openai", false), nil, cliSignedInSt, 10*time.Second)
	start := time.Now()
	f.welcome.IsFirstRun()
	if took := time.Since(start); took > time.Second {
		t.Errorf("IsFirstRun took %v with a 50ms wait", took)
	}
}

// Silent completion is a one-off: the next start finds the flag and asks
// nothing.
func TestIsFirstRun_SilentCompletionSticks(t *testing.T) {
	f := newFixture(t, cfgWith("claude", false), map[string]string{"claude": "sk-ant-x"}, cliMissingSt, 0)
	if f.welcome.IsFirstRun() {
		t.Fatal("wizard shown with a stored key")
	}
	if !f.settings.Get().CompletedSetup {
		t.Fatal("setup not marked complete")
	}
	if f.welcome.IsFirstRun() {
		t.Error("wizard shown on the next start")
	}
	if n := f.probes.Load(); n != 0 {
		t.Errorf("probed the CLI %d times; the flag alone should answer", n)
	}
}

func TestCompleteSetup_SetsFlag(t *testing.T) {
	f := newFixture(t, cfgWith("openai", false), nil, cliMissingSt, 0)
	if err := f.welcome.CompleteSetup(); err != nil {
		t.Fatalf("CompleteSetup: %v", err)
	}
	if f.welcome.IsFirstRun() {
		t.Error("expected IsFirstRun=false after CompleteSetup()")
	}
	if !f.settings.Get().CompletedSetup {
		t.Error("expected settings.CompletedSetup=true after CompleteSetup()")
	}
}

func TestCompleteSetup_IsIdempotent(t *testing.T) {
	f := newFixture(t, cfgWith("openai", false), nil, cliMissingSt, 0)
	for i := 0; i < 2; i++ {
		if err := f.welcome.CompleteSetup(); err != nil {
			t.Fatalf("CompleteSetup #%d: %v", i+1, err)
		}
	}
	if f.welcome.IsFirstRun() {
		t.Error("expected IsFirstRun=false after double CompleteSetup()")
	}
}

// CompleteSetup leaves every other setting alone — in particular the provider
// the wizard has just saved.
func TestCompleteSetup_KeepsTheChosenProvider(t *testing.T) {
	f := newFixture(t, cfgWith("openai", false), nil, cliMissingSt, 0)
	if err := f.settings.SetActiveProvider("ollama"); err != nil {
		t.Fatal(err)
	}
	if err := f.welcome.CompleteSetup(); err != nil {
		t.Fatal(err)
	}
	if got := f.settings.Get().ActiveProvider; got != "ollama" {
		t.Errorf("active provider = %q, want ollama", got)
	}
}
