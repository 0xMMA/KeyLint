# Platform-Aware Updater Fix — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix GH #18 — the "Download & Install" button fails on Windows because `selfupdate.Apply()` cannot write to `C:\Program Files\` and the downloaded asset is an NSIS installer, not a raw binary.

**Architecture:** Split `DownloadAndInstall()` into a shared download-to-temp phase and platform-specific install phase. Windows launches the NSIS installer and quits the app. Linux applies the raw binary via `selfupdate.Apply()`. A `quitFunc` callback lets the service trigger app shutdown without importing Wails directly.

**Tech Stack:** Go 1.26, minio/selfupdate v0.6.0, Wails v3, Angular v21, Vitest

**Spec:** `docs/superpowers/specs/2026-04-05-updater-platform-aware-install-design.md`

---

## File Structure

| File | Purpose |
|------|---------|
| `internal/features/updater/model.go` | Add `InstallResult` type |
| `internal/features/updater/service.go` | Refactor `DownloadAndInstall()` — download to temp, delegate to `applyPlatformUpdate()`, add `quitFunc` |
| `internal/features/updater/install_windows.go` | `applyPlatformUpdate()`: launch NSIS installer via `exec.Command` |
| `internal/features/updater/install_linux.go` | `applyPlatformUpdate()`: `selfupdate.Apply()` from temp file |
| `internal/features/updater/install_test.go` | Regression test + all new behavior tests |
| `main.go` | Set `quitFunc` on updater service after Wails app is created |
| `frontend/src/app/core/wails.service.ts` | Update `downloadAndInstall()` return type to `Promise<InstallResult>` |
| `frontend/src/testing/wails-mock.ts` | Update mock to return `InstallResult` |
| `frontend/src/app/features/settings/settings.component.ts` | Handle `restart_required` in UI |
| `frontend/src/app/features/settings/settings.component.spec.ts` | Test restart-required UI path |

---

### Task 1: Regression Test — Prove the Bug

**Files:**
- Create: `internal/features/updater/install_test.go`

This test documents WHY the old `selfupdate.Apply()` approach was replaced: it fails with a permission error when the executable lives in a read-only directory (like `C:\Program Files\`).

- [ ] **Step 1: Write the regression test**

Create `internal/features/updater/install_test.go`:

```go
package updater

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minio/selfupdate"
)

// TestSelfupdateFailsOnReadOnlyDirectory documents the root cause of GH #18:
// selfupdate.Apply() cannot write to directories that require elevated permissions
// (e.g. C:\Program Files\ on Windows). This test ensures we never regress to the
// old in-place replacement approach for directories that aren't user-writable.
func TestSelfupdateFailsOnReadOnlyDirectory(t *testing.T) {
	dir := t.TempDir()
	fakeBin := filepath.Join(dir, "KeyLint.exe")
	if err := os.WriteFile(fakeBin, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Make the directory read-only so selfupdate cannot create .KeyLint.exe.new
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	err := selfupdate.Apply(strings.NewReader("new binary"), selfupdate.Options{
		TargetPath: fakeBin,
	})

	if err == nil {
		t.Fatal("expected permission error when applying update to read-only directory, got nil — " +
			"this means the old selfupdate.Apply() approach would silently corrupt the install")
	}

	// The error message varies by OS: "permission denied" (Linux) or "Access is denied" (Windows).
	errMsg := strings.ToLower(err.Error())
	if !strings.Contains(errMsg, "permission denied") && !strings.Contains(errMsg, "access is denied") {
		t.Errorf("expected permission-related error, got: %v", err)
	}
}
```

- [ ] **Step 2: Run the test to confirm it passes (proving the bug exists)**

Run: `go test ./internal/features/updater/ -run TestSelfupdateFailsOnReadOnlyDirectory -v`

Expected: PASS — the test confirms that `selfupdate.Apply()` fails on a read-only directory, which is exactly the bug from GH #18.

- [ ] **Step 3: Commit**

```bash
git add internal/features/updater/install_test.go
git commit -m "test(updater): regression test proving GH #18 selfupdate permission failure"
```

---

### Task 2: Backend — Add InstallResult Type, applyFunc Hook, and Platform Files

**Files:**
- Modify: `internal/features/updater/model.go` — add `InstallResult`
- Modify: `internal/features/updater/service.go` — add `quitFunc`, `SetQuitFunc()`, `applyFunc` test hook, refactor `DownloadAndInstall()`
- Create: `internal/features/updater/install_linux.go` — Linux `applyPlatformUpdate()`
- Create: `internal/features/updater/install_windows.go` — Windows `applyPlatformUpdate()`
- Modify: `internal/features/updater/install_test.go` — tests for new download + install flow

**Design note:** `selfupdate.Apply()` with default options replaces `os.Executable()` — the test runner itself. To avoid corrupting the test binary, the Service has an `applyFunc` field that overrides the platform function in tests. This is only used in tests; production code always uses the real `applyPlatformUpdate`.

- [ ] **Step 1: Write tests for the new download + install flow**

Append to `internal/features/updater/install_test.go`:

```go
import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	// ... existing imports plus these
)

