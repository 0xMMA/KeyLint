# Fix Prompt — Eval Status

> Suite added 2026-09-18. The silent grammar fix had no eval until then: every
> change to its prompt was a guess.

Pyramidize's numbers live in [`docs/pyramidize/quality-status.md`](../pyramidize/quality-status.md).
Two files rather than one, because the two suites share nothing but the harness —
different samples, different checks, different baselines — and a single document
mixing them would be read wrong more often than it would be read right. Each
links to the other; `CLAUDE.md` points at both.

---

## What the suite is

15 samples in `test-data/fix-samples/`, each a directory of three files:

| File | Purpose |
|---|---|
| `input.md` | what a user typed, errors and all |
| `reference.md` | one acceptable correction, written by hand |
| `notes.md` | why the sample exists, and the machine-readable part the checks read |

All 15 are synthetic. **None reuses the five example pairs inside the prompt
itself** — a sample the prompt already contains measures recall of its own
examples, not the rule behind them.

They cover: German noun capitalisation, English grammar and contractions,
anglicisms that must stay (`Deploy`, `Cache`, `Standup`), a German word in
English text that must go (`Lieferschein`), deliberate code-switching, ASCII
umlauts, punctuation, a Markdown snippet, a chat message whose register must
survive, a long run-on paragraph, product names — and one sentence that is
already correct and must come back untouched.

### The checks

Deterministic first, because the judge should only be asked what genuinely needs
judgement:

- **required_fixes** — the corrections the sample exists for must be present.
- **language_preserved** — anchor words survive; the translations that would mean
  the text changed language do not appear.
- **no_commentary** — rule 6. Both the announcing kind (`Here's the corrected…`)
  and the trailing kind (`(No corrections needed…)`), in German and English.
- **tone_preserved** — the fragments carrying the register are still there.
- **length_ratio** — catches expansion into commentary and truncation.
- **left_alone** — only for the already-correct sample: it must be returned
  identical.
- **markdown_preserved** — headings, bullets, fences, inline code and bold are
  counted on both sides.

**None of this can be satisfied by echoing the input.** That is the cheapest
wrong answer a model can give, and `required_fixes` is what forbids it — a test
in the normal suite asserts exactly that.

Two more properties are enforced in CI without an API call: the human reference
must pass the checks it is the answer to (if it does not, the checks are wrong,
not the model), and the raw input must **fail** them (if it does not, the sample
asks nothing).

### The judge

Same pinned instrument as the Pyramidize suite — `claude-sonnet-4-5-20250929` at
temperature 0 — scoring four dimensions: correctness, meaning preserved, tone
preserved, and **no over-editing**. The last one is there because the expensive
failure in a correction tool is not a missed comma; it is rewriting text the
user did not ask to have rewritten.

---

## Baseline — 2026-09-18

Three runs, `claude` / `claude-haiku-4-5-20251001`, which is the shipped default
for Fix.

| Metric | Mean | Range | Spread |
|---|:---:|:---:|:---:|
| Avg deterministic | **0.9326** | 0.9267–0.9444 | 0.0178 |
| Avg judge overall | **0.8844** | 0.8567–0.9033 | 0.0467 |
| Samples passing | 12 | 12–12 of 15 | 0 |

Judge dimensions, averaged over all 45 scored samples:

| correctness | meaning preserved | tone preserved | no over-editing |
|:---:|:---:|:---:|:---:|
| 0.95 | 0.98 | 0.96 | **0.86** |

Prompt hash `7d7b15a7510f98ae`, judge
`claude-sonnet-4-5-20250929` @ temperature 0, baseline in
`test-data/eval-baselines/2026-09-18T07-32-30/baseline.json`.

**A delta smaller than the spread is not evidence.** Those numbers are
`max − min` over three runs of identical code, not a ± half-width.

### What it found

The same three samples fail in every run — `samplesPassing` has a spread of
zero across all three, so these are properties of the prompt, not noise.

**1. It comments on its own work.** Given the already-correct sentence, the
model returns it and appends `(No corrections needed—this text is grammatically
correct and clear.)` — in two runs of three, with the third still editing the
text. Rule 6 says "make direct corrections without explanations, comments, or
questions". In the silent-hotkey flow there is no UI to strip that: the
parenthetical is pasted into the user's document.

**2. It translates deliberate code-switching.** The mixed sample ends with an
English clause in otherwise German text — `…, i will look into it tomorrow
morning.` All three runs render it as German. Rule 4 forbids translating, and
rule 8 exists specifically to protect this: *"it reflects how multilingual minds
naturally reach for the nearest available word across languages"*.

**3. It is inconsistent about sentence-initial capitalisation in casual text.**
The chat sample keeps `hey,` lowercase while capitalising `Hab` and `Kein
Stress` in the same message. Minor, and arguably a tone judgement — recorded
because it is reproducible, not because it is serious.

**The judge agrees from the other direction.** Correctness, meaning and tone all
sit at 0.95–0.98, while **no-over-editing is 0.86** — the lowest dimension by a
clear margin, and independently the same conclusion the deterministic checks
reach. What this prompt gets wrong is not missing errors; it is doing more than
it was asked.

Nothing here is acted on in this PR. The baseline exists so that a prompt change
can be shown to help.

---

## Running it

```
./scripts/eval.sh --suite fix --runs 3
./scripts/eval-aggregate.sh --compare test-data/eval-baselines/<ts>/baseline.json <run-dir>...
```

The same caveats as the other suite apply, and for the same reasons: one run is
a sample and not a measurement, `--compare` asks whether the two observed
intervals overlap rather than whether a mean moved, and it refuses two sides
that did not measure the same configuration. `summary.json` additionally records
a **prompt hash**, so a comparison can tell "the prompt changed" from "the model
did".

## No prompt changes were made

Deliberately. The point of a first baseline is to measure what ships, and a
prompt tuned against the samples that judge it stops being measurable — see
`feedback_no_overfitting` in the roadmap's rules.
