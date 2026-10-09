package welcome

import (
	"strings"
	"time"

	"keylint/internal/features/settings"
	"keylint/internal/llm"
	"keylint/internal/logger"
)

// cliProbeWait is how long the first-run decision waits for the Claude Code
// CLI probe before calling its answer unknown.
//
// The decision runs while the app starts, so it must not sit on a cold probe
// for the probe's full 25 s budget. Five seconds covers a cold Node start on
// Windows; a probe still running after that keeps going in the background and
// fills the settings cache, so the wizard — if it opens — gets the answer
// without spawning again.
const cliProbeWait = 5 * time.Second

// keyProviders are the providers that work with an API key alone, in the
// order a replacement is picked from. Bedrock is left out: it does not work
// yet (#22, #23), and its env var, AWS_SECRET_ACCESS_KEY, is on many machines
// for reasons that have nothing to do with KeyLint.
var keyProviders = []string{llm.ProviderOpenAI, llm.ProviderClaude}

// Service handles first-run detection and setup completion.
type Service struct {
	settings *settings.Service

	// cliStatus asks whether the Claude Code CLI is signed in. nil means the
	// settings service's cached probe; a test swaps in its own.
	cliStatus func() llm.ClaudeCodeStatus
	// probeWait bounds the wait for cliStatus; zero means cliProbeWait.
	probeWait time.Duration
}

// NewService creates a new WelcomeService.
func NewService(s *settings.Service) *Service {
	return &Service{settings: s}
}

// cliState is what the first-run decision knows about the CLI.
type cliState int

const (
	cliUnknown   cliState = iota // probe still running when the wait ran out
	cliMissing                   // not installed
	cliSignedOut                 // installed, not signed in
	cliSignedIn                  // ready to use
)

func (c cliState) String() string {
	switch c {
	case cliMissing:
		return "not installed"
	case cliSignedOut:
		return "not signed in"
	case cliSignedIn:
		return "signed in"
	}
	return "unknown (probe still running)"
}

// IsFirstRun reports whether the setup wizard should open.
//
// Only for someone with nothing usable: completed_setup unset AND no API key
// for any provider (env or keyring), no signed-in Claude Code CLI, and no
// saved active provider that can work. Anyone with something usable is never
// asked for it again — their setup is marked complete here, silently, and the
// log line says why. The flag alone is not trusted to mean "new user": it has
// been seen false on a machine whose keys and CLI were all in place.
//
// Never returns or logs a key's value: only whether one is set. (On the way,
// GetKeyStatus fetches it from the keyring and drops it.)
func (s *Service) IsFirstRun() bool {
	cfg := s.settings.Get()
	if cfg.CompletedSetup {
		return false
	}
	// No settings file at all is a first start, whatever the machine holds: an
	// API key in the environment or a signed-in CLI belongs to other tools
	// until the user says KeyLint may use it, and this is the one screen that
	// shows the shortcut. The wizard preselects what works, so it is one
	// click, never a question asked twice. No probe here, so a first start
	// never waits on the CLI.
	if !settings.LoadOutcome(s.settings).Found {
		logger.Info("welcome: showing the setup wizard, first start (no settings file)")
		return true
	}

	d := s.decide(cfg)
	if d.use == "" {
		logger.Info("welcome: showing the setup wizard, nothing usable is configured",
			"active_provider", cfg.ActiveProvider,
			"keys", d.keySummary(),
			"claude_code_cli", d.cli.String())
		return true
	}

	err := settings.Update(s.settings, func(c *settings.Settings) {
		c.CompletedSetup = true
		if d.switchTo != "" {
			c.ActiveProvider = d.switchTo
		}
	})
	if err != nil {
		// Still no wizard: the user has something that works, and a failed
		// write only means this decision is made again at the next start.
		logger.Warn("welcome: could not save the completed setup", "err", err)
	}
	logger.Info("welcome: setup already usable, skipping the wizard",
		"reason", d.reason,
		"active_provider", cfg.ActiveProvider,
		"switched_to", d.switchTo,
		"keys", d.keySummary(),
		"claude_code_cli", d.cli.String())
	return false
}

// CompleteSetup marks the setup wizard as done.
//
// Through settings.Update, so a save that lands meanwhile (a provider switch)
// is not overwritten by settings read before it.
func (s *Service) CompleteSetup() error {
	return settings.Update(s.settings, func(c *settings.Settings) { c.CompletedSetup = true })
}

