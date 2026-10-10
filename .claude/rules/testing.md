# Testing Rules

## Vitest (Frontend)

- Use `async/await`, not `fakeAsync`/`tick` — no zone.js
- `vi.mock()` of a relative (or absolute) path is blocked by the Angular Vitest runner — up to 21.2.0 it was blocked outright, since 21.2.1 only paths starting with `.` or `/` throw. The bindings and every app module are imported by relative path, so in practice: use Angular DI mocking instead
- Mock pattern: `createWailsMock()` from `frontend/src/testing/wails-mock.ts`, inject via `{ provide: WailsService, useValue: wailsMock }`
- DOM-first assertions: assert on what the user sees (`data-testid` attributes), not internal state
- After `fixture.detectChanges()`, always `await fixture.whenStable()` before querying async-loaded DOM
- NG0100 fix: call `fixture.detectChanges()` twice, or pre-set async state before the first call
- Zoneless: a plain field set from the test does not mark the view dirty, so `detectChanges()` skips it and then reports NG0100. Mark it first — `fixture.debugElement.injector.get(ChangeDetectorRef).markForCheck()` (see `fix.component.spec.ts`) — or drive the state through signals
- CDK overlays (kl-select, klTooltip) render into `.cdk-overlay-container` on `<body>`, outside the fixture: query `document`, and empty the container in `afterEach` (spec files share a document)

## jsdom Polyfills

jsdom 30 still defines none of `matchMedia`, `ResizeObserver` or
`IntersectionObserver` — verified against the library and in a spec run on its
own.

No spec needs a stub today. The ones the suite carried were for PrimeNG's tabs
(`ResizeObserver`) and overlays (`matchMedia`) and went with it in E11. KeyLint's
own primitives and the CDK overlay they use need neither: `kl-tabs` measures its
row with a `ResizeObserver` where one exists and skips it where none does, so
under jsdom it measures once, on first render (every spec file was run on its
own to check). If new code or a new library needs one, the file
that needs it sets it itself — **in a full run a spec may see a global anyway**,
set by another file, so a green suite is no proof that your stub is there (see
the runner section below). And it is never permission to use `matchMedia` in
app code for theme detection, see `architecture.md`.

## The Test Runner

`ng test` runs through `@angular/build:unit-test` with `runner: vitest`
(`frontend/angular.json`). Two things about it are worth knowing before you
change anything.

**Spec files are NOT isolated from each other.** The builder sets
`isolate: false` (`@angular/build/src/builders/unit-test/runners/vitest/plugins.js`),
so the files that land in one fork share a process, a `globalThis` and one jsdom
window. Measured on this suite (when it still carried PrimeNG stubs): 11 of
14 files landed in a single fork and saw `ResizeObserver` and `matchMedia`
already defined on entry, set by two files that happened to run before them.
Which fork a file lands in changes between runs, so a spec that depends on
another file's global passes or fails by scheduling.

So: every file that needs a stub sets its own, and a spec that has just been
written or changed gets one run on its own —
`npx ng test --include "src/path/to.spec.ts"` — before you trust it. The same
applies to anything else process-global: module singletons, leftover TestBed
state, patched prototypes.

**vitest stays on 4.x.** The builder declares vitest as an optional peer at
`^4.0.8` on every published 21.2.x and on 22.0.x–22.1.x. 22.2.0 (2026-09-23)
is the first to accept `^5.0.0`, so this lifts with an Angular upgrade to
>= 22.2, not with any Angular 22. vitest 5
does run — the suite passes and the builder's watch path works — but it makes
`npm install` fail outright with `ERESOLVE could not resolve`, for every
developer and every package, until someone adds `--legacy-peer-deps`. `npm ci`
is unaffected, so CI would never have caught it. A test-runner major is not
worth that, so the bump stopped at 4.1.11.

Once `@angular/build` is on >= 22.2 (the first to declare `^5`), two things need handling in the same
change: vitest 5 flips `clearMocks` from `false` to `true` by default and the
builder does not set it, so a mock's calls no longer survive into the next test
— an assertion on call counts that should fail could start passing. It also
changes the default reporter, which swallows `console.log` (`--reporters=verbose`
brings it back).

## Go Tests

Settings tests use `XDG_CONFIG_HOME` env override to redirect file I/O to a temp dir.

## E2E (Playwright)

- CI runs the whole suite (`build-linux.yml`, job `e2e`): add a spec and it runs, no allowlist to update
- Generated output — `e2e/screenshots/`, `playwright-report/`, `test-results/` — is gitignored; `git status` must stay clean after a run
- Config auto-starts `ng serve` on port 4200
- A spec that needs a configured app (keys set, CLI signed in, a saved active
  provider) uses `e2e/support/fake-backend.ts`: it answers the settings
  service's binding calls from in-memory state via `page.route()` and records
  what the app saved. Every other service still 404s, as without it
- No E2E test reaches a provider. The app calls Go through Wails, and `ng serve`
  has no Wails bridge, so anything needing a real completion cannot run under
  Playwright as the suite stands. `silent-fix.spec.ts` is skipped for that
  reason and its header explains what reviving it would take — the
  `page.route()` proxy and the `_e2e_apikey_claude` localStorage key in it
  belong to a browser-mode fallback that no longer exists.
