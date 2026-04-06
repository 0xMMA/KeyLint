package shortcut

import (
	"testing"
	"time"
)

func TestDetector_SinglePress(t *testing.T) {
	d := NewDetector(200 * time.Millisecond)
	defer d.Stop()

	d.Press()

	select {
	case r := <-d.Result():
		if r != Single {
			t.Fatalf("expected Single, got %v", r)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for result")
	}
}

func TestDetector_DoublePress(t *testing.T) {
	d := NewDetector(200 * time.Millisecond)
	defer d.Stop()

	d.Press()
	d.Press() // within threshold

	select {
	case r := <-d.Result():
		if r != Double {
			t.Fatalf("expected Double, got %v", r)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for result")
	}
}

func TestDetector_SlowDoublePress(t *testing.T) {
	d := NewDetector(200 * time.Millisecond)
	defer d.Stop()

	d.Press()
	// Wait for first single to fire
	r1 := <-d.Result()
	if r1 != Single {
		t.Fatalf("expected first Single, got %v", r1)
	}

	d.Press()
	r2 := <-d.Result()
	if r2 != Single {
		t.Fatalf("expected second Single, got %v", r2)
	}
}

func TestDetector_TriplePress(t *testing.T) {
	d := NewDetector(200 * time.Millisecond)
	defer d.Stop()

	d.Press()
	d.Press()
	d.Press() // third press starts new cycle

	// First result: Double from presses 1+2
	r1 := <-d.Result()
	if r1 != Double {
		t.Fatalf("expected Double, got %v", r1)
	}

	// Second result: Single from press 3 (after timeout)
	select {
	case r2 := <-d.Result():
		if r2 != Single {
			t.Fatalf("expected Single, got %v", r2)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for second result")
	}
}

func TestDetector_Stop(t *testing.T) {
	d := NewDetector(200 * time.Millisecond)
	d.Stop()

	// Result channel should be closed after Stop
	_, ok := <-d.Result()
	if ok {
		t.Fatal("expected result channel to be closed")
	}
}
