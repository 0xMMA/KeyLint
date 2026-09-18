package updater

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"keylint/internal/features/settings"
)

func newTestService(version string, srv *httptest.Server) *Service {
	s := NewService(version, nil)
	s.releasesAPIURL = srv.URL + "/releases"
	return s
}

func newTestServiceWithChannel(version string, channel string, srv *httptest.Server) *Service {
	// Create a settings service and set the channel.
	settingsSvc, err := settings.NewService()
	if err != nil {
		// In tests this should not fail; if it does, fall back to nil.
		s := NewService(version, nil)
		s.releasesAPIURL = srv.URL + "/releases"
		return s
	}
	cfg := settingsSvc.Get()
	cfg.UpdateChannel = channel
	_ = settingsSvc.Save(cfg)

	s := NewService(version, settingsSvc)
	s.releasesAPIURL = srv.URL + "/releases"
	return s
}

func serveGitHubReleases(t *testing.T, releases []githubRelease) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(releases); err != nil {
			t.Errorf("failed to encode releases: %v", err)
		}
	}))
}

func makeRelease(tag string, prerelease bool, draft bool, assetNames ...string) githubRelease {
	assets := make([]githubAsset, len(assetNames))
	for i, name := range assetNames {
		assets[i] = githubAsset{
			Name:               name,
			BrowserDownloadURL: "https://example.com/" + name,
		}
	}
	return githubRelease{
		TagName:    tag,
		Name:       tag,
		Body:       "Release " + tag,
		Draft:      draft,
		Prerelease: prerelease,
		Assets:     assets,
	}
}

func TestCheckForUpdate_UpdateAvailable(t *testing.T) {
	releases := []githubRelease{
		makeRelease("v3.7.0", false, false, "KeyLint-v3.7.0-linux-amd64"),
	}
	srv := serveGitHubReleases(t, releases)
	defer srv.Close()

	svc := newTestService("3.6.0", srv)
	info, err := svc.CheckForUpdate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !info.IsAvailable {
		t.Errorf("expected update to be available")
	}
	if info.LatestVersion != "3.7.0" {
		t.Errorf("LatestVersion = %q, want %q", info.LatestVersion, "3.7.0")
	}
	if info.CurrentVersion != "3.6.0" {
		t.Errorf("CurrentVersion = %q, want %q", info.CurrentVersion, "3.6.0")
	}
}

func TestCheckForUpdate_AlreadyUpToDate(t *testing.T) {
	releases := []githubRelease{
		makeRelease("v3.6.0", false, false, "KeyLint-v3.6.0-linux-amd64"),
	}
	srv := serveGitHubReleases(t, releases)
	defer srv.Close()

	svc := newTestService("3.6.0", srv)
	info, err := svc.CheckForUpdate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.IsAvailable {
		t.Errorf("expected no update when already up to date")
	}
}

func TestCheckForUpdate_NetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	svc := newTestService("3.6.0", srv)
	_, err := svc.CheckForUpdate()
	if err == nil {
		t.Error("expected error for non-200 response, got nil")
	}
}

func TestCheckForUpdate_DevVersionSkipped(t *testing.T) {
	releases := []githubRelease{
		makeRelease("v3.7.0", false, false),
	}
	srv := serveGitHubReleases(t, releases)
	defer srv.Close()

	svc := newTestService("dev", srv)
	info, err := svc.CheckForUpdate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.IsAvailable {
		t.Errorf("dev version should never report an update as available")
	}
}

func TestCheckForUpdate_DraftSkipped(t *testing.T) {
	releases := []githubRelease{
		makeRelease("v9.0.0", false, true, "KeyLint-v9.0.0-linux-amd64"),  // draft
		makeRelease("v3.6.0", false, false, "KeyLint-v3.6.0-linux-amd64"), // current
	}
	srv := serveGitHubReleases(t, releases)
	defer srv.Close()

	svc := newTestService("3.6.0", srv)
	info, err := svc.CheckForUpdate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.IsAvailable {
		t.Errorf("draft releases should be skipped")
	}
}

func TestCheckForUpdate_StableChannelSkipsPrerelease(t *testing.T) {
	releases := []githubRelease{
		makeRelease("v4.0.0-alpha", true, false, "KeyLint-v4.0.0-alpha-linux-amd64"),
		makeRelease("v3.6.0", false, false, "KeyLint-v3.6.0-linux-amd64"),
	}
	srv := serveGitHubReleases(t, releases)
	defer srv.Close()

	svc := newTestServiceWithChannel("3.6.0", "stable", srv)
	info, err := svc.CheckForUpdate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.IsAvailable {
		t.Errorf("stable channel should skip pre-release entries")
	}
}

