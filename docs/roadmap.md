# KeyLint Roadmap

> Last updated: 2026-09-18 · Owner: Michael · Source of truth for priorities.
> Tracking: each epic links to a GitHub issue. This file holds the *why* and the order; issues hold the *what*.

---

## 1. Where we are (September 2026)

| Item | State |
|---|---|
| Latest release | `v4.4.3-beta` (2026-04-05) |
| Last commit on `main` | 2026-04-06 — ~5 months idle |
| Build / tests | `go build` ✅ · `go test ./internal/...` ✅ · Vitest 131/131 ✅ |
| Open PR | #31 shortcut single/double press (26 commits, CI green, mergeable, 0 behind main, **not merged**) |
| Open issues | #21–#30 (April triage: UI bugs, Bedrock, light theme, shortcut) |
| Stale remote branches | `feat/pyramidize`, `fix/updater-platform-aware-install` (both already merged, safe to delete) |

**Stack drift since April:**

| Dependency | Pinned | Latest | Note |
|---|---|---|---|
| `wailsapp/wails/v3` (Go) | `v3.0.0-alpha.72` | `v3.0.0-beta.23` | v3 Beta shipped Aug 2026; community reports alpha→beta is a version bump, no known breaking changes |
| `@wailsio/runtime` (npm) | `3.0.0-alpha.79` | `3.0.0-beta.23` | already **mismatched** with the Go side (alpha.72 vs alpha.79) |
| Angular | 21.2.0 | 22.1.x | |
| PrimeNG / `@primeuix/themes` | 21.1.3 / 2.0.3 | 22.1.x / 3.0.x | move together |
| TypeScript | 5.9 | 7.0 | TS 7 = Go-native compiler; only when Angular CLI supports it |
| Vitest / Playwright / jsdom | 4.0 / 1.58 / 28 | 5.0 / 1.63 / 30 | |

**Provider code today:** six hand-rolled HTTP functions (`enhance/service.go` ×3, `pyramidize/api_*.go` ×3), model IDs hardcoded in each, no `context.Context`, no timeouts, no retries, no tests for `enhance`. Bedrock is a stub that returns an error.

---

## 2. Principles (decisions, not suggestions)

1. **One-shot first.** Current frontier models (Opus 5 / Sonnet 5) make multi-call self-QA/refine pipelines the exception, not the default. Pipeline stages must earn their place in the eval. No LangChain, no Eino, no agent framework for what is a single prompt.
2. **We do not own provider plumbing.** Vendor-maintained SDKs (`anthropic-sdk-go`, `openai-go`) and locally installed agent CLIs do the talking. KeyLint owns one small interface (`internal/llm`) and nothing else. Hand-rolled HTTP against provider APIs is technical debt.
3. **Two on-ramps, equal citizens.** *BYOK* (API key in keyring) and *Bring Your Own Agent* (locally installed Claude Code, later Codex CLI / Gemini CLI). Subscription users should never have to create an API key.
4. **Windows-first.** Linux is a dev convenience. Every UX decision is judged on Windows + WebView2.
5. **Eval-gated prompt changes.** Any prompt or pipeline change to Pyramidize or Fix runs through `scripts/eval.sh` before merge. No tuning to individual samples.

---

## 3. Epics in priority order

### P0 — unblock

#### E0 · Land PR #31 (shortcut single/double press, configurable shortcuts)
The branch went further than the PR body says: `RegisterHotKey` was replaced by a `WH_KEYBOARD_LL` low-level hook, shortcuts are configurable with a recorder UI (see `.worktrees/shortcut-double-press/docs/superpowers/specs/2026-04-06-keyboard-hook-configurable-shortcuts-design.md`). Windows retest 2026-09-17 found double-tap running the fix twice; root cause was a leaked Win32 timer (`SetTimer(NULL, …)` ignores the passed ID). Fixed on the branch in five commits (timer ID, pump-thread reset, in-flight guard with safety timeout, held-key absorption); follow-ups in #42 and #44. Branch frozen for the final retest.

