# ADR-002 — One-shot versus pipeline for email

> Status: accepted · Date: 2026-09-18 · Supersedes part of [ADR-001](adr-001-pipeline-architecture.md) · Issue: #34

## The question

ADR-001 designed Pyramidize as a 1–3 call pipeline: detect → foundation with an
embedded self-QA block → conditional refine when the model's own quality score
falls below a threshold. #34 asks whether a current frontier model makes a single
well-structured call good enough to retire that machinery.

## What the code already does

**The explicitly typed email path is one call.** Prompt variant v2 is the
default (`LatestEmailVariant = 2`); it drops the self-QA block and its schema is
`{fullDocument, headers, language}`. The refine trigger is
`QualityScore < threshold && len(QualityFlags) > 0`, so with no flags it cannot
fire.

**The shipped default is not that path.** Both the GUI dropdown
(`text-enhancement.component.ts:54`) and the CLI `-type` flag
(`internal/cli/pyramidize.go:33`) default to `auto`, which runs a detect call
first — the UI says "Step 1/2: Detecting…" while it happens. So a user who
pastes a mail and presses go spends **two** calls, and if detect answers `memo`,
`wiki` or `powerpoint`, that branch still carries self-QA and can refine: **up to
three**.

**And even one call is a matter of prompt compliance, not structure.** Schema
enforcement is off by default, so the model is sent no schema at all;
`foundationResult` has json tags for `qualityScore` and `qualityFlags`, and
`unmarshalRobust` fills them from whatever comes back. A model that volunteers
those fields walks straight into the refine gate. `TestTheOneShotPropertyDependsOnPromptCompliance`
demonstrates this against the real `Pyramidize` entry point.

The eval passes `DocumentType: "email"` explicitly, so **everything measured
below is the one-call path** — the best case, not the shipped default.

That still makes "build a v3 single-call variant" the wrong experiment: for the
typed path it would compare one one-shot prompt against another. **No v3 was
built.** The comparison #34 wants already exists and needed no new prompt code:
**v1 is the pipeline** (it emits the quality fields, so refine fires) and **v2 is
the one-shot**.

## Method

`./scripts/eval.sh --runs 3` per configuration, 13 German business email samples
(one mixes German and English heavily; the prompts preserve code-switching on
purpose), judge pinned to `claude-sonnet-4-5-20250929` at temperature
0, schema enforcement off. Every number is an interval over three runs.

No prompt was changed. No prompt was tuned to any sample.

## Results

| Configuration | Deterministic | Judge | Documents produced | Passing |
|---|:---:|:---:|:---:|:---:|
| Sonnet 4.6 + v2 (one-shot) | 0.8353 (0.8209–0.8446) | 0.8485 (0.8338–0.8569) | 13 of 13 | 7–9 of 13 |
| Sonnet 5 + v2 (one-shot) | **0.8651** (0.8606–0.8683) | 0.8251 (0.8000–0.8477) | **13 of 13** | 8–10 of 13 |
| Sonnet 5 + v1 (pipeline) | 0.7726 (0.7326–0.8058) | 0.8290 (0.8125–0.8417) | **11–12 of 13** | 9–10 of 11–12 |

### Read the denominators before the numbers

They are not the same across that table, and two of them flatter v1:

- **v1's deterministic mean is over all 13**, with a zero for each sample that
  produced nothing. **v1's judge mean is over the 11–12 that completed** — the
  eval divides by `judgeCount`. Two denominators in one row.
- **v1's pass count is out of what it produced**, v2's out of 13. Per produced
  document that is roughly 80% against 72% — but `quality-status.md` says not to
  compare pass counts unless they differ by more than 2, and these do not.
- `--compare` refuses this pair twice over: the configurations differ (prompt
  variant), **and** the judge did not score every sample on the v1 side, which is
  its own refusal. The hand comparison below walks into the second one; it is
  reported, not hidden.

### The pipeline loses documents, and on quality it is inconclusive

In every run, one or two of thirteen came back as

