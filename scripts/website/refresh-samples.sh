#!/usr/bin/env bash
# Records what KeyLint really returns for the website's sample texts.
#
#   scripts/website/refresh-samples.sh            # every sample
#   scripts/website/refresh-samples.sh chat fix   # only these
#
# Samples live in scripts/website/samples/: <name>.in.txt is what the demo
# types or selects, and this script writes <name>.out.txt (Fix) or
# <name>.out.md (Pyramidize) with the real answer, plus a line in
# recorded.txt saying when and from which commit. The captures only replay
# these files, so the site never shows an output KeyLint did not produce.
#
# Each sample is ONE real model call through the installed Claude Code CLI on
# the signed-in subscription (chat and fix: Fix, pyramidize: Pyramidize as
# email). The CLI runs against a throwaway settings file that sets only the
# provider, so ~/.config/KeyLint cannot change the answer and is not touched.
#
# Kinds: fix and chat are Fix samples, pyramidize is a Pyramidize one. A new
# sample needs its <name>.in.txt and a line in KIND below.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
SAMPLES="$HERE/samples"

declare -A KIND=([fix]=fix [chat]=fix [pyramidize]=pyramidize)

names=("$@")
if [ ${#names[@]} -eq 0 ]; then
  names=(fix chat pyramidize)
fi
for name in "${names[@]}"; do
  if [ -z "${KIND[$name]:-}" ]; then
    echo "unknown sample: $name (known: ${!KIND[*]})" >&2
    exit 3
  fi
  if [ ! -f "$SAMPLES/$name.in.txt" ]; then
    echo "missing $SAMPLES/$name.in.txt" >&2
    exit 3
  fi
done

if ! claude auth status 2>/dev/null | grep -q '"loggedIn": *true'; then
  echo "the Claude Code CLI is not signed in: run \`claude\` once and sign in" >&2
  exit 3
fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# The Go binary embeds the frontend build; build it when it is missing.
if [ ! -f "$ROOT/frontend/dist/browser/index.html" ]; then
  (cd "$ROOT/frontend" && npx ng build)
fi
echo "building KeyLint…"
(cd "$ROOT" && go build -tags gtk3 -o "$TMP/KeyLint" .)

mkdir -p "$TMP/config/KeyLint"
printf '{"active_provider":"claude-code"}\n' > "$TMP/config/KeyLint/settings.json"
# "-dirty" only when uncommitted changes can reach the answer (Go code, prompts).
COMMIT="$(cd "$ROOT" && git rev-parse --short HEAD)$(cd "$ROOT" && git diff --quiet HEAD -- internal main.go go.mod go.sum || echo -dirty)"
TODAY="$(date +%F)"

for name in "${names[@]}"; do
  in="$SAMPLES/$name.in.txt"
  case "${KIND[$name]}" in
    fix)
      out="$SAMPLES/$name.out.txt"
      echo "$name: Fix…"
      XDG_CONFIG_HOME="$TMP/config" "$TMP/KeyLint" -fix -f "$in" > "$TMP/out"
      ;;
    pyramidize)
      out="$SAMPLES/$name.out.md"
      echo "$name: Pyramidize as email…"
      XDG_CONFIG_HOME="$TMP/config" "$TMP/KeyLint" -pyramidize -type email -f "$in" > "$TMP/out"
      ;;
  esac
  if [ ! -s "$TMP/out" ]; then
    echo "$name: KeyLint returned nothing; $out is unchanged" >&2
    exit 1
  fi
  mv "$TMP/out" "$out"
  # One line per sample: replace this sample's line, keep the others.
  touch "$SAMPLES/recorded.txt"
  grep -v "^$name " "$SAMPLES/recorded.txt" > "$TMP/recorded" || true
  printf '%s %s KeyLint %s, claude-code, %s\n' "$name" "$TODAY" "$COMMIT" "${KIND[$name]}" >> "$TMP/recorded"
  sort "$TMP/recorded" > "$SAMPLES/recorded.txt"
  echo "$name: wrote ${out#"$ROOT/"}"
done

echo
echo "Review the outputs (git diff scripts/website/samples), then run scripts/website/capture.sh."
