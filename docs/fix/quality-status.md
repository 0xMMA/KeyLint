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

15 samples in `test-data/fix-samples/`, split into a tuning half and a held-out
half (see [The split](#the-split) below). Each sample is a directory of three
files:

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

### The split

The fifteen samples live in two directories — `test-data/fix-samples/tune/` (10)
and `holdout/` (5) — with `SPLIT.json` next to them recording how they got there.

It exists because of a measured failure. Six prompt variants were compared on
all fifteen samples and the best shipped; re-measuring that same prompt landed
between 0.9465 and 0.9537 across three independent triples, and the verdict
against the baseline flips between "overlap" and "improvement" depending on
which triple you take. Picking the best of six draws is itself a measurement,
and nothing in the suite could see it.

**The rule.** Order all fifteen samples by the per-sample deterministic spread
recorded in `test-data/eval-baselines/2026-09-18T09-07-42/baseline.json`,
descending, ties broken alphabetically. Every third sample joins the holdout.
The ordering, and the halves it produces, are frozen in `SPLIT.json`, and a test
re-derives them on every run.

| holdout (5) | tune (10) |
|---|---|
| `chat-ton`, `de-umlaute-ascii`, `en-grammar-basics`, `en-run-on`, `schon-korrekt` | `de-anglizismus-bleibt`, `de-nomen-gross`, `de-tech-terms-bleiben`, `en-french-word`, `en-german-word-ersetzen`, `interpunktion`, `langer-absatz`, `markdown-struktur`, `mixed-code-switching`, `produktnamen` |

**Ordering by measured difficulty rather than by name is the point**, and the
first version of this split got it wrong. That version sorted alphabetically and
carried two hand-written exceptions. A review took it apart:

- Renaming one sample rotated four of the five holdout members. Whoever wrote a
  new sample chose the outcome by choosing where it sorted — anything sorting
  after position 12 was a no-op, anything early reshuffled almost everything.
- The two exceptions amounted to exactly one swap against the plain rule, and
  that swap moved the highest-variance sample in the suite (`mixed-code-switching`,
  spread 0.1992, 0/3 passing, the open violation on #80) out of the half that
  may be iterated on, and a zero-variance already-fixed sample in. Whatever the
  intent, that is the swap that most flatters the tuning half.
- The resulting holdout had a mean per-sample spread of **0.0602** against
  **0.0303** for the fifteen samples it replaced. It was noisier than the thing
  it was supposed to sharpen.

The exceptions are gone. Spread ordering is blind to names, so a rename cannot
move a sample between halves, and the current split has a holdout spread of
**0.0233** against a tune half of 0.0474 — the variance now sits where the
iteration happens.

**What this holdout can and cannot do — measured, not predicted.** Three runs on
the shipped prompt, recorded in `test-data/eval-baselines/2026-09-18T10-19-42/`:

| | mean | range | spread |
|---|---|---|---|
| deterministic | **0.9986** | 0.9978–0.9992 | 0.0013 |
| judge overall | **0.9987** | 0.9960–1.0000 | 0.0040 |
| samples passing | 5 | 5–5 of 5 | 0 |

So it is an **excellent regression detector and useless for showing
improvement**. The noise floor is 0.0013, so a drop of a few thousandths is
already real — better resolution than the full suite gives. But there is
**0.0014 of headroom**: no prompt change can gain more than that here, whatever
it does.

An earlier draft of this section predicted a noise floor of ~0.023 by adding up
the per-sample spreads from the all-samples baseline. That was wrong by a factor
of twenty — averaging five samples cancels most of it. The number above replaces
a calculation with a measurement, which is the whole discipline this document is
supposed to be about.

**A caveat on the ordering the split is derived from.** Per-sample spread over
three runs is itself a noisy statistic. `de-umlaute-ascii` recorded 0.1102 in
the all-samples baseline and 0.0018 here, on the same prompt — so the ranking
that decided the halves rests on estimates that move. The split is still far
better than an alphabetical one, but a second all-samples baseline would firm up
the ordering, and the manifest records which baseline it came from precisely so
that this is checkable rather than assumed.

**And the real finding underneath all of it: the Fix suite is saturated.** Ten
of the fifteen samples sit at a ceiling with zero spread, and the five now held
out average 0.9986. A holdout number cannot mean "this prompt is better" until
the suite has samples with headroom — which is a bigger and more useful piece of
work than any further tuning of the split.

**Two constraints were dropped**, and both deserve naming because they were
asked for:

- *The holdout should contain a mixed-language sample.* There is exactly one,
  and putting it in the holdout costs both halves: the holdout inherits the
  worst variance in the suite, and the tuning half loses the only material for
  the one violation still open on #80. It is in `tune`.
- *`schon-korrekt` should stay out of the holdout.* It is in the holdout, at
  position 15 by the rule. It is a ceiling sample, which is exactly what a
  regression guard wants: it is the sample that caught the trailing-commentary
  violation, and noticing if that comes back is worth more than keeping it
  tunable.

**Adding or removing a sample re-derives the split** and retires any held-out
measurement taken before it — the ordering shifts and the halves change. The
manifest has to be regenerated from a fresh all-samples baseline, and the test
fails until it is.

### The protocol

- **Tune on `tune`**: `./scripts/eval.sh --suite fix --split tune --runs 3`.
  Iterate there as much as the question needs.
- **Measure `holdout` once, at the end**: `--split holdout --runs 3`. Once. A
  second look makes it a tuning set.
- **Report both**, with the number of tune-half iterations that produced the
  winner. A holdout number without that says less than it appears to.
- **Baselines measure `all`** (the default): a baseline describes what ships
  rather than a step in a search.
- Read a holdout result as a **regression check**, not as proof of improvement:
  it sits at 0.9986 with 0.0014 of headroom. Compare against
  `test-data/eval-baselines/2026-09-18T10-19-42/baseline.json`, recorded on the
  shipped prompt before any #80 work — measuring the old prompt leaks nothing
  and does not spend the one look the protocol allows.

`split` is part of the `configKey`, so `--compare` refuses a tune-half run
against an all-samples baseline. The key records how many samples a run
measured, never which, so `splitHash` is recorded next to the numbers and a
comparison says out loud when the two sides measured a different set.

### The checks

Deterministic first, because the judge should only be asked what genuinely needs
judgement:

- **required_fixes** — the corrections the sample exists for must be present.
  Matched on word boundaries; a casing fix also requires the wrong casing to be
  gone.
- **content_retained** — the author's own words must survive; a correction
  changes spelling and punctuation, it does not remove content. An ending is
  allowed to change (`schick` → `schicke`), because a correction that inflects a
  word has not dropped it.
- **resembles_reference** — edit distance on the raw text, against the human
  correction **and** against the input. A correction is closer to the corrected
  text than to the text it corrects; that comparison is the check, and the
  absolute floor is only the second gate.
- **punctuation_restored** — sentence boundaries, counted as the delta the
  reference had to close, full stops and commas in one pool. Half the gap
  passes: where a boundary goes, and which mark carries it, are judgement calls;
  whether the text has one is not.
- **sentence_capitals** — sentences that start in lower case, counted against
  the reference rather than against zero.
- **language_preserved** — anchor words survive; the translations that would mean
  the text changed language do not appear, in their inflected forms too
  (`Besprechungen`, `Dienste`).
- **no_commentary** — rule 6. Both the announcing kind (`Here's the corrected…`)
  and the trailing kind (`(No corrections needed…)`), in German and English,
  wherever in the output they sit.
- **tone_preserved** — the fragments carrying the register are still there.
- **length_ratio** — catches expansion into commentary and truncation.
- **left_alone** — only for the already-correct sample: it must be returned
  identical.
- **markdown_preserved** — headings, bullets, fences, inline code and bold are
  counted on both sides.

**Four of these exist because the others can be gamed**, and each was added
after an evasion was built and measured rather than imagined:

| Evasion | What it scored before | What closed it |
|---|---|---|
| Echo the input | passed `resembles_reference` on 14 of 15 samples, 1.000 on five | resemblance measured against the input as well |
| A bag of the expected strings, padded to length | **1.000 with every check passing on 13 of 15** | `content_retained`, `resembles_reference` |
| A truncation padded back into the length band | 1.000 | `content_retained` |
| Search-and-replace the `must-change` pairs into the input | **passed every check on 8 of 15**, including `interpunktion` at 1.000 with no punctuation in it | `punctuation_restored`, `sentence_capitals` |

| Rewrite the message in the model's own words | 0.955 | the drift bound in `resembles_reference` |

All five are tests in the normal suite, run against every sample, so they cannot
quietly stop being true. Two more properties are enforced in CI without an API
call: the human reference must pass the checks it is the answer to (if it does
not, the checks are wrong, not the model), and the raw input must **fail** them
(if it does not, the sample asks nothing).

The suite also has to agree with itself. Three samples disagreed about what
"correct" means and were reconciled in the same round: `de-anglizismus-bleibt`
demanded `schick` → `schicke` while two other samples keep the same colloquial
elision; `mixed-code-switching` translated `red` → `rot` in the one sample whose
thesis is that the author's word choice survives; `de-tech-terms-bleiben`
expanded `hab` → `habe`. A suite that argues with itself measures nothing, so
the rule is written down where it is enforced, in the samples' own notes:
**elisions stay, the author's word choice stays.**

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
| Avg deterministic | **0.9263** | 0.9112–0.9362 | 0.0250 |
| Avg judge overall | **0.8520** | 0.8353–0.8820 | 0.0467 |
| Samples passing | 11 | 11–11 of 15 | 0 |

Judge dimensions, averaged over all 45 scored samples:

| correctness | meaning preserved | tone preserved | no over-editing |
|:---:|:---:|:---:|:---:|
| 0.94 | 0.95 | 0.92 | **0.80** |

Prompt hash `7d7b15a7510f98ae`, checks version 2, judge
`claude-sonnet-4-5-20250929` @ temperature 0, baseline in
`test-data/eval-baselines/2026-09-18T08-14-39/baseline.json`.

**A delta smaller than the spread is not evidence.** Those numbers are
`max − min` over three runs of identical code, not a ± half-width.

**It is not comparable with the first baseline** (`2026-09-18T07-42-57`), and
`--compare` will say so rather than print a delta. The prompt did not change —
the hash is identical — but the checks did, and a number produced by a different
instrument is a different number. That is what `checksVersion` in the configKey
is for; without it the first comparison after this round would have reported the
instrument's move in the voice of a regression. The earlier baseline is kept as
a record of what version 1 measured, not as something to measure against.

**Cost:** 90 API calls for the three runs (15 samples × one fix + one judge ×
3), about three and a half minutes wall clock. Cheap enough that "run it three
times" is the default rather than a luxury — which is the whole reason the
guard against quoting a single run can afford to be strict.

### What it found

Four samples fail, and the same four fail in every run. `samplesPassing` was 11
of 15 in all three, with a spread of zero.

**1. It answers the message instead of correcting it — two runs of three.** The
chat sample opens `hey, kannst du kurz draufschauen?`. Two of the three runs
replied to it:

> Hey, klar schau ich mir das an! Allerdings sehe ich hier keinen Text oder
> Code, den ich überprüfen könnte. Kannst du mir das zeigen, das du gebastelt
> hast? :)

> Kein Problem! Ich bin bereit – schreib einfach hin, was du checken möchtest. :)

The second one keeps 8% of the author's words. In the silent-hotkey flow this
replaces the user's selected text with the model's answer to it — the original
is gone, not merely annotated, which makes this worse than the parenthetical
below. It is the failure a shape guard cannot catch: both replies are
well-formed prose of a plausible length. What catches it is `resembles_reference`
requiring the output to be closer to the correction than to the original; both
score 0.30 and 0.27.

The third run corrects the commas and capitalises nothing: `hey, … hab da was
gebastelt, … kein stress`. So the sample failed 3 of 3, for two unrelated
reasons.

**2. It comments on its own work — every run, identical text.** Given the
already-correct sentence, all three runs returned it and appended `(No
corrections needed—this text is grammatically correct and clear.)`. Rule 6 says
"make direct corrections without explanations, comments, or questions". Four
checks now catch it — `no_commentary`, `left_alone`, `length_ratio` (1.68) and
the drift bound — which is a measure of how far outside the contract it sits.

This one has a history worth recording. An earlier version of `no_commentary`
required the remark to end with a closing bracket, and one run wrote it as
`*(No corrections needed—…)*` — the same violation in italics. It passed, and
that run scored *higher* than the two that committed it in plain text. A second
version matched the phrase but anchored on `$` without `(?m)`, which in Go means
end of text and does not match before a final newline: the same remark followed
by a line break escaped the check entirely. The recorded runs caught it only
because their outputs happened to arrive without a trailing newline.

**3. It translates deliberate code-switching — every run.** The mixed sample
ends with an English clause in otherwise German text: `…, i will look into it
tomorrow morning.` All three runs render it in German. Rule 4 forbids
translating, and rule 8 exists specifically to protect this: *"it reflects how
multilingual minds naturally reach for the nearest available word across
languages"*. One of the three also translated `red` → `rot` in the same
sentence, which the sample now forbids after its reference was corrected to stop
doing the same thing.

**4. A mandatory German comma, never restored — every run.**
`de-anglizismus-bleibt` needs `das Standup bleibt, wie es ist`; no run supplies
the comma. One of the three also left `auf nachmittag` uncorrected. This is the
finding the punctuation check exists to make, and the one the suite was blind to
before this round.

**The judge's weakest dimension is over-editing**, at 0.80 against 0.92–0.95 for
the other three. That is *not* independent corroboration: on the twelve
scorecards the deterministic checks already flag, `noOverEditing` averages
**0.40** and `overall` **0.51**; on the other 33 they are **0.95** and **0.98**.
The judge is agreeing about the same samples, not finding new ones. What it adds
is a name for the pattern — what this prompt gets wrong is not missing errors,
it is doing more than it was asked.

Nothing here was acted on in the PR that recorded it. The baseline exists so
that a prompt change can be shown to help — which is what the next section does,
against exactly these numbers.

---

## Prompt change — 2026-09-18 (#80)

The first baseline found three rule violations. Two are fixed and hold in every
run. The third is not. No output guard ships with this — three attempts at one
are written up on #83, with what each measured.

| | before `7d7b15a7510f98ae` | after `p2-f344eb5f4fd0f30a` |
|---|---|---|
| Avg deterministic | 0.9263 (0.9112–0.9362) | 0.9537 (0.9381–0.9684) |
| Avg judge overall | 0.8520 (0.8353–0.8820) | **0.9369** (0.9320–0.9453) |
| Samples passing | 11 (11–11) | 11.3 (10–12) |

`--compare` reports `improvement: deterministic improvement, judge improvement`.
Baselines `2026-09-18T08-14-39/` and `2026-09-18T09-07-42/`, the second recorded
from a clean tree at `689a5bf`.

**Take the deterministic half with more salt than that verdict carries.** This
prompt was measured three times, in three sets of three runs:

| when | deterministic | judge | passing | in the repo? |
|---|---|---|---|---|
| during the six-variant search | 0.9513 (0.9473–0.9585) | 0.9453 | 12 | **no** |
| with the output guard in place | 0.9465 (0.9331–0.9683) | 0.9524 | 11 | **no** |
| final, guard removed | 0.9537 (0.9381–0.9684) | 0.9369 | 11.3 | yes |

Three measurements of identical prompt text spanning 0.9465–0.9537 on the mean,
and the verdict against the baseline flips between "overlap" and "improvement"
depending on which triple is used: the middle one overlaps, the other two do
not, and the disjointness of the final one rests on 0.9381 against 0.9362. Nine
runs say the deterministic effect is real but sits at the edge of what three
runs can resolve. **The judge improvement is the robust half** — 0.93 to 0.95
against a baseline of 0.85, with disjoint ranges in all three measurements.

The first two rows were measured in the session that produced this change and
are **not recorded in the repository**; the intermediate baselines were deleted
as superseded, which destroyed the evidence for a claim made from them. Treat
them as reported, not as checkable. Keeping superseded baselines costs a few
kilobytes and is the obvious lesson.

### What the prompt fixed

**Answering the message instead of correcting it: gone, 3 of 3.** `chat-ton`
goes from 0/3 passing to 3/3, deterministic 0.5432 → 0.9972, judge 0.25 → 0.97.
Before, two of three runs replied to the chat sample and one kept 8% of the
author's words — the violation that destroyed the user's text rather than
decorating it.

**Commentary on correct text: gone, 3 of 3.** `schon-korrekt` comes back
byte-identical in every run: 0/3 → 3/3, deterministic 0.66 → 1.000, judge 0.30 →
1.00. Nothing post-processes the model's reply, so this is the model's own
behaviour and not a clean-up.

Both come from the same two additions: naming the input as a delimited document
that is not addressed to the model, and stating what to return when there is
nothing to correct.

### What it cost

| sample | deterministic | passing | what happens |
|---|---|---|---|
| `en-german-word-ersetzen` | 0.9874 → **0.8433** | **3/3 → 1/3** | `Lieferschein` kept instead of replaced |
| `markdown-struktur` | 0.9987 → **0.9312** | **3/3 → 1/3** | heading and list items keep their lower case |
| `de-umlaute-ascii` | 0.9982 → 0.9615 | 3/3 → 2/3 | a sentence-initial capital missed in one run |
| `mixed-code-switching` | 0.8613 → 0.7357 | 0/3 → 0/3 | already failing; judge 0.57 → 0.37 |

The markdown one is user-visible: the prompt tells the model the text is a
complete piece of writing whose first sentence is a first sentence, which
restored sentence-initial capitals in prose and did nothing for headings and
bullets. Traded knowingly against two violations that damaged the user's text.

### The obvious fix for violation 2 is the wrong one, measured

Strengthening rule 4 to cover parts of the text as well as the whole —
"translating one clause is as wrong as translating everything" — reads like the
fix and is not. Six variants were measured; none of the intermediate baselines
is in the repo, so the numbers below are reported rather than checkable:

| prompt | deterministic | judge | passing | wording |
|---|---|---|---|---|
| `7d7b15a7510f98ae` | 0.9263 | 0.8520 | 11 | the baseline |
| `90bf9672318a301e` | 0.8943 | 0.9400 | 8 | rule 4 strengthened, rule 8 defaulted to keep, voice clause in rule 3 |
| `36348b7cb2f7eb85` | 0.9168 | 0.9389 | 9 | rule 3 reverted, rule 8 scoped to single words |
| `05ec62ceb461bd55` | 0.9299 | 0.9491 | 10.7 | "character for character" softened |
| `4b4b263c9b108d7a` | 0.9492 | 0.9476 | 11.3 | rule 8's exclusion widened again |
| **`f344eb5f4fd0f30a`** | 0.9513 / 0.9465 / **0.9537** | 0.9453 / 0.9524 / **0.9369** | 12 / 11 / **11.3** | **rule 4 back as it was — shipped** |

Two findings, neither visible from reading the prompt:

1. **The stronger rule 4 never stopped the clause translation** — 0 of 3 runs in
   two of the iterations carrying it, 1 of 3 in the first.
2. **It suppressed rule 8 instead.** `en-german-word-ersetzen` — `Lieferschein`
   → delivery note, the same shape as the prompt's own second example — failed
   on every run of every iteration with the stronger rule 4. Restoring rule 4
   improved it but did **not** restore it: 1 run of 3 now, against 3 of 3 before
   any of this. An earlier draft said it "came back as soon as rule 4 was
   restored"; that was wrong.

So rule 4 stands as it was, rule 8 carries the distinction in one sentence, and
violation 2 stays open on #80.

### The selection bias, named

Six variants were measured on the same fifteen samples and the best shipped.
That is a maximum over six draws, and reporting its interval as if it were one
pre-registered comparison overstates it — which the three measurements of the
winner above demonstrate rather than merely assert.

The revisions themselves were driven by diagnosed regressions with named causes
— rule 3's addition cost sentence-initial capitals, rule 8's rewrite cost the
single-word replacement the prompt's own examples teach, "character for
character" leaked past its condition — not by hunting for a better score. The
variant that did best on the sample the exercise was about
(`90bf9672318a301e`, the only one where any run kept the English clause) is not
the one that shipped, because it was worse everywhere else. But one decision —
restoring rule 4 — was triggered by a single named sample failing, which is a
decision rule on a test case however good the reasoning around it.

**What this suite needs before the next prompt change is a held-out split.** Ten
samples to tune against, five never looked at until the end.

---

## Running it

```
./scripts/eval.sh --suite fix --runs 3
./scripts/eval-aggregate.sh --compare test-data/eval-baselines/2026-09-18T09-07-42/baseline.json <run-dir>...
```

`--variant` and `--schema` configure the Pyramidize pipeline and are rejected
here rather than ignored; the Fix prompt has no variants and returns text.

The same caveats as the other suite apply, and for the same reasons: one run is
a sample and not a measurement, `--compare` asks whether the two observed
intervals overlap rather than whether a mean moved, and it refuses two sides
that did not measure the same configuration.

What counts as the same configuration is the point of the `configKey`: suite,
provider, model, judge, variant, schema enforcement, threshold, sample count and
**checksVersion**. The prompt hash is recorded and reported next to the verdict
but is deliberately **not** in the key — it was, for one revision, and that made
the suite refuse the one comparison it exists to make. A prompt change is the
question; a change to the checks is a change of instrument, and those are
opposite things.

## What this suite cannot see

Written down because a checklist of what an instrument catches is half a
description. Everything below is a known hole, not a suspicion:

- **`no_commentary` matches phrases, not intent.** A user whose own text
  contains "keine Änderungen nötig" or "the sentence is already correct" gets
  their correct output scored as commentary. The negation is what makes the
  phrase meta, and nothing distinguishes the model saying it from the author
  saying it.
- **Forbidden words cover inflection, not composition.** `Besprechungen` and
  `Dienste` are caught; `Tagesordnungspunkte` is not, because the set of
  compounds built from a German noun is open. A sample that wants a compound
  refused has to list it.
- **`punctuation_restored` counts, it does not place, and it does not care
  which mark.** Full stops and commas share one pool, so a comma-spliced
  sentence scores like a properly ended one, a mark in the wrong position scores
  like a mark in the right one, and an output that scatters twice as many as the
  reference is capped at full marks rather than penalised. This is deliberate:
  the first version counted the two kinds separately and called a correct output
  wrong in all three runs, because the model ended a sentence with a full stop
  where the reference used a comma. It answers "did the model punctuate at all",
  which is the failure that was actually observed, and nothing finer.
- **`sentence_capitals` nets out.** An output that lowercases one sentence and
  capitalises another the reference left lower-case scores the same as a correct
  one.
- **The drift bound mostly renames a failure.** In a correction task the
  reference is near the input by construction, so an output far from one is
  usually far from the other: of the two rewrites tested, both would also have
  failed the reference floor. What the bound adds is the right diagnosis —
  "rewritten rather than corrected" — and a guard for samples whose reference
  legitimately restructures.
- **15 samples, all synthetic, written by the same hand as the checks.** A blind
  spot shared between a sample and the check that scores it is invisible from
  the inside; three such disagreements were found by a review rather than by the
  suite, and there is no reason to think they were the last three.
- **Nothing checks the model's reply before it reaches the clipboard.** The Fix
  hotkey pastes what the model returns over the user's selection. Three attempts
  at a deterministic guard are on #83; each one silently returned the user's
  uncorrected text on some ordinary input, which is worse than the behaviour it
  prevents. Until one works, the prompt is the only defence and the eval is the
  only way to know it still holds.
- **The judge is the same model family as the model under test**, at a pinned
  dated snapshot. It agrees with the deterministic checks about which samples are
  bad; on the samples they already flag, `noOverEditing` collapses while the
  other dimensions hold, which is the one thing it adds that the checks do not.

## On not tuning to the samples

The first baseline was recorded with no prompt change at all, deliberately: the
point of a first baseline is to measure what ships. The change above was made
afterwards, against that recorded baseline, and the rule it was made under is
`feedback_no_overfitting` in the roadmap's rules — general principles only, no
sample-specific wording, no sample text in the prompt.

Worth being precise about what that rule permits, because five prompt revisions
were measured before one shipped and that is a shape overfitting also has. The
distinction is what drove each revision. Every one of them was a response to a
diagnosed regression with a named cause — rule 3's addition cost sentence-initial
capitals, rule 8's rewrite cost the single-word replacement the prompt's own
examples teach, "character for character" leaked past its condition and stopped
the model capitalising headings — and not to a score that wanted improving. The
revision that scored best on the sample that motivated the whole exercise
(`90bf9672318a301e`, the only one where any run kept the English clause) is not
the one that shipped, because it was worse everywhere else.

What would have been overfitting, and was not done: adding an example pair
resembling a sample, naming a sample's vocabulary in a rule, or writing "do not
translate an English clause inside German text" — which would have taught the
one case the suite happens to test instead of the principle behind it.
