# Research: NLP & LangChain for Pyramidize Quality

> Research conducted 2026-03-30. Findings parked for future pickup.

---

## NLP for Deterministic Eval Checks

Current deterministic checks use surface-form string matching (capitalized words), which is fundamentally broken for German text (all nouns capitalized) and can't handle paraphrasing.

### Recommended: Sentence Embeddings

Model: `paraphrase-multilingual-MiniLM-L12-v2` (50+ languages, 384-dim, ~440MB ONNX)

- **Info coverage:** Embed input sentences + output sentences, check cosine similarity matrix. Every input sentence should have at least one output match above threshold (~0.7). Completely sidesteps German capitalization problem.
- **Hallucination detection:** Output sentences with no similar input sentence are potentially hallucinated.
- **German quality:** Excellent with the multilingual model.
- **Performance:** ~100-200ms per email on CPU.

**Go integration:** `knights-analytics/hugot` (ONNX Runtime wrapper) or Python helper script for eval-only context.

**For eval-only (recommended first step):** A ~30 line Python script using `sentence-transformers`. The eval already requires API keys and network — Python is acceptable there.

### Other Approaches Evaluated

| Approach | Verdict |
|----------|---------|
| NER (named entities) | Partially useful — better term extraction but doesn't solve paraphrase. `tsawler/prose` for Go (weak German), `hugot` + transformer NER (good but same ONNX dep). |
| YAKE keyword extraction | No Go implementation. Marginal improvement over current heuristics. |
| Regex/heuristic NER | Zero deps, catches ticket IDs, person names, URLs. Good quick win. |

---

## NLP as Live QA Stage (Post-LLM Gate)

### Tier 1: Zero dependencies (do now)
- Move existing deterministic checks into live pipeline
- Add prose detection regex (are bullets actually bullets?)
- Add formality heuristic (did greeting style change?)
- Extract dates, amounts, URLs alongside capitalized words

### Tier 2: Sentence embeddings (~440MB model)
- Replace term overlap with semantic similarity for info-loss detection
- ~400ms on CPU, fits interactive budget
- Go-native via `hugot` or ONNX Runtime

### Tier 3: NLI entailment (~1.1GB model)
- Independent hallucination detection using cross-lingual NLI model
- Architecturally most valuable: catches what self-eval misses (models blind to own hallucinations)
- ~800ms with narrowing optimization

---

## LangChain / LangGraph Findings

### langchaingo (Go): Dead
- 1 commit in 5 months, 398 open issues, 162 unmerged PRs, still v0.1.x after 3 years
- "Is this project dead?" issue filed with zero responses
- **Do not adopt.**

### LangChain.js: Alive and Active
- 14.3M monthly downloads, multiple releases/week, maintained by LangChain Inc.
- LangGraph.js production-ready (used by Elastic, 11x.ai)
- Security concern: 3 CVEs in late 2025/early 2026 (CVSS 9.3 deserialization flaw)

### LangChain.js + Wails v3: Works With Caveats
- **CORS is the issue, not WebKit2GTK.** Wails serves from `wails://wails` origin — external APIs reject it. Affects ALL platforms, not just Linux.
- Tauri disabled CORS (why v1 worked). Wails enforces it.
- Workaround: custom `fetch` wrapper routing through Wails RPC to Go
- Anthropic now supports browser CORS with special header; OpenAI does NOT
- LangChain.js 1.0+ broke browser support (`node:async_hooks`). Must pin to 0.3.x or polyfill.
- **Decision: parked.** Too many workarounds. Revisit if Wails fixes CORS.

### Eino (ByteDance): The Go Alternative
- `github.com/cloudwego/eino` — 10.3k stars, surpassed langchaingo
- Active daily commits, backed by ByteDance
- Graph workflows with Go generics, circuit breakers, exponential backoff
- **Worth evaluating if Go-native orchestration is needed.**

### Vercel AI SDK: TypeScript Leader
- 40.9M monthly downloads (3x LangChain.js), but N/A for Go backend architecture

### Honest Assessment
LangChain doesn't improve output quality directly. V1's quality came from the **multi-agent architecture** (independent specialist verification), not from LangChain's abstractions. We can rebuild that architecture in Go without any framework.

---

## Actionable Next Steps (When Revisiting)

1. **Quickest quality win:** Reimplement v1's 4-specialist architecture in pure Go (goroutines + errgroup)
2. **Eval improvement:** Python sentence-transformer script for semantic eval checks
3. **If orchestration grows:** Evaluate Eino as Go-native framework
4. **If LangChain.js revisited:** Check Wails CORS status and LangChain.js 1.0+ browser support
