#!/usr/bin/env bash
# Records what KeyLint really returns for the website's sample texts.
#
#   scripts/website/refresh-samples.sh            # every sample
#   scripts/website/refresh-samples.sh chat fix   # only these
#
# Samples live in scripts/website/samples/: <name>.in.txt is what the demo
# types or selects, and this script writes <name>.out.txt (Fix) or
# <name>.out.md (Pyramidize) with the real answer, plus a line in
# recorded.txt: when, from which commit, through which CLI version, and a
# hash of the input and the output. frontend/e2e/website.spec.ts checks the
# hashes, so an edited input or a hand-polished output fails CI until this
# script has recorded it again. The captures only replay these files.
#
# Each sample is ONE real model call through the installed Claude Code CLI on
# the signed-in subscription (chat and fix: Fix, pyramidize: Pyramidize as
# email). Record once and keep what comes back: re-rolling until an output
# looks good would make the site show a lucky answer, not a typical one.
#
# Isolation (Linux; the script is a dev tool): KeyLint reads a throwaway
# settings file that sets only the provider (XDG_CONFIG_HOME), so
# ~/.config/KeyLint can neither change the answer nor be touched, and
# KEYLINT_PYRAMIDIZE_SCHEMA is unset. KeyLint itself strips the CLI's
# credential and model variables before every call (internal/llm).
#
# Kinds: fix and chat are Fix samples, pyramidize is a Pyramidize one. A new
# sample needs its <name>.in.txt and an entry in KIND below.
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

# The sign-in as KeyLint's calls will see it: the same variables
# internal/llm strips (blockedCLIEnv) are unset here, and
# internal/llm/claudecode_evalsh_test.go fails when the lists drift.
if ! env -u ANTHROPIC_API_KEY -u ANTHROPIC_AUTH_TOKEN -u ANTHROPIC_BASE_URL \
        -u CLAUDE_CODE_OAUTH_TOKEN -u ANTHROPIC_MODEL \
        -u ANTHROPIC_DEFAULT_OPUS_MODEL -u ANTHROPIC_DEFAULT_SONNET_MODEL \
        -u ANTHROPIC_DEFAULT_HAIKU_MODEL -u ANTHROPIC_SMALL_FAST_MODEL \
        -u CLAUDE_CODE_USE_BEDROCK -u CLAUDE_CODE_USE_VERTEX -u AWS_BEARER_TOKEN_BEDROCK \
        -u MAX_THINKING_TOKENS -u CLAUDE_CODE_MAX_OUTPUT_TOKENS -u CLAUDE_CODE_DISABLE_AUTO_MEMORY \
        claude auth status 2>/dev/null | jq -e '.loggedIn == true' >/dev/null 2>&1; then
  echo "The Claude Code CLI is not signed in. Run \`claude\` in a terminal and sign in, then retry." >&2
  exit 3
fi
CLI_VERSION="$(claude --version 2>/dev/null | awk '{print $1}')"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# The Go binary embeds the frontend build; build it when it is missing.
FRONTEND="$ROOT/frontend"
if [ ! -f "$FRONTEND/dist/browser/index.html" ]; then
  if [ ! -f "$FRONTEND/node_modules/.package-lock.json" ] \
    || [ "$FRONTEND/package-lock.json" -nt "$FRONTEND/node_modules/.package-lock.json" ]; then
    (cd "$FRONTEND" && npm ci --no-audit --no-fund)
  fi
  (cd "$FRONTEND" && npx ng build)
fi
echo "building KeyLint…"
(cd "$ROOT" && go build -tags gtk3 -o "$TMP/KeyLint" .)

mkdir -p "$TMP/config/KeyLint"
printf '{"active_provider":"claude-code"}\n' > "$TMP/config/KeyLint/settings.json"
# "-dirty" when uncommitted or untracked changes can reach the answer: the Go
# code (prompts are Go) and its modules, tests aside. Nothing under internal/
# is embedded.
COMMIT="$(cd "$ROOT" && git rev-parse --short HEAD)"
if [ -n "$(cd "$ROOT" && git status --porcelain -- internal main.go go.mod go.sum ':!*_test.go')" ]; then
  COMMIT="$COMMIT-dirty"
fi
TODAY="$(date +%F)"

hash12() { sha256sum < "$1" | cut -c1-12; }

for name in "${names[@]}"; do
  in="$SAMPLES/$name.in.txt"
  case "${KIND[$name]}" in
    fix)
      out="$SAMPLES/$name.out.txt"
      echo "$name: Fix…"
      env -u KEYLINT_PYRAMIDIZE_SCHEMA XDG_CONFIG_HOME="$TMP/config" \
        "$TMP/KeyLint" -fix -f "$in" > "$TMP/out"
      ;;
    pyramidize)
      out="$SAMPLES/$name.out.md"
      echo "$name: Pyramidize as email…"
      env -u KEYLINT_PYRAMIDIZE_SCHEMA XDG_CONFIG_HOME="$TMP/config" \
        "$TMP/KeyLint" -pyramidize -type email -f "$in" > "$TMP/out"
      ;;
  esac
  # A failed call has exited above (set -e) and left the old output and its
  # line alone; from here on both are replaced together.
  mv "$TMP/out" "$out"
  touch "$SAMPLES/recorded.txt"
  grep -v "^$name " "$SAMPLES/recorded.txt" > "$TMP/recorded" || true
  printf '%s %s KeyLint %s, claude-code %s, %s, in=%s out=%s\n' \
    "$name" "$TODAY" "$COMMIT" "$CLI_VERSION" "${KIND[$name]}" "$(hash12 "$in")" "$(hash12 "$out")" >> "$TMP/recorded"
  LC_ALL=C sort "$TMP/recorded" > "$SAMPLES/recorded.txt"
  echo "$name: wrote ${out#"$ROOT/"}"
done

echo
echo "Review the outputs (git diff scripts/website/samples), then run scripts/website/capture.sh."
