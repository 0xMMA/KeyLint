# Pyramidize Output Quality — Status

> Last updated: 2026-10-14

> The Fix prompt has its own suite and baseline: [`docs/fix/quality-status.md`](../fix/quality-status.md).

The first part is where Pyramidize stands today. Everything under
[History](#history) is the record of how it got there, newest first. It stays
because ADRs and PRs quote its numbers, and because most of what it learned
about measuring still holds.

---

## Where it stands (2026-10-14)

**What ships: prompt v3 ([ADR-003](adr-003-prompt-v3.md)).** The user picks the
form (email, chat, memo, wiki, presentation) and one model call writes it. There
is no detect call, no self-QA, no refine pass and no quality threshold. The only
other input is the optional instructions field. Instructions win over the voice
defaults, and content they state is added where it belongs; the model adds
nothing on its own (owner decision 2026-10-13). Mail history in the input is
context, never output (2026-10-11). v2 stays selectable with `--variant 2`: email
only, byte-identical, always `professional`/`professional`. The prompt on `main`
(77b7bd8) is the one measured below: nothing under `internal/features/pyramidize/`
changed between the baseline's commit and the merge.

**What is measured: the email form, and only it.** The suite has 13 anonymised
German business emails. Two of them carry mail history; one mixes German and
English heavily, a code-switch the prompts keep on purpose.

### Reference baseline: 2026-10-13

`test-data/eval-baselines/2026-10-13T00-36-32/baseline.json`. Three runs, full
coverage (13 of 13 for each judge in every run).

| Metric | Mean | Range |
|---|:---:|:---:|
| Deterministic | 0.9013 | 0.8974–0.9054 |
| Judge Haiku 5.5 | 0.8544 | 0.8492–0.8577 |
| Judge Sonnet 5.5 | 0.8272 | 0.8223–0.8354 |
| Samples passing (all deterministic checks) | 10 | 9–11 |

The configuration:
- Pipeline: `claude-code` / `sonnet` → `claude-sonnet-5-5`, thinking on.
- Prompt: v3 email, `promptHash` `p1-b42c33b84682302a`. Schema enforcement off.
- Scoring: `checksVersion` 2, `judgePromptHash` `j1-fda30fd7bd94a7fa`.
- Run: CLI 2.1.296, gitSHA `52f19d8`.
- Threshold: 0.65 is recorded but acts on nothing since v3.

Measure a change against it with:

    ./scripts/eval.sh --runs 3 --compare test-data/eval-baselines/2026-10-13T00-36-32/baseline.json

That is 117 calls on the subscription: 13 samples × (one pipeline call + two
judge calls) × 3 runs. Each run took about four minutes on 2026-10-13.

### Per sample (2026-10-13)

Sorted by deterministic score. Mean over three runs, with the deterministic range
in brackets. A sample passes a run when every deterministic check passes.

| Sample | Deterministic | Haiku 5.5 | Sonnet 5.5 | Passes |
|---|:---:|:---:|:---:|:---:|
| email-passwort erhalten | 0.79 (0.79–0.80) | 0.86 | 0.81 | 0 of 3 |
| email-software-deployment | 0.80 (0.77–0.84) | 0.86 | 0.88 | 1 of 3 |
| email-meeting-request | 0.83 (0.80–0.85) | 0.85 | 0.80 | 0 of 3 |
| email-onboarding-erfolgreich | 0.88 (0.85–0.90) | 0.90 | 0.88 | 3 of 3 |
| email-diagnose-update *(history)* | 0.90 (0.87–0.93) | 0.87 | 0.89 | 2 of 3 |
| email-answer-info-update | 0.90 (0.87–0.92) | 0.87 | 0.76 | 3 of 3 |
| email-kpi-request | 0.90 (0.89–0.92) | 0.89 | 0.90 | 3 of 3 |
| email-project-status | 0.92 (0.90–0.94) | 0.86 | 0.85 | 3 of 3 |
| email-adverity-field-structuring | 0.94 (0.92–0.95) | 0.67 | 0.72 | 3 of 3 |
| email-airbyte-escalation | 0.94 (0.93–0.95) | 0.86 | 0.83 | 3 of 3 |
| email-dataquality | 0.97 (0.95–0.98) | 0.85 | 0.79 | 3 of 3 |
| email-dataquality-followup | 0.97 (0.96–0.98) | 0.91 | 0.85 | 3 of 3 |
| email-dataquality-reply-to-feedback *(history)* | 0.98 (0.95–1.00) | 0.86 | 0.81 | 3 of 3 |

### Where it is weak

- **Two samples fail every run, while the judges score them normally**
  (0.80–0.86). Which check fails them on 2026-10-13 is not known (see the next
  point). On 2026-10-12, the last time per-check numbers were read, it was the
  term checks:
  - `email-passwort erhalten`: `no_hallucination` was at 0.31–0.50.
  - `email-meeting-request`: `info_coverage` was at 0.60–0.63. Its input is
    casual spoken German with many informal capitalised words.

  If that still holds, these are the check limits listed below more than
  writing faults.
- **Two more pass only some runs.** `email-software-deployment` passes 1 of 3,
  with the widest deterministic range (0.77–0.84). `email-diagnose-update`
  passes 2 of 3. Which check fails is not known. The run folders of the
  2026-10-12 and 2026-10-13 baselines lived in worktrees that have since been
  removed, so only the committed aggregates are left.
- **The judges' lowest sample passes every check.**
  `email-adverity-field-structuring` scores 0.94 deterministically, but Haiku
  gives it 0.67 and Sonnet 0.72. It also has the widest judge spread
  (0.60–0.75 and 0.62–0.78). Sonnet has scored it 0.64–0.72 on every
  two-judge baseline. Haiku moves more. It gave 0.74 and 0.77 on 2026-10-11,
  and 0.85 (0.82–0.87) on 2026-10-12, also on v3. The drop to 0.67 is the
  largest per-sample move between the last two baselines. Why the
  judges mark it down has not been examined on v3.
- **Other per-sample moves from 2026-10-12 to 2026-10-13** with disjoint ranges
  (indicative only: three runs per sample):
  - `email-passwort erhalten` went down with both judges (Haiku 0.90 → 0.86,
    Sonnet 0.86 → 0.81).
  - `email-answer-info-update` went up with Sonnet (0.67 → 0.76).
- **The judges disagree per sample, not only in level.** Sonnet sits 0.027
  below Haiku overall. On `email-answer-info-update` it sits 0.10 below, with
  disjoint ranges (0.75–0.78 against 0.86–0.87). On `email-kpi-request` they
  agree (0.90 and 0.89). That is why they are reported separately and never
  averaged.
- `no_history_reproduction` scored 1.0 in all six runs of the two history
  samples, on both 2026-10-12 and 2026-10-13: no reply copied the mails it
  answers.

### What the numbers do not cover

- **Chat, memo, wiki and presentation.** Their v3 sections ship unmeasured.
  Measuring them needs three things:
  - Samples. They are meant to be real anonymised texts from the owner, and
    there are none yet.
  - A harness that runs other forms. Today it sends every sample as email and
    reads only `pyramidal-emails/`.
  - Per-form deterministic checks. `structure` wants a subject and a header, so
    every chat output would fail it today. That change bumps `checksVersion`
    (roadmap E3 phase 1).

  The old pipeline for comparison is in the `v4.7.1-beta` tag.
- **Instructions.** No sample carries any, so every run takes the "keep the
  author's voice" path. Whether "content the user states is added" works, and
  whether instructions ever make the model add more than they state, is
  untested.
- **The model question on v3.** Sonnet 5.5 is the only pipeline model measured.
  Opus 5.5, and maybe Haiku 5.5, are still to run.
- **Schema enforcement on v3.** It stays off until v3 is measured with and
  without it.
- **v2 on the current instrument.** v2 has not been rerun. v3 replaced it on the
  old instrument's comparison (2026-10-11, in History below).
- **Isolation, one gap.** Every CLI call runs in an empty directory with
  auto-memory off (#127). Whether user-level instructions
  (`~/.claude/CLAUDE.md`) still reach the CLI is not verified.

### Deterministic check limitations

These still hold, and they probably explain most of the failing samples above:

- `info_coverage` extracts capitalised words, which is noisy for German, where
  every noun is capitalised. Casual input suffers most.
- `no_hallucination` cannot tell restructuring vocabulary from an invented
  claim. The business-vocabulary exclusion list softens that but does not solve
  it.
- There is no semantic similarity. That would need an NLP model; see
  [research-nlp-langchain.md](research-nlp-langchain.md).
- `structure` is email-shaped: it wants a subject line and a header. That is
  right for email, but not for the other forms (see above).

The judges have limits of their own. Through the CLI they have no temperature
pin, and neither 5.5 model has a dated snapshot, so the full model ID is the
pin.

### The rules for reading a number

- **One run is a sample, not a measurement.** Quote `--runs 3` only, and on both
  sides of a comparison.
- **`--compare` asks whether intervals overlap, not whether a mean moved.**
  - A metric is better or worse only when the two ranges are disjoint.
  - Any metric clearly worse is a regression.
  - An improvement needs one metric clearly better and none worse.
  - The reasoning is in [The noise floor](#the-noise-floor). Its numbers are
    the 2026-09-18 instrument's; the reasoning holds.
- **Pass counts move by 2 between identical runs** (9–11 here). Do not read a
  difference of 2 or less.
- **The `configKey` names the instrument.** It holds:
  - suite, provider and resolved model
  - the judges and their temperatures
  - variant and schema enforcement
  - threshold, sample count and split
  - `checksVersion`, thinking and `judgePromptHash`

  A change to any of them reads "not comparable", never as a delta. So the v2
  rerun and the schema on/off comparison must be read by hand, interval against
  interval. The prompt itself is reported (`promptHash`) but not keyed, so a
  prompt change can be measured.
- **Run folders are local.** `test-data/eval-runs/` is gitignored, and removing
  the worktree a run was made in deletes them; only `baseline.json` is
  committed. Copy the folders out first if the per-check numbers might matter
  later.

### Next

Items 1, 3 and 4 are on the [roadmap](../roadmap.md) (E3); item 2 is a
proposal. Only item 1 waits on something outside the repo.

1. **The other forms.** First the owner's real samples for chat, memo, wiki
   and presentation. Then a harness that runs those forms, and per-form
   deterministic checks (E3 phase 1, a `checksVersion` bump). Measure each form
   on those, then answer the model question.
2. **Samples that carry instructions.** One or two, including one that states
   content, so the 2026-10-13 rule is exercised. Proposed, not decided.
3. **Schema enforcement on and off for v3.**
4. **Instrument follow-ups.**
   - The Fix judge should record its own prompt hash. `judgePromptHash` is
     Pyramidize-only, so Fix keys as `unrecorded`.
   - Optionally, give the judges the marked history split.

Earlier ideas, not scheduled (E9 is parked): semantic eval checks (sentence
embeddings instead of term overlap), and NLP info-loss detection. A deterministic
gate inside the live pipeline is also on that list, though "one call, no quality
threshold" (ADR-003) argues against it.

### How it is measured

- **Framework:** deterministic checks plus LLM judges, build-tagged
  `//go:build eval` and never part of `go test`.
- **Samples:** `test-data/pyramidal-emails/`. Mail history is marked
  `<!-- history -->` … `<!-- /history -->`, in the format of `eval_samples.go`.
  The markers are removed before the input reaches Pyramidize or a judge.
- **Checks:** `structure`, `info_coverage` (the author's message only) and
  `no_hallucination` (the whole input, with a ratio over output terms the
  history does not hold). Samples with history also get
  `no_history_reproduction`. These checks are `checksVersion` 2.
- **Judges:** `claude-haiku-5-5` and `claude-sonnet-5-5`, both through the
  Claude Code CLI. Their prompt, template, schema and output limit are keyed as
  `judgePromptHash`.
- **Results:** `test-data/eval-runs/<timestamp>/` holds `summary.json`,
  `results.jsonl` and `samples/`. `--runs n` writes
  `test-data/eval-baselines/<timestamp>/baseline.json`.
- **Requires:** a signed-in Claude Code CLI (`claude auth status`), no API key.
  The API route (`EVAL_JUDGE_PROVIDER=claude ./scripts/eval.sh --provider claude`,
  with `ANTHROPIC_API_KEY` in `.env`) is a different instrument again.
  **No API-era baseline compares with any run of this instrument**, whichever
  route it takes.

---

## History

Roughly newest first. A section that ran over several days sits at the date
it started. Present tense in a section below describes the time it was
written: "current reference", "the shipped default", "still open" and
"pending" all mean *then*. Where that has since become false in a way that
misleads, a note in brackets says so. What holds now is the part above.

### Eval Scores — baseline of 2026-10-13 (choice-page prompt)

Control run after E3 phase 3 changed the v3 prompt: `<user_choices>` holds
only the user's instructions (no style or relationship), and instructions may
add content the user states (`<fidelity>` and `<style>` allow it). Same
instrument as 2026-10-12 (`checksVersion` 2, `judgePromptHash`
`j1-fda30fd7bd94a7fa`, CLI 2.1.296), `promptHash` `p1-b42c33b84682302a`,
gitSHA `52f19d8`, three runs, 13 samples, full coverage.
`test-data/eval-baselines/2026-10-13T00-36-32/baseline.json`.

| Metric | 2026-10-12 | 2026-10-13 | `--compare` |
|---|:---:|:---:|---|
| Deterministic | 0.9039 (0.9000–0.9081) | 0.9013 (0.8974–0.9054) | overlap |
| Judge Haiku 5.5 | 0.8536 (0.8523–0.8546) | 0.8544 (0.8492–0.8577) | overlap |
| Judge Sonnet 5.5 | 0.8087 (0.8054–0.8115) | **0.8272** (0.8223–0.8354) | **improvement**, ranges disjoint |
| Samples passing | 10.3 (10–11) | 10 (9–11) | — |

**Verdict: improvement (Sonnet judge), nothing worse.** No regression from
dropping style and relationship. Read it with two limits:

- **The new instructions rule is not exercised.** No sample carries
  instructions, so every run took the "without instructions, keep the
  author's voice" path; what moved is the wording around it (the removed
  style block among it). Whether "content the user states is added" works is
  untested by this suite.
- `no_history_reproduction` stayed 1.0 in all six runs of the two samples
  with history.

---

### Eval Scores — baseline of 2026-10-12 (prompt v3, first run on the mail-history instrument)

Three runs of prompt v3 (email form, `promptHash` `p1-21668d0c1f756770`) on the
instrument of the mail-history change: `sonnet` → `claude-sonnet-5-5` through
the CLI (2.1.296), both judges, 13 samples, full coverage in every run,
`checksVersion` 2, `judgePromptHash` `j1-fda30fd7bd94a7fa`, gitSHA `4d7751d`.
`test-data/eval-baselines/2026-10-12T01-03-13/baseline.json`.

| Metric | Mean | Range |
|---|:---:|:---:|
| Deterministic | 0.9039 | 0.9000–0.9081 |
| Judge Haiku 5.5 | 0.8536 | 0.8523–0.8546 |
| Judge Sonnet 5.5 | 0.8087 | 0.8054–0.8115 |
| Samples passing | 10.3 | 10–11 |

Nothing older compares with it: the checks, the judge prompt and the prompt
all changed. Read the numbers as the starting point, not as a move.

- **The rule holds.** `no_history_reproduction` scored 1.0 in all six runs of
  the two samples with history: no 8-word window of a quoted mail reached the
  output. `email-dataquality-reply-to-feedback`, which appended a recap of the
  quoted mails under the old rule (judges 0.64 and 0.61), now answers without
  one: deterministic 0.98, Haiku 0.82, Sonnet 0.75 (means of three runs).
  `email-diagnose-update`: 0.90, 0.86, 0.89.
- **Weakest samples, deterministically:** `email-passwort erhalten` (0.76,
  `no_hallucination` 0.31–0.50, while both judges give it 0.86–0.90) and
  `email-meeting-request` (0.83, `info_coverage` 0.60–0.63) — the term checks
  [Deterministic check limitations](#deterministic-check-limitations) above
  describes.
- **No v2 run on this instrument.** v3 replaced v2 by the old instrument's
  comparison; a v2 rerun (another 117 calls) is needed only if that comparison
  is to be repeated on the new instrument.

---

### Instrument change — 2026-10-11: mail history is context

**Mail history in the input is context, never part of the output** (owner
decision, [ADR-003 addendum](adr-003-prompt-v3.md)). The prompt changed with it,
and so did the instrument, so that a reply which answers without restating the
mails it answers is not scored down:

- **Samples mark their history** with `<!-- history -->` … `<!-- /history -->`
  lines in the raw input (format in `internal/features/pyramidize/eval_samples.go`):
  `email-diagnose-update` and `email-dataquality-reply-to-feedback`, the only two
  of the 13 that carry quoted or forwarded mails. The markers are removed
  before the input reaches Pyramidize or a judge.
- **`info_coverage` counts key terms of the author's own message only**;
  `no_hallucination` still reads the whole input, so a fact the reply takes
  from the history is not counted as invented, but its ratio leaves out output
  terms the history holds — pasted history used to dilute it and raise the
  score. **`no_history_reproduction`** (samples with history only) fails an
  output sharing 12+ consecutive words with the history or 10 %+ of its 8-word
  windows; its score is 1 minus that share. **`checksVersion` 2** (1 was never
  recorded; the aggregate reads a missing field as 1). A near-miss marker
  (`<!--history-->`, other case, inline, outside the fence) is refused, not
  ignored.
- **The judges are told** that history is context, that the candidate must not
  reproduce it, and that leaving it out is no completeness flaw. Their prompt,
  user-message template, schema and output limit are recorded as one
  **`judgePromptHash`** in `summary.json` and the baseline, and it
  is the last field of the `configKey` (runs before it key as `unrecorded`).
- **Every baseline below reads "not comparable"** against a run on this
  instrument, both 2026-10-11 references included. New v2 and v3 runs were
  pending then; v3 ran on 2026-10-12, v2 has not been rerun.

---

### Instrument change — 2026-10-11: two judges

**Two judges since 2026-10-11 (E3 phase 1).** Every sample is scored by
`claude-haiku-5-5` and `claude-sonnet-5-5`, both through the Claude Code CLI,
and each judge is reported on its own — `summary.json` has `judges`,
`avgJudges` and `judgeCounts`, a baseline has one interval per judge under
`judges`, and nothing averages across them.

- **Pinned by ID.** Neither model has a dated snapshot: the full ID is the pin.
  Each judge records the ID that answered (`resolvedModel`; both smoke calls on
  2026-10-11, CLI 2.1.296, answered with the requested ID), and the aggregate
  names each judge column by that ID, as the key does.
- **The judge set is in the `configKey`**, so **every single-judge baseline
  below reads "not comparable" against a two-judge run** — there were no
  two-judge numbers yet when this was written; the first are the 2026-10-11
  baseline below. `EVAL_JUDGES=a,b,c` chooses the set (a bridge run adds
  `claude-sonnet-4-5-20250929` as a third); `EVAL_JUDGE_MODEL` names a single
  judge.
- **`--compare` judges each metric** (deterministic, then each judge) by the
  interval rule: any metric clearly worse is a regression, and the output names
  which (`regressedMetrics`), so a regression carried by one judge alone reads
  as that; an improvement needs one metric clearly better and none worse. A
  judge with no interval on one side is "not comparable", never an overlap.
- **Temperature.** Through the CLI no judge is pinned (`null` plus a note).
  Through the API (`EVAL_JUDGE_PROVIDER=claude`) the 5.5 judges are not pinned
  either — Anthropic rejects a non-default temperature from the 4.7 generation
  on — and record `null` with a note saying so; the Sonnet 4.5 bridge judge
  keeps temperature 0.
- **The CLI version is reported, not keyed** (`claudeCodeVersion`, next to the
  verdict like `promptHash`): the CLI updates itself, and a keyed version
  would make every baseline incomparable within days.
- **Isolation.** Since #127 every CLI call runs in an empty KeyLint-owned
  directory with Claude Code's auto-memory off. Runs before that, including the
  2026-10-04 baselines below, may have carried the auto-memory of the
  checkout's directory into every call.

---

### Eval Scores — prompt v3 against v2 (2026-10-11)

Three runs of prompt v3 (email form, `promptHash` `p1-31eace752eb07743`),
same instrument as the v2 reference below: `sonnet` → `claude-sonnet-5-5`
through the CLI, both judges, 13 samples, full coverage in every run.
`test-data/eval-baselines/2026-10-11T22-39-34/baseline.json`.

`--compare` refuses the pair (the prompt variant is in the `configKey`), so the
intervals are read by hand with its rule: a metric is better or worse only when
the ranges do not overlap.

| Metric | v2 (reference) | v3 | Reading |
|---|:---:|:---:|---|
| Deterministic | 0.8756 (0.8716–0.8830) | **0.9281** (0.9186–0.9355) | **better**, ranges disjoint |
| Judge Haiku 5.5 | 0.8408 (0.8346–0.8523) | 0.8515 (0.8392–0.8585) | inconclusive, overlap |
| Judge Sonnet 5.5 | 0.8092 (0.7992–0.8254) | 0.7872 (0.7638–0.8023) | inconclusive, overlap of 0.003 |
| Samples passing | 9 (9–9) | 11.7 (11–12) | +2.7, above the "more than 2" bar |

**By the rule, v3 is an improvement: deterministic, with no judge clearly
worse.** Two things to read alongside it:

- **Part of the deterministic gain is the shape.** v3 always gives an email its
  full shape (decision of 2026-10-11) and keeps thread history; the structure
  and coverage checks reward both. The two judges do not see a gain.
- **One sample turned against v3: `email-dataquality-reply-to-feedback`.**
  Deterministic 0.78 → 0.93, Haiku 0.87 → 0.64, Sonnet 0.77 → 0.61. Both judges
  say the same thing: the reply appends a long recap of the two quoted mails it
  answers, which buries the answer. That is v3's "earlier messages in a thread
  are context: they move to the end, they do not drop out" doing what it says.
  `email-diagnose-update`, where v2 dropped needed thread history, goes the
  other way (deterministic 0.70 → 0.93, judges level). How a reply should treat
  the mails it quotes was a product question — **answered the same day:
  history is context, never output** (see "Instrument change — mail history"
  above). This comparison was scored by the old instrument.

### Eval Scores — baseline of 2026-10-11 (two judges, then the reference for v3)

**The reference for E3 phase 2 at the time** (the mail-history instrument made
it "not comparable" later the same day). Three runs of the shipped email prompt
v2 through the Claude Code CLI, scored by both judges. v3 against v2 was
compared against this file:
`test-data/eval-baselines/2026-10-11T21-59-15/baseline.json`.

| Metric | Mean | Min | Max | Spread |
|--------|:---:|:---:|:---:|:---:|
| Deterministic | **0.8756** | 0.8716 | 0.8830 | 0.0113 |
| Judge Haiku 5.5 | **0.8408** | 0.8346 | 0.8523 | 0.0177 |
| Judge Sonnet 5.5 | **0.8092** | 0.7992 | 0.8254 | 0.0262 |
| Samples passing | 9 | 9 | 9 | 0 |

- Pipeline `sonnet` → `claude-sonnet-5-5`, prompt v2, schema off, 13 samples,
  CLI 2.1.296, gitSHA `c10706a`. Both judges scored 13 of 13 in every run; no
  refine call fired (v2 cannot trigger one).
- The two judges disagree in level (Sonnet 5.5 sits about 0.03 below Haiku 5.5)
  and per sample; that is why they are reported separately and never averaged.
- Not comparable with anything below (different judges, and the 2026-10-04
  runs predate the CLI isolation fix). No bridge run was made: the API-era
  baselines are not comparable with this instrument on any route, so a third
  judge would have bought nothing.

### Eval Scores — baseline of 2026-10-04 (Claude Code CLI — a new instrument)

> **Possibly affected, superseded.** These runs predate the CLI isolation fix
> (#127): Claude Code's auto-memory for the checkout's directory may have been
> injected into every pipeline and judge call. They are superseded by the
> two-judge baselines of 2026-10-11 and will not be re-recorded. The numbers stay as a record;
> read nothing into them beyond that.

**Every eval call goes through the owner's subscription now.** Pipeline and
judge both run through the installed Claude Code CLI (`claude-code`); no API key
is read. That was the owner's decision, made knowing that it costs the judge its
temperature pin and that **every number below this section becomes history**:
none of the API-era rows is comparable with this one, and `--compare` refuses
the pair (exit 2) instead of printing a delta.

What makes it a different instrument:

- **The judge is not pinned to a temperature.** It is still the dated snapshot
  `claude-sonnet-4-5-20250929` — the CLI takes the dated ID and answers with it
  (`modelUsage` named it in every call) — but the CLI has no temperature flag.
  `summary.json` records `"temperature": null` and a note rather than a 0 that
  never reached the model, and the temperature is part of the `configKey`. The
  judge's spread below (0.0662) is nearly three times Sonnet 4.6's 0.0231 and
  about 1.4 times Sonnet 5's 0.0477 through the API. Why is unconfirmed: the
  missing pin, the possible auto-memory leak, or both.
- **The pipeline runs on the alias users get.** `sonnet` resolved to
  `claude-sonnet-5-5` in every call of all three runs. That is a newer
  generation than any API-era row, so the gap below is model *and* route *and*
  judge sampling at once — it cannot be attributed to any one of them.
- **Resolved IDs are recorded and keyed.** `resolvedModel` and
  `judge.resolvedModel` come from the CLI's `modelUsage`; the `configKey` uses
  them in place of the requested names. When `sonnet` moves to the next
  generation, a comparison against this baseline reads "not comparable" rather
  than crediting the move to the prompt.

Three runs, `claude-code` / `sonnet` → `claude-sonnet-5-5`, judge `claude-code` /
`claude-sonnet-4-5-20250929` (temperature unpinned), prompt v2, schema
enforcement off, threshold 0.65, 13 samples, gitSHA `f8e362c`. Baseline in
`test-data/eval-baselines/2026-10-04T22-01-17/baseline.json`.

| Metric | Mean | Min | Max | Spread |
|--------|:---:|:---:|:---:|:---:|
| Avg deterministic | **0.8780** | 0.8706 | 0.8836 | 0.0131 |
| Avg judge overall | **0.7682** | 0.7323 | 0.7985 | 0.0662 |
| Samples passing | 9.7 | 9/13 | 10/13 | 1 sample |

All 13 documents produced in every run; the refine step never fired (0 calls).
Failing in all three runs: `email-dataquality-reply-to-feedback`,
`email-diagnose-update`, `email-meeting-request`.

Next to the API-era rows, **labelled not comparable** — different route,
different judge sampling, and for the pipeline a different model:

| Configuration | Deterministic | Judge | Passing |
|---|:---:|:---:|:---:|
| **CLI: Sonnet alias → `claude-sonnet-5-5` + v2 (2026-10-04)** | **0.8780** (0.8706–0.8836) | **0.7682** (0.7323–0.7985) | 9–10 |
| API: Sonnet 4.6 + v2 (2026-09-18) | 0.8353 (0.8209–0.8446) | 0.8485 (0.8338–0.8569) | 7–9 |
| API: Sonnet 5 + v2 (2026-09-18) | 0.8651 (0.8606–0.8683) | 0.8251 (0.8000–0.8477) | 8–10 |
| API: Opus 5 + v2 (2026-09-25) | 0.8878 (0.8848–0.8895) | 0.8197 (0.8123–0.8238) | 10 |

*Unconfirmed, given the note above:* the shape resembled what Opus 5 showed —
the deterministic checks liking the newer model, the judge marking it down,
with rationales about subject lines that pack several messages. Whether that
survives an isolated run is for the two-judge baseline to show. (Not
followed up.)

**Cost:** 13 samples × (one pipeline call + one judge call) × 3 runs on the
subscription, 12.5–14 minutes per run — close enough to the old 900 s `go test`
limit that `eval.sh` raised it (3600 s then; 10800 s since the second judge — the Fix suite did lose a run to the old limit).
CLI version on the machine at the time: 2.1.289. These baselines predate the
`claudeCodeVersion` field, so the files do not carry it; runs recorded from now
on do, and `--compare` names a CLI version change next to the verdict.

---

### Eval Scores — baseline of 2026-09-18 (API era — history, not comparable with any CLI-era run)

**Three runs of the same commit, same model, same judge.** Run individually they
report 0.82, 0.84 and 0.84 on the deterministic checks — that variation is the
provider's, not the pipeline's, and it is the reason this table exists in this
shape.

| Metric | Mean | Min | Max | Spread |
|--------|:---:|:---:|:---:|:---:|
| Avg deterministic | **0.8353** | 0.8209 | 0.8446 | 0.0237 |
| Avg judge overall | **0.8485** | 0.8338 | 0.8569 | 0.0231 |
| Samples passing | — | 7/13 | 9/13 | 2 samples |

#### Other configurations measured (2026-09-18)

Same suite, same judge, three runs each. Reported side by side because
`--compare` refuses every one of these pairs: it requires both sides to share
model, prompt variant and the rest of the configuration, and these differ in
exactly those fields. See [ADR-002](adr-002-one-shot-vs-pipeline.md).

| Configuration | Deterministic | Judge | Documents produced | Passing |
|---|:---:|:---:|:---:|:---:|
| Sonnet 4.6 + v2 — the baseline above | 0.8353 (0.8209–0.8446) | 0.8485 (0.8338–0.8569) | 13 of 13 | 7–9 |
| Sonnet 5 + v2 (one-shot, shipped prompt) | 0.8651 (0.8606–0.8683) | 0.8251 (0.8000–0.8477) | 13 of 13 | 8–10 |
| Sonnet 5 + v1 (pipeline, self-QA + refine) | 0.7726 (0.7326–0.8058) | 0.8290 (0.8125–0.8417) | 11–12 of 13 | 9–10 |
| Sonnet 5 + v1, completed documents only | 0.8611 (0.8444–0.8730) | — | — | — |
| Opus 5 + v2 (2026-09-25, Pyramidize limit 16000 — see note) | 0.8878 (0.8848–0.8895) | 0.8197 (0.8123–0.8238) | 13 of 13 | 10 |

**Opus 5** (`test-data/eval-baselines/2026-09-25T00-47-48/`, gitSHA `df1ed3d`, three runs):
deterministic is clearly above both Sonnet rows — its range overlaps neither. The
judge is **below Sonnet 4.6 with disjoint ranges** and overlaps Sonnet 5, so
against 4.6 this is "improvement: deterministic, regression: judge". It is the
first model here with a judge *regression* by this document's rule: Sonnet 5's
judge range (0.8000–0.8477) overlaps 4.6's, which makes its lower mean
inconclusive, not a regression. The judge's rationales repeatedly mark down the
same thing on the low-scoring samples: a subject line that packs several
messages instead of leading with one. Read that as a hint about where the
disagreement lies, not as a finding: it is one judge model's taste, and the
deterministic checks reward informative subjects. Opus 5 ran with the
Pyramidize output limit at 16000 (the other rows ran at 4096); v2 produced all
13 documents in every run, so the limit did not decide any sample here, but
`--compare` cannot see that difference — `max_tokens` is not in `summary.json`.

Two readings that are easy to get wrong here:

**v1's low deterministic score is failed documents, not bad writing.** One or two
of the thirteen samples hit `stop_reason: max_tokens` against the shared
`maxTokens = 4096` in every run. A zeroed sample costs a 13-sample mean about
0.065. Scored over what it produced, v1 sits at 0.8611 (0.8444–0.8730); v2 over
the same samples is 0.8707 (0.8618–0.8775). Overlapping, therefore inconclusive.
Note also that the v1 row mixes denominators: its deterministic mean is over all
13 with zeros, its judge mean over the 11–12 that completed. What filled the
budget was thinking, not the document: Sonnet 5 thinks by default when a request
says nothing about thinking, every thinking token counts against `max_tokens`,
and the thinking block comes back empty, so a response showed a small document
and hid the rest. This stood here as unexplained until 2026-09-25 — see
[Why Sonnet 5 hit the output limit](#why-sonnet-5-hit-the-output-limit).

**Sonnet 5 is not simply better than 4.6 on the shipped prompt.** Deterministic
is clearly up — the intervals do not overlap. The judge is inconclusive, its mean
is lower, and its spread roughly doubles (0.0477 against 0.0231). By this
document's own rule that is "improvement: deterministic" — an improvement in one
metric with no regression in the other. The default model is unchanged anyway,
because the work that measured it was asked for a stricter bar; see
[ADR-002](adr-002-one-shot-vs-pipeline.md), which records both readings.
(Since 2026-10-08 the default is the `sonnet` alias, E2 step 5.)

**These rows measure the explicitly typed email path.** The shipped default is
`auto`, which spends a detect call first — two calls, or three if detection
picks a document type that still runs self-QA. (Then. Since v3, `auto` is a
preset lookup, not a call, and every run is one call: ADR-003.)

#### Why Sonnet 5 hit the output limit

**Single API calls, not an eval run.** Each row below is one request, made on
2026-09-24/25 with the exact foundation-step request the eval sends (email, v1 or
v2, `professional`/`professional`, no schema), captured from `Pyramidize` and
replayed through the SDK so `stop_reason` and `usage` could be read. There are
no scores here and none of these numbers is comparable with the tables above.
"Visible" is the returned text measured with `count_tokens` (a few tokens of
message framing included); "thinking" is output minus visible. Every Sonnet 5
response carried a thinking block with empty text.

| Sample | Prompt | Model | `max_tokens` | `stop_reason` | Output | Visible ≈ | Thinking ≈ |
|---|:---:|---|:---:|---|:---:|:---:|:---:|
| reply-to-feedback | v1 | Sonnet 5 | 4096 | end_turn | 3672 | 774 | 2898 |
| reply-to-feedback | v1 | Sonnet 5 | 4096 | end_turn | 2352 | 709 | 1643 |
| project-status | v1 | Sonnet 5 | 4096 | end_turn | 3526 | 1219 | 2307 |
| project-status | v1 | Sonnet 5 | 4096 | end_turn | 3808 | 1257 | 2551 |
| reply-to-feedback | v1 | Sonnet 5 | 16000 | end_turn | 2794 | 673 | 2121 |
| reply-to-feedback | v1 | Sonnet 5 | 16000 | end_turn | 3421 | 707 | 2714 |
| project-status | v1 | Sonnet 5 | 16000 | end_turn | 3584 | 1148 | 2436 |
| project-status | v1 | Sonnet 5 | 16000 | end_turn | 3974 | 1230 | 2744 |
| reply-to-feedback | v2 | Sonnet 5 | 4096 | end_turn | 3082 | 820 | 2262 |
| project-status | v2 | Sonnet 5 | 4096 | end_turn | 3822 | 904 | 2918 |
| reply-to-feedback | v1 | Opus 5 | 4096 | end_turn | 1460 | 725 | 735 |
| reply-to-feedback | v1 | Sonnet 5 | 2048 | **max_tokens** | 2048 | 0 | 2048 |
| project-status | v2 | Sonnet 5 | 2048 | **max_tokens** | 2048 | ~305, cut off | ~1743 |

What this shows:

- **Thinking is 60–80 % of a Sonnet 5 Pyramidize reply** on the two largest
  samples. The document itself is 700–1250 tokens, which is why every produced
  document looked far under budget.
- **At 4096 the call runs close to the ceiling.** The highest total at 4096 was
  3822, 274 tokens short (3974 was seen at 16000, where the limit is not in
  play). How long the model thinks varies from call to call, so a
  share of calls going over is what the eval's "ordinary document, or a runaway
  past 4096" pattern looks like — no loop in the generation needed. At 2048 the
  cut-off is reproduced outright, with not one visible character on v1.
- **v2 is exposed as well.** Its two calls here spent 3082 and 3822. On Sonnet 5
  v2 lost no document in three eval runs (the other three of ADR-002's "six"
  were Sonnet 4.6, which does not think by default); with two calls it cannot
  be said whether v2 thinks less than v1 or had more luck. ADR-002 lists this
  ("a prompt property that v2 shares latently") as something that would weaken
  its reliability argument.
- **None of the eight v1 calls at 4096 or higher went over**, where the
  2026-09-18 runs lost 4 of 6 attempts on these two samples. Eight calls are too
  few to say whether today's thinking is shorter or 09-18 was unlucky; the
  mechanism does not depend on which.
- **Opus 5 thought far less on the same request** (735 against 1643–2898) —
  one call, indicative only.

The self-QA *instruction* that ADR-002 named as a candidate is therefore not
needed to explain the failures. Whether v1's prompt raises the failure rate —
by making the model think longer — is still open: 4 of 6 v1 attempts against 0
of 6 v2 attempts on these samples on 2026-09-18 says it may, and eight calls
today say nothing either way. (Moot since v3: v1 was removed.)

**What changed:** Pyramidize's `max_tokens` is now 16000, up from 4096. It is a
ceiling, not a spend, and the model does not see it: the four v1 calls at 16000
above spent about what those at 4096 did (2794–3974 against 2352–3808).
Thinking and effort were left at the model's default on purpose — turning them
down changes output quality and belongs to an eval-gated decision (E3, #34). The
Pyramidize judge now has its own limit of 4096, what it sent before, so the
instrument did not move with the pipeline.

Do not expect a failed eval run to name thinking as the cause from now on. At
16000 inside a 90 s per-attempt timeout, a thinking runaway ends as "context
deadline exceeded", which carries no usage at all.

**Runs from before this change are not comparable on this point, and
`--compare` cannot tell.** Every baseline up to and including 2026-09-18 ran at
4096; `max_tokens` is not in the recorded configuration. A v1 run on Sonnet 5
that no longer zeroes a sample will look like a gain against those baselines —
read it as the limit first, not the prompt or the model.

For Pyramidize the tighter bound is now time: each attempt has a 90 s timeout,
which at the 90–125 tokens/s seen here is roughly 8000–11000 tokens. A
generation that runs longer ends as a timeout — retried once by the SDK within
the feature's 120 s overall deadline — rather than as an output-limit error.

**Fix is unchanged at 2048, on purpose, and it has the same problem.** Sonnet 5
on a 3.7 KB selection spent all 2048 on thinking twice out of two and returned
no text; at 16000 the same request finished with 5361 output tokens after about
45 s. (The one Opus 5 Fix call, on a short sample, finished at 223 tokens; Opus
5 was not tried on the long selection.) Fix is a silent hotkey: a fix that
succeeds after most of a minute, with no feedback, pastes into whatever window
has focus by then. Whether Fix should fail fast or wait for every answer is an
open product decision, so it keeps its limit. (Settled since by #93/#110: it
waits with a tray dot and pastes only into the source window; roadmap E2.) What changed is the message: a
reply spent entirely on reasoning now says so and tells the user to pick a
faster model or shorten the text, instead of only "try a shorter selection".

#### The noise floor

**A single-run delta of less than 0.024 (deterministic) or 0.023 (judge) is not
evidence of anything.** Those numbers are `max − min` over three runs — the full
observed interval, not a ± half-width and not an error bar from a model of the
noise. A difference that size has already been produced without a line of the
pipeline changing.

Better still, do not compare means at all. `--compare` asks the only question
three runs can answer: does the new interval overlap the old one? A new set whose
worst run still beats the baseline's best run is a real improvement; one whose
best run loses to the baseline's worst is a real regression; anything that
overlaps is inconclusive, however far the means have moved.

The pass count is worse: it moved between **7 and 9 of 13** across those same
three runs. "9 of 13" in the March table below was one run's number. Do not
compare pass counts at all unless the difference is larger than 2.

Use `./scripts/eval.sh --runs 3` for anything you intend to quote, and
`./scripts/eval.sh --compare <baseline.json>` to have the comparison made for
you — it stays silent inside the noise floor and exits non-zero only past it.

#### Configuration

| | |
|---|---|
| Provider / model | `claude` / `claude-sonnet-4-6` (the default until 2026-10-08; it is now the `sonnet` alias, recorded as the resolved ID, so newer runs read "not comparable" against this baseline) |
| Judge | `claude` / `claude-sonnet-4-5-20250929` @ temperature 0 |
| Prompt variant | v2 |
| Schema enforcement | false |
| Quality threshold | 0.65 |
| Samples | 13 |
| Code | working tree of `ab1dc68`, committed as the `chore/eval-baseline` branch |

The summaries record `ab1dc68-dirty` because the runs were made from the working
tree that became this branch, before it was committed. Everything else in them
is enough to reproduce the run; see `test-data/eval-baselines/2026-09-18T04-03-05/baseline.json`.

**The pipeline model is an alias and cannot be pinned.** The account lists
`claude-sonnet-4-6` with no dated form behind it, so what that name resolves to
can change without a commit here — which is one of the things #53 suspected. The
judge is pinned to a dated snapshot precisely because the instrument must not
move; the pipeline stays on the alias because that is what users get. When the
numbers shift and nothing in this repository did, suspect that alias first.

#### Run folders

- `test-data/eval-runs/2026-09-18T03-53-02`
- `test-data/eval-runs/2026-09-18T03-56-20`
- `test-data/eval-runs/2026-09-18T03-59-47`

`test-data/eval-runs/` is gitignored, so these exist only on the machine that
produced them. The aggregate is committed.

#### Per sample

Sorted by score. The range columns are `max − min` over the three runs, per
sample — which is where the variance actually lives.

| Sample | Deterministic | Range | Judge | Range |
|--------|:---:|:---:|:---:|:---:|
| email-diagnose-update | 0.74 | 0.08 | 0.88 | 0.00 |
| email-meeting-request | 0.76 | 0.10 | 0.73 | 0.10 |
| email-adverity-field-structuring | 0.77 | 0.22 | 0.83 | 0.08 |
| email-dataquality-reply-to-feedback | 0.77 | 0.02 | 0.82 | 0.18 |
| email-passwort erhalten | 0.78 | 0.08 | 0.88 | 0.00 |
| email-onboarding-erfolgreich | 0.82 | 0.02 | 0.90 | 0.00 |
| email-dataquality | 0.85 | 0.09 | 0.85 | 0.08 |
| email-software-deployment | 0.88 | 0.03 | 0.90 | 0.00 |
| email-airbyte-escalation | 0.88 | 0.04 | 0.87 | 0.08 |
| email-dataquality-followup | 0.88 | 0.08 | 0.86 | 0.06 |
| email-answer-info-update | 0.89 | 0.05 | 0.77 | 0.05 |
| email-project-status | 0.90 | 0.11 | 0.84 | 0.06 |
| email-kpi-request | 0.94 | 0.02 | 0.90 | 0.05 |

`email-adverity-field-structuring` alone moves by 0.22 between identical runs. A
change that moves the overall mean by 0.02 may well be that one sample having a
good day.

#### What the March numbers were, and why they looked like a regression

| Metric | March 2026 | Now (3 runs) |
|--------|:---:|:---:|
| Avg deterministic | 0.8162 and 0.8379 (two Sonnet 4.6 runs) | 0.8353 (0.8209–0.8446) |
| Avg judge overall | 0.8915 and 0.8969 | 0.8485 (0.8338–0.8569) |
| Samples passing | 8/13 in both archived runs | 7–9/13 |

#53 reported an unchanged `main` scoring 0.768 against the documented 0.82 and
read it as a regression. The evidence does not support that reading, for three
reasons of decreasing certainty:

**1. March was never one number.** The archived runs hold two complete Sonnet
4.6 runs from that week: `2026-03-29T22-03-24` at 0.8162 and
`2026-03-31T23-56-48` at 0.8379. Those two span 0.022 — the same interval as the
three September runs. The table said 0.82 because someone had to pick one. (It
also said 9 of 13 passing; both archived runs pass 8, so that figure came from a
third run or from nowhere. Which is the point.)

**2. Strictly, they cannot be compared at all — and that is the lesson.**
Running `./scripts/eval-aggregate.sh` over those two March runs exits 2: their
summaries predate the recorded configuration, so `promptVariant`, `judge` and
`schemaEnforcement` are missing, and the tool refuses to average runs whose
configuration it cannot establish. Every number in this section from before
September is therefore an indication, not a measurement. That refusal is the
feature: it is what stops the next person building an argument on two runs that
may not have measured the same thing.

**3. The judge column was never comparable either.** March ran an unpinned
judge; this baseline pins `claude-sonnet-4-5-20250929` at temperature 0. The
0.89 and the 0.8485 are two different instruments, and the gap between them says
nothing about the pipeline.

**The 0.768 is consistent with a bug that is now fixed, but the run is gone.**
#53 records that `email-dataquality-reply-to-feedback` failed in it with
`invalid character '(' after array element`. A failed sample scores zero, and
that sample averages 0.768 across the September runs, so losing it costs a
13-sample mean about 0.059 — while the reported gap to the March table was
0.052. The arithmetic fits, but the run directory it refers to no longer exists
on any machine here, so this is a plausible reconstruction and not a
measurement. What *is* verifiable: #54 fixed the parser (`unmarshalRobust`
repairs array elements, with a regression test), and across the three September
runs there were **no sample errors at all**.

### Superseded: the March table

Kept for provenance. Produced by a single run, with an unpinned judge, before
the parser fix — not a baseline to compare against.

#### March 2026 numbers (2026-03-29)

| Metric | Baseline | After Improvements | Opus |
|--------|:---:|:---:|:---:|
| Avg deterministic | 0.40 | **0.82** | 0.84 |
| Avg judge overall | 0.89 | **0.89** | 0.89 |
| Samples passing | 0/13 | **9/13** | 9/13 |
| Structure pass rate | 0/13 | 13/13 | 13/13 |
| Hallucination pass rate | 0/13 | 12/13 | 13/13 |

---

### Improvements made (March 2026)

Prompt v2 era, before the pipeline was dropped; the prompt items describe
prompts v3 replaced.

#### Deterministic Check Calibration
- Bold header detection (email prompts produce `**bold**` not `#` markdown)
- German compound noun decomposition (hyphen/slash/prefix-suffix splitting)
- Business vocabulary exclusion list (~80 common German/English restructuring terms)
- Percentage-based hallucination threshold (15% of output terms, not absolute count)
- Case-insensitive test-data parsing with typo tolerance

#### Prompt Improvements (All 4 Doc Types)
- V1-derived structure rules: explicit structure template (header + bullet points, not prose)
- Standalone content-statement headers (UBERSCHRIFTEN-REGELN from v1 archive)
- Analysis phase: scan all input for relevant info before restructuring
- Style rules: compact, bullets for details, no filler
- Tone preservation: no formality escalation, no person-switch, no editorial additions
- Fidelity specialist added to self-eval (5th specialist lens)
- Subject line guidance: 3 segments ideal, more is acceptable but reduces scannability

---

### Open issues as of March 2026

What became of them, as of 2026-10-13 (v3, `checksVersion` 2):

- `email-diagnose-update`'s dropped history was part of the question that
  2026-10-11 answered with "history is context". It now passes 2 of 3 runs,
  and the judges give it 0.87 and 0.89.
- `email-dataquality-reply-to-feedback` passes every run at 0.98: the reply
  answers without a recap, and coverage counts only the author's message.
- `email-adverity-field-structuring` passes every check (0.94), but it is now
  the judges' lowest sample.
- `email-meeting-request` still fails every run (on coverage, when last read
  on 2026-10-12).

The model behaviours below were seen on Sonnet 4.6 with v2 and have not been
re-checked on v3. The current list is [Where it is weak](#where-it-is-weak).

#### 4 Failing Samples (info_coverage)

| Sample | Coverage | Judge Overall | Root Cause |
|--------|:---:|:---:|---|
| email-diagnose-update | 29% | 0.82 | Model drops mail-thread history context |
| email-dataquality-reply-to-feedback | 38% | 0.89 | Very long input, key-term extraction noise |
| email-meeting-request | 43% | 0.93 | Casual spoken input, many informal terms |
| email-adverity-field-structuring | 75% | 0.80 | URL fragments in key terms |

Note: Only `email-diagnose-update` is a real quality problem (judge completeness 0.78). The other 3 are measurement noise — judge scores are high (0.80-0.93).

#### Persistent Model Behaviors
- **Process-label headers:** Despite explicit rules, model occasionally uses "Erhoffte Erkenntnisse", "Nächste Schritte" instead of content statements
- **Editorial additions:** "Kein Handlungsbedarf" added when not in original input
- **Tone drift:** Short informal emails get slightly over-formatted with bold headers and bullets
- **Opus handles mail history better:** Judge completeness 0.93 vs 0.82 (Sonnet) on diagnose-update — partly a model capability gap

The deterministic check limitations listed here in March still hold; they
moved to [Deterministic check limitations](#deterministic-check-limitations)
above.

---

### Quality improvement roadmap (March 2026)

Superseded by [Next](#next) above. Kept for the record:

1. ~~**V1 specialist architecture** — Reimplement 4 parallel specialist agents in Go for independent quality verification (see ADR-001)~~ — **parked.** [ADR-002](adr-002-one-shot-vs-pipeline.md) found the 2-call pipeline buys no measured quality over a single call for email; a 6-call architecture needs evidence before it is proposed again. `roadmap.md` already parks it. ADR-003 then removed the pipeline altogether.
2. **Semantic eval checks** — Python sentence-transformer script for eval, replacing term overlap
3. **Inline deterministic QA** — Move existing checks into live pipeline as post-LLM gate
4. **NLP info-loss detection** — Sentence embeddings via hugot/ONNX for live QA (Tier 2)