// decision is the outcome of the first-run check.
type decision struct {
	// use is the provider that works, or "" when nothing does.
	use string
	// switchTo is set when use is not the saved active provider and the saved
	// one cannot work at all, so the active provider moves to use.
	switchTo string
	reason   string
	keys     map[string]settings.KeyStatus
	cli      cliState
}

// keySummary names where each provider's key comes from: "openai=none
// claude=keyring". Sources only — a value never reaches the log.
func (d decision) keySummary() string {
	parts := make([]string, 0, len(keyProviders))
	for _, p := range keyProviders {
		parts = append(parts, p+"="+d.keys[p].Source)
	}
	return strings.Join(parts, " ")
}

// decide works out whether anything usable is configured, asking the CLI only
// when the answer depends on it.
func (s *Service) decide(cfg settings.Settings) decision {
	d := decision{keys: map[string]settings.KeyStatus{}, cli: cliUnknown}
	var withKey []string
	for _, p := range keyProviders {
		st := s.settings.GetKeyStatus(p)
		d.keys[p] = st
		if st.IsSet {
			withKey = append(withKey, p)
		}
	}
	active := cfg.ActiveProvider

	// The saved provider works without asking the CLI anything.
	switch {
	case d.keys[active].IsSet:
		d.use, d.reason = active, "active provider has a key"
		return d
	case active == llm.ProviderOllama:
		// Never the default (that is OpenAI), so somebody chose it. Whether the
		// daemon is up right now is the settings page's business, not a reason
		// to walk them through setup again.
		d.use, d.reason = active, "active provider is Ollama, which needs no key"
		return d
	}

	// Everything left depends on the CLI: either it is the saved provider, or
	// it comes first among the replacements, as it does in the wizard.
	d.cli = s.probeCLI()

	if active == llm.ProviderClaudeCode {
		switch d.cli {
		case cliSignedIn:
			d.use, d.reason = active, "active provider is the signed-in Claude Code CLI"
			return d
		case cliUnknown:
			// Chosen explicitly and not known to be broken. Unknown is not
			// unusable.
			d.use, d.reason = active, "active provider is the Claude Code CLI; its probe has not answered yet"
			return d
		}
		// Chosen but not working. A stored key still counts as usable, but the
		// choice of the CLI over a paid API stays the user's: no switch.
		if len(withKey) > 0 {
			d.use, d.reason = withKey[0], "a key is stored (active Claude Code CLI is "+d.cli.String()+")"
		}
		return d
	}

	// The saved provider is a key provider with no key, or a value this build
	// does not know (empty, or written by a newer build): it cannot work as it
	// is, so the active provider moves to what does — the same default the
	// wizard would preselect. Bedrock alone is kept: it is a choice the
	// settings page names and explains, not a value it cannot read.
	switch {
	case d.cli == cliSignedIn:
		d.use, d.reason = llm.ProviderClaudeCode, "the Claude Code CLI is signed in"
	case len(withKey) > 0:
		d.use, d.reason = withKey[0], "a key is stored"
	default:
		return d
	}
	if active != providerBedrock {
		d.switchTo = d.use
	}
	return d
}

// providerBedrock is a provider settings can name but no build offers yet
// (#22, #23); see UNAVAILABLE_PROVIDERS in frontend/src/app/core/constants.ts.
const providerBedrock = "bedrock"

// probeCLI asks for the CLI's status, waiting at most probeWait. A probe that
// is still running counts as unknown; it carries on in the background.
func (s *Service) probeCLI() cliState {
	ask := s.cliStatus
	if ask == nil {
		ask = func() llm.ClaudeCodeStatus { return s.settings.GetClaudeCodeStatus(false) }
	}
	wait := s.probeWait
	if wait <= 0 {
		wait = cliProbeWait
	}

	// Buffered, so a probe that answers after the wait does not leak its
	// goroutine blocked on a send nobody reads.
	answer := make(chan llm.ClaudeCodeStatus, 1)
	go func() { answer <- ask() }()

	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case st := <-answer:
		switch {
		case !st.Installed:
			return cliMissing
		case !st.LoggedIn:
			return cliSignedOut
		default:
			return cliSignedIn
		}
	case <-timer.C:
		return cliUnknown
	}
}
