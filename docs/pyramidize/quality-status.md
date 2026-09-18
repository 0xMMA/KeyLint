# Pyramidize Output Quality — Status & Open Issues

> Last updated: 2026-09-18

---

## Eval Infrastructure

- **CLI mode:** `-fix` and `-pyramidize` commands dispatch headlessly from `main.go`
- **Eval framework:** Deterministic checks + LLM-as-judge, build-tagged (`//go:build eval`)
- **Test data:** 13 anonymized German business email samples in `test-data/pyramidal-emails/`
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
| Samples passing | 9/13 (one run) | 7–9/13 |

#53 reported an unchanged `main` scoring 0.768 against the documented 0.82 and
read it as a regression. The evidence does not support that reading, for three
reasons of decreasing certainty:

**1. March was never one number.** The archived runs under
`test-data/eval-runs/` hold two complete Sonnet 4.6 runs from that week:
`2026-03-29T22-03-24` at 0.8162 and `2026-03-31T23-56-48` at 0.8379. Those two
span 0.022 — the same interval as the three September runs. The table said 0.82
because someone had to pick one. Both March runs overlap the September range, so
by the test this document now recommends, nothing happened between March and
September.

**2. The judge column was never comparable.** March ran an unpinned judge;
this baseline pins `claude-sonnet-4-5-20250929` at temperature 0. The 0.89 and
the 0.8485 are two different instruments, and the gap between them says nothing
about the pipeline.

**3. The 0.768 is consistent with a bug that is now fixed, but the run is
gone.** #53 records that `email-dataquality-reply-to-feedback` failed in it with
`invalid character '(' after array element`. A failed sample scores zero, and
that sample averages 0.768 across the September runs, so losing it costs a
13-sample mean about 0.059 — while the reported gap to the March table was
0.052. The arithmetic fits, but the run directory it refers to no longer exists
on any machine here, so this is a plausible reconstruction and not a measurement.
What *is* verifiable: #54 fixed the parser (`unmarshalRobust` repairs array
elements, with a regression test), and across the three September runs there
were **no sample errors at all**.

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

1. **V1 specialist architecture** — Reimplement 4 parallel specialist agents in Go for independent quality verification (see ADR-001)
2. **Semantic eval checks** — Python sentence-transformer script for eval, replacing term overlap
3. **Inline deterministic QA** — Move existing checks into live pipeline as post-LLM gate
4. **NLP info-loss detection** — Sentence embeddings via hugot/ONNX for live QA (Tier 2)
