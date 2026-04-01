# Prompt Variant v2 — Evaluation Analysis

**Date:** 2026-04-01
**Prompt:** v2 email prompt (self-contained, no selfQA)
**Models tested:** Claude Sonnet 4.6, Claude Opus 4.6
**Samples:** 13 anonymized German business emails
**Eval framework:** deterministic checks (structure, info_coverage, hallucination) + LLM-as-judge (Sonnet)

---

## 1. Results Summary

### v2/Sonnet vs v2/Opus — per-sample scores

| Sample | S det | O det | S judge | O judge | S pyramid | O pyramid | S clarity | O clarity | S complete | O complete | S tone | O tone |
|---|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|
| adverity-field-structuring | **0.83** | 0.77 | **0.88** | 0.78 | 0.90 | 0.85 | 0.92 | 0.88 | 0.90 | **0.95** | **0.82** | 0.65 |
| airbyte-escalation | 0.91 | **0.93** | **0.93** | 0.91 | 0.92 | 0.88 | 0.91 | **0.92** | 0.97 | 0.97 | **0.95** | 0.93 |
| answer-info-update | 0.84 | **0.87** | 0.85 | 0.85 | 0.85 | 0.85 | **0.90** | 0.88 | **1.00** | 0.95 | 0.75 | **0.82** |
| dataquality | 0.88 | **0.93** | 0.91 | **0.92** | 0.88 | **0.92** | 0.92 | **0.93** | 0.95 | 0.95 | **0.93** | 0.90 |
| dataquality-followup | 0.89 | **0.97** | **0.91** | 0.85 | **0.90** | 0.82 | **0.88** | 0.80 | **1.00** | 0.95 | 0.92 | 0.92 |
| dataquality-reply-to-feedback | 0.78 | 0.77 | 0.86 | **0.89** | 0.85 | **0.88** | 0.88 | **0.90** | 0.95 | 0.95 | 0.82 | **0.85** |
| diagnose-update | 0.67 | **0.77** | 0.80 | **0.88** | 0.85 | 0.85 | 0.88 | **0.90** | 0.75 | **0.88** | 0.82 | **0.92** |
| kpi-request | **0.94** | 0.86 | 0.88 | **0.91** | 0.85 | **0.90** | 0.88 | **0.92** | **0.97** | 0.95 | **0.92** | 0.90 |
| meeting-request | 0.78 | **0.86** | **0.92** | 0.91 | **0.95** | 0.90 | **0.95** | 0.92 | **0.98** | 0.95 | 0.85 | **0.88** |
| onboarding-erfolgreich | 0.85 | **0.89** | **0.92** | 0.91 | 0.90 | 0.90 | **0.95** | 0.93 | 0.95 | 0.95 | **0.90** | 0.88 |
| passwort erhalten | **0.75** | 0.70 | **0.90** | 0.85 | **0.92** | 0.85 | **0.90** | 0.88 | **0.98** | 0.95 | **0.85** | 0.82 |
| project-status | **0.90** | 0.87 | **0.95** | 0.90 | **0.95** | 0.90 | **0.95** | 0.92 | **0.98** | 0.95 | **0.92** | 0.82 |
| software-deployment | **0.88** | 0.86 | 0.95 | 0.95 | 0.95 | 0.95 | 0.95 | 0.95 | **0.98** | 0.97 | **0.95** | 0.92 |
| **AVERAGE** | 0.84 | **0.85** | **0.90** | 0.89 | **0.90** | 0.88 | **0.91** | 0.90 | **0.95** | 0.94 | **0.88** | 0.86 |

### Win counts

| | Deterministic | Judge Overall | Tone |
|---|:-:|:-:|:-:|
| Sonnet wins | 5 | 5 | 7 |
| Opus wins | 7 | 3 | 4 |
| Tie | 1 | 5 | 2 |

### Aggregate comparison (all v2, all samples)

| Metric | v1/Sonnet (PR baseline) | v2/Sonnet | v2/Opus |
|---|:-:|:-:|:-:|
| Avg deterministic | 0.82 | **0.84** | 0.85 |
| Avg judge overall | 0.89 | **0.90** | 0.89 |
| Samples passing (det) | 9/13 | 8/13 | 8/13* |
| Prompt size | ~2.5KB + selfQA | ~1.2KB | ~1.2KB |
| Pipeline steps | foundation + selfQA + conditional refine | foundation only | foundation only |

