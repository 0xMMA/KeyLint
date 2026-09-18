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
# Shared jq prelude. keyFrom is the one definition of what makes two runs the
# same measurement, so the aggregate and the comparison cannot drift apart.
#
# A run recorded before the suite field existed WAS a pyramidize run — there was
# no other suite — so defaulting it is a fact about those files, not a guess.
#
# split IS in the key: the Fix samples are divided into a tuning half and a
# held-out half, and a number from ten samples is not a number from fifteen. Runs
# that predate the split, and every pyramidize run, key as "all".
#
# checksVersion IS in the key, for the opposite reason: the checks are what the
# suite measures WITH, and a run scored by a different instrument is a different
# measurement however similar the prompt was. Anything else that sits between
# the model and the score — post-processing of the model reply, say — belongs
# here for the same reason, and would need its own version field.
#
# promptHash is deliberately NOT in the key. It was, for one revision, and that
# made the suite refuse the comparison it exists to make: change the prompt, and
# every before/after pair reads "not comparable: different configuration". The
# hash answers a different question — "did the prompt move, or did the model?" —
# so it is reported next to the verdict instead, where it informs the reader
# without silencing the number.
JQ_ROUND='def r4: (. * 10000 | round) / 10000;
def keyFrom(c): [(c.suite // "pyramidize"), c.provider, c.model,
                 (c.judge.provider // "none"), (c.judge.model // "none"),
                 (c.promptVariant|tostring), (c.schemaEnforcement|tostring),
                 (c.qualityThreshold|tostring), (c.sampleCount|tostring),
                 ((c.checksVersion // 1)|tostring), (c.split // "all")] | join("|");
# A baseline written before the suite name joined the key stored eight fields.
# Upgrading it on READ is what keeps the baselines recorded so far usable;
# refusing them would have made a key format change quietly discard every
# measurement the project has.
def upgradeKey(k): (k | split("|")) as $p
                 | if ($p|length) == 8 then ((["pyramidize"] + $p + ["1", "all"]) | join("|"))
                   elif ($p|length) == 9 then (($p + ["1", "all"]) | join("|"))
                   elif ($p|length) == 10 then (($p + ["all"]) | join("|"))
                   else k end;
def baselineKey(b): if (b.config|type) == "object" then keyFrom(b.config) else upgradeKey(b.configKey // "unknown") end;'

PER_SAMPLE=$(cat "${RESULTS[@]}" | jq -s "$JQ_ROUND"'
    group_by(.name) | map({
        name: .[0].name,
        runs: length,
        errors: (map(select(.error != null and .error != "")) | length),
        deterministic: (map(.deterministic.overallScore) | {mean: ((add / length) | r4), min: (min | r4), max: (max | r4), spread: ((max - min) | r4)}),
        judge: (map(select(.judge != null) | .judge.overall) |
                if length == 0 then null
                else {mean: ((add / length) | r4), min: (min | r4), max: (max | r4), spread: ((max - min) | r4)} end),
        passed: (map(select(.deterministic.allPassed == true)) | length),
        refined: (map(select(.appliedRefinement == true)) | length)
    })')

# Refine calls per run. The run folders are gitignored, so a baseline that does
# not carry this cannot answer "did the pipeline arm ever make a second call?"
# once the runs are gone — which is exactly the gap ADR-002 had to admit.
# null, not 0, when the field is absent: runs recorded before appliedRefinement
# existed would otherwise read as a measured "refine never fired", which is a
# different claim from "nobody wrote it down".
REFINED=$(for f in "${RESULTS[@]}"; do
    jq -s 'if any(.[]; has("appliedRefinement")) then [.[] | select(.appliedRefinement == true)] | length else null end' "$f"
done | jq -s 'if any(.[]; . == null) then null
              else {mean: ((add / length) * 100 | round / 100), min: min, max: max} end')

# Samples passing, per run. The documented baseline quotes this and nothing
# computed it, so nobody could check the claim it makes about its own noise.
PASSED=$(for f in "${RESULTS[@]}"; do
    jq -s '[.[] | select(.deterministic.allPassed == true)] | length' "$f"
done | jq -s "$JQ_ROUND"'{mean: ((add / length) | r4), min: min, max: max, spread: (max - min)}')

RUNS_JSON=$(printf '%s\n' "${RUN_DIRS[@]}" | jq -R . | jq -s -c .)

AGGREGATE=$(jq -s --argjson perSample "$PER_SAMPLE" --argjson runs "$RUNS_JSON" --argjson passed "$PASSED" --argjson refined "$REFINED" "$JQ_ROUND"'
    {
        createdAt: (.[0].timestamp),
        runCount: length,
        runs: $runs,
        config: {
            suite: (.[0].suite // "pyramidize"),
            promptHash: (.[0].promptHash // null),
            checksVersion: (.[0].checksVersion // 1),
            split: (.[0].split // "all"),
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
        # suite and promptHash are in the key on purpose. Without suite, two
        # different eval suites are kept apart only by their values happening to
        # differ; without promptHash, a comparison cannot tell "the prompt
        # changed" from "the model did", which is the whole reason the hash is
        # recorded.
        configKey: keyFrom(.[0]),
        configConsistent: (map(keyFrom(.)) | unique | length == 1),
        # A judge that failed on some samples leaves a mean over a smaller set.
        judgeCoverage: {min: (map(.judgeCount // 0) | min), max: (map(.judgeCount // 0) | max), of: (.[0].sampleCount)},
        # Same question for the deterministic half: a run whose API calls failed
        # measured fewer samples than it set out to, and its average describes
        # the ones that answered.
        scoredCoverage: {min: (map(.scoredCount // .sampleCount) | min), max: (map(.scoredCount // .sampleCount) | max), of: (.[0].sampleCount)},
        deterministic: (map(.avgDeterministic) | {mean: ((add / length) | r4), min: (min | r4), max: (max | r4), spread: ((max - min) | r4)}),
        judge: (map(select(.avgJudge != null) | .avgJudge) |
                if length == 0 then null
                else {mean: ((add / length) | r4), min: (min | r4), max: (max | r4), spread: ((max - min) | r4)} end),
        samplesPassing: $passed,
        refineCalls: $refined,
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
    | baselineKey($was) as $wasKey
    # Each metric judged on its own: does the new interval clear the old one?
    | (if $now.deterministic.max < $was.deterministic.min then "regression"
       elif $now.deterministic.min > $was.deterministic.max then "improvement"
       else "overlap" end) as $detVerdict
    | (if ($now.judge.mean == null) or ($was.judge.mean == null) then "overlap"
       elif $now.judge.max < $was.judge.min then "regression"
       elif $now.judge.min > $was.judge.max then "improvement"
       else "overlap" end) as $judgeVerdict
    | ("deterministic " + $detVerdict) as $detLabel
    | ("judge " + $judgeVerdict) as $judgeLabel
    | {
        promptHash: {baseline: ($was.config.promptHash // null), now: ($now.config.promptHash // null),
                     changed: (($was.config.promptHash // null) != ($now.config.promptHash // null))},
        baseline: {config: $wasKey, runs: $was.runCount, gitSHA: $was.config.gitSHA,
                   deterministic: $was.deterministic, judge: $was.judge, samplesPassing: $was.samplesPassing},
        now:      {config: $now.configKey, runs: $now.runCount, gitSHA: $now.config.gitSHA,
                   deterministic: $now.deterministic, judge: $now.judge, samplesPassing: $now.samplesPassing},
        deterministicVerdict: $detVerdict,
        judgeVerdict: $judgeVerdict,
        deterministicDelta: (($now.deterministic.mean - $was.deterministic.mean) | r4),
        judgeDelta: ((($now.judge.mean // 0) - ($was.judge.mean // 0)) | r4),
        verdict: (
            if $wasKey != $now.configKey then
                "not comparable: different configuration"
            elif ($now.judgeCoverage.min < $now.judgeCoverage.of) or (($was.judgeCoverage.min // $was.config.sampleCount) < ($was.config.sampleCount)) then
                "not comparable: the judge did not score every sample"
            elif ($now.scoredCoverage.min < $now.scoredCoverage.of) or (($was.scoredCoverage.min // $was.config.sampleCount) < ($was.config.sampleCount)) then
                "not comparable: a run failed to score every sample"
            elif ($was.runCount < 2) or ($now.runCount < 2) then
                "indicative only: one run has no range to compare"
            else
                # Per metric, because "a regression" without naming which number
                # moved sends the reader back to the raw runs. A regression in
                # one metric is still a regression; an improvement is only
                # claimed when nothing moved the other way.
                ([($detVerdict | select(. == "regression")), ($judgeVerdict | select(. == "regression"))] | length) as $regressions
                | ([($detVerdict | select(. == "improvement")), ($judgeVerdict | select(. == "improvement"))] | length) as $improvements
                | if $regressions > 0 then
                      "regression: " + ([$detLabel, $judgeLabel] | map(select(test("regression"))) | join(", "))
                  elif $improvements > 0 and ([$detVerdict, $judgeVerdict] | map(select(. == "regression")) | length) == 0 then
                      "improvement: " + ([$detLabel, $judgeLabel] | map(select(test("improvement"))) | join(", "))
                  else
                      "inconclusive: the ranges overlap"
                  end
            end
        )
      }')

printf '%s\n' "$VERDICT"

# The hash is not in the key, so it cannot silence a comparison — but a reader
# who does not know the prompt moved will attribute the move to the model.
if [[ "$(printf '%s' "$VERDICT" | jq -r '.promptHash.changed')" == "true" ]]; then
    printf 'The prompt changed between these two sides (%s -> %s). Whatever moved, the prompt is a candidate.\n' \
        "$(printf '%s' "$VERDICT" | jq -r '.promptHash.baseline // "unrecorded"')" \
        "$(printf '%s' "$VERDICT" | jq -r '.promptHash.now // "unrecorded"')" >&2
fi

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
