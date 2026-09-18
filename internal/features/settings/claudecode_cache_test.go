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

// TestAForcedProbeIsNotSharedWithAWaiterThatWantedFresh: force means "ignore
// what we have", so a forced caller must not be handed another caller's older
// in-flight answer as if it were its own probe.
func TestAForcedProbeStillProbes(t *testing.T) {
	svc, calls := serviceWithCountingProbe()

	svc.GetClaudeCodeStatus(false)
	svc.GetClaudeCodeStatus(true)

	if calls() != 2 {
		t.Errorf("probes = %d, want 2", calls())
	}
}
