# Refine Requirements — Interactive Requirements Engineering

Take a plan file and refine its requirements through structured exploration, interactive Q&A with ASCII mockups, and play-pretend walkthroughs that surface gaps naturally.

**Input:** `$ARGUMENTS` is the path to the plan file (e.g. `PYRAMIDIZE.md`, `docs/plan.md`).

If `$ARGUMENTS` is empty, use `AskUserQuestion` to ask: "Which plan file should I refine? (relative path from project root)"

---

## Rules

- **Max 3-4 questions per round.** Never wall-of-text the user.
- **Never assume — ask when ambiguous.** A wrong assumption costs more than a question.
- **Never copy v1 blindly.** If there's prior art, question whether old decisions still apply.
- **Always show, don't tell.** Every question with a UI or layout implication gets an ASCII mockup. Abstract descriptions are not acceptable — make it concrete.
- **Stay in character during Phase 3.** Narrate as if the feature exists. Break character only to surface a gap, then resume.
- **Requirements and design only.** Never ask about implementation details (JSON parsing, HTTP clients, DI wiring, test scaffolding) — those are the developer's domain.
- **Capture decisions immediately.** After each round, note what was decided before moving on.

---

## Phase 1 — Deep Exploration (silent)

Do all of this silently. Do NOT output anything to the user yet.

1. Read the plan file at `$ARGUMENTS`.
2. Read `CLAUDE.md` and any architecture docs it references (`.claude/docs/architecture.md`, `.claude/docs/testing.md`, etc.).
3. Search the codebase for existing implementations related to the plan's feature area — look at the code, not just file names. Check for archived or previous versions if referenced.
4. Read any related Angular components, Go services, and shared utilities that the feature will touch or extend.
5. Build a mental model of:
   - What exists today that this feature builds on
   - What constraints the current architecture imposes
   - What patterns the codebase already uses (and should be followed)
   - Where the plan has gaps, ambiguities, or implicit assumptions

When done, output a single short message: "I've explored the codebase and the plan. Starting requirements review — Phase 2."

---

## Phase 2 — Structured Requirements Review

Go through the plan section by section. For each section that has decisions to make or ambiguities to resolve:

1. Present 3-4 questions (never more per round).
2. Every question MUST include:
   - **2-4 concrete options** (labelled A, B, C, D)
   - **ASCII preview mockup** for any option that affects layout, UI, or user-visible behaviour
   - **Your recommendation** with a brief rationale (1 sentence)
3. Use `AskUserQuestion` to collect the user's choices.
4. After each round, summarize decisions made in a compact list before moving to the next section.

Example question format:
```
**Q2: How should the error state appear?**

Option A — Inline below the action area:
┌─────────────────────────────────────┐
│  [Action Button]                    │
│  ❌ Step 2/3 failed: timeout.       │
│     [Retry] [Settings →]           │
└─────────────────────────────────────┘

Option B — Toast notification:
┌─────────────────────────────────────┐
│  [Action Button]                    │
│                    ┌──────────────┐ │
│                    │ ❌ Timeout   │ │
│                    │ [Retry]      │ │
│                    └──────────────┘ │
└─────────────────────────────────────┘

Recommendation: A — keeps error context near the action.
```

Continue until all sections have been reviewed. Then announce: "Requirements review complete. Moving to play-pretend walkthrough — Phase 3."

---

## Phase 3 — Play-Pretend Walkthrough

Walk through the feature as if it's already built and shipping. You narrate in present tense. The user is the product owner / architect — you ask them requirements and design questions, never code-level implementation details.

### How to narrate

Speak as if you're a QA tester or product reviewer using the finished feature for the first time:

> "I open the app and navigate to the Pyramidize page. The left panel shows a doc type selector set to AUTO, a style dropdown, and a relationship dropdown. Below them is a large Pyramidize button with a Ctrl+Enter hint. The canvas area is empty — I see a placeholder with ghost text showing a sample pyramidized email..."

### When to pause and ask

Pause the narration whenever:
- The spec doesn't say what should happen → surface the gap
- Two requirements seem to conflict → ask which takes priority
- A behaviour feels wrong from a UX perspective → propose an alternative
- An edge case isn't covered → ask for the desired behaviour

When pausing, break character briefly:

> "**Gap found:** The spec doesn't say what happens when the user clicks Pyramidize again while the canvas already has edits from a previous run. Should it:
> A) Overwrite canvasText with the new result (edits lost)
> B) Ask 'Re-pyramidize from original? Your canvas edits will be lost' with [Yes] [No]
> C) Create a new trace entry and overwrite silently (edits recoverable via trace log)
>
> Recommendation: C — edits are never truly lost thanks to the trace log."

Then use `AskUserQuestion` to get the decision, note it, and resume narrating.

### Minimum scenarios to walk through

Cover ALL of these angles (not just UI walkthroughs):

1. **Happy path** — the golden scenario, start to finish
2. **First-time user** — no config, no presets, empty state
3. **Returning user** — presets exist, muscle memory, what's faster now
4. **Error / timeout** — API fails mid-pipeline, what does the user see and do
5. **Interruption / cancel** — user cancels during processing, closes mid-edit
6. **Edge cases** — empty input, very long input, mixed languages, rapid repeated actions
7. **State & lifecycle** — navigate away and back, minimize to tray, close window
8. **Hotkey vs manual** — differences in flow, what's available vs hidden
9. **Architecture choices** — new service vs extending existing, separate route vs same page, shared state
10. **Scope boundaries** — "this could grow into X — is X in scope or deferred?"

When all scenarios are covered, announce: "Play-pretend walkthrough complete. Moving to gap resolution — Phase 4."

---

## Phase 4 — Gap Resolution

1. Compile all remaining gaps, open questions, and ambiguities discovered during Phases 2 and 3.
2. Present them as a numbered list, grouped by theme (UI, behaviour, state, error handling, scope).
3. Ask in batches of 3-4 using `AskUserQuestion`.
4. For each gap, either:
   - Resolve it with the user's decision, OR
   - Mark it explicitly as **out of scope** with a reason

Continue until all gaps are resolved or marked out-of-scope. Then announce: "All gaps resolved. Updating the plan — Phase 5."

---

## Phase 5 — Document Update

1. Update the plan file (`$ARGUMENTS`) with all decisions made during this session:
   - Add/modify requirement entries
   - Update scoping decisions table
   - Update out-of-scope table
   - Add any new sections needed (e.g., new user stories, new NFRs)
   - Add a "Last updated" timestamp
2. Present a completion checklist:

```
Requirements Refinement Complete
────────────────────────────────
✅ Plan explored and understood
✅ N questions resolved across M rounds
✅ K scenarios walked through
✅ J gaps resolved, L marked out-of-scope
✅ Plan file updated: [filename]

Want to do one more round? (e.g., "walk through the admin scenario" or "what about offline mode?")
```

3. Use `AskUserQuestion` to ask if the user wants one more round.
   - If yes: return to Phase 3 or Phase 4 as appropriate, then repeat Phase 5.
   - If no: end the session.