- [ ] Windows smoke test: single Ctrl+G → silent fix, hold-Ctrl-double-tap → Pyramidize + focus, recorder saves/reloads, no stray "g" typed into the foreground app
- [x] Update PR body to match the branch (hook + configurable shortcuts)
- [ ] `review-pr`, merge, close #30, delete worktree + branch
- [ ] Delete merged remotes `feat/pyramidize`, `fix/updater-platform-aware-install` (after #31 lands)

Why first: it touches `main.go`, settings model, `wails.service.ts`, and every component that subscribes to shortcuts. Everything below conflicts with it if it sits longer.

#### E1 · Claude Code as a first-class provider (`claude -p`) — **shipped 2026-09-18** (#45, #49)
Issue: #32 (closed). Structured output via `--json-schema` moved to #47; small follow-ups in #48. Discovery and completion must run in the same stripped environment (#49) — the probe once reported an `ANTHROPIC_API_KEY` session that completions could not use.

**User story.** Welcome wizard detects an installed `claude` binary → one click "Use Claude Code" → done. No API key, no console account. Fix and Pyramidize run through the user's own subscription. Same for the `-fix` CLI.

**Verified 2026-09-17 on this machine (Claude Code 2.1.274, logged in via claude.ai):**

```
echo "<text>" | claude -p --model haiku --output-format json \
  --no-session-persistence --tools "" --strict-mcp-config \
  --setting-sources "" --disable-slash-commands \
  --system-prompt "<fix prompt>"
```

| Metric | Value |
|---|---|
| Wall clock (Haiku, short text) | 2.2 s |
| Reported cost | `total_cost_usd: 0.0024` (billed to subscription) |
| Envelope fields | `result`, `is_error`, `terminal_reason`, `duration_ms`, `total_cost_usd`, `modelUsage`, `session_id` |

**Gotchas found:**
- `--bare` looks ideal (skips hooks, plugins, CLAUDE.md discovery) but **also skips credential reads** → `"Not logged in · Please run /login"` even when logged in. Do not use it; use `--setting-sources "" --strict-mcp-config --disable-slash-commands --tools ""` instead.
- `--json-schema` gives structured output for Pyramidize's JSON contract.
- Windows: the binary is `claude.exe` (native installer) or `claude.cmd` (npm shim). Resolve with `exec.LookPath` plus known install dirs; spawn with `HideWindow: true` so no console flashes from a tray app.
- Linux/macOS launched from a desktop launcher may not have `~/.local/bin` on `PATH`; probe known dirs.
- Login state: `claude auth status` returns JSON with `loggedIn` — use it for the wizard/health check and for actionable error messages ("open a terminal, run `claude`, sign in").
- Timeouts: wrap in `context.WithTimeout` (60–120 s), kill the process group on cancel.

**Terms of use constraints (Anthropic, checked 2026-09-17):**
- Allowed shape: run the **unmodified** Claude Code binary the user installed; each user signs in through Anthropic's own flow with their **own** subscription or key; KeyLint never reads, stores, or forwards OAuth tokens or credentials. This matches the "Can customers offer Claude Code in their products?" rules on the Claude Code legal page.
- Naming: we may say in plain text that KeyLint "uses the installed Claude Code CLI"; we may not use the Claude Code / Anthropic name or logo as part of a KeyLint feature name or brand.
- **Policy risk:** Anthropic announced, then paused (June 15, 2026), moving `claude -p` / Agent SDK / third-party app usage from subscription limits to a separate monthly credit. Today it draws from the subscription as before; Anthropic said it will give advance notice before any change. This is why BYOK stays an equal on-ramp — the CLI path must never be the only way.

**Scope:**
- [x] `internal/llm` interface (see E2 step 1) so the CLI provider is one file, not two copies
- [x] `internal/llm` Claude Code client: discovery, health check, one-shot call, JSON envelope parsing
  - [ ] structured-output mode via `--json-schema` (deferred: callers parse defensively today)
- [x] Settings: provider `claude-code` (no key UI; shows detected binary path + login state)
- [x] Welcome wizard: detect → one-click select; fallback to BYOK
- [x] Pyramidize + Fix + `-fix` CLI use it
- [x] Model picker for this provider uses aliases (`opus`, `sonnet`, `haiku`) — the CLI resolves the current generation
- [ ] Follow-up (separate issue when E1 ships): same interface for Codex CLI (`codex exec`) and Gemini CLI (`gemini -p`) → OpenAI/ChatGPT and Google subscribers

### P1 — foundation

#### E2 · Provider layer on vendor SDKs (stop maintaining API clients)
Issue: #33.

**Decision:** two vendor SDKs plus a 30-line interface. Evaluated and rejected: `langchaingo` (dead), `cloudwego/eino` (agent framework, overkill), unified wrappers (`omnillm`, `go-llm`, `any-llm-go`, `goai`, `pgedge-go-llm-lib`) — all small, young, and a third party between us and the vendors, i.e. the same maintenance risk we are trying to leave.

| Need | Covered by | Notes |
|---|---|---|
| Anthropic API | `github.com/anthropics/anthropic-sdk-go` (v1.73) | official |
| AWS Bedrock | same SDK, `bedrock.NewMantleClient` | closes #23 for free, #22 becomes moot |
| Google Vertex | same SDK, `vertex.WithGoogleAuth` | optional |
| OpenAI | `github.com/openai/openai-go/v3` (v3.61) | official |
| Ollama, OpenRouter, LM Studio, Groq, Mistral, … | `openai-go` + `option.WithBaseURL` | OpenAI-compatible endpoints; Ollama serves `/v1` |
| Claude Code CLI | `internal/llm/claudecli` (E1, #32) | spawn |

**Steps (each shippable):**
1. **Done (#39).** Introduce `internal/llm`: `Client` interface (`Complete(ctx, Request) (Response, error)`, `Request{System, User, Model, JSONSchema, MaxTokens}`), a registry keyed by provider ID, and move the six existing functions behind it **unchanged** (mechanical). Add `httptest`-based tests. `enhance` and `pyramidize` stop knowing about providers.
2. **Done (#45).** E1 plugs in here.
3. **Done (#33 step 3).** Replaced hand-rolled Anthropic/OpenAI/Ollama implementations with the two SDKs; also closes #41 (unredacted error bodies). Timeouts, retries and error types come from the SDK. Ollama moved to its OpenAI-compatible `/v1` endpoint, so the system prompt is a real system message and `Config.PromptSeparator` is gone — note that the delimiters went with it (`enhance` no longer sends its `Text: ` label, `pyramidize` no longer its `---` fence), which is a prompt change for a small local model and was **not** eval'd; JSON mode now also reaches Ollama, where the native endpoint ignored it. **Costs 15.3 MB of shipped binary** (12.0 → 27.3 MB with release flags) — the two SDKs weigh roughly 9 MB and 11 MB standalone, and there is no build tag to trim the generated type surface. Accepted: 27 MB is fine for a desktop installer, and the size is the price of not owning provider clients. The SDKs' environment defaults are switched off so `ANTHROPIC_BASE_URL`/`OPENAI_BASE_URL` cannot redirect a request, and their machine-fingerprint headers (`X-Stainless-*`, SDK `User-Agent`) are stripped. Bedrock stayed out: it needs a region field, a model-ID default per call site, and the AWS SDK — that is #23.
4. Model IDs become settings data with sane defaults per provider, not constants in six files. Where the provider has a models endpoint, offer a live list.

#### E3 · Prompt and core-logic overhaul: back to one-shot
Issue: #34.

Hypothesis: with Opus 5 / Sonnet 5 a single well-structured call matches the detect → foundation → self-QA → refine pipeline on the existing eval. If true, the refine stage and the quality-threshold setting go away, latency halves, and the code shrinks.

- [ ] Re-baseline the current pipeline on Sonnet 5 and Opus 5 (`scripts/eval.sh --provider claude --model …`) — numbers into `docs/pyramidize/quality-status.md`
- [ ] Prompt variant `v3`: single call, structured output via `--json-schema` / `output_config.format`, no self-QA JSON
- [ ] Compare via eval; keep whichever wins, delete the loser. Rules from `feedback_no_overfitting`: general principles only
- [ ] Fix prompt: same exercise with a small sample set (currently no eval for Fix — add one, 10–15 samples)
- [ ] Parked until eval says otherwise: v1 specialist architecture, NLP QA stages (`docs/pyramidize/adr-001-pipeline-architecture.md`)

#### E4 · Dependency upgrade wave
Issue: #35.

Order matters:
1. Wails: bump Go module to `v3.0.0-beta.23` **and** `@wailsio/runtime` to the same version; regenerate bindings; smoke test tray, window hide/show, events, Windows build.
2. Angular 22 + PrimeNG 22 + `@primeuix/themes` 3 together (`ng update`); re-check the custom wizard (`@switch`) and dark-mode CSS overrides in `styles.scss`, which patch PrimeNG gaps that may be fixed or moved.
3. Vitest 5, Playwright 1.63, jsdom 30, Prettier.
4. TypeScript 7: only once `@angular/build` lists it as supported.
5. Node 24 LTS stays.

Definition of done: CI green on Linux + Windows, `wails3 dev` works, one manual Windows run.

### P2 — polish and parity

#### E5 · UI modernization and fixes
- **Fix page redesign** (#36) — low-mid prio per Michael. Intent: it should feel like a tool, not two grey boxes. Direction to explore with `frontend-design`: input/result as one surface with a visible diff of what changed, provider/model chip, character count, keyboard hints (Ctrl+Enter), clear empty state, result actions (copy / paste back / undo).
- **Input border glitch** (#37) — **fixed (#40)**: the `.app-dark` override gave `.p-select-label` an opaque background that painted over the parent's rounded corners. Underlying cause (Aura dark tokens not propagated to components) goes with #35.
- **Settings stays rendered after version click** (#38) — new, repro in the issue.
- Existing April triage: #21 `vv` version prefix, #24 hide unimplemented themes, #26 toggle knob clipping, #28 global-instruction input light background, #29 Apply button focus, #27 side-by-side on wide screens.

#### E6 · Bedrock end-to-end UI (#23) — after E2 step 3, mostly settings UI work.
#### E7 · Light theme (#25) — after E4 (PrimeNG 22 may change token layout).
#### E8 · Feature parity leftovers (moved from the old `TODO.md`)
- Linux global hotkey (currently a stub) — low, Linux is dev-only
- HTML clipboard (CF_HTML) paste-back for Outlook/Teams — medium, real user value on Windows
#### E9 · Pyramidize quality track — parked (see E3). Revisit only with eval evidence.

---

## 4. Suggested release train

| Release | Contents |
|---|---|
| `v4.5.0-beta` | E0 (PR #31) · E2 step 1 · E1 Claude Code provider · quick UI bugs #21 #24 #26 #28 |
| `v4.6.0-beta` | E2 steps 3–4 (vendor SDKs, data-driven models) · E3 one-shot overhaul with eval |
| `v4.7.0-beta` | E4 upgrade wave · Fix page redesign · border glitch |
| `v5.0.0` | Bedrock UI · light theme · HTML clipboard · Codex/Gemini CLI providers |

---

## 5. Housekeeping (no release needed)

- [x] PR #31 description is stale vs. branch content
- [x] `frontend/e2e/shell-menu-deep{,2,3,4,5}.spec.ts` are exploratory layout probes from a debugging session — consolidate into one spec or delete (#43: hover-expand, two-tone logo and active-icon colour folded into `shell-menu.spec.ts`, the rest deleted)
- [ ] `awalsh128/cache-apt-pkgs-action@latest` (`build-linux.yml` ×3, `release.yml`) is a third-party action on a mutable tag in a fork-triggered workflow — pin to a commit SHA
- [ ] `.claude/rules/angular-components.md` exists but is not referenced from `CLAUDE.md`
- [x] `internal/features/enhance` has no tests — fixed in #39
- [ ] `docs/pyramidize/ux-roadmap.md` model strategy section is outdated (Sonnet 4.6 / GPT-5.2 era); superseded by E2 step 4
- [ ] Pin third-party GitHub Actions to commit SHAs, add Dependabot for actions (#51) — highest-severity CI finding
- [ ] Branch protection on `main` (required checks incl. `e2e`) — Michael, repo settings
- [ ] Shortcut robustness under rapid input (#42, #44) after #31 lands
- [ ] CLI `-fix` hangs on a never-closing stdin pipe (#46)
