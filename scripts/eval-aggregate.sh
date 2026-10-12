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
#
# The models in the key are the RESOLVED ones where a run recorded them. The
# pipeline asks for an alias ("sonnet"), and the day the provider moves that
# alias to a new generation, the same request is a different measurement. A key
# on the alias would call it the same configuration and report the model's move
# as the prompt's. Runs that predate resolvedModel key on the requested model,
# which for those runs was always a full ID.
#
# The judges are a SET since 2026-10-11 (two by default, Haiku 5.5 and Sonnet
# 5.5; see docs/pyramidize/quality-status.md). Their three key fields — provider,
# resolved model, temperature — hold one entry per judge, joined with "+" and
# ordered by resolved model ID, so the order a run listed them in does not
# matter and a single-judge run keys exactly as it always did. A run with a
# different judge set (one added, one dropped, the old Sonnet 4.5 bridge judge
# alongside) is a different instrument and reads "not comparable".
#
# The judge's temperature is the twelfth field: 0 for the API judge, null for the
# Claude Code CLI, which cannot pin one. Provider alone already keeps the two
# judges apart; the temperature keeps a CLI judge from ever comparing equal to
# a pinned one, whatever a later run names its provider. Runs that predate the
# field were all judged at a pinned 0.
#
# thinking is the thirteenth field: whether the pipeline thought first. The CLI thinks
# by default ("on"); on the API Fix asks a fast-tier model not to ("off"), and
# other models are left to the provider ("n/a"). It moved the numbers once, so
# it is part of the measurement. Runs that predate it key as "unrecorded" if
# they went through the CLI (the 2026-10-04 first CLI baselines) and as "n/a"
# if they did not (every API-era run).
#
# judgePromptHash closes the key since 2026-10-11: the judges' system prompt is
# part of the instrument, like their models, and it changed when the judges
# were told that mail history is context (ADR-003, addendum). A hash rather
# than a version, so no edit to the prompt can slip through unbumped. Runs that
# predate it key as "unrecorded" — every Fix run among them, whose judge has a
# prompt of its own and records none.
#
# claudeCodeVersion is deliberately NOT in the key, for the promptHash reason
# (decision 2026-10-11): the CLI updates itself, and a key that changed with
# every update would make each baseline incomparable within days. It is
# recorded, listed in the baseline, and reported next to a verdict.
JQ_ROUND='def r4: (. * 10000 | round) / 10000;
def resolvedOr(r; m): if (r // "") == "" then m else r end;
def judgesOf(c): if (c.judges|type) == "array" then c.judges elif (c.judge|type) == "object" then [c.judge] else [] end;
# A judge is named by the ID that answered, as in the key — never by how a run
# spelled it. EVAL_JUDGES=haiku and claude-haiku-5-5 key alike, so they must
# land in the same column; keyed by the requested name, a regression on one
# side had no column on the other and passed as "overlap".
def judgeId(j): resolvedOr(j.resolvedModel; j.model);
# The Go side writes avgJudges, judgeCounts and the judges of each sample under the
# requested name; this maps that name to judgeId for one run.
def judgeRename(c): judgesOf(c) | map({key: .model, value: judgeId(.)}) | from_entries;
def renamed(c; obj): judgeRename(c) as $m | obj | with_entries(.key = ($m[.key] // .key));
def judgeIds(c): judgesOf(c) | map(judgeId(.));
def thinkingOf(c): if (c.thinking // "") != "" then c.thinking
                   elif c.provider == "claude-code" then "unrecorded" else "n/a" end;
def judgeTemp(j): if j == null then "none"
                  elif (j | has("temperature")) then (j.temperature | if . == null then "null" else (. + 0 | tostring) end)
                  else "0" end;
def judgeField(c; f): (judgesOf(c) | sort_by(resolvedOr(.resolvedModel; (.model // "none"))) | map(f)) as $v
                    | if ($v|length) == 0 then "none" else ($v | join("+")) end;
def keyFrom(c): [(c.suite // "pyramidize"), c.provider, resolvedOr(c.resolvedModel; c.model),
                 judgeField(c; .provider // "none"), judgeField(c; resolvedOr(.resolvedModel; (.model // "none"))),
                 (c.promptVariant|tostring), (c.schemaEnforcement|tostring),
                 (c.qualityThreshold|tostring), (c.sampleCount|tostring),
                 ((c.checksVersion // 1)|tostring), (c.split // "all"), judgeField(c; judgeTemp(.)),
                 thinkingOf(c), (c.judgePromptHash // "unrecorded")] | join("|");
# One interval per judge, never one across judges: two judges that disagree are
# information, and their mean would hide which one moved.
def stats: {mean: ((add / length) | r4), min: (min | r4), max: (max | r4), spread: ((max - min) | r4)};
def statsOrNull: if length == 0 then null else stats end;
# A run before the judge set wrote avgJudge/judgeCount for its one judge.
def judgeAvgs(s): if (s.avgJudges|type) == "object" then renamed(s; s.avgJudges)
                  elif (judgesOf(s)|length) == 1 and s.avgJudge != null then {(judgeId(judgesOf(s)[0])): s.avgJudge}
                  else {} end;
def judgeCounts(s): if (s.judgeCounts|type) == "object" then renamed(s; s.judgeCounts)
                    elif (judgesOf(s)|length) == 1 then {(judgeId(judgesOf(s)[0])): (s.judgeCount // 0)}
                    else {} end;
# One sample line of run s: {judgeId: overall}. Before the judge set a line
# carried .judge, which belongs to the one judge of that run.
def sampleScores(r; s): if (r.judges|type) == "object" then renamed(s; r.judges | map_values(.overall))
                        elif (r.judge|type) == "object" and (judgesOf(s)|length) == 1 then {(judgeId(judgesOf(s)[0])): r.judge.overall}
                        else {} end;
# A baseline written before the suite name joined the key stored eight fields.
# Upgrading it on READ is what keeps the baselines recorded so far usable;
# refusing them would have made a key format change quietly discard every
# measurement the project has. Stored keys of 8 to 11 fields predate the judge
# temperature and thinking fields, and every one of them was an API run judged
# at a pinned 0. A 12-field key has the temperature but not thinking, so it
# gains "n/a". Every key shorter than 14 fields predates the judge prompt hash
# and gains "unrecorded". A 14-field key is current when its last field is a
# judge prompt hash ("j1-…") or "unrecorded", and is kept as it is; otherwise
# it is the one PR revision (never on main) that ended in the CLI version, which
# is no longer keyed and gives way to "unrecorded". Every baseline written since
# carries its config, and baselineKey reads that instead of the string.
def upgradeKey(k): (k | split("|")) as $p
                 | if ($p|length) == 8 then ((["pyramidize"] + $p + ["1", "all", "0", "n/a", "unrecorded"]) | join("|"))
                   elif ($p|length) == 9 then (($p + ["1", "all", "0", "n/a", "unrecorded"]) | join("|"))
                   elif ($p|length) == 10 then (($p + ["all", "0", "n/a", "unrecorded"]) | join("|"))
                   elif ($p|length) == 11 then (($p + ["0", "n/a", "unrecorded"]) | join("|"))
                   elif ($p|length) == 12 then (($p + ["n/a", "unrecorded"]) | join("|"))
                   elif ($p|length) == 13 then (($p + ["unrecorded"]) | join("|"))
                   elif ($p|length) == 14 and (($p[13] | startswith("j1-")) or $p[13] == "unrecorded") then k
                   elif ($p|length) == 14 then (($p[:13] + ["unrecorded"]) | join("|"))
                   else k end;
def baselineKey(b): if (b.config|type) == "object" then keyFrom(b.config) else upgradeKey(b.configKey // "unknown") end;'

# The judges of the first run name the judge columns; configConsistent below
# refuses runs whose judge sets differ.
JUDGE_IDS=$(jq -c "$JQ_ROUND"'judgeIds(.)' "${SUMMARIES[0]}")

# Each results line gets its run's judge scores under judgeId, which needs that
# run's summary; the lines are then pooled across runs.
SCORED_RESULTS=$(for i in "${!RESULTS[@]}"; do
    jq -c --slurpfile s "${SUMMARIES[$i]}" "$JQ_ROUND"'. + {judgeScores: sampleScores(.; $s[0])}' "${RESULTS[$i]}"
done)

PER_SAMPLE=$(printf '%s\n' "$SCORED_RESULTS" | jq -s --argjson ids "$JUDGE_IDS" "$JQ_ROUND"'
    group_by(.name) | map(. as $rows | {
        name: .[0].name,
        runs: length,
        errors: (map(select(.error != null and .error != "")) | length),
        deterministic: (map(.deterministic.overallScore) | stats),
        judges: ($ids | map(. as $id | {key: $id, value: ([$rows[] | .judgeScores[$id] | select(. != null)] | statsOrNull)}) | from_entries),
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

AGGREGATE=$(jq -s --argjson perSample "$PER_SAMPLE" --argjson runs "$RUNS_JSON" --argjson passed "$PASSED" --argjson refined "$REFINED" --argjson ids "$JUDGE_IDS" "$JQ_ROUND"'
    . as $all
    | {
        createdAt: (.[0].timestamp),
        runCount: length,
        runs: $runs,
        config: {
            suite: (.[0].suite // "pyramidize"),
            promptHash: (.[0].promptHash // null),
            checksVersion: (.[0].checksVersion // 1),
            # Keyed, see the prelude; null for runs that predate it.
            judgePromptHash: (.[0].judgePromptHash // null),
            split: (.[0].split // "all"),
            splitHash: (.[0].splitHash // null),
            gitSHA: .[0].gitSHA,
            provider: .[0].provider,
            # The ID that answered (runs since 2026-10-08 record it here); the
            # key uses it.
            model: .[0].model,
            modelRequested: (.[0].modelRequested // null),
            # Only first-generation CLI runs (2026-10-04) wrote model = alias and
            # the ID here; null for everything else.
            resolvedModel: (if (.[0].resolvedModel // "") == "" then null else .[0].resolvedModel end),
            # Every CLI version the runs saw, comma-joined; null when none
            # recorded one. Not keyed — reported next to a verdict instead.
            claudeCodeVersion: (map(.claudeCodeVersion // "" | select(. != "")) | unique
                                | if length == 0 then null else join(",") end),
            thinking: (if (.[0].thinking // "") == "" then null else .[0].thinking end),
            promptVariant: .[0].promptVariant,
            # Every judge of the run, each with the ID that answered for it.
            judges: judgesOf(.[0]),
            schemaEnforcement: .[0].schemaEnforcement,
            qualityThreshold: .[0].qualityThreshold,
            sampleCount: .[0].sampleCount
        },
        # The fingerprint two runs must share to be averaged together. Model and
        # judge are not enough: a v1 run and a v2 run, or 13 samples against 40,
        # are different measurements however similar the numbers look.
        # suite is in the key on purpose: without it, two different eval suites
        # would be kept apart only by their values happening to differ.
        # promptHash is NOT — see the prelude above for why.
        configKey: keyFrom(.[0]),
        configConsistent: (map(keyFrom(.)) | unique | length == 1),
        # A judge that failed on some samples leaves a mean over a smaller set;
        # per judge, because one judge failing says nothing about the other.
        judgeCoverage: ($ids | map(. as $id | {key: $id, value: {
            min: ([$all[] | judgeCounts(.)[$id] // 0] | min),
            max: ([$all[] | judgeCounts(.)[$id] // 0] | max),
            of: ($all[0].sampleCount)}}) | from_entries),
        # Same question for the deterministic half: a run whose API calls failed
        # measured fewer samples than it set out to, and its average describes
        # the ones that answered.
        scoredCoverage: {min: (map(.scoredCount // .sampleCount) | min), max: (map(.scoredCount // .sampleCount) | max), of: (.[0].sampleCount)},
        deterministic: (map(.avgDeterministic) | stats),
        # One interval per judge, keyed by the model the run asked for.
        judges: ($ids | map(. as $id | {key: $id, value: ([$all[] | judgeAvgs(.)[$id] | select(. != null)] | statsOrNull)}) | from_entries),
        samplesPassing: $passed,
        refineCalls: $refined,
        perSample: $perSample
    }' "${SUMMARIES[@]}")

# Averaging runs that measured different things produces a number that describes
# nothing. Refuse rather than emit it with a flag nobody reads.
if [[ "$(printf '%s' "$AGGREGATE" | jq -r .configConsistent)" != "true" ]]; then
    echo "These runs do not share a configuration, so their mean would describe nothing:" >&2
    for f in "${SUMMARIES[@]}"; do
        jq -r "$JQ_ROUND"'"  \(input_filename): \(.provider)/\(.model) resolved=\(.resolvedModel // "-") judges=\(judgesOf(.) | map("\(.model)->\(.resolvedModel // "-")@\(.temperature)") | join(",")) thinking=\(.thinking // "-") cli=\(.claudeCodeVersion // "-") checks=\(.checksVersion // 1) judgePrompt=\(.judgePromptHash // "-") v\(.promptVariant) schema=\(.schemaEnforcement) samples=\(.sampleCount)"' "$f" >&2
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
    # Per-judge intervals of a baseline. One written before the judge set has a
    # single .judge interval, which belongs to its one judge.
    def baselineJudges(b; nowIds):
        if (b.judges|type) == "object" then b.judges
        else (judgeIds(b.config // {}) | if length == 1 then . elif (nowIds|length) == 1 then nowIds else [] end) as $one
             | if ($one|length) == 1 and (b.judge|type) == "object" then {($one[0]): b.judge} else {} end
        end;
    # Lowest judge count per judge, as [id, min] pairs. A baseline from before
    # the judge set stored one {min, max, of}.
    def coverageMins(b): if (b.judgeCoverage|type) != "object" then []
                         elif (b.judgeCoverage|has("of")) then [["the judge", b.judgeCoverage.min]]
                         else (b.judgeCoverage | to_entries | map([.key, .value.min])) end;
    # The interval rule, for one metric: does the new range clear the old one?
    def rangeVerdict(n; w): if n == null or w == null or n.mean == null or w.mean == null then "overlap"
                            elif n.max < w.min then "regression"
                            elif n.min > w.max then "improvement"
                            else "overlap" end;
    . as $now
    | $base[0] as $was
    | baselineKey($was) as $wasKey
    | (judgeIds($now.config)) as $ids
    | baselineJudges($was; $ids) as $wasJudges
    # Each metric judged on its own: the deterministic score and every judge.
    | rangeVerdict($now.deterministic; $was.deterministic) as $detVerdict
    | ($ids | map({key: ., value: rangeVerdict($now.judges[.]; $wasJudges[.])}) | from_entries) as $judgeVerdicts
    | ([{label: "deterministic", verdict: $detVerdict}]
       + ($ids | map({label: ("judge " + .), verdict: $judgeVerdicts[.]}))) as $metrics
    # Every judge either side names. Equal keys mean equal judge sets, so a
    # judge with no interval on one side is a broken record, not an overlap.
    | (($ids + ($wasJudges | keys)) | unique) as $allIds
    | ($allIds | map(select($now.judges[.] == null or $wasJudges[.] == null))) as $missing
    | ((coverageMins($now) | map(select(.[1] < $now.config.sampleCount)))
       + (coverageMins($was) | map(select(.[1] < ($was.config.sampleCount // 0))))) as $shortJudges
    | {
        splitMembership: {baseline: ($was.config.splitHash // null), now: ($now.config.splitHash // null),
                          changed: (($was.config.splitHash // null) != ($now.config.splitHash // null))},
        promptHash: {baseline: ($was.config.promptHash // null), now: ($now.config.promptHash // null),
                     changed: (($was.config.promptHash // null) != ($now.config.promptHash // null))},
        claudeCodeVersion: {baseline: ($was.config.claudeCodeVersion // null), now: ($now.config.claudeCodeVersion // null),
                            changed: (($was.config.claudeCodeVersion // null) != ($now.config.claudeCodeVersion // null))},
        baseline: {config: $wasKey, runs: $was.runCount, gitSHA: $was.config.gitSHA,
                   deterministic: $was.deterministic, judges: $wasJudges, samplesPassing: $was.samplesPassing},
        now:      {config: $now.configKey, runs: $now.runCount, gitSHA: $now.config.gitSHA,
                   deterministic: $now.deterministic, judges: $now.judges, samplesPassing: $now.samplesPassing},
        deterministicVerdict: $detVerdict,
        judgeVerdicts: $judgeVerdicts,
        # Which metrics flagged the verdict, by name: a regression carried by
        # one judge alone reads as exactly that.
        regressedMetrics: ($metrics | map(select(.verdict == "regression") | .label)),
        improvedMetrics: ($metrics | map(select(.verdict == "improvement") | .label)),
        deterministicDelta: (($now.deterministic.mean - $was.deterministic.mean) | r4),
        judgeDeltas: ($ids | map(. as $id | {key: $id, value:
            (if ($now.judges[$id].mean == null) or ($wasJudges[$id].mean == null) then null
             else (($now.judges[$id].mean - $wasJudges[$id].mean) | r4) end)}) | from_entries),
        verdict: (
            if $wasKey != $now.configKey then
                "not comparable: different configuration"
            # Pipeline failures first: a sample the pipeline never produced
            # also has no judge score, and blaming the judge for it sends the
            # reader to the wrong place.
            elif ($now.scoredCoverage.min < $now.scoredCoverage.of) or (($was.scoredCoverage.min // $was.config.sampleCount) < ($was.config.sampleCount)) then
                "not comparable: a run failed to score every sample"
            elif ($shortJudges | length) > 0 then
                "not comparable: " + ($shortJudges | map(.[0] | if . == "the judge" then . else "judge " + . end) | unique | join(", ")) + " did not score every sample"
            elif ($missing | length) > 0 then
                "not comparable: no interval for " + ($missing | map("judge " + .) | join(", ")) + " on one side"
            elif ($was.runCount < 2) or ($now.runCount < 2) then
                "indicative only: one run has no range to compare"
            else
                # Per metric, because "a regression" without naming which number
                # moved sends the reader back to the raw runs. Any metric
                # clearly worse is a regression; an improvement is claimed only
                # when at least one metric is clearly better and none is worse.
                # The judges are never averaged: one judge clearly worse is a
                # regression even if the other moved up.
                ($metrics | map(select(.verdict == "regression"))) as $regressions
                | ($metrics | map(select(.verdict == "improvement"))) as $improvements
                | if ($regressions | length) > 0 then
                      "regression: " + ($regressions | map(.label + " regression") | join(", "))
                  elif ($improvements | length) > 0 then
                      "improvement: " + ($improvements | map(.label + " improvement") | join(", "))
                  else
                      "inconclusive: the ranges overlap"
                  end
            end
        )
      }
    # Named only when the verdict is that comparison: a metric cannot flag a
    # comparison that was refused.
    | .regressedMetrics = (if (.verdict | startswith("regression")) then .regressedMetrics else [] end)
    | .improvedMetrics = (if (.verdict | startswith("improvement")) then .improvedMetrics else [] end)')

printf '%s\n' "$VERDICT"

# The hash is not in the key, so it cannot silence a comparison — but a reader
# who does not know the prompt moved will attribute the move to the model.
# The key records how many samples a run measured, never which. Two five-sample
# holdouts with one swapped in and one out key identically.
if [[ "$(printf '%s' "$VERDICT" | jq -r '.splitMembership.changed')" == "true" ]]; then
    echo "These two sides measured a different set of samples, not just a different number of them." >&2
fi
if [[ "$(printf '%s' "$VERDICT" | jq -r '.promptHash.changed')" == "true" ]]; then
    printf 'The prompt changed between these two sides (%s -> %s). Whatever moved, the prompt is a candidate.\n' \
        "$(printf '%s' "$VERDICT" | jq -r '.promptHash.baseline // "unrecorded"')" \
        "$(printf '%s' "$VERDICT" | jq -r '.promptHash.now // "unrecorded"')" >&2
fi

# Same reasoning for the CLI: its defaults sit between the model and the score,
# and an upgrade can move a number with nothing in this repository moving.
if [[ "$(printf '%s' "$VERDICT" | jq -r '.claudeCodeVersion.changed')" == "true" ]]; then
    printf 'The Claude Code CLI changed between these two sides (%s -> %s). Whatever moved, the CLI is a candidate.\n' \
        "$(printf '%s' "$VERDICT" | jq -r '.claudeCodeVersion.baseline // "unrecorded"')" \
        "$(printf '%s' "$VERDICT" | jq -r '.claudeCodeVersion.now // "unrecorded"')" >&2
fi

case "$(printf '%s' "$VERDICT" | jq -r .verdict)" in
    regression*)
        printf 'Flagged by: %s (of %s). On that metric every run of the new code scored below every run of the baseline. Advisory — re-run before believing it.\n' \
            "$(printf '%s' "$VERDICT" | jq -r '.regressedMetrics | join(", ")')" \
            "$(printf '%s' "$VERDICT" | jq -r '(["deterministic"] + (.judgeVerdicts | keys | map("judge " + .))) | join(", ")')" >&2
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
