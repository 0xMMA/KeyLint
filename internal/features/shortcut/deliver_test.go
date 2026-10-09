package shortcut

import (
	"testing"
	"time"
)

// TestDeliverNeverBlocks: on Windows the sender is the keyboard hook's thread,
// and blocking it stalls every keystroke on the machine.
func TestDeliverNeverBlocks(t *testing.T) {
	ch := make(chan ShortcutEvent, eventBuffer)
	ev := ShortcutEvent{Source: "hotkey", Action: "fix"}

	for i := 0; i < eventBuffer; i++ {
		if !deliver(ch, ev) {
			t.Fatalf("event %d dropped while the buffer had room", i+1)
		}
	}

	done := make(chan bool, 1)
	go func() { done <- deliver(ch, ev) }()
	select {
	case ok := <-done:
		if ok {
			t.Error("an event beyond the buffer was reported as delivered")
		}
	case <-time.After(time.Second):
		t.Fatal("deliver blocked on a full buffer")
	}
}

// TestBurstWhileTheConsumerIsBusyArrives: the old buffer of 2 filled while
// the consumer copied for a Pyramidize press (up to a second), and the hook
// thread then waited on it. A burst of presses in that second must all be
// delivered, in order, once the consumer is back.
func TestBurstWhileTheConsumerIsBusyArrives(t *testing.T) {
	ch := make(chan ShortcutEvent, eventBuffer)
	actions := []string{"pyramidize", "fix", "fix", "pyramidize", "fix", "fix", "fix", "fix"}
	for _, a := range actions { // nobody is reading
		if !deliver(ch, ShortcutEvent{Source: "hotkey", Action: a}) {
			t.Fatalf("press %q dropped during a busy second", a)
		}
	}
	for i, want := range actions {
		if got := (<-ch).Action; got != want {
			t.Errorf("event %d = %q, want %q", i, got, want)
		}
	}
}
