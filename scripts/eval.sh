#!/usr/bin/env bash
set -euo pipefail

# Automated evaluation runner for pyramidize quality.
# Wraps `go test -tags eval` and prints a summary.
#
# By default the pipeline AND the judge run through the installed Claude Code
# CLI (provider claude-code) on the signed-in subscription: no API key is read,
# and none is needed. --provider claude (and EVAL_JUDGE_PROVIDER=claude for the
# judge) measure against the API instead, with ANTHROPIC_API_KEY from .env.
#
# Usage:
#   ./scripts/eval.sh                          # one run of the pyramidize suite
#   ./scripts/eval.sh --suite fix --runs 3     # the silent grammar fix instead
#   ./scripts/eval.sh --suite fix --split tune --runs 3   # the 10 tuning samples
#   ./scripts/eval.sh --suite fix --split holdout --runs 3  # the 5 held back, once
#   ./scripts/eval.sh --provider claude --model claude-sonnet-4-6   # the API, needs a key
#   EVAL_JUDGE_PROVIDER=claude ./scripts/eval.sh --provider claude  # API judge too
#   ./scripts/eval.sh --provider openai --model gpt-4o
#   ./scripts/eval.sh --variant 1              # run with prompt variant v1
#   ./scripts/eval.sh --schema                 # enforce the JSON schemas (off by default, see schemas.go)
#   ./scripts/eval.sh --runs 3                 # run n times and write an aggregate baseline.json
#   ./scripts/eval.sh --runs 3 --compare path/to/baseline.json
#   ./scripts/eval.sh --suite fix --thinking off --runs 3   # Fix through the CLI without thinking
#   ./scripts/eval.sh --dry-run                # print the resolved configuration, run nothing
#
# --compare wants --runs on both sides. A single run has no range, so it gets an
# "indicative only" answer rather than a verdict: believing a one-run delta is
# the mistake this exists to stop.
#
# Why --runs exists: one run is a sample, not a measurement. The same commit and
# the same model produce different numbers from one hour to the next, so a
# single-run delta cannot tell a real change from provider noise. Three runs give
# a spread, and the spread is what makes a later comparison mean anything.

cd "$(git rev-parse --show-toplevel)"
command -v jq >/dev/null || { echo "jq is required" >&2; exit 3; }

RUNS=1
COMPARE=""
WRITE_BASELINE=0
# Which suite to run. pyramidize is the default because it is the older one and
# every recorded baseline belongs to it; --suite fix measures the silent grammar
# fix instead. They write the same summary.json shape, so eval-aggregate.sh
# reads either — but a baseline from one is not comparable with the other, and
# the configKey carries the model and variant that say so.
SUITE=pyramidize
VARIANT_SET=0
SCHEMA_SET=0
SPLIT_SET=0
THINKING_SET=0
DRY_RUN=0

while [[ $# -gt 0 ]]; do
    case "$1" in
        --provider) export EVAL_PROVIDER="$2"; shift 2 ;;
        --model)    export EVAL_MODEL="$2"; shift 2 ;;
        --variant)  export EVAL_VARIANT="$2"; VARIANT_SET=1; shift 2 ;;
        --schema)   export KEYLINT_PYRAMIDIZE_SCHEMA=1; SCHEMA_SET=1; shift ;;
        --runs)     RUNS="$2"; WRITE_BASELINE=1; shift 2 ;;
        --suite)    SUITE="$2"; shift 2 ;;
        --split)    export EVAL_SPLIT="$2"; SPLIT_SET=1; shift 2 ;;
        --compare)  COMPARE="$2"; shift 2 ;;
        --thinking) export EVAL_THINKING="$2"; THINKING_SET=1; shift 2 ;;
        --dry-run)  DRY_RUN=1; shift ;;
        *)          echo "Unknown flag: $1" >&2; exit 1 ;;
    esac
done

