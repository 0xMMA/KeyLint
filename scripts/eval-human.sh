#!/usr/bin/env bash
set -euo pipefail

# Interactive human evaluation for pyramidize quality.
# Runs each test-data sample through the CLI, shows a 3-pane comparison,
# and collects human scores.
#
# Usage:
#   ./scripts/eval-human.sh
#   ./scripts/eval-human.sh --provider claude --model claude-sonnet-4-6

cd "$(git rev-parse --show-toplevel)"

PROVIDER=""
MODEL=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --provider) PROVIDER="$2"; shift 2 ;;
        --model)    MODEL="$2"; shift 2 ;;
        *)          echo "Unknown flag: $1" >&2; exit 1 ;;
    esac
done

# Build the binary first.
echo "Building KeyLint..."
go build -o bin/KeyLint .

TIMESTAMP=$(date +"%Y-%m-%dT%H-%M-%S")
RUN_DIR="test-data/eval-runs/${TIMESTAMP}"
SAMPLES_DIR="${RUN_DIR}/samples"
mkdir -p "$SAMPLES_DIR"

RESULTS_FILE="${RUN_DIR}/results.jsonl"
TESTDATA_DIR="test-data/pyramidal-emails"

CLI_FLAGS=""
[[ -n "$PROVIDER" ]] && CLI_FLAGS="$CLI_FLAGS --provider $PROVIDER"
[[ -n "$MODEL" ]]    && CLI_FLAGS="$CLI_FLAGS --model $MODEL"

TOTAL=0
SCORED=0
SCORE_SUM=0

for file in "$TESTDATA_DIR"/*.md; do
    TOTAL=$((TOTAL + 1))
    NAME=$(basename "$file" .md)

    # Parse raw input and baseline from the test-data file.
    RAW=$(sed -n '/^# Raw Input$/,/^# User accepted output$/{ /^# /d; p; }' "$file" | sed '/^```/d')
    BASELINE=$(sed -n '/^# User accepted output$/,$ { /^# /d; p; }' "$file" | sed '/^```/d')

    # Run pyramidize CLI.
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "  Sample ${TOTAL}: ${NAME}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

    # Write just the raw input to a temp file (strip markdown markers).
    TMPFILE=$(mktemp)
    echo "$RAW" > "$TMPFILE"
    OUTPUT=$(./bin/KeyLint -pyramidize -type email $CLI_FLAGS -f "$TMPFILE" 2>/dev/null || echo "[ERROR: pyramidize failed]")
    rm -f "$TMPFILE"

    # Save the generated output.
    echo "$OUTPUT" > "${SAMPLES_DIR}/${NAME}.md"

    # Display 3-pane comparison.
    echo ""
    echo "┌─── RAW INPUT (first 20 lines) ───"
    echo "$RAW" | head -20
    echo "└───"
    echo ""
    echo "┌─── BASELINE ───"
    echo "$BASELINE" | head -30
    echo "└───"
    echo ""
    echo "┌─── NEW OUTPUT ───"
    echo "$OUTPUT" | head -30
    echo "└───"
    echo ""

    # Prompt for human score.
    while true; do
        read -rp "Score (1-5), [s]kip, [q]uit: " REPLY
        case "$REPLY" in
            [1-5])
                SCORED=$((SCORED + 1))
                SCORE_SUM=$((SCORE_SUM + REPLY))
                echo "{\"name\":\"${NAME}\",\"humanScore\":${REPLY}}" >> "$RESULTS_FILE"
                break
                ;;
            s|S)
                echo "{\"name\":\"${NAME}\",\"humanScore\":null,\"skipped\":true}" >> "$RESULTS_FILE"
                break
                ;;
            q|Q)
                echo ""
                echo "Quitting. Results so far saved to: ${RUN_DIR}"
                exit 0
                ;;
            *)
                echo "Enter 1-5, s, or q."
                ;;
        esac
    done
done

# Write summary.
AVG="0"
if [[ $SCORED -gt 0 ]]; then
    AVG=$(echo "scale=2; $SCORE_SUM / $SCORED" | bc)
fi

cat > "${RUN_DIR}/summary.json" <<EOF
{
  "timestamp": "${TIMESTAMP}",
  "provider": "${PROVIDER:-<from settings>}",
  "model": "${MODEL:-<provider default>}",
  "mode": "human",
  "sampleCount": ${TOTAL},
  "scoredCount": ${SCORED},
  "avgHumanScore": ${AVG}
}
EOF

echo ""
echo "=== HUMAN EVAL COMPLETE ==="
echo "Samples: ${TOTAL}, Scored: ${SCORED}, Avg: ${AVG}/5"
echo "Results: ${RUN_DIR}"
