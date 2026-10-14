#!/usr/bin/env bash
# Re-records the screenshots and demo clips of keylint.io (website/img/).
#
#   scripts/website/capture.sh                       # everything
#   scripts/website/capture.sh stills fix-clip       # only these
#
# Targets:
#   stills           Fix page and Settings › AI providers (capture.spec.ts)
#   fix-clip         Silent Fix in a chat app (demos.spec.ts)
#   pyramidize-clip  Pyramidize from a webmail client (demos.spec.ts)
#
# Steps: npm ci when the lock file changed, a production build of the
# frontend, the Playwright captures against it, then encode.py, which writes
# the WebP/WebM files into website/img/. Nothing here calls a model: the
# captures replay scripts/website/samples/ (refresh those first with
# refresh-samples.sh when a prompt or the sample text changed).
#
# First run only: a Python venv with Pillow in scripts/website/.venv
# (gitignored) and Playwright's Chromium and ffmpeg.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
FRONTEND="$ROOT/frontend"

targets=("$@")
if [ ${#targets[@]} -eq 0 ]; then
  targets=(stills fix-clip pyramidize-clip)
fi
# Test titles start with "still:" or "clip: <name>", so one --grep picks them.
greps=() encode=()
for t in "${targets[@]}"; do
  case "$t" in
    stills)          greps+=("still: "); encode+=(fix providers) ;;
    fix-clip)        greps+=("clip: Silent Fix"); encode+=(demo-fix) ;;
    pyramidize-clip) greps+=("clip: Pyramidize"); encode+=(demo-pyramidize) ;;
    *) echo "unknown target: $t (stills, fix-clip, pyramidize-clip)" >&2; exit 3 ;;
  esac
done

# Dependencies: only when package-lock.json is newer than the installed tree.
if [ ! -f "$FRONTEND/node_modules/.package-lock.json" ] \
  || [ "$FRONTEND/package-lock.json" -nt "$FRONTEND/node_modules/.package-lock.json" ]; then
  (cd "$FRONTEND" && npm ci --no-audit --no-fund)
fi
(cd "$FRONTEND" && npx playwright install chromium ffmpeg >/dev/null)

VENV="$HERE/.venv"
if [ ! -x "$VENV/bin/python" ]; then
  python3 -m venv "$VENV"
  "$VENV/bin/pip" install --quiet pillow
fi

(cd "$FRONTEND" && npx ng build)

(cd "$ROOT" && NODE_PATH="$FRONTEND/node_modules" "$FRONTEND/node_modules/.bin/playwright" test \
  -c scripts/website --grep "$(IFS='|'; echo "${greps[*]}")")

"$VENV/bin/python" "$HERE/encode.py" "${encode[@]}"
echo
echo "Done. Look at website/img/ (git diff --stat website/img) before committing."
