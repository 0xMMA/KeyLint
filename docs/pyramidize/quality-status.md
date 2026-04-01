# Pyramidize Output Quality — Status & Open Issues

> Last updated: 2026-03-30

---

## Eval Infrastructure

- **CLI mode:** `-fix` and `-pyramidize` commands dispatch headlessly from `main.go`
- **Eval framework:** Deterministic checks + LLM-as-judge, build-tagged (`//go:build eval`)
- **Test data:** 13 anonymized German business email samples in `test-data/pyramidal-emails/`
- **Results:** Logged to `test-data/eval-runs/<timestamp>/` with `summary.json`, `results.jsonl`, `samples/`
- **Run:** `EVAL_PROVIDER=claude go test -tags eval ./internal/features/pyramidize/ -v -timeout 600s`
- **Requires:** `.env` with `ANTHROPIC_API_KEY` in project root

---

## Eval Scores (2026-03-29)

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
