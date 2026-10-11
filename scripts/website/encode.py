"""Turns the raw captures in scripts/website/out/ into the files keylint.io uses.

Run after the captures (see capture.spec.ts for the full command):
- stills (fix, providers) become WebP,
- each demo clip becomes a WebM (VP8) with the recorded frame timing, plus a
  WebP poster of its last frame (also what reduced-motion visitors see).

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


def resized(im: Image.Image, width: int) -> Image.Image:
    im = im.convert("RGB")
    return im.resize((width, round(im.height * width / im.width)), Image.LANCZOS)


def still(name: str) -> None:
    im = resized(Image.open(OUT / f"{name}.png"), STILL_WIDTH)
    dest = IMG / f"{name}.webp"
    im.save(dest, "WEBP", quality=88, method=6)
    print(f"{dest.name}: {im.width}x{im.height}, {dest.stat().st_size // 1024} KB")


def playwright_ffmpeg() -> str:
    root = os.environ.get("PLAYWRIGHT_BROWSERS_PATH") or os.path.expanduser("~/.cache/ms-playwright")
    found = sorted(glob.glob(os.path.join(root, "ffmpeg-*", "ffmpeg-*")))
    if not found:
        sys.exit(f"no Playwright ffmpeg under {root}: run `npx playwright install ffmpeg` in frontend/")
    return found[-1]


def clip(name: str) -> None:
    data = json.loads((OUT / f"{name}-frames" / "frames.json").read_text())
    frames, end = data["frames"], data["end"]
    start = frames[0]["t"]
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
    last = resized(Image.open(frames[-1]["file"]), CLIP_WIDTH)
    last.save(poster, "WEBP", quality=85, method=6)
    print(f"{dest.name}: {last.width}x{last.height}, {total:.1f} s, {dest.stat().st_size // 1024} KB; "
          f"{poster.name}: {poster.stat().st_size // 1024} KB")


def main() -> int:
    IMG.mkdir(parents=True, exist_ok=True)
    for name in ("fix", "providers"):
        still(name)
    for name in ("demo-fix", "demo-pyramidize"):
        clip(name)
    return 0


if __name__ == "__main__":
    sys.exit(main())
