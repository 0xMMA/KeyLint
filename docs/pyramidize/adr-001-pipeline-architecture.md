# ADR-001: Pipeline Architecture — Single-Call Self-QA vs Multi-Agent Specialists

**Status:** Accepted (with caveats — see Future Considerations)
**Date:** 2026-03-08 (original), updated 2026-03-30 with v1 analysis findings

---

## Context

The Pyramidize feature restructures unstructured text into pyramid-principle formatted documents. The key architectural question is how many LLM calls to make and how to verify output quality.

## Decision

**2-call adaptive pipeline:**
1. Foundation generation with embedded self-QA (1 call)
2. Conditional refinement triggered by quality flags (0-1 calls)
3. Document type detection if AUTO mode (0-1 calls)

Total: 1-3 API calls per document.

## Rationale (March 2026)

Modern frontier models (GPT-4o, Claude Sonnet 4.6+) handle multi-criteria evaluation in a single well-structured prompt. A single call with embedded specialist criteria (subject, MECE, completeness, style, fidelity) matches or beats sequential specialist calls at a fraction of the latency and cost.

The self-evaluation produces `qualityScore` and `qualityFlags[]` in the same JSON response. Only when quality falls below threshold (default 0.65) does a second call run with targeted remediation.

## What V1 Did Differently

The archived v1 (`archive/v1-tauri-rust/.../pyramidal-agent.service.ts`) used **6 LLM calls** with LangChain.js TypeScript:

1. Document type detection (1 call)
2. Foundation generation (1 call)
3. **4 parallel specialist agents** via `RunnableParallel` (4 concurrent calls):
   - Subject Line Specialist — validates format, improves information density (confidence-gated)
   - Header Structure Specialist — MECE validation, auto-fixes headers
   - Information Completeness Specialist — compares original vs output, risk scoring
   - Style & Language Specialist — language consistency, professional tone
4. Integration coordinator — selective merge where confidence > 0.7

**V1 quality advantages:**
- Independent verification: 4 separate LLM calls each with focused critique prompts
- Confidence-gated selective merge: only apply high-confidence improvements
- Information completeness as a dedicated LLM check (not string matching)
- Per-specialist fallback: graceful degradation if any specialist fails

**V1 cost:** 6 API calls per document, mitigated by parallel execution (wall-clock ~ 2 calls).

## Known Limitations of Current Approach

1. **Self-eval blindspot:** The model that generated the output grades itself. Research confirms models are systematically blind to their own hallucinations. An independent check (different model or NLP-based) would catch issues the self-eval misses.

2. **Binary refine-or-not:** Current pipeline either refines everything or nothing. V1's selective merge (confidence > 0.7 per specialist) was more nuanced.

3. **Process-label headers persist:** Despite explicit rules forbidding them ("Erhoffte Erkenntnisse", "Nächste Schritte"), the model occasionally uses process labels instead of content statements. A dedicated header specialist would catch this more reliably.

4. **Editorial additions:** The model sometimes adds interpretive conclusions ("Kein Handlungsbedarf") not present in the original input. The fidelity specialist in self-eval catches this inconsistently.

## Future Considerations

- **Reintroduce parallel specialists in Go:** Use goroutines + errgroup for parallel LLM calls. The v1 architecture's quality benefits are real. Cost: 4 additional API calls (parallel, so ~1 extra round-trip of latency).
- **NLP-based QA stage:** Sentence embeddings (`paraphrase-multilingual-MiniLM-L12-v2`) for semantic info-loss detection, NLI for independent hallucination checking. See `docs/pyramidize/research-nlp-langchain.md`.
- **Eino framework:** If Go-native orchestration complexity grows, `github.com/cloudwego/eino` (ByteDance, 10k+ stars) is the only healthy Go LLM framework with graph workflows.

## Eval Results (as of 2026-03-29)

| Metric | Sonnet 4.6 | Opus 4.6 |
|--------|:---:|:---:|
| Avg deterministic | 0.82 | 0.84 |
| Avg judge overall | 0.89 | 0.89 |
| Samples passing (of 13) | 9 | 9 |
