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

// TestEventBufferCoversABusyConsumer: the old buffer of 2 filled while the
// consumer copied from the foreground window for a Pyramidize press, and the
// hook thread then waited on it.
func TestEventBufferCoversABusyConsumer(t *testing.T) {
	if eventBuffer < 8 {
		t.Errorf("eventBuffer = %d, want room for a burst of presses during a one-second copy", eventBuffer)
	}
}
