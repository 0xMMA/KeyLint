//go:build !windows

package clipboard

import (
	"fmt"
	"os/exec"
	"strings"

	"keylint/internal/logger"
)

// Service reads from and writes to the system clipboard.
// Uses xclip, xsel, or wl-paste/wl-copy on Linux (tries each in order).
type Service struct{}

// NewService creates a new ClipboardService.
func NewService() *Service { return &Service{} }

// Read returns the current clipboard text content.
// Tries xclip → xsel → wl-paste. Returns a clear error if none are available.
func (s *Service) Read() (string, error) {
	tools := []struct {
		name string
		args []string
	}{
		{"xclip", []string{"-selection", "clipboard", "-o"}},
		{"xsel", []string{"--clipboard", "--output"}},
		{"wl-paste", []string{"--no-newline"}},
	}

	for _, t := range tools {
		if _, err := exec.LookPath(t.name); err != nil {
			continue // tool not installed — skip silently
		}
		out, err := exec.Command(t.name, t.args...).Output()
		if err == nil {
			text := strings.TrimRight(string(out), "\n")
			logger.Debug("clipboard: read", "tool", t.name, "len", len(text))
			return text, nil
		}
		// Tool exists but failed (e.g. no $DISPLAY) — log and try next
		logger.Warn("clipboard: read failed", "tool", t.name, "err", err)
	}

	return "", fmt.Errorf(
		"clipboard read failed: no clipboard tool available — install xclip, xsel, or wl-paste (Wayland)",
	)
}

// Write sets the clipboard text content.
// Tries xclip → xsel → wl-copy. Returns a clear error if none are available.
func (s *Service) Write(text string) error {
	logger.Debug("clipboard: write", "len", len(text))

	tools := []struct {
		name string
		args []string
	}{
		{"xclip", []string{"-selection", "clipboard"}},
		{"xsel", []string{"--clipboard", "--input"}},
		{"wl-copy", nil},
	}

	for _, t := range tools {
		if _, err := exec.LookPath(t.name); err != nil {
			continue
		}
		cmd := exec.Command(t.name, t.args...)
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err == nil {
			return nil
		} else {
			logger.Warn("clipboard: write failed", "tool", t.name, "err", err)
		}
	}

	return fmt.Errorf(
		"clipboard write failed: no clipboard tool available — install xclip, xsel, or wl-copy (Wayland)",
	)
}