*One Opus sample errored on first run (API timeout), rerun scored det=0.97/judge=0.85.

---

## 2. Structural Patterns: Sonnet vs Opus on v2

### Pattern A: Sub-bullet nesting depth

- **Sonnet:** 2–3 levels of nesting in complex emails. Details supporting a conclusion are nested under that conclusion's header.
- **Opus:** 1–2 levels. Flattens hierarchy into longer linear bullet lists.

Example (`email-onboarding-erfolgreich`):
- Sonnet nests `@Nate: kannst du...` and `@Alex: alternativ...` as indented sub-bullets under the "MS-Teams Berechtigungen fehlen" header.
- Opus flattens all bullets to the same level under the header.

**Significance:** The Pyramid Principle IS hierarchy. Details belong under their governing conclusion. Sonnet respects this; Opus erodes it.

### Pattern B: Header promotion — conclusions vs scope descriptions

Both models average ~3.8 headers per sample. The difference is *what gets promoted to a header*.

- **Sonnet:** Promotes conclusions and decisions only. Scope/detail stays as sub-bullets.
- **Opus:** Also promotes scope descriptions to peer-level headers (process labels).

**Key example (`email-diagnose-update`):**

Sonnet (2 headers):
```
**Diagnostics-Update wird Montag ~14 Uhr in FinReg deployed**
- Entscheidung nach Rücksprache mit Ethan
- Umfang: erweiterte Log-Einträge + Console Logs direkt in Extension

**Ziel: Fehlerursache bei weißer Seite / verschluckten Webview-Fehlern eingrenzen**
```

Opus (3 headers):
```
**Diagnostics-Update wird Montag 14 Uhr in FinReg deployt**
- Minimalistische Diagnostics-Funktion als Release/Update

**Update umfasst erweiterte Logs und Konsoleneinsicht**     ← process label
- Erweiterte Logeinträge
- Console Logs direkt in Extension

**Ziel: Erkenntnisse zu den bekannten Fehlerquellen**
```

Opus's second header "Update umfasst..." is a **process label** (describes what the update contains, not a conclusion) and a **MECE violation** (the update scope IS the deployment — same concern split into two headers). Sonnet correctly nests scope as a sub-bullet of the deployment decision.

**Human author assessment:** Sonnet's output is structurally superior here. The judge scored Opus higher (0.88 vs 0.80), revealing a judge blind spot — it rewards more headers without checking MECE compliance.

Other samples where this pattern appears:
- `answer-info-update`: Opus uses topic-label headers where Sonnet uses content statements
- `airbyte-escalation`: Opus creates 5 headers vs Sonnet's 4, splitting out a detail Sonnet kept nested

### Pattern C: Keyword lift-and-shift fidelity

- **Sonnet:** Preserves more original German phrasing verbatim. Lifts key terms directly from the input.
- **Opus:** Paraphrases more aggressively. Rewrites phrases into its own words.

Impact:
- Opus scores lower on deterministic info_coverage for keyword-heavy samples (kpi-request: 0.86 vs 0.94)
- Opus scores lower on tone (avg 0.86 vs 0.88) because paraphrasing shifts register
- The deterministic checker penalizes both models for valid paraphrasing, but Opus more so

### Pattern D: Tone preservation on casual inputs

Largest gap: `adverity-field-structuring` — Opus tone=0.65 vs Sonnet tone=0.82.

Judge rationale for Opus: *"the candidate feels more like a structured ticket or bulletin with bold headers... strips away conversational warmth. The greeting is retained, but the personal touch ('bitte melde dich Jordan gerne') is reduced to an impersonal '@Jordan: bei Rückfragen gerne melden.'"*

Opus's strength (action-oriented directness) becomes a weakness when the input is casual and personal. Sonnet is better at matching the register of the input.

### Pattern E: Where Opus genuinely wins

Opus is better at extracting every detail from long, messy, multi-part inputs:
- `dataquality-followup`: det=0.97 vs 0.89 — Opus preserved more from a complex reply chain
- `diagnose-update`: det=0.77 vs 0.67 — more keywords lifted from the thread history
- `meeting-request`: det=0.86 vs 0.78 — better keyword retention from stream-of-consciousness input