func TestCheckForUpdate_PrereleaseChannelIncludesAll(t *testing.T) {
	releases := []githubRelease{
		makeRelease("v4.0.0-alpha", true, false, "KeyLint-v4.0.0-alpha-linux-amd64"),
		makeRelease("v3.6.0", false, false, "KeyLint-v3.6.0-linux-amd64"),
	}
	srv := serveGitHubReleases(t, releases)
	defer srv.Close()

	svc := newTestServiceWithChannel("3.6.0", "pre-release", srv)
	info, err := svc.CheckForUpdate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !info.IsAvailable {
		t.Errorf("pre-release channel should include all non-draft releases")
	}
	if info.LatestVersion != "4.0.0-alpha" {
		t.Errorf("LatestVersion = %q, want %q", info.LatestVersion, "4.0.0-alpha")
	}
}

func TestCheckForUpdate_AutoDetectFromVersion(t *testing.T) {
	releases := []githubRelease{
		makeRelease("v4.1.8-alpha", true, false, "KeyLint-v4.1.8-alpha-linux-amd64"),
		makeRelease("v4.1.7", false, false, "KeyLint-v4.1.7-linux-amd64"),
	}
	srv := serveGitHubReleases(t, releases)
	defer srv.Close()

	// Current version is pre-release → auto-detect should include pre-releases.
	svc := newTestService("4.1.7-alpha", srv)
	info, err := svc.CheckForUpdate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !info.IsAvailable {
		t.Errorf("auto-detect from pre-release version should include pre-release candidates")
	}
	if info.Channel != "pre-release" {
		t.Errorf("Channel = %q, want %q", info.Channel, "pre-release")
	}
}

func TestGetVersion(t *testing.T) {
	releases := []githubRelease{
		makeRelease("v3.6.0", false, false),
	}
	srv := serveGitHubReleases(t, releases)
	defer srv.Close()

	svc := newTestService("3.6.0", srv)
	if got := svc.GetVersion(); got != "3.6.0" {
		t.Errorf("GetVersion() = %q, want %q", got, "3.6.0")
	}
}

func TestParseVersion(t *testing.T) {
	cases := []struct {
		input string
		want  parsedVersion
	}{
		{"4.1.8", parsedVersion{4, 1, 8, 3, 0}},
		{"v4.1.8", parsedVersion{4, 1, 8, 3, 0}},
		{"4.1.8-alpha", parsedVersion{4, 1, 8, 0, 0}},
		{"4.1.8-beta", parsedVersion{4, 1, 8, 1, 0}},
		{"4.1.8-rc", parsedVersion{4, 1, 8, 2, 0}},
		{"4.1.8-rc2", parsedVersion{4, 1, 8, 2, 2}},
		{"4.1.8-alpha1", parsedVersion{4, 1, 8, 0, 1}},
		{"v4.1.8-beta3", parsedVersion{4, 1, 8, 1, 3}},
		{"1.0", parsedVersion{1, 0, 0, 3, 0}},
		{"1", parsedVersion{1, 0, 0, 3, 0}},
	}
	for _, tc := range cases {
		got := parseVersion(tc.input)
		if got != tc.want {
			t.Errorf("parseVersion(%q) = %+v, want %+v", tc.input, got, tc.want)
		}
	}
}

func TestIsNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		// Basic version comparisons.
		{"3.7.0", "3.6.0", true},
		{"3.6.1", "3.6.0", true},
		{"4.0.0", "3.9.9", true},
		{"3.6.0", "3.6.0", false},
		{"3.5.0", "3.6.0", false},
		{"v3.7.0", "3.6.0", true},
		{"3.7.0", "v3.6.0", true},

		// Pre-release comparisons.
		{"4.1.8-alpha", "4.1.7-alpha", true},  // patch bump
		{"4.1.8-beta", "4.1.8-alpha", true},   // suffix ordering
		{"4.1.8-rc", "4.1.8-beta", true},      // rc > beta
		{"4.1.8", "4.1.8-rc", true},           // stable > any pre-release
		{"4.1.8-alpha", "4.1.8", false},        // pre-release < stable at same version
		{"4.1.8-rc2", "4.1.8-rc1", true},      // numeric suffix
		{"4.1.8-alpha", "4.1.7", true},         // higher patch even with suffix
		{"v4.1.8-alpha", "v4.1.7-alpha", true}, // v-prefix preserved
		{"4.1.8-rc1", "4.1.8-rc2", false},     // lower numeric suffix
		{"4.1.8-alpha", "4.1.8-alpha", false},  // same pre-release
	}
	for _, tc := range cases {
		got := isNewer(tc.latest, tc.current)
		if got != tc.want {
			t.Errorf("isNewer(%q, %q) = %v, want %v", tc.latest, tc.current, got, tc.want)
		}
	}
}
