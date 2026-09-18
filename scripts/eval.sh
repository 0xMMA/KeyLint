#!/usr/bin/env bash
set -euo pipefail

# Automated evaluation runner for pyramidize quality.
# Wraps `go test -tags eval` and prints a summary.
#
# Usage:
#   ./scripts/eval.sh                          # one run of the pyramidize suite
#   ./scripts/eval.sh --suite fix --runs 3     # the silent grammar fix instead
#   EVAL_PROVIDER=claude EVAL_MODEL=claude-sonnet-4-6 ./scripts/eval.sh
#   ./scripts/eval.sh --provider openai --model gpt-4o
#   ./scripts/eval.sh --variant 1              # run with prompt variant v1
#   ./scripts/eval.sh --schema                 # enforce the JSON schemas (off by default, see schemas.go)
#   ./scripts/eval.sh --runs 3                 # run n times and write an aggregate baseline.json
#   ./scripts/eval.sh --runs 3 --compare path/to/baseline.json
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

# Load credentials from .env, and only credentials. Sourcing the whole file
# would let it set EVAL_PROVIDER or EVAL_MODEL behind the flags parsed below —
# the same trap the Go side had, in the opposite direction.
if [[ -f .env ]]; then
    while IFS='=' read -r key value; do
        [[ "$key" == *_API_KEY ]] || continue
        export "$key=${value%$'\r'}"
    done < <(grep -E '^[A-Za-z_][A-Za-z0-9_]*=' .env)
fi

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
        *)          echo "Unknown flag: $1" >&2; exit 1 ;;
    esac
done

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
command -v jq >/dev/null || { echo "jq is required" >&2; exit 3; }

case "$SUITE" in
    pyramidize) SUITE_PKG=./internal/features/pyramidize/ ;;
    fix)        SUITE_PKG=./internal/features/enhance/ ;;
    *)          echo "--suite takes 'pyramidize' or 'fix'" >&2; exit 3 ;;
esac

# The Fix samples are split into a tuning half and a held-out half. Default is
# all fifteen, which is what a baseline should measure; --split narrows a run to
# one half. The half is recorded in summary.json and carried into the configKey,
# so --compare refuses to read a tune-half number against an all-samples one.
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
fi

echo "=== KeyLint Eval: $SUITE ==="
echo "Provider: ${EVAL_PROVIDER:-<eval default: claude>}"
echo "Model:    ${EVAL_MODEL:-<provider default>}"
if [[ "$SUITE" != "fix" ]]; then
    echo "Variant:  ${EVAL_VARIANT:-0 (latest)}"
fi
if [[ "$SUITE" == "fix" ]]; then
    echo "Split:    ${EVAL_SPLIT:-all}"
fi
echo "Judge:    ${EVAL_JUDGE_PROVIDER:-claude} / ${EVAL_JUDGE_MODEL:-<pinned>} @ temp 0"
echo "Runs:     $RUNS"
echo ""

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
    go test -tags eval "$SUITE_PKG" -v -timeout 900s 2>&1 | tee /dev/stderr | tail -1
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