```
foundation step failed: Claude: the result exceeded the output limit
```

— `stop_reason: max_tokens` against the shared `maxTokens = 4096`. One zero
costs a 13-sample mean about 0.065, which is most of the gap between 0.7726 and
0.8651.

Scored over the documents it produced, v1 is 0.8611 (0.8444–0.8730) against the
0.8651 headline for v2 — but those are different denominators. Two comparisons
that are not:

| Both arms restricted the same way | v1 | v2 |
|---|:---:|:---:|
| Excluding the two samples that ever failed under v1 | 0.8626 (0.8373–0.8848) | 0.8707 (0.8618–0.8775) |

Overlapping, comfortably. The judge intervals overlap too (0.8125–0.8417 against
0.8000–0.8477), and there the v1 figure is already completed-only because the
eval divides by `judgeCount`.

- **Quality: inconclusive.** Per document produced, the pipeline and the one-shot
  are indistinguishable on this suite.
- **Reliability: the one-shot wins, and not narrowly.** v1 dropped 1–2 of 13 on
  every run; v2 dropped none in six runs across two models.

### The failure mode is not explained

Four failures, not three: `email-project-status` and
`email-dataquality-reply-to-feedback` each failed in two of the three runs (1, 1
and 2 per run).

Both are large inputs: reply-to-feedback is the suite's **largest** raw input
(3537 characters, 3683 bytes) and project-status its **third** (2273 / 2299). An
earlier draft called the second "mid-sized" — that was `wc -c` on the sample
files, which include the reference output, and it was wrong.

Size alone does not explain it, and neither does the self-QA **payload**:
`"qualityScore": 0.92, "qualityFlags": []` is roughly fifteen tokens. The largest
document any run produced is 2585 bytes, comfortably under a quarter of the
4096-token budget, and on the two failing samples v1's successful outputs are
0.93× and 1.10× the size of v2's. The failures are bimodal — an ordinary
document, or a runaway past 4096 tokens — which is a generation that does not
terminate, not a ceiling gradually approached.

What that argument does **not** rule out is the self-QA *instruction*.
`prompts_selfqa.go` tells the model to assess its output "as five internal
specialists" and to compute a score from their sub-scores before responding.
Fifteen tokens of result do not imply fifteen tokens of work: a model that
narrates that assessment, or restates the document to grade it, produces exactly
the bimodal shape observed. **This is a named candidate, not a finding** — it was
not tested, and testing it means a run with the self-QA block removed from v1,
which is a prompt change and therefore its own experiment.

**Why v1 runs away on the longest inputs and v2 does not is unexplained.** It is
reproducible — four failures across three runs, concentrated on two samples — and
it is the strongest argument against v1, but what is recorded here is a
description, not a diagnosis.

