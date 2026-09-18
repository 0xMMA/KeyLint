# Testing Rules

## Vitest (Frontend)

- Use `async/await`, not `fakeAsync`/`tick` — no zone.js
- `vi.mock()` is blocked by the Angular Vitest runner — use Angular DI mocking instead
- Mock pattern: `createWailsMock()` from `frontend/src/testing/wails-mock.ts`, inject via `{ provide: WailsService, useValue: wailsMock }`
- DOM-first assertions: assert on what the user sees (`data-testid` attributes), not internal state
- After `fixture.detectChanges()`, always `await fixture.whenStable()` before querying async-loaded DOM
- NG0100 fix: call `fixture.detectChanges()` twice, or pre-set async state before the first call

## jsdom Polyfills

Still true on jsdom 30, checked inside the running test environment rather than
against the library: `matchMedia`, `ResizeObserver` and `IntersectionObserver`
are all undefined. Re-check with a throwaway spec before assuming a bump changed
it — the library and the environment the builder sets up are not the same thing.

TabList uses `ResizeObserver` which doesn't exist in jsdom. Add at the top of specs that import tab components:

```typescript
(globalThis as any)['ResizeObserver'] = class { observe() {} unobserve() {} disconnect() {} };
```

PrimeNG's overlay (any `p-select` dropdown that a spec opens) asks `matchMedia`
whether to go modal, and jsdom has none. Stub it to "no match" in specs that
open one. This is a jsdom gap in a library, not permission to use `matchMedia`
in app code — KeyLint's own code must not read it for theme detection, see
`architecture.md`.

## The Test Runner

`ng test` runs through `@angular/build:unit-test` with `runner: vitest`
(`frontend/angular.json`). The builder declares vitest as an **optional** peer
at `^4.0.8` — including on `@angular/build@22`, the newest published — while the
repo runs vitest 5. That is deliberate and measured: `npm ci` resolves it
cleanly, all 198 tests pass, and a deliberately broken assertion still fails.
What it means is that the pairing is unsupported rather than merely untested, so
a builder-side breakage has no upstream guarantee behind it. Read the
`ERESOLVE overriding peer dependency` line in an install log as expected, not as
a problem to fix by downgrading something else.

Each spec file gets its own environment, so a global stub (`ResizeObserver`,
`matchMedia`) belongs in every file that needs it. Do not rely on another
file having set one: leakage between files was observed once and did not
reproduce in ten runs, which is the worst kind of dependency to have.

## Go Tests

Settings tests use `XDG_CONFIG_HOME` env override to redirect file I/O to a temp dir.

## E2E (Playwright)

- CI runs the whole suite (`build-linux.yml`, job `e2e`): add a spec and it runs, no allowlist to update
- Generated output — `e2e/screenshots/`, `playwright-report/`, `test-results/` — is gitignored; `git status` must stay clean after a run
- Config auto-starts `ng serve` on port 4200
- Anthropic API is CORS-blocked in browsers — use `page.route()` proxy pattern
- API key injection: `localStorage.setItem('_e2e_apikey_claude', key)`
