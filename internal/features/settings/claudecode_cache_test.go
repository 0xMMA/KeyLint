package settings

import (
	"context"
	"sync"
	"testing"
	"time"

	"keylint/internal/llm"
)

// The probe spawns processes, and four screens ask for it — the Pyramidize page
// on load and on every provider change, the settings card, and the welcome
// wizard. #55: that was one spawn per ask.

func serviceWithCountingProbe() (*Service, func() int) {
	var mu sync.Mutex
	calls := 0
	svc := &Service{current: Default()}
	svc.probeClaudeCode = func(context.Context) llm.ClaudeCodeStatus {
		mu.Lock()
		calls++
		mu.Unlock()
		return llm.ClaudeCodeStatus{Installed: true, Path: "/usr/bin/claude", LoggedIn: true}
	}
	return svc, func() int {
		mu.Lock()
		defer mu.Unlock()
		return calls
	}
}

func TestTheProbeRunsOncePerTTLHoweverManyScreensAsk(t *testing.T) {
	svc, calls := serviceWithCountingProbe()

	for i := 0; i < 5; i++ {
		if got := svc.GetClaudeCodeStatus(false); !got.Installed {
			t.Fatalf("call %d returned %+v", i, got)
		}
	}

	if calls() != 1 {
		t.Errorf("probes = %d, want 1 — the cache is what #55 asked for", calls())
	}
}

func TestForceAlwaysProbes(t *testing.T) {
	svc, calls := serviceWithCountingProbe()

	svc.GetClaudeCodeStatus(false)
	svc.GetClaudeCodeStatus(true)
	svc.GetClaudeCodeStatus(true)

	if calls() != 3 {
		t.Errorf("probes = %d, want 3 — a user pressing re-check has just changed something", calls())
	}
}

// TestAnExpiredEntryIsProbedAgain: someone who signs in elsewhere and comes back
// should see it without hunting for the button.
func TestAnExpiredEntryIsProbedAgain(t *testing.T) {
	svc, calls := serviceWithCountingProbe()

	svc.GetClaudeCodeStatus(false)
	svc.claudeCodeMu.Lock()
	svc.claudeCodeAt = time.Now().Add(-claudeCodeStatusTTL - time.Second)
	svc.claudeCodeMu.Unlock()
	svc.GetClaudeCodeStatus(false)

	if calls() != 2 {
		t.Errorf("probes = %d, want 2", calls())
	}
}

// TestTheCacheIsSafeUnderConcurrentAsks: Wails serves every RPC on its own
// goroutine, and the four screens can ask at once. Run under -race.
func TestTheCacheIsSafeUnderConcurrentAsks(t *testing.T) {
	svc, _ := serviceWithCountingProbe()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			svc.GetClaudeCodeStatus(i%4 == 0)
		}(i)
	}
	wg.Wait()

	if got := svc.GetClaudeCodeStatus(false); !got.Installed {
		t.Errorf("status = %+v, want the probe's answer", got)
	}
}