// serveAsset creates an httptest server that serves the given payload for any request.
func serveAsset(payload []byte) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(payload)
	}))
}

// makeReleaseWithURL creates a release whose asset URL points to a custom server.
func makeReleaseWithURL(tag string, assetName string, assetURL string) githubRelease {
	return githubRelease{
		TagName:    tag,
		Name:       tag,
		Body:       "Release " + tag,
		Draft:      false,
		Prerelease: false,
		Assets: []githubAsset{{
			Name:               assetName,
			BrowserDownloadURL: assetURL,
		}},
	}
}

func TestDownloadAndInstall_DownloadsToTempAndCallsApply(t *testing.T) {
	payload := []byte("fake-installer-payload-12345")
	assetSrv := serveAsset(payload)
	defer assetSrv.Close()

	releases := []githubRelease{
		makeReleaseWithURL("v9.0.0", "KeyLint-v9.0.0-linux-amd64", assetSrv.URL+"/dl"),
	}
	releaseSrv := serveGitHubReleases(t, releases)
	defer releaseSrv.Close()

	svc := newTestService("1.0.0", releaseSrv)

	// Override the platform apply function to capture and verify the temp file
	// instead of calling selfupdate.Apply (which would corrupt the test binary).
	var capturedPath string
	var capturedContent []byte
	svc.applyFunc = func(_ *Service, tmpPath string) (InstallResult, error) {
		capturedPath = tmpPath
		data, err := os.ReadFile(tmpPath)
		if err != nil {
			t.Fatalf("applyFunc: failed to read temp file: %v", err)
		}
		capturedContent = data
		return InstallResult{RestartRequired: false}, nil
	}

	result, err := svc.DownloadAndInstall()
	if err != nil {
		t.Fatalf("DownloadAndInstall() error: %v", err)
	}
	if result.RestartRequired {
		t.Error("expected RestartRequired=false from test applyFunc")
	}
	if capturedPath == "" {
		t.Fatal("applyFunc was never called — download phase may have failed")
	}
	if !bytes.Equal(capturedContent, payload) {
		t.Errorf("temp file content = %q, want %q", capturedContent, payload)
	}
}

func TestDownloadAndInstall_ApplyFuncRestartRequired(t *testing.T) {
	assetSrv := serveAsset([]byte("installer"))
	defer assetSrv.Close()

	releases := []githubRelease{
		makeReleaseWithURL("v9.0.0", "KeyLint-v9.0.0-linux-amd64", assetSrv.URL+"/dl"),
	}
	releaseSrv := serveGitHubReleases(t, releases)
	defer releaseSrv.Close()

	svc := newTestService("1.0.0", releaseSrv)
	svc.applyFunc = func(_ *Service, _ string) (InstallResult, error) {
		return InstallResult{RestartRequired: true}, nil
	}

	result, err := svc.DownloadAndInstall()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.RestartRequired {
		t.Error("expected RestartRequired=true (simulating Windows path)")
	}
}

