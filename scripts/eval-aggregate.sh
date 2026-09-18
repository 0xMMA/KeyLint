#!/usr/bin/env bash
set -euo pipefail

# Aggregates one or more eval-run directories into a baseline document, and
# optionally compares them against an earlier one.
#
#   ./scripts/eval-aggregate.sh <run-dir>...                       # print the aggregate
#   ./scripts/eval-aggregate.sh --compare <baseline.json> <run-dir>...
#
# Split out of eval.sh so it can be exercised on recorded runs without spending
# an API call: every number the eval reports is computed here.
#
# The spread (max - min across runs) is the point. One run is a sample, not a
# measurement — the same commit and model produce different numbers from one
# hour to the next, so a delta smaller than the spread is not evidence.

# Exit codes are a contract; callers branch on them.
#   0  aggregate printed, or a comparison that found nothing to report
#   1  a regression past the noise floor
#   2  the two sides are not comparable (different configuration)
#   3  usage, missing input, unusable baseline — nothing was measured
readonly EXIT_REGRESSION=1 EXIT_NOT_COMPARABLE=2 EXIT_USAGE=3

usage() { echo "usage: $0 [--compare baseline.json] <run-dir>..." >&2; exit "$EXIT_USAGE"; }

COMPARE=""
ARGS=()
while (( $# > 0 )); do
    case "$1" in
        --compare)
            [[ -n "${2:-}" ]] || { echo "--compare needs a file" >&2; usage; }
            COMPARE="$2"; shift 2 ;;
        -h|--help) usage ;;
        *) ARGS+=("$1"); shift ;;
    esac
done
set -- "${ARGS[@]}"

