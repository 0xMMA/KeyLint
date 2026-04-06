# Shortcut Double-Press Detection — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add single/double press detection to the Ctrl+G hotkey so single press always triggers silent fix and double press opens Pyramidize UI.

**Architecture:** Go-side state machine in `main.go` buffers hotkey events with a 200ms timer. Emits `shortcut:single` or `shortcut:double` as distinct Wails events. Frontend subscribes to the appropriate event per component. The old undifferentiated `shortcut:triggered` event is removed.

**Tech Stack:** Go 1.26, Wails v3, Angular v21, RxJS, Vitest

**Spec:** `docs/superpowers/specs/2026-04-06-shortcut-double-press-design.md`

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Modify | `main.go:34,131-147` | Register new events, replace shortcut goroutine with state machine |
| Modify | `frontend/src/app/core/wails.service.ts:39-44,52-61,242-246` | Replace `shortcutTriggered$` with `shortcutSingle$` and `shortcutDouble$` |
| Modify | `frontend/src/testing/wails-mock.ts:34-42` | Replace mock subjects to match new observables |
| Modify | `frontend/src/app/features/fix/fix.component.ts:107` | Subscribe to `shortcutSingle$` |
| Modify | `frontend/src/app/features/text-enhancement/text-enhancement.component.ts:1096` | Subscribe to `shortcutDouble$` |
| Modify | `frontend/src/app/layout/shell.component.ts:102,113-117,153-155` | Add `shortcutDouble$` subscription for navigation to `/enhance` |
| Modify | `frontend/src/app/features/fix/fix.component.spec.ts:113-126` | Update tests for `shortcutSingle$` |
| Modify | `frontend/src/app/features/text-enhancement/text-enhancement.component.spec.ts:253-331` | Update tests for `shortcutDouble$` |
| Modify | `frontend/src/app/layout/shell.component.spec.ts` | Add test for navigate on `shortcutDouble$` |
| Create | `internal/features/shortcut/detect.go` | Standalone double-press detector (testable without Wails) |
| Create | `internal/features/shortcut/detect_test.go` | Unit tests for detector state machine |

---

### Task 1: Go double-press detector — test and implementation

**Files:**
- Create: `internal/features/shortcut/detect.go`
- Create: `internal/features/shortcut/detect_test.go`

The detector is a standalone struct with no Wails dependency. It receives raw press events and emits classified results (`Single` or `Double`) via a channel. This makes it fully testable with fake time.

- [ ] **Step 1: Write the failing tests**

Create `internal/features/shortcut/detect_test.go`:

```go
package shortcut

import (
	"testing"
	"time"
)

func TestDetector_SinglePress(t *testing.T) {
	d := NewDetector(200 * time.Millisecond)
	defer d.Stop()

	d.Press()

	select {
	case r := <-d.Result():
		if r != Single {
			t.Fatalf("expected Single, got %v", r)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for result")
	}
}

func TestDetector_DoublePress(t *testing.T) {
	d := NewDetector(200 * time.Millisecond)
	defer d.Stop()

	d.Press()
	d.Press() // within threshold

	select {
	case r := <-d.Result():
		if r != Double {
			t.Fatalf("expected Double, got %v", r)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for result")
	}
}

func TestDetector_SlowDoublePress(t *testing.T) {
	d := NewDetector(200 * time.Millisecond)
	defer d.Stop()

	d.Press()
	// Wait for first single to fire
	r1 := <-d.Result()
	if r1 != Single {
		t.Fatalf("expected first Single, got %v", r1)
	}

	d.Press()
	r2 := <-d.Result()
	if r2 != Single {
		t.Fatalf("expected second Single, got %v", r2)
	}
}

func TestDetector_TriplePress(t *testing.T) {
	d := NewDetector(200 * time.Millisecond)
	defer d.Stop()

	d.Press()
	d.Press()
	d.Press() // third press starts new cycle

	// First result: Double from presses 1+2
	r1 := <-d.Result()
	if r1 != Double {
		t.Fatalf("expected Double, got %v", r1)
	}

	// Second result: Single from press 3 (after timeout)
	select {
	case r2 := <-d.Result():
		if r2 != Single {
			t.Fatalf("expected Single, got %v", r2)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for second result")
	}
}

func TestDetector_Stop(t *testing.T) {
	d := NewDetector(200 * time.Millisecond)
	d.Stop()

	// Result channel should be closed after Stop
	_, ok := <-d.Result()
	if ok {
		t.Fatal("expected result channel to be closed")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/features/shortcut/ -run TestDetector -v`