# The providers this run will use, resolved the way the Go side resolves them:
# the command line or shell first, then a non-credential line in .env (the Go
# side fills an empty EVAL_* from it), then the defaults — kept in step with
# defaultEvalProvider and defaultJudgeProvider in both eval_config.go files.
#
# .env is parsed the way godotenv (the Go side) reads it: comments and blank
# lines skipped, an optional `export ` prefix, single or double quotes around
# the value, an inline ` # comment` after an unquoted one. Every function here
# returns 0 whatever it finds — under `set -e` a lookup that finds nothing must
# not end the script, least of all with exit 1, which means "regression".
dotenv_entries() {
    [[ -f .env ]] || return 0
    local line key value
    while IFS= read -r line || [[ -n "$line" ]]; do
        line="${line%$'\r'}"
        line="${line#"${line%%[![:space:]]*}"}"
        [[ -z "$line" || "$line" == \#* ]] && continue
        if [[ "$line" =~ ^export[[:space:]]+(.*)$ ]]; then
            line="${BASH_REMATCH[1]}"
        fi
        [[ "$line" =~ ^([A-Za-z_][A-Za-z0-9_.]*)[[:space:]]*=[[:space:]]*(.*)$ ]] || continue
        key="${BASH_REMATCH[1]}"
        value="${BASH_REMATCH[2]}"
        if [[ "$value" =~ ^\"(.*)\"[[:space:]]*(#.*)?$ ]]; then
            value="${BASH_REMATCH[1]}"
        elif [[ "$value" =~ ^\'(.*)\'[[:space:]]*(#.*)?$ ]]; then
            value="${BASH_REMATCH[1]}"
        else
            value="${value%%[[:space:]]#*}"
            value="${value%"${value##*[![:space:]]}"}"
        fi
        printf '%s\t%s\n' "$key" "$value"
    done < .env
    return 0
}
dotenv_value() {
    dotenv_entries | awk -F'\t' -v k="$1" '$1 == k { v = $2 } END { print v }'
}
PIPELINE_PROVIDER="${EVAL_PROVIDER:-$(dotenv_value EVAL_PROVIDER)}"
PIPELINE_PROVIDER="${PIPELINE_PROVIDER:-claude-code}"
JUDGE_PROVIDER="${EVAL_JUDGE_PROVIDER:-$(dotenv_value EVAL_JUDGE_PROVIDER)}"
JUDGE_PROVIDER="${JUDGE_PROVIDER:-claude-code}"
uses_api_key() { [[ "$1" == "claude" || "$1" == "openai" ]]; }

# Load credentials from .env, and only credentials — and only when the run has
# an API provider in it. Sourcing the whole file would let it set EVAL_PROVIDER
# or EVAL_MODEL behind the flags parsed above; loading a key into a run that
# goes entirely through the CLI would put a credential next to a process that
# must not use one. (The CLI client strips it from the child anyway; a key that
# is never loaded cannot be the one that answered.) The Go side applies the
# same rule on its own reading of .env.
LOADED_KEYS=()
if uses_api_key "$PIPELINE_PROVIDER" || uses_api_key "$JUDGE_PROVIDER"; then
    while IFS=$'\t' read -r key value; do
        [[ "$key" == *_API_KEY ]] || continue
        # An empty line copied from .env.example must not blank a key the
        # shell exports.
        [[ -n "$value" ]] || continue
        export "$key=$value"
        LOADED_KEYS+=("$key")
    done < <(dotenv_entries)
fi

# A signed-out CLI fails every sample one by one; say so once, up front. Only
# when the binary is on PATH — KeyLint also finds it in other places, and the
# Go side reports a missing one per sample with the right wording.
if [[ "$PIPELINE_PROVIDER" == "claude-code" || "$JUDGE_PROVIDER" == "claude-code" ]] && command -v claude >/dev/null; then
    # The same variables internal/llm strips before every CLI call
    # (blockedCLIEnv): detection must see the account the eval will run as.
    if ! env -u ANTHROPIC_API_KEY -u ANTHROPIC_AUTH_TOKEN -u ANTHROPIC_BASE_URL \
            -u CLAUDE_CODE_OAUTH_TOKEN -u ANTHROPIC_MODEL \
            -u ANTHROPIC_DEFAULT_OPUS_MODEL -u ANTHROPIC_DEFAULT_SONNET_MODEL \
            -u ANTHROPIC_DEFAULT_HAIKU_MODEL -u ANTHROPIC_SMALL_FAST_MODEL \
            -u CLAUDE_CODE_USE_BEDROCK -u CLAUDE_CODE_USE_VERTEX -u AWS_BEARER_TOKEN_BEDROCK \
            -u MAX_THINKING_TOKENS -u CLAUDE_CODE_MAX_OUTPUT_TOKENS \
            claude auth status 2>/dev/null | jq -e '.loggedIn == true' >/dev/null 2>&1; then
        echo "The Claude Code CLI is not signed in. Run \`claude\` in a terminal and sign in, then retry." >&2
        exit 3
    fi
fi

if ! [[ "$RUNS" =~ ^[0-9]+$ ]] || (( RUNS < 1 )); then
    echo "--runs takes a positive integer" >&2
    exit 3
fi
if (( WRITE_BASELINE )) && (( RUNS < 2 )); then
    # A one-run baseline records a spread of zero, and every later comparison
    # against it then reads a rounding difference as a regression.
    echo "--runs 1 cannot produce a baseline: one run has no spread. Use --runs 3." >&2
    exit 3
fi
if [[ -n "$COMPARE" && ! -f "$COMPARE" ]]; then
    echo "No such baseline: $COMPARE" >&2
    exit 3
fi

case "$SUITE" in
    pyramidize) SUITE_PKG=./internal/features/pyramidize/ ;;
    fix)        SUITE_PKG=./internal/features/enhance/ ;;
    *)          echo "--suite takes 'pyramidize' or 'fix'" >&2; exit 3 ;;
esac

# The Fix samples are split into a tuning half and a held-out half. Default is
# all fifteen, which is what a baseline should measure; --split narrows a run to
# one half. The half is recorded in summary.json and carried into the configKey,
# so --compare refuses to read a tune-half number against an all-samples one.
# Only the flag decides the split. Without this the Go side would pick EVAL_SPLIT
# up out of .env — it fills any EVAL_* key whose environment value is empty —
# and the header below would print "all" while the run measured the holdout.
# Same trap the *_API_KEY filter above exists for, one variable further on.
# Exported rather than unset: an unset variable is exactly the "empty" the Go
# side fills from .env, so unsetting it let the file decide after all.
if (( ! SPLIT_SET )); then
    export EVAL_SPLIT=all
fi
# Thinking follows the same rule, for the same reason: only the flag decides.
if (( ! THINKING_SET )); then
    export EVAL_THINKING=default
fi
case "${EVAL_SPLIT:-all}" in
    all|tune|holdout) ;;
    *) echo "--split takes 'all', 'tune' or 'holdout'" >&2; exit 3 ;;
esac
if (( SPLIT_SET )) && [[ "$SUITE" != "fix" ]]; then
    echo "--split applies to the fix suite; the pyramidize samples are not split" >&2
    exit 3
fi

# Both of these configure the Pyramidize pipeline and nothing else. Accepting
# them under --suite fix printed "Variant: 2" in the header and measured variant
# 0 — a run that says it tested something it did not.
if [[ "$SUITE" == "fix" ]]; then
    if (( VARIANT_SET )); then
        echo "--variant applies to the pyramidize prompts; the Fix prompt has no variants" >&2
        exit 3
    fi
    if (( SCHEMA_SET )); then
        echo "--schema enforces the pyramidize JSON schemas; the Fix suite returns text" >&2
        exit 3
    fi
else
    # Pyramidize thinking is left as the provider sets it, on both routes.
    if (( THINKING_SET )); then
        echo "--thinking applies to the fix suite; Pyramidize thinking is left to the provider" >&2
        exit 3
    fi
fi
case "${EVAL_THINKING:-default}" in
    on|off|default) ;;
    *) echo "--thinking takes 'on', 'off' or 'default'" >&2; exit 3 ;;
