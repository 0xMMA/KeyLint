# Platform-Aware Update Installation

**Date:** 2026-04-05
**Issue:** [#18](https://github.com/0xMMA/KeyLint/issues/18) — App update via "Download & Install" button fails on Windows
**Status:** Approved

## Problem

`DownloadAndInstall()` uses `minio/selfupdate.Apply()` to replace the running executable in-place. This fails on Windows for two reasons:

1. **Wrong asset type**: The Windows release asset (`*-windows-amd64-setup`) is an NSIS installer, not a raw binary. Piping it through `selfupdate.Apply()` would produce a corrupt executable even if it succeeded.
2. **No elevation**: The app installs to `C:\Program Files\KeyLint\KeyLint\`. Writing to Program Files requires admin privileges. `selfupdate.Apply()` creates `.KeyLint.exe.new` in the same directory without UAC elevation, triggering "Access is denied."

## Solution

Split `DownloadAndInstall()` into platform-specific install strategies. Both platforms share the download-to-temp phase; the install phase diverges.

### Flow

```
DownloadAndInstall()
  1. CheckForUpdate()                    // unchanged
  2. HTTP GET release asset → temp file  // shared
  3. Platform-specific install:
     ├── [windows] os.StartProcess(temp.exe) → return ErrRestartRequired
     └── [linux]   selfupdate.Apply(tempFile) → return nil
```

### Windows Strategy

1. Download the NSIS installer to `os.CreateTemp("", "KeyLint-update-*.exe")`.
2. Launch the installer via `os.StartProcess` (detached — survives parent exit). The NSIS installer's own manifest requests UAC elevation.
3. Return sentinel `ErrRestartRequired`. The caller (frontend bridge or main.go) uses this to quit the app gracefully so the installer can replace the locked executable.

### Linux Strategy

1. Download the raw binary to `os.CreateTemp("", "KeyLint-update-*")`.
2. Apply via `selfupdate.Apply(file, selfupdate.Options{})`.
3. If permission error, return a descriptive message: `"update downloaded to <path> — install manually or run with appropriate permissions"`.

### Sentinel Error

```go
var ErrRestartRequired = errors.New("update requires application restart")
```

Callers check `errors.Is(err, ErrRestartRequired)` to distinguish "quit now, installer is running" from actual failures.

## File Changes

| File | Change |
|------|--------|
| `internal/features/updater/service.go` | Refactor `DownloadAndInstall()` to download to temp, delegate to `applyPlatformUpdate()` |
| `internal/features/updater/install_windows.go` | `applyPlatformUpdate()`: launch NSIS installer, return `ErrRestartRequired` |
| `internal/features/updater/install_linux.go` | `applyPlatformUpdate()`: `selfupdate.Apply()` from temp file |
| `internal/features/updater/install_test.go` | Regression test (prove bug) + new behavior tests |
| `frontend/.../settings.component.ts` | Handle restart-required response (show "closing" message, trigger app quit) |
| `frontend/.../settings.component.spec.ts` | Test for restart-required UI path |
| `frontend/.../wails.service.ts` | No change — `downloadAndInstall()` RPC signature unchanged |

## Testing Plan

### Regression Tests (prove the bug)

1. **Test selfupdate fails on read-only directory**: Create a temp executable in a read-only dir, attempt `selfupdate.Apply()` targeting it, assert permission error. Documents WHY the old approach was replaced.
2. **Test Windows asset is an installer, not a raw binary**: Assert `matchPlatformAsset()` returns a `*-setup*` URL on Windows — proving the downloaded file is an NSIS installer that cannot be used as a binary replacement.

### Fix Tests (prove it works)

3. **Test download-to-temp**: httptest server serves fake bytes → assert temp file exists with correct content and size.
4. **Test Windows install path**: Provide a mock/stub process launcher → assert it receives the temp `.exe` path, assert `ErrRestartRequired` is returned.
5. **Test Linux install path**: `selfupdate.Apply()` against a writable temp target → assert success, assert no `ErrRestartRequired`.
6. **Test error paths**: Download returns HTTP 500 → error. Download returns empty body → error. Temp file creation fails → error.

### Frontend Tests

7. **Test restart-required UI**: Mock `downloadAndInstall()` to return restart-required → assert "closing" message shown.
8. **Test error UI**: Mock `downloadAndInstall()` to throw → assert error message shown (existing test, verify still passes).

### Manual Verification

- Build Windows installer, install to Program Files, trigger update → NSIS wizard launches, app closes, update completes.
- This step cannot be automated in CI (requires Windows GUI + real GitHub release).

## Frontend UX Change

Current behavior after successful install:
> "Update installed! Restart the app to use the new version."

New behavior (Windows):
> "Installing update... the app will close shortly."

Then the app quits after a brief delay (1-2s) to let the user read the message. The NSIS installer takes over.

Linux behavior unchanged — same "Restart" message as before.

## Dependencies

- No new dependencies. `minio/selfupdate` stays for Linux. `os.StartProcess` is stdlib.
- NSIS installer is already built by CI (`release.yml`).

## Out of Scope

- Delta updates, silent background updates (future: Velopack when Go SDK matures).
- Automatic restart after update on Linux.
- macOS support.
