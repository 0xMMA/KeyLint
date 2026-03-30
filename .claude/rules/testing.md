# Testing Rules

## Vitest (Frontend)

- Use `async/await`, not `fakeAsync`/`tick` — no zone.js
- `vi.mock()` is blocked by the Angular Vitest runner — use Angular DI mocking instead
- Mock pattern: `createWailsMock()` from `frontend/src/testing/wails-mock.ts`, inject via `{ provide: WailsService, useValue: wailsMock }`
- DOM-first assertions: assert on what the user sees (`data-testid` attributes), not internal state
- After `fixture.detectChanges()`, always `await fixture.whenStable()` before querying async-loaded DOM
- NG0100 fix: call `fixture.detectChanges()` twice, or pre-set async state before the first call

## ResizeObserver Polyfill

TabList uses `ResizeObserver` which doesn't exist in jsdom. Add at the top of specs that import tab components:

```typescript
(globalThis as any)['ResizeObserver'] = class { observe() {} unobserve() {} disconnect() {} };
```

## Go Tests

Settings tests use `XDG_CONFIG_HOME` env override to redirect file I/O to a temp dir.

## E2E (Playwright)

- Config auto-starts `ng serve` on port 4200
- Anthropic API is CORS-blocked in browsers — use `page.route()` proxy pattern
- API key injection: `localStorage.setItem('_e2e_apikey_claude', key)`