esac

echo "=== KeyLint Eval: $SUITE ==="
echo "Provider: $PIPELINE_PROVIDER"
echo "Model:    ${EVAL_MODEL:-<provider default>} (the resolved ID is recorded in summary.json)"
if [[ "$SUITE" != "fix" ]]; then
    echo "Variant:  ${EVAL_VARIANT:-0 (latest)}"
fi
if [[ "$SUITE" == "fix" ]]; then
    echo "Split:    ${EVAL_SPLIT:-all}"
    echo "Thinking: ${EVAL_THINKING:-default} (CLI only; recorded in summary.json)"
fi
if uses_api_key "$JUDGE_PROVIDER" || [[ "$JUDGE_PROVIDER" == "ollama" ]]; then
    JUDGE_TEMP="temp 0"
else
    JUDGE_TEMP="temp unpinned (the CLI has no temperature flag)"
fi
echo "Judge:    $JUDGE_PROVIDER / ${EVAL_JUDGE_MODEL:-<pinned>} @ $JUDGE_TEMP"
echo "Runs:     $RUNS"
echo ""

if (( DRY_RUN )); then
    # Machine-readable, for scripts/eval.sh's own tests.
    echo "dry-run pipeline=$PIPELINE_PROVIDER judge=$JUDGE_PROVIDER keys=${LOADED_KEYS[*]:-none}"
    exit 0
