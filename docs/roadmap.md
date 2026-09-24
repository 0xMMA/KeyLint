# KeyLint Roadmap

> Last updated: 2026-09-24 · Owner: Michael · Source of truth for priorities.
> Tracking: each epic links to a GitHub issue. This file holds the *why* and the order; issues hold the *what*.

---

## 1. Where we are (September 2026)

> **2026-09-24:** no code since 2026-09-18. This pass reconciles the file with the repo: #47 and #55 closed, WebView2 verification shipped (#66/#74), E4 step 3 partly done (#84).
>
> **Progress log 2026-09-17/18:** E1 shipped (Claude Code provider, #45/#49/#54/#72), E2 complete (#39/#52/#57/#60), E3 measured (ADR-002 #63; Fix eval #79/#82/#88 — suite saturated, harder samples needed before more prompt work), eval instrument hardened (#62), CI: e2e + race detector + bindings drift + pinned actions + WebView2 signature check (#50/#65/#73/#74/#81/#84/#87). Waiting on Michael: #31 Windows retest, ADR-002 decisions (Sonnet 5 default, AUTO cost, dead threshold slider, `--json` score), #38 DevTools check, #61 refresh feel.

| Item | State |
|---|---|
| Latest release | `v4.4.3-beta` (2026-04-05) |
| Last commit on `main` | 2026-09-18 — restart sprint (#39–#88) |
| Build / tests | `go build` ✅ · `go test ./internal/...` ✅ · Vitest 198/198 ✅ (2026-09-24) |
| Open PR | #31 shortcut single/double press — branch frozen, waiting on the Windows retest |
| Open issues | April triage #21–#30 · shortcut robustness #42 #44 · #34 E3 · #35 E4 · #36 Fix page · #38 shell · #56 shell specs · #61 model-list refresh · #80 #83 Fix prompt |
| Stale remote branches | `feat/pyramidize`, `fix/updater-platform-aware-install` (both already merged, safe to delete) |

**Stack drift since April:**

| Dependency | Pinned | Latest | Note |
|---|---|---|---|
| `wailsapp/wails/v3` (Go) | `v3.0.0-alpha.72` | `v3.0.0-beta.23` | v3 Beta shipped Aug 2026; community reports alpha→beta is a version bump, no known breaking changes |
| `@wailsio/runtime` (npm) | `3.0.0-alpha.79` | `3.0.0-beta.23` | already **mismatched** with the Go side (alpha.72 vs alpha.79) |
| Angular | 21.2.0 | 22.1.x | |
| PrimeNG / `@primeuix/themes` | 21.1.3 / 2.0.3 | 22.1.x / 3.0.x | move together |
| TypeScript | 5.9 | 7.0 | Angular 22 requires 6.0 (`>=6.0 <6.1`); TS 7 = Go-native compiler, only when Angular CLI supports it |
| Vitest / Playwright / jsdom | 4.1 / 1.63 / 30 | 5.0 / 1.63 / 30 | #84; Vitest 5 blocked by the `@angular/build` peer range (`^4.0.8` up to 22.1.x; 22.2.0 accepts `^5`), see `.claude/rules/testing.md` |

**Provider code today:** one `internal/llm` package behind a `Client` interface — the vendor SDKs for Claude, OpenAI and Ollama, plus the spawned Claude Code CLI — with contexts, timeouts, SDK retries and tests on every path (#39, #45, #33 step 3). Model IDs are still constants at the two call sites; that is step 4. Bedrock is still a stub that returns an error (#23).

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
Issue: #32 (closed). The #48 follow-ups shipped. Structured output via `--json-schema` is wired but **off by default** — #47 closed as parked, see E2 step 3b. Discovery and completion must run in the same stripped environment (#49) — the probe once reported an `ANTHROPIC_API_KEY` session that completions could not use.

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

#### E2 · Provider layer on vendor SDKs — **complete 2026-09-18** (#39, #45, #52, #57, #60)
Issue: #33 (closed). Follow-ups: #47 schema enforcement (closed, parked default-off), #55 status caching (done, #72), #61 list refresh UX.

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
3. **Done (#33 step 3).** Replaced hand-rolled Anthropic/OpenAI/Ollama implementations with the two SDKs; also closes #41 (unredacted error bodies). Timeouts, retries and error types come from the SDK. Ollama moved to its OpenAI-compatible `/v1` endpoint, so the system prompt is a real system message and `Config.PromptSeparator` is gone — note that the delimiters went with it (`enhance` no longer sends its `Text: ` label, `pyramidize` no longer its `---` fence), which is a prompt change for a small local model and was **not** eval'd; JSON mode now also reaches Ollama, where the native endpoint ignored it. **Costs 15.3 MB of shipped binary** (12.0 → 27.3 MB with release flags) — the two SDKs weigh roughly 9 MB and 11 MB standalone, and there is no build tag to trim the generated type surface. Accepted: 27 MB is fine for a desktop installer, and the size is the price of not owning provider clients. The SDKs no longer take configuration from the environment: Anthropic through `WithoutEnvironmentDefaults`, OpenAI — which has no such option — by always passing the base URL explicitly and deleting the headers `OPENAI_ORG_ID`, `OPENAI_PROJECT_ID` and `OPENAI_CUSTOM_HEADERS` would set. That covers the variables those SDK versions document; a future one could add another, so the tests assert on the wire rather than on the option list. Machine-fingerprint headers (`X-Stainless-*`, SDK `User-Agent`) are stripped too. Bedrock stayed out: it needs a region field, a model-ID default per call site, and the AWS SDK — that is #23.
3b. **Wired, default off (#47 closed as parked).** `Request` gained `JSONSchema json.RawMessage` alongside `JSONMode`. Pyramidize has a schema per pipeline step (`pyramidize/schemas.go`, mirroring `types.go`) and every provider *can* enforce it in its own dialect — `response_format: json_schema` for OpenAI and the Ollama `/v1` endpoint, `output_config.format` for Anthropic, `--json-schema` for the Claude Code CLI, which also returns the parsed object in `structured_output`. `unmarshalRobust` stays as the fallback. Enforcement is behind `KEYLINT_PYRAMIDIZE_SCHEMA=1` / `./scripts/eval.sh --schema`: two eval runs showed it fixing one reproducible parse failure and collapsing a different sample to 0.1 with a document cut off mid-sentence, so it waits for E3 (#34) evidence on a reworked prompt.
4. **Done (#33 step 4).** Model IDs are settings data: `internal/llm/models.go` holds a default per provider *and per feature* (the silent fix wants a fast model, Pyramidize a stronger one) plus a curated fallback list, and `settings.json` carries the user's choice under `models`. Resolution is request override → settings → default. `ListModels` asks Anthropic and OpenAI through the SDKs and Ollama through `/api/tags`. `ModelList.Source` names what happened — `live`, `empty`, `unusable`, `unreachable`, `no-credentials`, `fixed` — because each is a different thing for the user to do; the UI has a sentence per case and the cache keeps a settled answer for 10 minutes and everything else for 30 seconds. Defaults are unchanged — moving them is E3's call and needs an eval.

#### E3 · Prompt and core-logic overhaul: back to one-shot
Issue: #34.

Hypothesis: with Opus 5 / Sonnet 5 a single well-structured call matches the detect → foundation → self-QA → refine pipeline on the existing eval. If true, the refine stage and the quality-threshold setting go away, latency halves, and the code shrinks.

- [x] **Prerequisite done (#53, #59):** the eval is trustworthy enough to compare against. Runs are isolated from the developer's settings and keyring, the judge is pinned to a dated snapshot at temperature 0, `summary.json` records the full effective config, and `--runs n` produces a baseline with a spread. The current default (`claude-sonnet-4-6`, schema off) is re-baselined in `quality-status.md` with its noise floor.
- [ ] Re-baseline on **Opus 5** — Sonnet 5 is done (`quality-status.md`, three runs: deterministic clearly up, judge inconclusive and noisier), Opus 5 is not
- [x] ~~Prompt variant `v3`: single call, no self-QA JSON~~ — **not built, see [ADR-002](pyramidize/adr-002-one-shot-vs-pipeline.md).** The typed email path has been a single call since v2, so a v3 would have compared two one-shot prompts. The pipeline-vs-one-shot question was answered with the existing v1 (pipeline) against v2 (one-shot): quality inconclusive, reliability clearly one-shot.
- [ ] Still open from that line: **structured output** (`--json-schema` / `output_config.format`). All three baselines ran with `schemaEnforcement: false`, so enforcement remains unmeasured (#47 is closed; turning it on is this epic's call)
- [ ] Still open: the **AUTO default costs two calls** (detect + foundation) and is what ships; changing it needs detection-accuracy evidence that does not exist yet
- [ ] Compare via eval; keep whichever wins, delete the loser. Rules from `feedback_no_overfitting`: general principles only
- [x] Fix prompt eval: 15 samples in `test-data/fix-samples/`, deterministic checks plus the same pinned judge, baselined on the shipped default — see [`docs/fix/quality-status.md`](fix/quality-status.md). It found three reproducible rule violations (trailing commentary on correct text, translating deliberate code-switching, and answering a chat-style message instead of correcting it); no prompt change in that PR, by the no-overfitting rule
- [x] Act on what the Fix eval found (#80): the input is delimited as a document and the output contract is stated. Two of the three violations are gone in all three runs — the model no longer answers a chat message (judge 0.25 → 0.97 on that sample) and no longer annotates correct text (0.30 → 1.00) — with no post-processing involved, so it is the prompt's own behaviour. Judge overall 0.8520 → 0.9369 with disjoint ranges in all three measurements of this prompt; the deterministic gain (0.9263 → 0.9537) sits at the edge of what three runs resolve and flips verdict between measurements. Two samples regressed (`en-german-word-ersetzen`, `markdown-struktur`, both 3/3 → 1/3). The third violation stays on #80, the output guard became #83. See [`docs/fix/quality-status.md`](fix/quality-status.md)
- [x] **Held-out split for the Fix suite (precondition for the next #80 work).** `test-data/fix-samples/{tune,holdout}/`, 10 and 5, derived by a stated rule rather than picked; `--split tune|holdout|all` with `all` the default, and `split` in the `configKey` so `--compare` refuses to read one half against the other. Protocol and the rule in [`docs/fix/quality-status.md`](fix/quality-status.md). The holdout is measured at 0.9986 (spread 0.0013) on the shipped prompt: a sharp regression detector with 0.0014 of headroom, so it cannot show improvement. That is the suite, not the split — ten of fifteen samples sit at a ceiling
- [ ] **The Fix suite is saturated and needs samples with headroom.** Ten of fifteen score at a ceiling with zero spread, and the five-sample holdout averages 0.9986 — so no measurement on it can say a prompt got better, only that it got worse. Worth more than further tuning of the split or the prompt
- [ ] Parked until eval says otherwise: v1 specialist architecture, NLP QA stages (`docs/pyramidize/adr-001-pipeline-architecture.md`)

#### E4 · Dependency upgrade wave
Issue: #35.

Order matters:
1. Wails: bump Go module to `v3.0.0-beta.23` **and** `@wailsio/runtime` to the same version; regenerate bindings; smoke test tray, window hide/show, events, Windows build.
2. Angular 22 + PrimeNG 22 + `@primeuix/themes` 3 together (`ng update`); re-check the custom wizard (`@switch`) and dark-mode CSS overrides in `styles.scss`, which patch PrimeNG gaps that may be fixed or moved.
3. ~~Playwright 1.63, jsdom 30, Prettier~~ **done (#84)**, Vitest to 4.1. Vitest 5 waits until `@angular/build` is on >= 22.2, the first to declare `^5` (i.e. with step 2) — then handle the `clearMocks` default flip and the reporter change in the same PR (`.claude/rules/testing.md`).
4. TypeScript: Angular dictates it — Angular 22 requires `>=6.0 <6.1`, so step 2 moves TypeScript 5.9 → 6.0 with it. TypeScript 7 only once `@angular/build` lists it.
5. Node 24 LTS stays.

Definition of done: CI green on Linux + Windows, `wails3 dev` works, one manual Windows run.

### P2 — polish and parity

#### E5 · UI modernization and fixes
- **Fix page redesign** (#36) — low-mid prio per Michael. Intent: it should feel like a tool, not two grey boxes. Direction to explore with `frontend-design`: input/result as one surface with a visible diff of what changed, provider/model chip, character count, keyboard hints (Ctrl+Enter), clear empty state, result actions (copy / paste back / undo).
- **Input border glitch** (#37) — **fixed (#40)**: the `.app-dark` override gave `.p-select-label` an opaque background that painted over the parent's rounded corners. Underlying cause (Aura dark tokens not propagated to components) goes with #35.
- **Settings stays rendered after version click** (#38) — new, repro in the issue.
- Existing April triage: #21 `vv` version prefix, #24 hide unimplemented themes, #26 toggle knob clipping, #28 global-instruction input light background, #29 Apply button focus, #27 side-by-side on wide screens.

#### E6 · Bedrock end-to-end UI (#23) — unblocked (E2 step 3 done), mostly settings UI work plus the region field and AWS SDK noted in E2 step 3. Closes #22.
#### E7 · Light theme (#25) — after E4 (PrimeNG 22 may change token layout).
#### E8 · Feature parity leftovers (moved from the old `TODO.md`)
- Linux global hotkey (currently a stub) — low, Linux is dev-only
- HTML clipboard (CF_HTML) paste-back for Outlook/Teams — medium, real user value on Windows
#### E9 · Pyramidize quality track — parked (see E3). Revisit only with eval evidence.

---

## 4. Suggested release train

| Release | Contents |
|---|---|
| `v4.5.0-beta` | E0 (PR #31) · ~~E2 step 1~~ ✅ · ~~E1 Claude Code provider~~ ✅ · quick UI bugs #21 #24 #26 #28 — waiting on E0 and the UI bugs |
| `v4.6.0-beta` | ~~E2 steps 3–4 (vendor SDKs, data-driven models)~~ ✅ · E3 one-shot overhaul with eval |
| `v4.7.0-beta` | E4 upgrade wave · Fix page redesign · border glitch |
| `v5.0.0` | Bedrock UI · light theme · HTML clipboard · Codex/Gemini CLI providers |

---

## 5. Housekeeping (no release needed)

- [x] PR #31 description is stale vs. branch content
- [x] `frontend/e2e/shell-menu-deep{,2,3,4,5}.spec.ts` are exploratory layout probes from a debugging session — consolidate into one spec or delete (#43: hover-expand, two-tone logo and active-icon colour folded into `shell-menu.spec.ts`, the rest deleted)
- [x] `GetClaudeCodeStatus` spawns two processes per call and is called from four uncached places (Pyramidize init and provider change, the settings card, the welcome wizard) — 0.13s on a native binary here, materially slower through a Windows `claude.cmd` shim, and the Pyramidize page's first paint waits on it. Cache it in the Go service with a bypass for the Re-check button. Done (#55, #72).
- [x] `awalsh128/cache-apt-pkgs-action@latest` (`build-linux.yml` ×**4**, `release.yml` ×3 — the count here was stale) pinned to `553a35bb` (v1.6.3). Upstream has since deprecated its own `latest` tag and tells consumers to pin
- [x] `.claude/rules/angular-components.md` exists but is not referenced from `CLAUDE.md` — deleted instead: it was a path-scoped copy of `architecture.md#component-structure`, which loads on every session anyway
- [x] `internal/features/enhance` has no tests — fixed in #39
- [x] `docs/pyramidize/ux-roadmap.md` model strategy section is outdated (Sonnet 4.6 / GPT-5.2 era); superseded by E2 step 4 — replaced by a pointer to `internal/llm/models.go`
- [x] Pin third-party GitHub Actions to commit SHAs, add Dependabot for actions (#51, #65, #73, #81). `actions/*` stay on floating major tags by design — GitHub's own org, and Dependabot is configured to ignore majors since #81 — so the workflows are hardened, not fully SHA-pinned
- [ ] Quarterly dependency hygiene — Dependabot deliberately stays silent on these, so this check is the only notice. (1) Actions: check `gh api repos/actions/<name>/releases/latest` for every `actions/*` in `.github/workflows/` and bump paired actions together (artifact upload/download, pages upload/deploy). (2) Framework majors: `cd frontend && npm outdated` for Angular, PrimeNG, `@primeuix/*`, primeicons, TypeScript, Vitest. (3) Wails: `go list -m -u github.com/wailsapp/wails/v3` and its open security advisories. Next due: 2026-12.
- [x] Add `gomod` and `npm` to Dependabot (#89) — monthly, grouped, 7-day npm cooldown (`packageManager` moved to npm 11.19 so it covers transitive packages too). It does **not** surface the drift that prompted this item: Wails is ignored on both sides, and framework majors (Angular, PrimeNG, `@primeuix/*`, primeicons, TypeScript) are blocked because they need `ng update` and a visual check. The quarterly check above surfaces those. Security updates skip the ignores, so an advisory can still arrive as a PR — as a notice, not merge-on-green
- [x] `MicrosoftEdgeWebview2Setup.exe` was curled from a redirector with no checksum or signature check and bundled into the installer users run — now `scripts/fetch-webview2.sh` refuses any file without a valid Microsoft signature, in both `release.yml` and `build-linux.yml` (#66, #74)
- [ ] Branch protection on `main` (required checks incl. `e2e`, `test` with race detector, bindings drift) — Michael, repo settings
- [ ] Shortcut robustness under rapid input (#42, #44) after #31 lands
- [x] CLI `-fix` hangs on a never-closing stdin pipe (#46) — stdin moved after the inline argument, plus a 15 s idle timeout that also catches a pipe which speaks once and stays open