Expected: compilation error — `NewDetector`, `Single`, `Double` not defined.

- [ ] **Step 3: Write the detector implementation**

Create `internal/features/shortcut/detect.go`:

```go
package shortcut

import "time"

// PressResult classifies a hotkey press as single or double.
type PressResult int

const (
	Single PressResult = iota
	Double
)

// Detector implements double-press detection with a configurable threshold.
// It receives raw press events via Press() and emits classified results on Result().
type Detector struct {
	threshold time.Duration
	presses   chan struct{}
	results   chan PressResult
	done      chan struct{}
}

// NewDetector creates and starts a detector goroutine.
func NewDetector(threshold time.Duration) *Detector {
	d := &Detector{
		threshold: threshold,
		presses:   make(chan struct{}),
		results:   make(chan PressResult, 2),
		done:      make(chan struct{}),
	}
	go d.loop()
	return d
}

// Press records a hotkey press. Non-blocking (channel send).
func (d *Detector) Press() {
	select {
	case d.presses <- struct{}{}:
	case <-d.done:
	}
}

// Result returns the channel that receives classified press results.
func (d *Detector) Result() <-chan PressResult {
	return d.results
}

// Stop shuts down the detector goroutine and closes the result channel.
func (d *Detector) Stop() {
	select {
	case <-d.done:
	default:
		close(d.done)
	}
}

func (d *Detector) loop() {
	defer close(d.results)

	var timer *time.Timer
	var timerC <-chan time.Time

	for {
		select {
		case <-d.done:
			if timer != nil {
				timer.Stop()
			}
			return

		case <-d.presses:
			if timer != nil {
				// Second press within window → double
				timer.Stop()
				timer = nil
				timerC = nil
				d.results <- Double
			} else {
				// First press → start detection window
				timer = time.NewTimer(d.threshold)
				timerC = timer.C
			}

		case <-timerC:
			// Timer expired → single press
			timer = nil
			timerC = nil
			d.results <- Single
		}
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/features/shortcut/ -run TestDetector -v`
Expected: all 5 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/features/shortcut/detect.go internal/features/shortcut/detect_test.go
git commit -m "feat(shortcut): add double-press detector with state machine (#30)"
```

---

### Task 2: Wire detector into `main.go` shortcut goroutine

**Files:**
- Modify: `main.go:34,131-147`

Replace the event registration and the shortcut goroutine with the state machine that uses the detector, emits distinct events, and focuses the window on double press.

- [ ] **Step 1: Update event registrations**

In `main.go`, replace line 34:

```go
// Before:
application.RegisterEvent[string]("shortcut:triggered")

// After:
application.RegisterEvent[string]("shortcut:single")
application.RegisterEvent[string]("shortcut:double")
```

- [ ] **Step 2: Replace the shortcut goroutine**

Replace `main.go:131-147` (the entire `go func()` block and its preceding comments) with:

```go
	// Double-press detection: single press → silent fix, double press → show Pyramidize UI.
	// Clipboard is captured on the first press (while source app still has focus).
	// The detector classifies presses and emits Single/Double results.
	detector := shortcut.NewDetector(200 * time.Millisecond)
	wailsApp.OnShutdown(func() { detector.Stop() })

	// Feed raw hotkey events into the detector; capture clipboard on each first press.
	go func() {
		ch := services.Shortcut.Triggered()
		for event := range ch {
			logger.Info("shortcut: triggered", "source", event.Source)
			pyramidizeSvc.CaptureSourceApp()
			if err := services.Clipboard.CopyFromForeground(); err != nil {
				logger.Warn("shortcut: CopyFromForeground failed", "err", err)
			}
			detector.Press()
		}
	}()

	// Consume classified results and emit the appropriate Wails event.
	go func() {
		for result := range detector.Result() {
			switch result {
			case shortcut.Single:
				logger.Info("shortcut: single press detected")
				wailsApp.Event.Emit("shortcut:single", "hotkey")
			case shortcut.Double:
				logger.Info("shortcut: double press detected")
				window.Show().Focus()
				wailsApp.Event.Emit("shortcut:double", "hotkey")
			}
		}
	}()
