package updater

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
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

func TestDownloadAndInstall_ApplyFuncError(t *testing.T) {
	assetSrv := serveAsset([]byte("binary"))
	defer assetSrv.Close()

	releases := []githubRelease{
		makeReleaseWithURL("v9.0.0", "KeyLint-v9.0.0-linux-amd64", assetSrv.URL+"/dl"),
	}
	releaseSrv := serveGitHubReleases(t, releases)
	defer releaseSrv.Close()

	svc := newTestService("1.0.0", releaseSrv)
	svc.applyFunc = func(_ *Service, _ string) (InstallResult, error) {
		return InstallResult{}, fmt.Errorf("simulated install failure")
	}

	_, err := svc.DownloadAndInstall()
	if err == nil {
		t.Fatal("expected error from applyFunc")
	}
	if !strings.Contains(err.Error(), "simulated install failure") {
		t.Errorf("expected applyFunc error to propagate, got: %v", err)
	}
}
