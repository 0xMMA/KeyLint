//go:build !windows

package notify

// logOnly is every platform but Windows: the Show log line is the whole
// notification. Linux is where KeyLint is developed, not where it is used.
type logOnly struct{}

func newPlatform(string, []byte, func(string)) platform { return logOnly{} }

func (logOnly) show(string, string, string) {}
