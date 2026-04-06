package shortcut

import "time"

// PressResult classifies a hotkey press as single or double.
type PressResult int

const (
	Single PressResult = iota
	Double
)

// Detector implements double-press detection with a configurable threshold.
// It receives raw press events via Press() and emits classified results on Result().
type Detector struct {
	threshold time.Duration
	presses   chan struct{}
	results   chan PressResult
	done      chan struct{}
}

// NewDetector creates and starts a detector goroutine.
func NewDetector(threshold time.Duration) *Detector {
	d := &Detector{
		threshold: threshold,
		presses:   make(chan struct{}),
		results:   make(chan PressResult, 2),
		done:      make(chan struct{}),
	}
	go d.loop()
	return d
}

// Press records a hotkey press. Non-blocking (channel send).
func (d *Detector) Press() {
	select {
	case d.presses <- struct{}{}:
	case <-d.done:
	}
}

// Result returns the channel that receives classified press results.
func (d *Detector) Result() <-chan PressResult {
	return d.results
}

// Stop shuts down the detector goroutine and closes the result channel.
func (d *Detector) Stop() {
	select {
	case <-d.done:
	default:
		close(d.done)
	}
}

func (d *Detector) loop() {
	defer close(d.results)

	var timer *time.Timer
	var timerC <-chan time.Time

	for {
		select {
		case <-d.done:
			if timer != nil {
				timer.Stop()
			}
			return

		case <-d.presses:
			if timer != nil {
				// Second press within window → double
				timer.Stop()
				timer = nil
				timerC = nil
				d.results <- Double
			} else {
				// First press → start detection window
				timer = time.NewTimer(d.threshold)
				timerC = timer.C
			}

		case <-timerC:
			// Timer expired → single press
			timer = nil
			timerC = nil
			d.results <- Single
		}
	}
}