Opus's completeness strength shows on inputs with high information density scattered across unstructured prose.

---

## 3. Deterministic Checker Limitations

### info_coverage false positives

4 samples failed info_coverage on v2/Sonnet. Human review found 2 genuine failures and 2 false positives:

| Sample | Det Score | Judge Score | Genuine info loss? |
|---|:-:|:-:|---|
| email-diagnose-update | 0.32 | 0.80 | **Yes** — thread history heavily compressed |
| email-dataquality-reply-to-feedback | 0.42 | 0.88 | **Yes** — KPI reference table dropped |
| email-meeting-request | 0.46 | 0.92 | **No** — paraphrased, all content preserved semantically |
| email-onboarding-erfolgreich | 0.69 | 0.92 | **No** — all "missing" terms are valid paraphrases |

The keyword-overlap approach penalizes:
1. Valid paraphrasing ("Kollege" → "meinem Kollegen", "freie Wahl" → "sehr flexibel")
2. German compound word decomposition
3. Discourse markers and filler words ("Gedankengänge", "Überlegungen", "Sachen", "Dinge")

### Improvement ideas for info_coverage

- **Semantic similarity** (embeddings-based) instead of keyword overlap — would catch paraphrases
- **Named entity extraction** — focus on people, dates, numbers, decisions rather than all nouns
- **Weighted terms** — proper nouns and numbers count more than common nouns and discourse markers

These are research items, not urgent fixes. The judge compensates for the checker's limitations.

---

## 4. Judge Limitations

### Missing MECE compliance evaluation

The current judge prompt scores: pyramidStructure, clarity, completeness, tonePreservation, overall.

It does NOT evaluate:
- Whether headers are conclusions vs process labels
- Whether details are properly nested under governing headers (MECE)
- Whether header promotion is justified

This is why the judge scored Opus higher on `diagnose-update` — more headers looked like better structure, but the extra header was a MECE violation.

### Suggested judge improvement

Add a `meceCompliance` dimension:
```
- meceCompliance: Are same-level headers mutually exclusive (no content overlap)?
  Are detail/scope items properly nested as sub-bullets under their governing header,
  NOT elevated to peer-level headers? A header that describes "what something contains"
  rather than stating a conclusion is a process label and should be a sub-bullet. Score: 0-1
```

---

## 5. Prompt Improvement Candidates for v2

### A. Nesting guidance (addresses Pattern B)

Add to `<headers>` section:
```
details supporting a conclusion belong as sub-bullets under that header.
do NOT promote scope, content, or "what it contains" descriptions to peer-level headers.
a header that could start with "umfasst", "enthält", "includes" is a detail, not a header.
```

### B. Keyword fidelity (addresses Pattern C)

Add to `<hard_rules>`:
```
prefer original phrasing over paraphrasing. lift key terms, names, and technical vocabulary verbatim from the input.
```

### C. Nesting template

The v1 prompt had a `<structure_template>` showing the expected format. V2 removed it. Re-adding a minimal version that shows nesting could help:
```
<structure_template>
**Conclusion or key message as header**
- Supporting detail 1
- Supporting detail 2
  - Sub-detail if needed

**Second equally important conclusion**
- Details
</structure_template>
```

---

## 6. Decision

**v2 + Sonnet is the shipping combination.** The leaner prompt plays to Sonnet's strengths (hierarchy, keyword fidelity, tone preservation) without needing the selfQA/refine pipeline overhead.

Prompt improvements (sections 5A/B/C) and judge improvements (section 4) are future work to be tested via the eval framework before landing.

---

## 7. Eval Run References

| Run | Timestamp | Provider | Model | Variant | Dir |
|---|---|---|---|---|---|
| v2/Sonnet | 2026-03-31T23-56-48 | claude | claude-sonnet-4-6 | 2 | `test-data/eval-runs/2026-03-31T23-56-48/` |
| v2/Opus | 2026-04-01T00-20-44 | claude | claude-opus-4-6 | 2 | `test-data/eval-runs/2026-04-01T00-20-44/` |
| v2/Opus rerun | 2026-04-01T00-30-03 | claude | claude-opus-4-6 | 2 | `test-data/eval-runs/2026-04-01T00-30-03/` |

Each run dir contains: `summary.json`, `results.jsonl`, and `samples/*.md` with generated outputs.
