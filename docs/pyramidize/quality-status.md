# Pyramidize Output Quality — Status & Open Issues

> Last updated: 2026-09-25

> The Fix prompt has its own suite and baseline: [`docs/fix/quality-status.md`](../fix/quality-status.md).

---

## Eval Infrastructure

- **CLI mode:** `-fix` and `-pyramidize` commands dispatch headlessly from `main.go`
- **Eval framework:** Deterministic checks + LLM-as-judge, build-tagged (`//go:build eval`)
- **Test data:** 13 anonymized German business email samples in `test-data/pyramidal-emails/` (one mixes German and English heavily — code-switching the prompts preserve on purpose)
- **Results:** Logged to `test-data/eval-runs/<timestamp>/` with `summary.json`, `results.jsonl`, `samples/`
- **Run:** `EVAL_PROVIDER=claude go test -tags eval ./internal/features/pyramidize/ -v -timeout 600s`
- **Requires:** `.env` with `ANTHROPIC_API_KEY` in project root

---

## Eval Scores — baseline of 2026-09-18

**Three runs of the same commit, same model, same judge.** Run individually they
report 0.82, 0.84 and 0.84 on the deterministic checks — that variation is the
provider's, not the pipeline's, and it is the reason this table exists in this
shape.

| Metric | Mean | Min | Max | Spread |
|--------|:---:|:---:|:---:|:---:|
| Avg deterministic | **0.8353** | 0.8209 | 0.8446 | 0.0237 |
| Avg judge overall | **0.8485** | 0.8338 | 0.8569 | 0.0231 |
| Samples passing | — | 7/13 | 9/13 | 2 samples |

### Other configurations measured (2026-09-18)

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

**These rows measure the explicitly typed email path.** The shipped default is
`auto`, which spends a detect call first — two calls, or three if detection
picks a document type that still runs self-QA.

### Why Sonnet 5 hit the output limit

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
- **At 4096 the call runs a few hundred tokens from the ceiling.** The highest
  total seen was 3974. How long the model thinks varies from call to call, so a
  share of calls going over is what the eval's "ordinary document, or a runaway
  past 4096" pattern looks like — no loop in the generation needed. At 2048 the
  cut-off is reproduced outright, with not one visible character on v1.