func TestDownloadAndInstall_NoUpdateAvailable(t *testing.T) {
	releases := []githubRelease{
		makeRelease("v1.0.0", false, false, "KeyLint-v1.0.0-linux-amd64"),
	}
	srv := serveGitHubReleases(t, releases)
	defer srv.Close()

	svc := newTestService("1.0.0", srv)
	_, err := svc.DownloadAndInstall()
	if err == nil {
		t.Fatal("expected error for no-update-available")
	}
	if !strings.Contains(err.Error(), "no update available") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDownloadAndInstall_DownloadHTTPError(t *testing.T) {
	errorSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer errorSrv.Close()

	releases := []githubRelease{
		makeReleaseWithURL("v9.0.0", "KeyLint-v9.0.0-linux-amd64", errorSrv.URL+"/missing"),
	}
	releaseSrv := serveGitHubReleases(t, releases)
	defer releaseSrv.Close()

	svc := newTestService("1.0.0", releaseSrv)
	_, err := svc.DownloadAndInstall()
	if err == nil {
		t.Fatal("expected error for HTTP 404 download")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected status code in error, got: %v", err)
	}
}

func TestDownloadAndInstall_EmptyBody(t *testing.T) {
	emptySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 200 OK but empty body
	}))
	defer emptySrv.Close()

	releases := []githubRelease{
		makeReleaseWithURL("v9.0.0", "KeyLint-v9.0.0-linux-amd64", emptySrv.URL+"/empty"),
	}
	releaseSrv := serveGitHubReleases(t, releases)
	defer releaseSrv.Close()

	svc := newTestService("1.0.0", releaseSrv)
	_, err := svc.DownloadAndInstall()
	if err == nil {
		t.Fatal("expected error for empty download body")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected 'empty' in error, got: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail (functions don't exist yet or signatures mismatch)**

Run: `go test ./internal/features/updater/ -run "TestDownloadAndInstall" -v`

Expected: FAIL — `DownloadAndInstall()` still returns `error` (not `InstallResult, error`), and `applyFunc` field doesn't exist.

- [ ] **Step 3: Add InstallResult to model.go**

Add to `internal/features/updater/model.go`, after the `UpdateInfo` struct:

```go
// InstallResult is returned by DownloadAndInstall to indicate the outcome.
// On Windows, RestartRequired is true because the NSIS installer needs the app to exit.
type InstallResult struct {
	RestartRequired bool `json:"restart_required"`
}
```

- [ ] **Step 4: Add quitFunc, applyFunc, and refactor DownloadAndInstall in service.go**

In `internal/features/updater/service.go`, update the imports, Service struct, add `SetQuitFunc`, and replace `DownloadAndInstall`:

```go
import (
	"errors"
	// ... existing imports plus:
	"io"
	"os"
)

type Service struct {
	currentVersion string
	releasesAPIURL string
	client         *http.Client
	settingsSvc    *settings.Service
	quitFunc       func()                                                  // called after launching installer on Windows; set via SetQuitFunc
	applyFunc      func(svc *Service, tmpPath string) (InstallResult, error) // override for testing; nil uses applyPlatformUpdate
}

// SetQuitFunc sets the callback invoked after launching the installer on Windows.
// The callback should wait briefly (for the frontend to display a message) then quit the app.
func (s *Service) SetQuitFunc(fn func()) {
	s.quitFunc = fn
}
```

Replace the `DownloadAndInstall` method entirely:

```go
// DownloadAndInstall fetches the release asset for the current platform, saves it
// to a temp file, and delegates to the platform-specific installer.
// On Windows this launches the NSIS setup and returns InstallResult{RestartRequired: true}.
// On Linux this applies the binary in-place via selfupdate.
func (s *Service) DownloadAndInstall() (InstallResult, error) {
	updateInfo, err := s.CheckForUpdate()
	if err != nil {
		return InstallResult{}, fmt.Errorf("checking for update: %w", err)
	}
	if !updateInfo.IsAvailable {
		return InstallResult{}, fmt.Errorf("no update available")
	}
	if updateInfo.ReleaseURL == "" {
		return InstallResult{}, fmt.Errorf("no download URL for current platform")
	}

	resp, err := s.client.Get(updateInfo.ReleaseURL)
	if err != nil {
		return InstallResult{}, fmt.Errorf("downloading update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return InstallResult{}, fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	// Write to a temp file so platform-specific code can work with a path on disk.
	tmpFile, err := os.CreateTemp("", "KeyLint-update-*.exe")
	if err != nil {
		return InstallResult{}, fmt.Errorf("creating temp file: %w", err)
	}

	n, err := io.Copy(tmpFile, resp.Body)
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return InstallResult{}, fmt.Errorf("writing update to temp file: %w", err)
	}
	tmpFile.Close()

	if n == 0 {
		os.Remove(tmpFile.Name())
		return InstallResult{}, errors.New("downloaded update is empty")
	}

	applyFn := applyPlatformUpdate
	if s.applyFunc != nil {
		applyFn = s.applyFunc
	}
	return applyFn(s, tmpFile.Name())
}
```

Remove the old unused imports (`"github.com/minio/selfupdate"`) from `service.go` — selfupdate is now only used in `install_linux.go`.

- [ ] **Step 5: Create the Linux install implementation**

Create `internal/features/updater/install_linux.go`:

```go
//go:build !windows

package updater

import (
	"fmt"
	"os"

	"github.com/minio/selfupdate"
)

// applyPlatformUpdate applies the downloaded binary in-place using selfupdate.
// On Linux the release asset is a raw executable, so direct replacement works
// as long as the target directory is writable.
func applyPlatformUpdate(svc *Service, tmpPath string) (InstallResult, error) {
	f, err := os.Open(tmpPath)
	if err != nil {
		return InstallResult{}, fmt.Errorf("opening downloaded update: %w", err)
	}
	defer f.Close()
	defer os.Remove(tmpPath)

	if err := selfupdate.Apply(f, selfupdate.Options{}); err != nil {
		return InstallResult{}, fmt.Errorf("applying update: %w — if this is a permission error, "+
			"the downloaded file is at %s — install manually or run with appropriate permissions",
			err, tmpPath)
	}

	return InstallResult{RestartRequired: false}, nil
}
```

- [ ] **Step 6: Create the Windows install implementation**

Create `internal/features/updater/install_windows.go`:

```go
//go:build windows

package updater

import (
	"fmt"
	"os"
	"os/exec"
)

// applyPlatformUpdate launches the downloaded NSIS installer and signals the app to quit.
// The NSIS installer's own manifest requests UAC elevation, so no special privileges
// are needed here — Windows will show the UAC prompt to the user.
func applyPlatformUpdate(svc *Service, tmpPath string) (InstallResult, error) {
	cmd := exec.Command(tmpPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		os.Remove(tmpPath)
		return InstallResult{}, fmt.Errorf("launching installer: %w", err)
	}

	// Schedule app quit so the installer can replace the locked executable.
	// The quitFunc is set by main.go and includes a short delay for the
	// frontend to display the "closing" message before the app exits.
	if svc.quitFunc != nil {
		go svc.quitFunc()
	}

	return InstallResult{RestartRequired: true}, nil
}
```

- [ ] **Step 7: Run all updater tests**

Run: `go test ./internal/features/updater/ -v`

Expected: ALL PASS — regression test still passes (proving the old bug), new tests pass with the refactored code.

- [ ] **Step 8: Commit**

```bash
git add internal/features/updater/model.go internal/features/updater/service.go \
       internal/features/updater/install_linux.go internal/features/updater/install_windows.go \
       internal/features/updater/install_test.go
git commit -m "fix(updater): platform-aware install — launch NSIS on Windows, selfupdate on Linux

Fixes #18. The old approach used selfupdate.Apply() which fails on Windows
because (a) the release asset is an NSIS installer, not a raw binary, and
(b) Program Files requires UAC elevation for writes."
```

---

### Task 3: Wire quitFunc in main.go

**Files:**
- Modify: `main.go:88-89` — store updater service in a variable, call `SetQuitFunc`

- [ ] **Step 1: Update main.go to set quitFunc on the updater service**

In `main.go`, replace line 88–89:

```go
// OLD:
wailsApp.RegisterService(application.NewService(updater.NewService(AppVersion, services.Settings)))

// NEW:
updaterSvc := updater.NewService(AppVersion, services.Settings)
updaterSvc.SetQuitFunc(func() {
    // Brief delay so the frontend can display the "closing" message.
    time.Sleep(2 * time.Second)
    wailsApp.Quit()
})
wailsApp.RegisterService(application.NewService(updaterSvc))
```

Add `"time"` to the imports in `main.go`.

- [ ] **Step 2: Build to verify compilation**

Run: `go build -o bin/KeyLint .`

Expected: Compiles cleanly.

- [ ] **Step 3: Regenerate Wails bindings (return type changed)**

Run: `wails3 generate bindings`

Note: This updates the auto-generated TypeScript bindings in `frontend/bindings/` to reflect the new `(InstallResult, error)` return type.

- [ ] **Step 4: Commit**

```bash
git add main.go frontend/bindings/
git commit -m "fix(updater): wire quitFunc in main.go, regenerate bindings"
```

---

### Task 4: Frontend — Handle InstallResult

**Files:**
- Modify: `frontend/src/app/core/wails.service.ts:163-165` — update return type
- Modify: `frontend/src/testing/wails-mock.ts:59` — update mock
- Modify: `frontend/src/app/features/settings/settings.component.ts:524-537,262-268` — handle `restart_required`

- [ ] **Step 1: Write the frontend test for restart-required**

Append to the `describe('About tab', ...)` block in `frontend/src/app/features/settings/settings.component.spec.ts`, after the existing `installUpdate()` test:

```typescript
    it('installUpdate() shows restart message when restart_required is true', async () => {
      wailsMock.downloadAndInstall.mockResolvedValue({ restart_required: true });
      component.updateInfo = { ...defaultUpdateInfo, is_available: true, latest_version: '3.7.0' };
      await component.installUpdate();
      expect(component.updateSuccess).toBe(true);
      expect(component.updateRestartRequired).toBe(true);
    });

    it('installUpdate() shows standard success when restart_required is false', async () => {
      wailsMock.downloadAndInstall.mockResolvedValue({ restart_required: false });
      component.updateInfo = { ...defaultUpdateInfo, is_available: true, latest_version: '3.7.0' };
      await component.installUpdate();
      expect(component.updateSuccess).toBe(true);
      expect(component.updateRestartRequired).toBe(false);
    });
```

- [ ] **Step 2: Run tests to confirm they fail**

Run: `cd frontend && npm test`

Expected: FAIL — `updateRestartRequired` property doesn't exist yet.

- [ ] **Step 3: Update wails.service.ts**

In `frontend/src/app/core/wails.service.ts`, change the `downloadAndInstall` method. First check the generated bindings to see the exact return type name, then update:

```typescript
downloadAndInstall(): Promise<InstallResult> {
    return UpdaterService.DownloadAndInstall();
}
```

Add the `InstallResult` type to the imports from the bindings (or define it in the `wails.service.ts` types section if that's where types live). The type shape is:

```typescript
export interface InstallResult {
  restart_required: boolean;
}
```

- [ ] **Step 4: Update wails-mock.ts**

In `frontend/src/testing/wails-mock.ts`, change line 59:

```typescript
// OLD:
downloadAndInstall: vi.fn().mockResolvedValue(undefined),

// NEW:
downloadAndInstall: vi.fn().mockResolvedValue({ restart_required: false }),
```

- [ ] **Step 5: Update settings.component.ts — add state and handle result**

In `frontend/src/app/features/settings/settings.component.ts`, add a new property after `updateSuccess`:

```typescript
updateRestartRequired = false;
```

Update the `installUpdate()` method:

```typescript
async installUpdate(): Promise<void> {
    this.updateInstalling = true;
    this.updateError = '';
    this.updateSuccess = false;
    this.updateRestartRequired = false;
    try {
      const result = await this.wails.downloadAndInstall();
      this.updateSuccess = true;
      this.updateRestartRequired = result.restart_required;
      this.updateInfo = null;
    } catch (e) {
      this.updateError = `Install failed: ${e instanceof Error ? e.message : String(e)}`;
    } finally {
      this.updateInstalling = false;
      this.cdr.detectChanges();
    }
  }
```

- [ ] **Step 6: Update the template success message**

In `frontend/src/app/features/settings/settings.component.ts`, replace the `updateSuccess` message block (lines 262-268):

```html
                @if (updateSuccess) {
                  <p-message
                    data-testid="update-success-msg"
                    severity="success"
                    [text]="updateRestartRequired
                      ? 'Update is installing — the app will close shortly.'
                      : 'Update installed! Restart the app to use the new version.'"
                    styleClass="mt-3"
                  />
                }
```

- [ ] **Step 7: Run frontend tests**

Run: `cd frontend && npm test`

Expected: ALL PASS — new tests pass, existing `installUpdate()` test passes because the mock now returns `{ restart_required: false }`.

- [ ] **Step 8: Commit**

```bash
git add frontend/src/app/core/wails.service.ts \
       frontend/src/testing/wails-mock.ts \
       frontend/src/app/features/settings/settings.component.ts \
       frontend/src/app/features/settings/settings.component.spec.ts
git commit -m "feat(updater): frontend handles restart_required from updater"
```

---

### Task 5: Final Verification

- [ ] **Step 1: Run all Go tests**

Run: `go test ./internal/... -v`

Expected: ALL PASS.

- [ ] **Step 2: Run all frontend tests**

Run: `cd frontend && npm test`

Expected: ALL PASS, 0 failures.

- [ ] **Step 3: Build the project**

Run: `cd frontend && npm run build && cd .. && go build -o bin/KeyLint .`

Expected: Compiles cleanly.

- [ ] **Step 4: Review the regression test one more time**

Run: `go test ./internal/features/updater/ -run TestSelfupdateFailsOnReadOnlyDirectory -v`

Expected: PASS — the regression test still documents that the old approach would fail, proving the bug existed and our new code path avoids it.

- [ ] **Step 5: Commit any remaining changes**

If the bindings regeneration or build produced changes not yet committed:

```bash
git add -A  # review what's staged first
git commit -m "chore(updater): final verification — all tests pass, build clean"
```
