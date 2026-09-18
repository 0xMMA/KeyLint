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

All 15 are synthetic, and none reproduces a prompt example verbatim. Two are
close enough to name, because a sample that rehearses an example measures recall
rather than the rule:

- `en-german-word-ersetzen` (`Lieferschein` → delivery note) sits in the same
  slot as the prompt's `Rechnung` → invoice: a German commercial-document noun
  inside an English sentence. Different word, same shape.
- `de-tech-terms-bleiben` and `de-anglizismus-bleibt` test the same rule as the
  prompt's fifth example — lowercase English loanwords in German prose, kept and
  capitalised — with different vocabulary.

`produktnamen` originally required `release` → `Release`, which is the *same
token and same correction* as that fifth example; it was rewritten to use
`Wartungsfenster` instead. Anything in this suite scoring a flat 1.000 in every
run deserves this suspicion before it is read as a strength.

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
- **content_retained** — the author's own words must survive; a correction
  changes spelling and punctuation, it does not remove content.
- **resembles_reference** — token F1 against the human correction, as a floor.
  Not to demand the reference verbatim, but because a real correction of this
  input lands near it and a bag of keywords does not.
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

**Two of these exist because the others can be gamed.** The first version of
this file had only presence tests and a length band, and a review built an
output consisting of nothing but the expected strings, padded to the right
length: it scored **1.000 with every check passing on 13 of the 15 samples**. A
truncation that dropped most of the content and was padded back scored 1.000
too.

`content_retained` and `resembles_reference` are what closed that. Word salad
now fails every sample, a padded truncation fails as well, and echoing the input
fails on every sample but the one where echoing is the right answer. All three
properties are tests in the normal suite, so they cannot quietly stop being
true.

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
| Avg deterministic | **0.9426** | 0.9226–0.9563 | 0.0336 |
| Avg judge overall | **0.8789** | 0.8433–0.9000 | 0.0567 |
| Samples passing | 11.7 | 11–12 of 15 | 1 |

Judge dimensions, averaged over all 45 scored samples:

| correctness | meaning preserved | tone preserved | no over-editing |
|:---:|:---:|:---:|:---:|
| 0.95 | 0.97 | 0.93 | **0.84** |

Prompt hash `7d7b15a7510f98ae`, judge `claude-sonnet-4-5-20250929` @ temperature
0, baseline in `test-data/eval-baselines/2026-09-18T07-42-57/baseline.json`.

**A delta smaller than the spread is not evidence.** Those numbers are
`max − min` over three runs of identical code, not a ± half-width.

### What it found

**1. It comments on its own work — every run.** Given the already-correct
sentence, the model returns it and appends `(No corrections needed - this text
is grammatically correct and clear.)`. Rule 6 says "make direct corrections
without explanations, comments, or questions". In the silent-hotkey flow there
is no UI to strip that: the parenthetical is pasted into the user's document.

This one has a history worth recording. An earlier version of `no_commentary`
required the remark to end with a closing bracket, and one run wrote it as
`*(No corrections needed—…)*` — the same violation in italics. It passed, and
that run scored *higher* than the two that committed it in plain text. The check
now matches the phrase rather than the punctuation.

**2. It translates deliberate code-switching — every run.** The mixed sample
ends with an English clause in otherwise German text: `…, i will look into it
tomorrow morning.` All three runs render it in German. Rule 4 forbids
translating, and rule 8 exists specifically to protect this: *"it reflects how
multilingual minds naturally reach for the nearest available word across
languages"*. It shows up on three checks at once — the English words are gone
from the content, the output has drifted from the reference, and the tone anchor
that was the whole clause is missing.

**3. Casual German loses its capitals — two runs of three.** The chat sample
comes back with the commas fixed and nothing capitalised: `hey, … hab da was
gebastelt … kein stress`. The third run capitalises consistently but rewrites
`kannst du kurz draufschauen?` into `schau gerne drauf!`, which is rule 5
territory. Either the sentence-initial capitals are missing or the sentence is.

**4. Minor: two required corrections are flaky.** `de-anglizismus-bleibt` wants
`auf nachmittag` → `auf den Nachmittag` and `schick` → `schicke`; one run got
both, and the other two each missed a different one. Model variance, not a
pattern.

**The judge's weakest dimension is over-editing**, at 0.84 against 0.93–0.97 for
the other three. That is *not* independent corroboration, and an earlier draft
of this document wrongly called it that: on the four samples the deterministic
checks already flag, `noOverEditing` averages **0.52**; on the other eleven it is
**0.96**, indistinguishable from the rest. The judge is agreeing about the same
samples, not finding new ones. What it adds is a name for the pattern — what
this prompt gets wrong is not missing errors, it is doing more than it was
asked.

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
that did not measure the same configuration. `summary.json` records a **prompt hash**, and
`eval-aggregate.sh` carries it into the baseline's `configKey` along with the
suite name — so a comparison refuses to mix the two suites, and can tell "the
prompt changed" from "the model did". Without that it was recorded and never
read, which is the same gap ADR-002 had to admit for `appliedRefinement`.

## No prompt changes were made

Deliberately. The point of a first baseline is to measure what ships, and a
prompt tuned against the samples that judge it stops being measurable — see
`feedback_no_overfitting` in the roadmap's rules.
