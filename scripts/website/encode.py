"""Turns the raw captures in scripts/website/out/ into the images keylint.io uses.

Run after capture.spec.ts (see its header for the full command). Needs only
Pillow: stills become WebP, the Pyramidize screencast an animated WebP whose
frame durations are the recorded ones.
"""
import json
import pathlib
import sys

from PIL import Image

HERE = pathlib.Path(__file__).resolve().parent
OUT = HERE / "out"
IMG = HERE.parent.parent / "website" / "img"

STILL_WIDTH = 1600     # ~2x the page's 852px content column
ANIM_WIDTH = 1600      # same as the stills; still well under 2 MB
MIN_FRAME_MS = 70      # cap at ~14 fps: closer frames are merged
FINAL_HOLD_MS = 3200   # the screencast stops sending once nothing changes


def resized(im: Image.Image, width: int) -> Image.Image:
    im = im.convert("RGB")
    return im.resize((width, round(im.height * width / im.width)), Image.LANCZOS)


def still(name: str) -> None:
    im = resized(Image.open(OUT / f"{name}.png"), STILL_WIDTH)
    dest = IMG / f"{name}.webp"
    im.save(dest, "WEBP", quality=88, method=6)
    print(f"{dest.name}: {im.width}x{im.height}, {dest.stat().st_size // 1024} KB")


def animation(name: str) -> None:
    frames = json.loads((OUT / f"{name}-frames" / "frames.json").read_text())
    images, durations = [], []
    for i, f in enumerate(frames):
        nxt = frames[i + 1]["t"] if i + 1 < len(frames) else None
        ms = round((nxt - f["t"]) * 1000) if nxt is not None else FINAL_HOLD_MS
        if images and durations[-1] < MIN_FRAME_MS:
            # Too close to the previous frame: show this one in its place.
            images[-1] = f["file"]
            durations[-1] += ms
        else:
            images.append(f["file"])
            durations.append(ms)
    pics = [resized(Image.open(p), ANIM_WIDTH) for p in images]
    dest = IMG / f"{name}.webp"
    pics[0].save(dest, "WEBP", save_all=True, append_images=pics[1:], duration=durations,
                 loop=0, quality=80, method=6, minimize_size=True)
    total = sum(durations) / 1000
    print(f"{dest.name}: {pics[0].width}x{pics[0].height}, {len(pics)} frames, {total:.1f} s, "
          f"{dest.stat().st_size // 1024} KB")


def main() -> int:
    IMG.mkdir(parents=True, exist_ok=True)
    for name in ("fix", "pyramidize", "providers"):
        still(name)
    animation("pyramidize-demo")
    return 0


if __name__ == "__main__":
    sys.exit(main())