```

- [ ] **Step 3: Verify it compiles**

Run: `go build -o bin/KeyLint .`
Expected: compiles cleanly with no errors.

- [ ] **Step 4: Commit**

```bash
git add main.go
git commit -m "feat(shortcut): wire double-press detector into main goroutine (#30)"
```

---

### Task 3: Update `WailsService` — replace `shortcutTriggered$` with `shortcutSingle$` / `shortcutDouble$`

**Files:**
- Modify: `frontend/src/app/core/wails.service.ts:39-44,52-61,242-246`

- [ ] **Step 1: Replace subjects and observables**

In `wails.service.ts`, replace lines 39-44:

```typescript
// Before:
  private readonly shortcutTriggered = new Subject<string>();
  ...
  readonly shortcutTriggered$: Observable<string> = this.shortcutTriggered.asObservable();

// After:
  private readonly shortcutSingle = new Subject<string>();
  private readonly shortcutDouble = new Subject<string>();
  ...
  /** Emits on single press of global shortcut (silent fix). */
  readonly shortcutSingle$: Observable<string> = this.shortcutSingle.asObservable();
  /** Emits on double press of global shortcut (open Pyramidize UI). */
  readonly shortcutDouble$: Observable<string> = this.shortcutDouble.asObservable();
```

- [ ] **Step 2: Update `listenToEvents()`**

Replace the `shortcut:triggered` listener (lines 54-56) with:

```typescript
      Events.On('shortcut:single', (ev) => {
        this.shortcutSingle.next(ev.data as string);
      }),
      Events.On('shortcut:double', (ev) => {
        this.shortcutDouble.next(ev.data as string);
      }),
```

- [ ] **Step 3: Update `ngOnDestroy()`**

Replace `this.shortcutTriggered.complete()` (line 244) with:

```typescript
    this.shortcutSingle.complete();
    this.shortcutDouble.complete();
```

- [ ] **Step 4: Commit**

```bash
git add frontend/src/app/core/wails.service.ts
git commit -m "feat(shortcut): split shortcutTriggered$ into shortcutSingle$ and shortcutDouble$ (#30)"
```

---

### Task 4: Update wails mock

**Files:**
- Modify: `frontend/src/testing/wails-mock.ts:34-42`

- [ ] **Step 1: Replace mock subjects**

In `wails-mock.ts`, replace lines 34-42:

```typescript
// Before:
  const shortcutTriggered$ = new Subject<string>();
  ...
    shortcutTriggered$: shortcutTriggered$.asObservable(),
    ...
    _shortcutTriggered$: shortcutTriggered$,

// After:
  const shortcutSingle$ = new Subject<string>();
  const shortcutDouble$ = new Subject<string>();
  ...
    shortcutSingle$: shortcutSingle$.asObservable(),
    shortcutDouble$: shortcutDouble$.asObservable(),
    ...
    _shortcutSingle$: shortcutSingle$,
    _shortcutDouble$: shortcutDouble$,
