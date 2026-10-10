package notify

import "testing"

// TestActivationReachesTheHandler: a click hands back the ID the notification
// was shown with, to whatever handler is set by then.
func TestActivationReachesTheHandler(t *testing.T) {
	toasts := New("KeyLint", nil)
	toasts.activated("before-a-handler") // must not panic

	var got []string
	toasts.OnActivate(func(id string) { got = append(got, id) })
	toasts.Show("silentfix-1", "Fix didn't run", "No API key for Anthropic.")
	toasts.activated("silentfix-1")

	if len(got) != 1 || got[0] != "silentfix-1" {
		t.Errorf("activations = %q, want [silentfix-1]", got)
	}
}
