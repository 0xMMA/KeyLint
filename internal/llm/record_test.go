package llm

import (
	"context"
	"testing"
)

type answeringClient struct{ model *string }

func (a answeringClient) Complete(context.Context, Request) (Response, error) {
	return Response{Text: "ok", Model: *a.model}, nil
}

// TestModelRecorderRecordsWhatAnswered: an eval records the model that ran,
// several when the generation moved mid-run, and the request only when nothing
// answered at all.
func TestModelRecorderRecordsWhatAnswered(t *testing.T) {
	var r ModelRecorder
	if got := r.Recorded("haiku"); got != "haiku" {
		t.Errorf("nothing answered: %q", got)
	}
	answer := "claude-haiku-5-5"
	newClient := r.Wrap(func(string, Config) (Client, error) { return answeringClient{model: &answer}, nil })
	complete := func() {
		c, _ := newClient(ProviderClaudeCode, Config{})
		_, _ = c.Complete(context.Background(), Request{})
	}
	complete()
	complete()
	if got := r.Recorded("haiku"); got != "claude-haiku-5-5" {
		t.Errorf("one model: %q", got)
	}
	answer = "claude-haiku-6"
	complete()
	if got := r.Recorded("haiku"); got != "claude-haiku-5-5+claude-haiku-6" {
		t.Errorf("two models: %q", got)
	}
}
