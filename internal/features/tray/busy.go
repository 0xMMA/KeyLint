package tray

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
)

// busyIconSize is the edge of the generated icon. The tray draws at 16–32 px,
// so the 1024 px app icon is scaled down here once rather than carried around.
const busyIconSize = 64

// busyDot is the colour of the "working" mark: amber, which reads as "in
// progress" on both a light and a dark taskbar, with a dark ring so it does
// not melt into a light one.
var (
	busyDot     = color.NRGBA{R: 0xF5, G: 0xA6, B: 0x23, A: 0xFF}
	busyDotRing = color.NRGBA{R: 0x1B, G: 0x26, B: 0x36, A: 0xFF}
)

// busyIcon returns the app icon with a dot in its lower-right corner, as PNG.
// The dot is about a third of the icon wide, so it is still a visible mark at
// the 16 px a tray icon is drawn at.
func busyIcon(appIcon []byte) ([]byte, error) {
	src, err := png.Decode(bytes.NewReader(appIcon))
	if err != nil {
		return nil, fmt.Errorf("decoding the app icon: %w", err)
	}
	dst := image.NewNRGBA(image.Rect(0, 0, busyIconSize, busyIconSize))
	scaleInto(dst, src)

	const (
		radius = busyIconSize * 17 / 100
		ring   = busyIconSize * 5 / 100
	)
	cx, cy := busyIconSize-radius-ring, busyIconSize-radius-ring
	fillCircle(dst, cx, cy, radius+ring, busyDotRing)
	fillCircle(dst, cx, cy, radius, busyDot)

	var out bytes.Buffer
	if err := png.Encode(&out, dst); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// scaleInto box-filters src into dst: each destination pixel averages the
// source pixels it covers, which keeps a 1024 → 64 reduction smooth without
// pulling in an imaging library for one icon.
func scaleInto(dst *image.NRGBA, src image.Image) {
	sb := src.Bounds()
	db := dst.Bounds()
	for y := 0; y < db.Dy(); y++ {
		y0 := sb.Min.Y + y*sb.Dy()/db.Dy()
		y1 := max(y0+1, sb.Min.Y+(y+1)*sb.Dy()/db.Dy())
		for x := 0; x < db.Dx(); x++ {
			x0 := sb.Min.X + x*sb.Dx()/db.Dx()
			x1 := max(x0+1, sb.Min.X+(x+1)*sb.Dx()/db.Dx())
			var r, g, b, a, n uint64
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					// RGBA() is alpha-premultiplied, so averaging it is correct
					// at transparent edges.
					pr, pg, pb, pa := src.At(sx, sy).RGBA()
					r, g, b, a = r+uint64(pr), g+uint64(pg), b+uint64(pb), a+uint64(pa)
					n++
				}
			}
			avg := color.RGBA64{R: uint16(r / n), G: uint16(g / n), B: uint16(b / n), A: uint16(a / n)}
			dst.Set(db.Min.X+x, db.Min.Y+y, avg)
		}
	}
}

func fillCircle(img draw.Image, cx, cy, r int, c color.Color) {
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			dx, dy := x-cx, y-cy
			if dx*dx+dy*dy <= r*r {
				img.Set(x, y, c)
			}
		}
	}
}
