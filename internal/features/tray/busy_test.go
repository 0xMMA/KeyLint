package tray

import (
	"bytes"
	"image/png"
	"os"
	"testing"
)

// TestBusyIcon checks the working mark is there and the icon is still the
// icon: the corner carries the amber dot, the middle does not.
func TestBusyIcon(t *testing.T) {
	appIcon, err := os.ReadFile("../../../build/appicon.png")
	if err != nil {
		t.Fatalf("read app icon: %v", err)
	}
	out, err := busyIcon(appIcon)
	if err != nil {
		t.Fatalf("busyIcon: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("busy icon is not a PNG: %v", err)
	}
	if b := img.Bounds(); b.Dx() != busyIconSize || b.Dy() != busyIconSize {
		t.Fatalf("size = %v, want %dx%d", b, busyIconSize, busyIconSize)
	}

	const radius = busyIconSize * 17 / 100
	const ring = busyIconSize * 5 / 100
	c := busyIconSize - radius - ring
	r, g, b, a := img.At(c, c).RGBA()
	wr, wg, wb, wa := busyDot.RGBA()
	if r != wr || g != wg || b != wb || a != wa {
		t.Errorf("dot centre = %v, want the amber dot", img.At(c, c))
	}
	if img.At(busyIconSize/2, busyIconSize/2) == img.At(c, c) {
		t.Error("the middle of the icon is amber too — the app icon was painted over")
	}
}

func TestBusyIconRejectsNonPNG(t *testing.T) {
	if _, err := busyIcon([]byte("not a png")); err == nil {
		t.Error("expected an error for bytes that are not a PNG")
	}
}

// TestSetBusyBeforeSetup must not panic: the hotkey can fire before the tray
// exists.
func TestSetBusyBeforeSetup(t *testing.T) {
	s := NewService(nil, nil)
	s.SetBusy(true)
	s.SetBusy(false)
}
