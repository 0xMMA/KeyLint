package shortcut

import "testing"

func TestParseKeyCombo_CtrlG(t *testing.T) {
	kc, err := ParseKeyCombo("ctrl+g")
	if err != nil {
		t.Fatal(err)
	}
	if kc.Modifiers != ModCtrl {
		t.Fatalf("expected ModCtrl (%d), got %d", ModCtrl, kc.Modifiers)
	}
	if kc.VK != 0x47 {
		t.Fatalf("expected VK 0x47, got 0x%X", kc.VK)
	}
}

func TestParseKeyCombo_CtrlShiftE(t *testing.T) {
	kc, err := ParseKeyCombo("ctrl+shift+e")
	if err != nil {
		t.Fatal(err)
	}
	if kc.Modifiers != ModCtrl|ModShift {
		t.Fatalf("expected ModCtrl|ModShift (%d), got %d", ModCtrl|ModShift, kc.Modifiers)
	}
	if kc.VK != 0x45 {
		t.Fatalf("expected VK 0x45 (E), got 0x%X", kc.VK)
	}
}

func TestParseKeyCombo_StandaloneF8(t *testing.T) {
	kc, err := ParseKeyCombo("f8")
	if err != nil {
		t.Fatal(err)
	}
	if kc.Modifiers != 0 {
		t.Fatalf("expected no modifiers, got %d", kc.Modifiers)
	}
	if kc.VK != 0x77 {
		t.Fatalf("expected VK 0x77 (F8), got 0x%X", kc.VK)
	}
}

func TestParseKeyCombo_TripleModifier(t *testing.T) {
	kc, err := ParseKeyCombo("ctrl+shift+alt+k")
	if err != nil {
		t.Fatal(err)
	}
	if kc.Modifiers != ModCtrl|ModShift|ModAlt {
		t.Fatalf("expected all three modifiers, got %d", kc.Modifiers)
	}
	if kc.VK != 0x4B {
		t.Fatalf("expected VK 0x4B (K), got 0x%X", kc.VK)
	}
}

func TestParseKeyCombo_Invalid(t *testing.T) {
	_, err := ParseKeyCombo("")
	if err == nil {
		t.Fatal("expected error for empty string")
	}
	_, err = ParseKeyCombo("ctrl+")
	if err == nil {
		t.Fatal("expected error for trailing +")
	}
	_, err = ParseKeyCombo("ctrl+shift")
	if err == nil {
		t.Fatal("expected error for modifiers-only combo")
	}
	_, err = ParseKeyCombo("ctrl+unknownkey")
	if err == nil {
		t.Fatal("expected error for unknown key name")
	}
}

func TestKeyCombo_DisplayString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ctrl+g", "Ctrl + G"},
		{"ctrl+shift+e", "Ctrl + Shift + E"},
		{"f8", "F8"},
		{"alt+f4", "Alt + F4"},
	}
	for _, tt := range tests {
		kc, err := ParseKeyCombo(tt.input)
		if err != nil {
			t.Fatalf("parse %q: %v", tt.input, err)
		}
		got := kc.DisplayString()
		if got != tt.expected {
			t.Errorf("DisplayString(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestKeyCombo_String(t *testing.T) {
	kc, _ := ParseKeyCombo("ctrl+shift+g")
	got := kc.String()
	if got != "ctrl+shift+g" {
		t.Errorf("String() = %q, want %q", got, "ctrl+shift+g")
	}
}