// TestConcurrentColdCallersShareOneProbe: four screens opening at once used to
// spawn four probes, because the probe ran outside the lock and nothing said
// "one is already on its way". That is the cost #55 exists to remove, and a
// sequential test cannot see it.
func TestConcurrentColdCallersShareOneProbe(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	release := make(chan struct{})

	svc := &Service{current: Default()}
	svc.probeClaudeCode = func(context.Context) llm.ClaudeCodeStatus {
		mu.Lock()
		calls++
		mu.Unlock()
		// Hold the probe open so every caller is inside the window.
		<-release
		return llm.ClaudeCodeStatus{Installed: true, LoggedIn: true}
	}

	results := make(chan llm.ClaudeCodeStatus, 4)
	for i := 0; i < 4; i++ {
		go func() { results <- svc.GetClaudeCodeStatus(false) }()
	}
	// Give them all time to arrive at the gate before it opens.
	time.Sleep(50 * time.Millisecond)
	close(release)

	for i := 0; i < 4; i++ {
		if got := <-results; !got.Installed {
			t.Errorf("caller %d got %+v", i, got)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if calls != 1 {
		t.Errorf("probes = %d, want 1 — four concurrent asks on a cold cache must share one", calls)
	}
}

// TestForceDoesNotSettleForAnInFlightAnswer is the invariant the Re-check
// button depends on.
//
// A probe can take up to claudeCodeStatusTimeout, so one that is already running
// may have started long before the button was pressed — handing its result back
// as though it were fresh is exactly the staleness the button exists to escape.
// The forced caller has to wait for it and then probe again.
//
// The earlier version of this test called the two in sequence, which proves
// nothing: it never overlapped them.
func TestForceDoesNotSettleForAnInFlightAnswer(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	started := make(chan struct{})
	release := make(chan struct{})

	svc := &Service{current: Default()}
	svc.probeClaudeCode = func(context.Context) llm.ClaudeCodeStatus {
		mu.Lock()
		calls++
		first := calls == 1
		mu.Unlock()
		if first {
			close(started)
			<-release
		}
		return llm.ClaudeCodeStatus{Installed: true, LoggedIn: true}
	}

	// A background probe, held open.
	go svc.GetClaudeCodeStatus(false)
	<-started

	// The button, pressed while that one is still running.
	done := make(chan struct{})
	go func() { defer close(done); svc.GetClaudeCodeStatus(true) }()

	// Give the forced caller time to reach the in-flight branch, then let the
	// first probe finish.
	time.Sleep(50 * time.Millisecond)
	close(release)
	<-done

	mu.Lock()
	defer mu.Unlock()
	if calls != 2 {
		t.Errorf("probes = %d, want 2 — the forced caller took the in-flight answer instead of asking again", calls)
	}
}

// TestATimedOutProbeIsNotCached: CheckClaudeCode has no error return, so a spawn
// cut short by the deadline comes back as "installed, not signed in". Caching
// that turns one slow spawn into a minute of wrong answers, and the welcome
// wizard has no re-check button to escape it with.
func TestATimedOutProbeIsNotCached(t *testing.T) {
	var mu sync.Mutex
	calls := 0

	svc := &Service{current: Default()}
	svc.probeClaudeCode = func(ctx context.Context) llm.ClaudeCodeStatus {
		mu.Lock()
		calls++
		n := calls
		mu.Unlock()
		if n == 1 {
			// Behave like a probe whose deadline expired.
			<-ctx.Done()
			return llm.ClaudeCodeStatus{Installed: true}
		}
		return llm.ClaudeCodeStatus{Installed: true, LoggedIn: true}
	}

	original := claudeCodeStatusTimeoutForTest
	claudeCodeStatusTimeoutForTest = 50 * time.Millisecond
	defer func() { claudeCodeStatusTimeoutForTest = original }()

	svc.GetClaudeCodeStatus(false)
	if got := svc.GetClaudeCodeStatus(false); !got.LoggedIn {
		t.Error("the timed-out answer was cached and served again")
	}

	mu.Lock()
	defer mu.Unlock()
	if calls != 2 {
		t.Errorf("probes = %d, want 2", calls)
	}
}

// TestANegativeAnswerExpiresQuickly: "not installed" is what a user is about to
// change, so it must not sit in the cache for a full minute.
func TestANegativeAnswerExpiresQuickly(t *testing.T) {
	absent := llm.ClaudeCodeStatus{}
	present := llm.ClaudeCodeStatus{Installed: true, LoggedIn: true}
	signedOut := llm.ClaudeCodeStatus{Installed: true}

	if claudeCodeTTL(absent) >= claudeCodeTTL(present) {
		t.Error("a missing CLI is cached as long as a working one")
	}
	if claudeCodeTTL(signedOut) >= claudeCodeTTL(present) {
		t.Error("a signed-out CLI is cached as long as a signed-in one")
	}
}
