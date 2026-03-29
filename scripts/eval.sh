#!/usr/bin/env bash
set -euo pipefail

# Automated evaluation runner for pyramidize quality.
# Wraps `go test -tags eval` and prints a summary.
#
# Usage:
#   ./scripts/eval.sh                          # use configured provider
#   EVAL_PROVIDER=claude EVAL_MODEL=claude-sonnet-4-6 ./scripts/eval.sh
#   ./scripts/eval.sh --provider openai --model gpt-4o

cd "$(git rev-parse --show-toplevel)"

# Parse optional --provider / --model flags.
while [[ $# -gt 0 ]]; do
    case "$1" in
        --provider) export EVAL_PROVIDER="$2"; shift 2 ;;
        --model)    export EVAL_MODEL="$2"; shift 2 ;;
        *)          echo "Unknown flag: $1" >&2; exit 1 ;;
    esac
done

echo "=== KeyLint Pyramidize Eval ==="
echo "Provider: ${EVAL_PROVIDER:-<from settings>}"
echo "Model:    ${EVAL_MODEL:-<provider default>}"
echo ""

go test -tags eval ./internal/features/pyramidize/ -v -timeout 600s 2>&1 | tee /dev/stderr | tail -1

# Find the newest eval-run directory and print its summary.
LATEST=$(ls -td test-data/eval-runs/*/ 2>/dev/null | head -1)
if [[ -n "$LATEST" ]]; then
    echo ""
    echo "=== Results saved to: $LATEST ==="
    if [[ -f "${LATEST}summary.json" ]]; then
        cat "${LATEST}summary.json"
    fi
fi