fi

RUN_DIRS=()
for (( i = 1; i <= RUNS; i++ )); do
    if (( RUNS > 1 )); then
        echo "--- run $i of $RUNS ---"
    fi
    # Runs already paid for must not be thrown away by `set -e`. A sample that
    # errors makes the Go test fail, and aborting here would discard the money
    # already spent on the earlier runs.
    BEFORE=$(ls -d test-data/eval-runs/*/ 2>/dev/null | sort || true)
    set +e
    # 7200 s covers the worst case the per-call deadlines allow: Fix 15 ×
    # (120 s fix + 180 s judge) = 4500 s, Pyramidize 13 × (2 × 120 s + 180 s)
    # = 5460 s with a refine on every sample. A go test timeout panics before
    # summary.json is written and throws the whole run away — the first CLI
    # attempt at the Fix baseline lost a run to the old 900 s — so this is a
    # safety net, not a budget. Typical runs take 12–15 minutes.
    go test -tags eval "$SUITE_PKG" -v -timeout 7200s 2>&1 | tee /dev/stderr | tail -1
    run_status=${PIPESTATUS[0]}
    set -e
    AFTER=$(ls -d test-data/eval-runs/*/ 2>/dev/null | sort || true)

    # The directory this invocation created, rather than whatever is newest
    # globally — two eval.sh processes would otherwise swap runs.
    LATEST=$(comm -13 <(printf '%s\n' "$BEFORE") <(printf '%s\n' "$AFTER") | tail -1)

    if [[ -z "$LATEST" || ! -f "${LATEST}summary.json" ]]; then
        echo "Run $i produced no summary.json (go test exited $run_status)." >&2
        if (( ${#RUN_DIRS[@]} > 0 )); then
            echo "The runs before it are kept; aggregate them by hand with:" >&2
            echo "  ./scripts/eval-aggregate.sh ${RUN_DIRS[*]}" >&2
        fi
        exit 3
    fi
    if (( run_status != 0 )); then
        # Some samples failed but the run still produced scores. A failed sample
        # scores zero, which is the honest number; the run directory says which.
        echo "  ! run $i reported failures (go test exited $run_status); its scores are included" >&2
    fi
    RUN_DIRS+=("$LATEST")
    echo "  → ${LATEST}"
done

echo ""
echo "=== Results ==="
cat "${RUN_DIRS[-1]}summary.json"

# --- aggregate -------------------------------------------------------------

# Every number below is computed by eval-aggregate.sh, which can be run against
# recorded runs without spending an API call.
AGG_ARGS=()
[[ -n "$COMPARE" ]] && AGG_ARGS+=(--compare "$COMPARE")

if (( WRITE_BASELINE )); then
    # Aggregated into a temporary file first: eval-baselines/ is tracked, and a
    # failed aggregate would otherwise leave an empty file there that --compare
    # would happily read as a baseline.
    TMP_BASELINE=$(mktemp)
    trap 'rm -f "$TMP_BASELINE"' EXIT
    if ! ./scripts/eval-aggregate.sh "${RUN_DIRS[@]}" > "$TMP_BASELINE"; then
        echo "These runs did not aggregate into a baseline; they are kept under test-data/eval-runs/." >&2
        exit 3
    fi
    OUT_DIR="test-data/eval-baselines/$(date +%Y-%m-%dT%H-%M-%S)"
    mkdir -p "$OUT_DIR"
    mv "$TMP_BASELINE" "$OUT_DIR/baseline.json"
    trap - EXIT
    echo ""
    echo "=== Baseline written to $OUT_DIR/baseline.json ==="
    jq '{runCount, config, deterministic, judge, samplesPassing}' "$OUT_DIR/baseline.json"
fi

if [[ -n "$COMPARE" ]]; then
    echo ""
    echo "=== Compared against $COMPARE ==="
    # Its exit code is this script's: 1 for a regression, 2 for two sides that
    # are not comparable, 3 for a usage or input problem.
    ./scripts/eval-aggregate.sh "${AGG_ARGS[@]}" "${RUN_DIRS[@]}"
fi