(( $# > 0 )) || usage
command -v jq >/dev/null || { echo "jq is required" >&2; exit "$EXIT_USAGE"; }

if [[ -n "$COMPARE" ]]; then
    [[ -f "$COMPARE" ]] || { echo "No such baseline: $COMPARE" >&2; exit "$EXIT_USAGE"; }
    # An empty or truncated baseline would otherwise reach jq as null and come
    # back as a confident-looking regression.
    jq -e '.deterministic.mean != null and .runCount != null' "$COMPARE" >/dev/null 2>&1 || {
        echo "Not a usable baseline (no deterministic.mean / runCount): $COMPARE" >&2
        exit "$EXIT_USAGE"
    }
fi

SUMMARIES=()
RESULTS=()
RUN_DIRS=()
for dir in "$@"; do
    dir="${dir%/}/"
    [[ -f "${dir}summary.json" ]] || { echo "Missing ${dir}summary.json" >&2; exit "$EXIT_USAGE"; }
    [[ -f "${dir}results.jsonl" ]] || { echo "Missing ${dir}results.jsonl" >&2; exit "$EXIT_USAGE"; }
    for seen in ${RUN_DIRS[@]+"${RUN_DIRS[@]}"}; do
        # The same run twice would report runCount 3 with a spread of 0, which
        # is the most misleading document this script can produce.
        [[ "$seen" == "$dir" ]] && { echo "Run directory given twice: $dir" >&2; exit "$EXIT_USAGE"; }
    done
    RUN_DIRS+=("$dir")
    SUMMARIES+=("${dir}summary.json")
    RESULTS+=("${dir}results.jsonl")
done

# Per sample across runs: the same spread, at the resolution that tells you
# whether one sample is carrying the whole difference.
# r4 keeps the stored numbers readable: float sums print as 0.7799999999999999,
# and a baseline a human has to read is worth four decimals.
JQ_ROUND='def r4: (. * 10000 | round) / 10000;'

PER_SAMPLE=$(cat "${RESULTS[@]}" | jq -s "$JQ_ROUND"'
    group_by(.name) | map({
        name: .[0].name,
        runs: length,
        errors: (map(select(.error != null and .error != "")) | length),
        deterministic: (map(.deterministic.overallScore) | {mean: ((add / length) | r4), min: (min | r4), max: (max | r4), spread: ((max - min) | r4)}),
        judge: (map(select(.judge != null) | .judge.overall) |
                if length == 0 then null
                else {mean: ((add / length) | r4), min: (min | r4), max: (max | r4), spread: ((max - min) | r4)} end),
        passed: (map(select(.deterministic.allPassed == true)) | length)
    })')

# Samples passing, per run. The documented baseline quotes this and nothing
# computed it, so nobody could check the claim it makes about its own noise.
PASSED=$(for f in "${RESULTS[@]}"; do
    jq -s '[.[] | select(.deterministic.allPassed == true)] | length' "$f"
done | jq -s "$JQ_ROUND"'{mean: ((add / length) | r4), min: min, max: max, spread: (max - min)}')

RUNS_JSON=$(printf '%s\n' "${RUN_DIRS[@]}" | jq -R . | jq -s -c .)

AGGREGATE=$(jq -s --argjson perSample "$PER_SAMPLE" --argjson runs "$RUNS_JSON" --argjson passed "$PASSED" "$JQ_ROUND"'
    {
        createdAt: (.[0].timestamp),
        runCount: length,
        runs: $runs,
        config: {
            gitSHA: .[0].gitSHA,
            provider: .[0].provider,
            model: .[0].model,
            promptVariant: .[0].promptVariant,
            judge: .[0].judge,
            schemaEnforcement: .[0].schemaEnforcement,
            qualityThreshold: .[0].qualityThreshold,
            sampleCount: .[0].sampleCount
        },
        # The fingerprint two runs must share to be averaged together. Model and
        # judge are not enough: a v1 run and a v2 run, or 13 samples against 40,
        # are different measurements however similar the numbers look.
        configKey: (.[0] | [.provider, .model, (.judge.provider // "none"), (.judge.model // "none"),
                            (.promptVariant|tostring), (.schemaEnforcement|tostring),
                            (.qualityThreshold|tostring), (.sampleCount|tostring)] | join("|")),
        configConsistent: (map([.provider, .model, (.judge.provider // "none"), (.judge.model // "none"),
                                (.promptVariant|tostring), (.schemaEnforcement|tostring),
                                (.qualityThreshold|tostring), (.sampleCount|tostring)] | join("|")) | unique | length == 1),
        # A judge that failed on some samples leaves a mean over a smaller set.
        judgeCoverage: {min: (map(.judgeCount // 0) | min), max: (map(.judgeCount // 0) | max), of: (.[0].sampleCount)},
        deterministic: (map(.avgDeterministic) | {mean: ((add / length) | r4), min: (min | r4), max: (max | r4), spread: ((max - min) | r4)}),
        judge: (map(select(.avgJudge != null) | .avgJudge) |
                if length == 0 then null
                else {mean: ((add / length) | r4), min: (min | r4), max: (max | r4), spread: ((max - min) | r4)} end),
        samplesPassing: $passed,
        perSample: $perSample
    }' "${SUMMARIES[@]}")

# Averaging runs that measured different things produces a number that describes
# nothing. Refuse rather than emit it with a flag nobody reads.
if [[ "$(printf '%s' "$AGGREGATE" | jq -r .configConsistent)" != "true" ]]; then
    echo "These runs do not share a configuration, so their mean would describe nothing:" >&2
    for f in "${SUMMARIES[@]}"; do
        jq -r '"  \(input_filename): \(.provider)/\(.model) judge=\(.judge.model // "none") v\(.promptVariant) schema=\(.schemaEnforcement) samples=\(.sampleCount)"' "$f" >&2
    done
    exit "$EXIT_NOT_COMPARABLE"
fi

if [[ -z "$COMPARE" ]]; then
    printf '%s\n' "$AGGREGATE"
    exit 0
fi

# Advisory only, and deliberately not wired into CI: it spends real money per
# invocation, and a judge scoring free text is not a unit test.
#
# The test compares RANGES, not a mean against someone else's spread. With three
# runs there is no distribution worth modelling, only an observed interval, so
# the only defensible statement is whether the two intervals overlap. A new mean
# that sits inside the baseline's observed range is a value the baseline itself
# produced; a new set whose best run is below the baseline's worst run is a
# difference no run of the old code showed.
#
# This cuts both ways on purpose: a single run has no range, so it gets no
# verdict. Believing a one-run delta is the mistake this whole change exists to
# stop, and the guard must not make it on the reader's behalf.
VERDICT=$(printf '%s\n' "$AGGREGATE" | jq --slurpfile base "$COMPARE" "$JQ_ROUND"'
    . as $now
    | $base[0] as $was
    | {
        baseline: {config: ($was.configKey // "unknown"), runs: $was.runCount, gitSHA: $was.config.gitSHA,
                   deterministic: $was.deterministic, judge: $was.judge, samplesPassing: $was.samplesPassing},
        now:      {config: $now.configKey, runs: $now.runCount, gitSHA: $now.config.gitSHA,
                   deterministic: $now.deterministic, judge: $now.judge, samplesPassing: $now.samplesPassing},
        deterministicDelta: (($now.deterministic.mean - $was.deterministic.mean) | r4),
        judgeDelta: ((($now.judge.mean // 0) - ($was.judge.mean // 0)) | r4),
        verdict: (
            if ($was.configKey // "unknown") != $now.configKey then
                "not comparable: different configuration"
            elif ($now.judgeCoverage.min < $now.judgeCoverage.of) or (($was.judgeCoverage.min // $was.config.sampleCount) < ($was.config.sampleCount)) then
                "not comparable: the judge did not score every sample"
            elif ($was.runCount < 2) or ($now.runCount < 2) then
                "indicative only: one run has no range to compare"
            elif ($now.deterministic.max < $was.deterministic.min)
                 or (($now.judge.max // 1) < ($was.judge.min // 0)) then
                "regression: the new range sits entirely below the old one"
            elif ($now.deterministic.min > $was.deterministic.max)
                 and (($now.judge.min // 0) > ($was.judge.max // 1)) then
                "improvement: the new range sits entirely above the old one"
            else
                "inconclusive: the ranges overlap"
            end
        )
      }')

printf '%s\n' "$VERDICT"

case "$(printf '%s' "$VERDICT" | jq -r .verdict)" in
    regression*)
        echo "Every run of the new code scored below every run of the baseline. Advisory — re-run before believing it." >&2
        exit "$EXIT_REGRESSION"
        ;;
    "not comparable"*)
        echo "The two sides measured different things; the deltas above are not a comparison." >&2
        exit "$EXIT_NOT_COMPARABLE"
        ;;
    "indicative only"*)
        echo "One side has a single run. Use --runs 3 on both sides for a verdict worth acting on." >&2
        ;;
esac