- **v2 is exposed as well.** Its two calls here spent 3082 and 3822. v2 lost no
  document in six eval runs; with two calls it cannot be said whether v2 thinks
  less than v1 or had more luck. ADR-002 lists this ("a prompt property that v2
  shares latently") as something that would weaken its reliability argument.
- **None of the eight v1 calls at 4096 or higher went over**, where the
  2026-09-18 runs lost 4 of 6 attempts on these two samples. Eight calls are too
  few to say whether today's thinking is shorter or 09-18 was unlucky; the
  mechanism does not depend on which.
- **Opus 5 thought far less on the same request** (735 against 1643–2898) —
  one call, indicative only.

The self-QA *instruction* that ADR-002 named as a candidate is therefore not
needed to explain the failures. Whether it adds to the thinking is not tested
here.

**What changed:** `max_tokens` is now 16000 for both features
(`llm.OutputTokenCeiling`), up from 4096 (Pyramidize) and 2048 (Fix). It is a
ceiling, not a spend, and the model does not see it: the four v1 calls at 16000
above spent about what those at 4096 did (2794–3974 against 2352–3808). Thinking and effort were left at the
model's default on purpose — turning them down changes output quality and
belongs to an eval-gated decision (E3, #34). A cut-off that happens anyway now
says the model used the output limit reasoning before answering, rather than the
generic "try a shorter selection", so a failed eval run records the cause.

The same trap hit Fix harder: Sonnet 5 on a 3.7 KB selection spent all of the
old 2048 on thinking twice out of two, and finished at 5361 output tokens once
the limit was 16000. A user who picked Sonnet 5 or Opus 5 for Fix got "try a
shorter selection" on text of ordinary length.

### The noise floor

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

### Configuration

| | |
|---|---|
| Provider / model | `claude` / `claude-sonnet-4-6` |
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

### Run folders

- `test-data/eval-runs/2026-09-18T03-53-02`
- `test-data/eval-runs/2026-09-18T03-56-20`
- `test-data/eval-runs/2026-09-18T03-59-47`

`test-data/eval-runs/` is gitignored, so these exist only on the machine that
produced them. The aggregate is committed.

### Per sample

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

### What the March numbers were, and why they looked like a regression

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

## Superseded: the March table

Kept for provenance. Produced by a single run, with an unpinned judge, before
the parser fix — not a baseline to compare against.

### March 2026 numbers (2026-03-29)

| Metric | Baseline | After Improvements | Opus |
|--------|:---:|:---:|:---:|
| Avg deterministic | 0.40 | **0.82** | 0.84 |
| Avg judge overall | 0.89 | **0.89** | 0.89 |
| Samples passing | 0/13 | **9/13** | 9/13 |
| Structure pass rate | 0/13 | 13/13 | 13/13 |
| Hallucination pass rate | 0/13 | 12/13 | 13/13 |

---

## Improvements Made

### Deterministic Check Calibration
- Bold header detection (email prompts produce `**bold**` not `#` markdown)
- German compound noun decomposition (hyphen/slash/prefix-suffix splitting)
- Business vocabulary exclusion list (~80 common German/English restructuring terms)
- Percentage-based hallucination threshold (15% of output terms, not absolute count)
- Case-insensitive test-data parsing with typo tolerance

### Prompt Improvements (All 4 Doc Types)
- V1-derived structure rules: explicit structure template (header + bullet points, not prose)
- Standalone content-statement headers (UBERSCHRIFTEN-REGELN from v1 archive)
- Analysis phase: scan all input for relevant info before restructuring
- Style rules: compact, bullets for details, no filler
- Tone preservation: no formality escalation, no person-switch, no editorial additions
- Fidelity specialist added to self-eval (5th specialist lens)
- Subject line guidance: 3 segments ideal, more is acceptable but reduces scannability

---

## Known Open Issues

### 4 Failing Samples (info_coverage)

| Sample | Coverage | Judge Overall | Root Cause |
|--------|:---:|:---:|---|
| email-diagnose-update | 29% | 0.82 | Model drops mail-thread history context |
| email-dataquality-reply-to-feedback | 38% | 0.89 | Very long input, key-term extraction noise |
| email-meeting-request | 43% | 0.93 | Casual spoken input, many informal terms |
| email-adverity-field-structuring | 75% | 0.80 | URL fragments in key terms |

Note: Only `email-diagnose-update` is a real quality problem (judge completeness 0.78). The other 3 are measurement noise — judge scores are high (0.80-0.93).

### Persistent Model Behaviors
- **Process-label headers:** Despite explicit rules, model occasionally uses "Erhoffte Erkenntnisse", "Nächste Schritte" instead of content statements
- **Editorial additions:** "Kein Handlungsbedarf" added when not in original input
- **Tone drift:** Short informal emails get slightly over-formatted with bold headers and bullets
- **Opus handles mail history better:** Judge completeness 0.93 vs 0.82 (Sonnet) on diagnose-update — partly a model capability gap

### Deterministic Check Limitations
- Info coverage uses capitalized-word extraction — fundamentally noisy for German
- No semantic similarity (would require NLP model, see research-nlp-langchain.md)
- Hallucination check can't distinguish legitimate restructuring vocab from invented claims

---

## Quality Improvement Roadmap

1. ~~**V1 specialist architecture** — Reimplement 4 parallel specialist agents in Go for independent quality verification (see ADR-001)~~ — **parked.** [ADR-002](adr-002-one-shot-vs-pipeline.md) found the 2-call pipeline buys no measured quality over a single call for email; a 6-call architecture needs evidence before it is proposed again. `roadmap.md` already parks it.
2. **Semantic eval checks** — Python sentence-transformer script for eval, replacing term overlap
3. **Inline deterministic QA** — Move existing checks into live pipeline as post-LLM gate
4. **NLP info-loss detection** — Sentence embeddings via hugot/ONNX for live QA (Tier 2)
