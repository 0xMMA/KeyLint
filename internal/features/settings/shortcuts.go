package settings

import (
	"fmt"

	"keylint/internal/features/shortcut"
	"keylint/internal/logger"
)

// The shortcut rules the Windows hook applies, kept here so the file and the
// settings screen never hold a value the hook would reject or silently change.
const (
	shortcutModeDoubleTap   = "double_tap"
	shortcutModeIndependent = "independent"
	minDoubleTapDelay       = 100
	maxDoubleTapDelay       = 500
)

// normalizeShortcuts puts unusable shortcut values read from a file back to
// working ones. An unparseable combo would otherwise stop the hook from
// installing at all — every shortcut dead, with only a log line — and an
// unknown mode or out-of-range delay would make the screen disagree with what
// the hook actually does.
func normalizeShortcuts(s *Settings) {
	d := Default()
	before := [4]any{s.ShortcutMode, s.ShortcutFix, s.ShortcutPyramidize, s.ShortcutDoubleTapDelay}
	if s.ShortcutMode != shortcutModeDoubleTap && s.ShortcutMode != shortcutModeIndependent {
		s.ShortcutMode = d.ShortcutMode
	}
	if shortcut.CheckCombo(s.ShortcutFix) != nil {
		s.ShortcutFix = d.ShortcutFix
	}
	if shortcut.CheckCombo(s.ShortcutPyramidize) != nil {
		s.ShortcutPyramidize = d.ShortcutPyramidize
	}
	switch {
	case s.ShortcutDoubleTapDelay == 0:
		s.ShortcutDoubleTapDelay = d.ShortcutDoubleTapDelay
	case s.ShortcutDoubleTapDelay < minDoubleTapDelay:
		s.ShortcutDoubleTapDelay = minDoubleTapDelay
	case s.ShortcutDoubleTapDelay > maxDoubleTapDelay:
		s.ShortcutDoubleTapDelay = maxDoubleTapDelay
	}
	after := [4]any{s.ShortcutMode, s.ShortcutFix, s.ShortcutPyramidize, s.ShortcutDoubleTapDelay}
	if before != after {
		logger.Warn("settings: unusable shortcut settings replaced",
			"mode", s.ShortcutMode, "fix", s.ShortcutFix,
			"pyramidize", s.ShortcutPyramidize, "delay", s.ShortcutDoubleTapDelay)
	}
}

// shortcutsChanged says whether a save touches anything the hook reads.
func shortcutsChanged(a, b Settings) bool {
	return a.ShortcutMode != b.ShortcutMode || a.ShortcutFix != b.ShortcutFix ||
		a.ShortcutPyramidize != b.ShortcutPyramidize ||
		a.ShortcutDoubleTapDelay != b.ShortcutDoubleTapDelay
}

// validateShortcuts rejects shortcut settings the hook cannot run with. Only
// called for a save that changes them, so a save of something else is never
// blocked by values that were already there.
func validateShortcuts(s Settings) error {
	if s.ShortcutMode != shortcutModeDoubleTap && s.ShortcutMode != shortcutModeIndependent {
		return fmt.Errorf("unknown shortcut mode %q", s.ShortcutMode)
	}
	if err := shortcut.CheckCombo(s.ShortcutFix); err != nil {
		return fmt.Errorf("fix shortcut: %w", err)
	}
	if s.ShortcutMode == shortcutModeIndependent {
		if err := shortcut.CheckCombo(s.ShortcutPyramidize); err != nil {
			return fmt.Errorf("pyramidize shortcut: %w", err)
		}
		if s.ShortcutPyramidize == s.ShortcutFix {
			return fmt.Errorf("the fix and pyramidize shortcuts are both %q", s.ShortcutFix)
		}
	}
	if s.ShortcutDoubleTapDelay < minDoubleTapDelay || s.ShortcutDoubleTapDelay > maxDoubleTapDelay {
		return fmt.Errorf("double-tap shortcut delay %d ms is outside %d-%d ms",
			s.ShortcutDoubleTapDelay, minDoubleTapDelay, maxDoubleTapDelay)
	}
	return nil
}