```

The full return block (lines 37-42) becomes:

```typescript
  return {
    shortcutSingle$: shortcutSingle$.asObservable(),
    shortcutDouble$: shortcutDouble$.asObservable(),
    settingsChanged$: settingsChanged$.asObservable(),
    // Expose subjects so tests can trigger events
    _shortcutSingle$: shortcutSingle$,
    _shortcutDouble$: shortcutDouble$,
    _settingsChanged$: settingsChanged$,
    ...
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/testing/wails-mock.ts
git commit -m "refactor(test): update wails mock for shortcutSingle$/shortcutDouble$ (#30)"
```

---

### Task 5: Update `FixComponent` — subscribe to `shortcutSingle$`

**Files:**
- Modify: `frontend/src/app/features/fix/fix.component.ts:107`
- Modify: `frontend/src/app/features/fix/fix.component.spec.ts:113-126`

- [ ] **Step 1: Update the test first**

In `fix.component.spec.ts`, replace lines 113-126:

```typescript
  it('shortcutSingle$ triggers fixClipboard', async () => {
    wailsMock.readClipboard.mockResolvedValue('shortcut clipboard');
    wailsMock._shortcutSingle$.next('hotkey');
    await new Promise(r => setTimeout(r, 0));
    expect(wailsMock.readClipboard).toHaveBeenCalled();
    expect(enhanceSpy).toHaveBeenCalledWith('shortcut clipboard');
  });

  it('ngOnDestroy unsubscribes from shortcut events', async () => {
    component.ngOnDestroy();
    wailsMock._shortcutSingle$.next('hotkey');
    await new Promise(r => setTimeout(r, 0));
    expect(wailsMock.readClipboard).not.toHaveBeenCalled();
  });
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd frontend && npx vitest run src/app/features/fix/fix.component.spec.ts`
Expected: FAIL — `_shortcutSingle$` does not exist on the mock (fixed in Task 4), or `shortcutSingle$` not on `WailsService` (fixed in Task 3). If Tasks 3 and 4 are done, the test fails because `fix.component.ts` still subscribes to the old `shortcutTriggered$`.

- [ ] **Step 3: Update the component**

In `fix.component.ts`, change line 107:

```typescript
// Before:
    this.sub = this.wails.shortcutTriggered$.subscribe(() => {

// After:
    this.sub = this.wails.shortcutSingle$.subscribe(() => {
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd frontend && npx vitest run src/app/features/fix/fix.component.spec.ts`
Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/app/features/fix/fix.component.ts frontend/src/app/features/fix/fix.component.spec.ts
git commit -m "feat(fix): subscribe to shortcutSingle$ for silent fix (#30)"
```

---

### Task 6: Update `TextEnhancementComponent` — subscribe to `shortcutDouble$`

**Files:**
- Modify: `frontend/src/app/features/text-enhancement/text-enhancement.component.ts:1096`
- Modify: `frontend/src/app/features/text-enhancement/text-enhancement.component.spec.ts:253-331`

- [ ] **Step 1: Update the tests first**

In `text-enhancement.component.spec.ts`, replace the shortcut test descriptions and subject references (lines 253-331). Every occurrence of `shortcutTriggered$` in test descriptions becomes `shortcutDouble$`, and every `_shortcutTriggered$` becomes `_shortcutDouble$`:

```typescript
  // ── 13. shortcutDouble$ with empty originalText sets originalText from clipboard ──

  it('shortcutDouble$ with empty originalText sets originalText from clipboard', async () => {
    component.originalTextView = '';
    wailsMock.readClipboard.mockResolvedValue('clipboard hotkey content');
    wailsMock.getSourceApp.mockResolvedValue('TestApp');

    wailsMock._shortcutDouble$.next('hotkey');
    await new Promise(r => setTimeout(r, 0));

    expect(wailsMock.readClipboard).toHaveBeenCalled();
    expect(component.originalTextView).toBe('clipboard hotkey content');
  });

  // ── 14. shortcutDouble$ with existing originalText shows confirm dialog ──

  it('shortcutDouble$ with existing originalText shows confirm dialog', async () => {
    component.originalTextView = 'existing content';
    wailsMock.readClipboard.mockResolvedValue('new clipboard content');
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false);

    wailsMock._shortcutDouble$.next('hotkey');
    await new Promise(r => setTimeout(r, 0));

    expect(confirmSpy).toHaveBeenCalled();
    // Since user cancelled, originalText should remain unchanged
    expect(component.originalTextView).toBe('existing content');
  });

  it('shortcutDouble$ with existing originalText and confirm=true replaces content', async () => {
    component.originalTextView = 'existing content';
    wailsMock.readClipboard.mockResolvedValue('new clipboard content');
    vi.spyOn(window, 'confirm').mockReturnValue(true);

    wailsMock._shortcutDouble$.next('hotkey');
    await new Promise(r => setTimeout(r, 0));

    expect(component.originalTextView).toBe('new clipboard content');
  });
```

And the `ngOnDestroy` test (around line 325):

```typescript
  it('ngOnDestroy unsubscribes from shortcut events', async () => {
    component.ngOnDestroy();
    const prevReadCount = (wailsMock.readClipboard as ReturnType<typeof vi.fn>).mock.calls.length;
    wailsMock._shortcutDouble$.next('hotkey');
    await new Promise(r => setTimeout(r, 0));
    expect((wailsMock.readClipboard as ReturnType<typeof vi.fn>).mock.calls.length).toBe(prevReadCount);
  });
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd frontend && npx vitest run src/app/features/text-enhancement/text-enhancement.component.spec.ts`
Expected: FAIL — component still subscribes to old `shortcutTriggered$`.

- [ ] **Step 3: Update the component**

In `text-enhancement.component.ts`, change line 1096:

```typescript
// Before:
    this.sub = this.wails.shortcutTriggered$.subscribe(async () => {

// After:
    this.sub = this.wails.shortcutDouble$.subscribe(async () => {
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd frontend && npx vitest run src/app/features/text-enhancement/text-enhancement.component.spec.ts`
Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/app/features/text-enhancement/text-enhancement.component.ts frontend/src/app/features/text-enhancement/text-enhancement.component.spec.ts
git commit -m "feat(enhance): subscribe to shortcutDouble$ for pyramidize UI (#30)"
```

---

### Task 7: Add navigation on double press to `ShellComponent`

**Files:**
- Modify: `frontend/src/app/layout/shell.component.ts:102,113-117,153-155`
- Modify: `frontend/src/app/layout/shell.component.spec.ts`

ShellComponent is always mounted, so it handles the "navigate to `/enhance` on double press" logic.

- [ ] **Step 1: Write the failing test**

Add to the end of `shell.component.spec.ts` (before the closing `});`):

```typescript
  it('navigates to /enhance on shortcutDouble$', async () => {
    const fixture = await createAndWait('dark');
    const router = TestBed.inject(Router);
    const navigateSpy = vi.spyOn(router, 'navigate').mockResolvedValue(true);

    wailsMock._shortcutDouble$.next('hotkey');
    await fixture.whenStable();

    expect(navigateSpy).toHaveBeenCalledWith(['/enhance']);
  });
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd frontend && npx vitest run src/app/layout/shell.component.spec.ts`
Expected: FAIL — `_shortcutDouble$` does not exist (after Task 4 it exists, but ShellComponent doesn't subscribe to it yet, so navigate is never called).

- [ ] **Step 3: Update the component**

In `shell.component.ts`:

1. Change the `sub` field to an array (line 102):

```typescript
// Before:
  private sub?: Subscription;

// After:
  private subs: Subscription[] = [];
```

2. Update `ngOnInit()` (lines 113-117):

```typescript
// Before:
  ngOnInit(): void {
    void this.applyTheme();
    void this.loadVersionInfo();
    this.sub = this.wails.settingsChanged$.subscribe(() => void this.applyTheme());
  }

// After:
  ngOnInit(): void {
    void this.applyTheme();
    void this.loadVersionInfo();
    this.subs.push(
      this.wails.settingsChanged$.subscribe(() => void this.applyTheme()),
      this.wails.shortcutDouble$.subscribe(() => {
        void this.router.navigate(['/enhance']);
      }),
    );
  }
```

3. Update `ngOnDestroy()` (lines 153-155):

```typescript
// Before:
  ngOnDestroy(): void {
    this.sub?.unsubscribe();
  }

// After:
  ngOnDestroy(): void {
    this.subs.forEach(s => s.unsubscribe());
  }
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd frontend && npx vitest run src/app/layout/shell.component.spec.ts`
Expected: all tests PASS (including the new navigation test).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/app/layout/shell.component.ts frontend/src/app/layout/shell.component.spec.ts
git commit -m "feat(shell): navigate to /enhance on shortcutDouble$ (#30)"
```

---

### Task 8: Full test suite verification

**Files:** None (verification only)

- [ ] **Step 1: Run all frontend tests**

Run: `cd frontend && npm test`
Expected: 0 failures. All tests pass — no leftover references to `shortcutTriggered$`.

- [ ] **Step 2: Run all Go tests**

Run: `go test ./internal/...`
Expected: all pass, including the new `TestDetector_*` tests.

- [ ] **Step 3: Verify full build**

Run: `go build -o bin/KeyLint .`
Expected: compiles cleanly.

- [ ] **Step 4: Search for stale references**

Run: `grep -r "shortcutTriggered" frontend/src/ --include="*.ts" -l`
Expected: no results. All references have been updated.

Run: `grep -r "shortcut:triggered" . --include="*.go" --include="*.ts" -l`
Expected: no results (except possibly the design spec).

- [ ] **Step 5: Commit any fixups if needed**

If stale references were found, fix them and commit.
