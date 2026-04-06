package shortcut

import (
	"fmt"
	"strings"
)

// Modifier bitmask flags matching Win32 modifier virtual key codes.
type Modifier uint8

const (
	ModCtrl  Modifier = 1 << iota // VK_CONTROL (0x11)
	ModShift                      // VK_SHIFT   (0x10)
	ModAlt                        // VK_MENU    (0x12)
	ModWin                        // VK_LWIN    (0x5B)
)

// KeyCombo represents a parsed keyboard shortcut (modifier bitmask + trigger key).
type KeyCombo struct {
	Modifiers Modifier
	VK        uint16 // Windows virtual key code
	KeyName   string // lowercase key name, e.g. "g", "f8"
}

// modifierNames maps string names to modifier flags.
var modifierNames = map[string]Modifier{
	"ctrl":  ModCtrl,
	"shift": ModShift,
	"alt":   ModAlt,
	"win":   ModWin,
}

// modifierDisplay is the display-order list of modifiers.
var modifierDisplay = []struct {
	Flag Modifier
	Name string
}{
	{ModCtrl, "Ctrl"},
	{ModShift, "Shift"},
	{ModAlt, "Alt"},
	{ModWin, "Win"},
}

// keyNames maps lowercase key names to Windows virtual key codes.
var keyNames = map[string]uint16{
	// Letters A-Z
	"a": 0x41, "b": 0x42, "c": 0x43, "d": 0x44, "e": 0x45,
	"f": 0x46, "g": 0x47, "h": 0x48, "i": 0x49, "j": 0x4A,
	"k": 0x4B, "l": 0x4C, "m": 0x4D, "n": 0x4E, "o": 0x4F,
	"p": 0x50, "q": 0x51, "r": 0x52, "s": 0x53, "t": 0x54,
	"u": 0x55, "v": 0x56, "w": 0x57, "x": 0x58, "y": 0x59, "z": 0x5A,
	// Digits 0-9
	"0": 0x30, "1": 0x31, "2": 0x32, "3": 0x33, "4": 0x34,
	"5": 0x35, "6": 0x36, "7": 0x37, "8": 0x38, "9": 0x39,
	// Function keys F1-F12
	"f1": 0x70, "f2": 0x71, "f3": 0x72, "f4": 0x73,
	"f5": 0x74, "f6": 0x75, "f7": 0x76, "f8": 0x77,
	"f9": 0x78, "f10": 0x79, "f11": 0x7A, "f12": 0x7B,
	// Punctuation / special
	";": 0xBA, "=": 0xBB, ",": 0xBC, "-": 0xBD, ".": 0xBE,
	"/": 0xBF, "`": 0xC0, "[": 0xDB, "\\": 0xDC, "]": 0xDD, "'": 0xDE,
	"space": 0x20, "enter": 0x0D, "tab": 0x09, "escape": 0x1B,
	"backspace": 0x08, "delete": 0x2E, "insert": 0x2D,
	"home": 0x24, "end": 0x23, "pageup": 0x21, "pagedown": 0x22,
	"up": 0x26, "down": 0x28, "left": 0x25, "right": 0x27,
}

// vkToName is the reverse map, built on init.
var vkToName map[uint16]string

func init() {
	vkToName = make(map[uint16]string, len(keyNames))
	for name, vk := range keyNames {
		if existing, ok := vkToName[vk]; !ok || len(name) < len(existing) {
			vkToName[vk] = name
		}
	}
}

// ParseKeyCombo parses a shortcut string like "ctrl+g" or "f8" into a KeyCombo.
func ParseKeyCombo(s string) (KeyCombo, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return KeyCombo{}, fmt.Errorf("empty key combo")
	}
	parts := strings.Split(s, "+")
	var mods Modifier
	var triggerKey string

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return KeyCombo{}, fmt.Errorf("empty part in key combo %q", s)
		}
		if mod, ok := modifierNames[part]; ok {
			mods |= mod
		} else {
			if triggerKey != "" {
				return KeyCombo{}, fmt.Errorf("multiple trigger keys in %q: %q and %q", s, triggerKey, part)
			}
			triggerKey = part
		}
	}

	if triggerKey == "" {
		return KeyCombo{}, fmt.Errorf("no trigger key in %q (modifiers only)", s)
	}

	vk, ok := keyNames[triggerKey]
	if !ok {
		return KeyCombo{}, fmt.Errorf("unknown key name %q", triggerKey)
	}

	return KeyCombo{Modifiers: mods, VK: vk, KeyName: triggerKey}, nil
}

// DisplayString returns a human-readable representation like "Ctrl + Shift + G".
func (kc KeyCombo) DisplayString() string {
	var parts []string
	for _, md := range modifierDisplay {
		if kc.Modifiers&md.Flag != 0 {
			parts = append(parts, md.Name)
		}
	}
	parts = append(parts, strings.ToUpper(kc.KeyName))
	return strings.Join(parts, " + ")
}

// String returns the canonical lowercase format like "ctrl+shift+g".
func (kc KeyCombo) String() string {
	var parts []string
	for _, md := range modifierDisplay {
		if kc.Modifiers&md.Flag != 0 {
			parts = append(parts, strings.ToLower(md.Name))
		}
	}
	parts = append(parts, kc.KeyName)
	return strings.Join(parts, "+")
}
