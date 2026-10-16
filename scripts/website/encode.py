"""Turns the raw captures in scripts/website/out/ into the files keylint.io uses.

Run by capture.sh after the captures (`encode.py [name...]`, all by default):
- stills (fix, providers, pyramidize) become WebP, and the hero's peeks
  (CROPS) are cut from them,
- each demo clip becomes a WebM (VP8) with the recorded frame timing, plus a
  WebP poster (also what reduced-motion visitors see): the frame at the scene's
  "poster" mark, else its last frame. A "start" mark trims the clip to begin
  START_LEAD seconds before it.

Needs Pillow and the ffmpeg build that Playwright installs next to its
browsers (it can encode VP8 from piped JPEG frames, which is all this needs).
"""
import glob
import io
import json
import os
import pathlib
import subprocess
import sys

from PIL import Image

HERE = pathlib.Path(__file__).resolve().parent
OUT = HERE / "out"
IMG = HERE.parent.parent / "website" / "img"

STILL_WIDTH = 1600   # ~2x the page's 852px content column
CLIP_WIDTH = 1600    # same; the clips are recorded at 2560x1600
FPS = 25
START_LEAD = 0.8     # seconds of the scene kept before its "start" mark


def resized(im: Image.Image, width: int) -> Image.Image:
    im = im.convert("RGB")
    return im.resize((width, round(im.height * width / im.width)), Image.LANCZOS)


def still(name: str) -> None:
    im = resized(Image.open(OUT / f"{name}.png"), STILL_WIDTH)
    dest = IMG / f"{name}.webp"
    im.save(dest, "WEBP", quality=88, method=6)
    print(f"{dest.name}: {im.width}x{im.height}, {dest.stat().st_size // 1024} KB")


def crop(name: str) -> None:
    source, (x0, y0, x1, y1) = CROPS[name]
    im = Image.open(OUT / f"{source}.png").convert("RGB")
    k = im.width / 1264  # the capture's device scale factor
    part = im.crop(tuple(round(v * k) for v in (x0, y0, x1, y1)))
    dest = IMG / f"{name}.webp"
    part.save(dest, "WEBP", quality=86, method=6)
    print(f"{dest.name}: {part.width}x{part.height} (shown at {x1 - x0}x{y1 - y0}), {dest.stat().st_size // 1024} KB")


def playwright_ffmpeg() -> str:
    root = os.environ.get("PLAYWRIGHT_BROWSERS_PATH") or os.path.expanduser("~/.cache/ms-playwright")
    found = sorted(glob.glob(os.path.join(root, "ffmpeg-*", "ffmpeg-*")))
    if not found:
        sys.exit(f"no Playwright ffmpeg under {root}: run `npx playwright install ffmpeg` in frontend/")
    return found[-1]


def clip(name: str) -> None:
    data = json.loads((OUT / f"{name}-frames" / "frames.json").read_text())
    frames, end, marks = data["frames"], data["end"], data.get("marks", {})
    start = frames[0]["t"]
    if "start" in marks:
        start = max(start, marks["start"] - START_LEAD)
    # The screencast sends a frame only when something changed; the end time
    # holds the last one. Guard against a clock that does not match.
    if not 0 <= end - frames[-1]["t"] < 30:
        end = frames[-1]["t"] + 2
    total = end - start
    count = round(total * FPS)

    dest = IMG / f"{name}.webm"
    cmd = [
        playwright_ffmpeg(), "-y", "-loglevel", "error",
        "-f", "image2pipe", "-framerate", str(FPS), "-c:v", "mjpeg", "-i", "pipe:0",
        "-vf", "format=yuv420p",
        "-c:v", "libvpx", "-crf", "6", "-b:v", "1800k", "-qmin", "0", "-qmax", "36",
        "-deadline", "good", "-cpu-used", "0", "-auto-alt-ref", "1", "-lag-in-frames", "16",
        "-g", str(FPS * 10), "-an", str(dest),
    ]
    proc = subprocess.Popen(cmd, stdin=subprocess.PIPE)
    current, cached, jpeg = -1, None, b""
    for n in range(count):
        t = start + n / FPS
        while current + 1 < len(frames) and frames[current + 1]["t"] <= t:
            current += 1
        if current != cached:
            buf = io.BytesIO()
            resized(Image.open(frames[max(current, 0)]["file"]), CLIP_WIDTH).save(buf, "JPEG", quality=95, subsampling=0)
            jpeg, cached = buf.getvalue(), current
        proc.stdin.write(jpeg)
    proc.stdin.close()
    if proc.wait() != 0:
        sys.exit(f"ffmpeg failed for {name}")

    poster = IMG / f"{name}.webp"
    at = marks.get("poster")
    pick = frames[-1] if at is None else frames[max(i for i, f in enumerate(frames) if f["t"] <= at)]
    last = resized(Image.open(pick["file"]), CLIP_WIDTH)
    last.save(poster, "WEBP", quality=85, method=6)
    print(f"{dest.name}: {last.width}x{last.height}, {total:.1f} s, {dest.stat().st_size // 1024} KB; "
          f"{poster.name}: {poster.stat().st_size // 1024} KB")


STILLS = ("fix", "providers", "pyramidize")
# Readable details of the stills for the hero's peeks, at the app's own size:
# name -> (still, box in CSS pixels of the 1264x740 window). Kept at the
# capture's 2x, so they are sharp where the page shows them at 1x.
CROPS = {
    "peek-fix": ("fix", (736, 90, 1220, 300)),
    "peek-pyramidize": ("pyramidize", (245, 134, 770, 470)),
}
CLIPS = ("demo-fix", "demo-pyramidize")


def main(names: list[str]) -> int:
    """Encodes the named captures, or all of them when none is named."""
    IMG.mkdir(parents=True, exist_ok=True)
    for name in names or [*STILLS, *CLIPS]:
        if name in STILLS:
            still(name)
            for peek, (source, _) in CROPS.items():
                if source == name:
                    crop(peek)
        elif name in CLIPS:
            clip(name)
        else:
            sys.exit(f"unknown capture: {name} (stills: {', '.join(STILLS)}; clips: {', '.join(CLIPS)})")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
