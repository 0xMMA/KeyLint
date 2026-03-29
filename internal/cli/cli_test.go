package cli

import (
	"testing"
)

func TestRunUnknownCommand(t *testing.T) {
	err := Run([]string{"-unknown"}, nil, nil)
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
	want := `unknown command: "-unknown"`
	if err.Error() != want {
		t.Fatalf("got %q, want %q", err.Error(), want)
	}
}

func TestRunNoArgs(t *testing.T) {
	err := Run(nil, nil, nil)
	if err == nil {
		t.Fatal("expected error when no args provided")
	}
}