> **Addendum 2026-09-25 — explained.** The budget went on thinking. Sonnet 5
> thinks by default, thinking counts against `max_tokens`, and the thinking block
> comes back empty, so only the small document was visible. Single calls on the
> two failing samples spent 2352–3974 output tokens against a visible document of
> about 700–1250; at 2048 the cut-off reproduces with no visible text. v2 spent
> 3082 and 3822 on the same two samples, so it shares the exposure, which
> weakens the reliability argument (see "What would change these decisions").
> The mechanism is the model's default; whether v1's prompt raises the rate is
> still open. The limit is now 16000; thinking and effort are unchanged. Evidence and caveats: [quality-status.md](quality-status.md#why-sonnet-5-hit-the-output-limit).

### Whether the pipeline arm ever made a second call is unrecorded

`AppliedRefinement` was not written into the run records, so this comparison
cannot show that refine ever fired. The latency figures bound it loosely — if
refine had fired often, v1 would be much slower than the ~1.6 s/sample difference
observed — which leaves open that both arms were effectively one-shot and the
experiment measured prompt wording after all.

The eval now records `appliedRefinement` per sample and `eval-aggregate.sh`
projects the per-run count into `baseline.json`, so it survives the gitignored
run folders. For runs recorded before the field existed — including every run
behind this ADR — the aggregate reports `null` rather than zero, because "nobody
wrote it down" is not the same claim as "it never fired". **This comparison
cannot answer it.**

### Cost and latency

v1 spends strictly more per call (self-QA fields) plus a second call whenever
refine fires. Wall time per sample, scraped from one run's console output of each
and including the judge call in both, was 22.9 s for v2 over 13 samples and
24.5 s for v1 over the 11 that completed. Nothing records timing, so these are
indicative only, they are not product latency, and they should not be quoted.

### Sonnet 5 against Sonnet 4.6 on the shipped prompt

- **Deterministic: a clean improvement.** The intervals do not overlap — Sonnet
  5's worst run (0.8606) beats Sonnet 4.6's best (0.8446).
- **Judge: inconclusive and noisier.** The intervals overlap, the mean is lower
  (0.8251 against 0.8485), and the spread roughly doubles (0.0477 against
  0.0231).

Applying this project's rule **by hand** — an improvement in one metric with no
regression in the other — that reads as "improvement: deterministic". The script
will not say so: `eval-aggregate.sh --compare` puts the model in its
`configKey`, so across two models it answers **"not comparable: different
configuration"** and never reaches the verdict branch. The reading above is
mine, by the script's rule, not the script's output.

The task that commissioned this work asked for a stricter bar — better on *both*
intervals — before recommending a default switch, and that bar is not met. Both
readings are recorded because they differ, and the difference is the decision.

## Decisions

1. **Keep v2 for email. Do not build a v3.** The typed email path is already
   one-shot and the pipeline buys no measured quality on this suite.
2. **The pipeline machinery stays**, because `memo`, `wiki` and `powerpoint`
   still use it. ADR-001's pipeline rationale is superseded for email only.
3. **AUTO remains the shipped default and remains two calls.** This ADR does not
   change it: nothing here measures detection accuracy, and defaulting to `email`
   would be a product decision made on no evidence. It is recorded as the reason
   the headline claim is about the typed path only.
4. **Do not switch the default model in this change.** The deterministic result
   argues for Sonnet 5; the judge result does not. Worth its own PR with the
   trade-off stated — see the two readings above.
5. **`PyramidizeQualityThreshold` is dead for the default document type.** The
   settings slider is live and persisted, and for email v2 it can never do
   anything. Kept for the other three types; it should not be presented to users
   as if it applied to what they are most likely restructuring.
6. **`-pyramidize --json` reports `qualityScore: 0` for every v2 email.** CLAUDE.md
   advertises that flag as "JSON output with quality score". The field is the Go
   zero value because nothing produces it. Either stop emitting it for v2 or
   document that it is meaningless — it is currently a documented output that
   lies. Not fixed here; it is a behaviour change, not a measurement.

## What this ADR did not test

- **Schema enforcement.** The roadmap's v3 was "single call, *structured output*"
  — all three baselines ran with `schemaEnforcement: false`. Whether enforcing
  the schema changes quality is untested and remains open under #47/#34.
- **Opus 5.** The roadmap asks for a re-baseline on Sonnet 5 *and* Opus 5. Only
  Sonnet 5 was run.
- **memo, wiki, powerpoint.** All three still append `selfQABlock` and use
  `documentSchema`, and none has any eval coverage — the suite is 13 email
  samples. Retiring the refine stage outright needs samples for them, or a
  decision to move them to a v2-style prompt on the email result alone.
- **Detection accuracy**, which is what would justify changing the AUTO default.

## What would change these decisions

- Eval samples for memo/wiki/powerpoint showing the pipeline earns its keep
  there. Without them, decision 2 is inertia rather than evidence.
- An explanation for the v1 runaway. If it turns out to be a prompt property that
  v2 shares latently, the reliability argument weakens.
- A run with `appliedRefinement` recorded, showing refine fired often enough that
  the v1 arm really was a pipeline.
- A judge result on a later model where the pipeline's interval sits entirely
  above the one-shot's.
