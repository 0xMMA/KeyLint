package updater

// The dev channel publishes prereleases tagged v0.0.0-pr.<N> and v0.0.0-main
// to the same GitHub Releases list every client reads. This file proves that
// no client offers one as an update.
//
// It deliberately uses only what the updater package had at v4.4.3-beta, the
// last release in the field: NewService, the releasesAPIURL field,
// githubRelease and githubAsset, CheckForUpdate, and settings.NewService.
// So the same file can be dropped into a checkout of that tag and run there:
//
//	git worktree add /tmp/kl-v443 v4.4.3-beta
//	cp internal/features/updater/devbuild_safety_test.go /tmp/kl-v443/internal/features/updater/
//	(cd /tmp/kl-v443 && go test ./internal/features/updater/ -run TestDevReleases)
//
// Keep it that way: a helper from a newer version would make it uncompilable
// at the tag, and the proof about old clients would quietly stop being one.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"

	"keylint/internal/features/settings"
)

// devReleasesAsPublished mirrors what CI publishes: prereleases, with the same
// asset names a real release has, so an old client's asset matcher finds them.
func devReleasesAsPublished(prerelease bool) []githubRelease {
	var out []githubRelease
	for _, tag := range []string{"v0.0.0-main", "v0.0.0-pr.1", "v0.0.0-pr.106", "v0.0.0-pr.999999"} {
		out = append(out, githubRelease{
			TagName:    tag,
			Name:       tag,
			Body:       "Commit: 0123456789abcdef0123456789abcdef01234567",
			Prerelease: prerelease,
			Assets: []githubAsset{
				{Name: "KeyLint-" + tag + "-windows-amd64-setup.exe", BrowserDownloadURL: "https://example.com/" + tag + "-setup.exe"},
				{Name: "KeyLint-" + tag + "-linux-amd64", BrowserDownloadURL: "https://example.com/" + tag + "-linux"},
			},
		})
	}
	return out
}

func safetyRelease(tag string, prerelease bool) githubRelease {
	return githubRelease{
		TagName:    tag,
		Name:       tag,
		Prerelease: prerelease,
		Assets: []githubAsset{
			{Name: "KeyLint-" + tag + "-windows-amd64-setup.exe", BrowserDownloadURL: "https://example.com/" + tag + "-setup.exe"},
			{Name: "KeyLint-" + tag + "-linux-amd64", BrowserDownloadURL: "https://example.com/" + tag + "-linux"},
		},
	}
}

// safetyService builds a service with an explicit update channel, keeping the
// settings file in a temp dir. settings.NewService is the constructor both
// this version and v4.4.3-beta have.
func safetyService(t *testing.T, version, channel string, releases []githubRelease) *Service {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(releases)
	}))
	t.Cleanup(srv.Close)

	var settingsSvc *settings.Service
	if channel != "" {
		envKey := "XDG_CONFIG_HOME"
		if runtime.GOOS == "windows" {
			envKey = "APPDATA"
		}
		original, had := os.LookupEnv(envKey)
		os.Setenv(envKey, t.TempDir())
		t.Cleanup(func() {
			if had {
				os.Setenv(envKey, original)
			} else {
				os.Unsetenv(envKey)
			}
		})
		var err error
		settingsSvc, err = settings.NewService()
		if err != nil {
			t.Fatalf("settings.NewService: %v", err)
		}
		cfg := settingsSvc.Get()
		cfg.UpdateChannel = channel
		if err := settingsSvc.Save(cfg); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}
	s := NewService(version, settingsSvc)
	s.releasesAPIURL = srv.URL + "/releases"
	return s
}

// Versions a client in the field can be running: releases, their unprefixed
// form, `git describe` output from CI artifacts, and versions that do not
// exist yet.
var fieldVersions = []string{
	"v4.4.3-beta", "4.4.3-beta", "v4.2.1-alpha", "v3.5.0", "3.5.0",
	"v4.4.3-beta-12-gabc1234", "v4.4.3-beta-12-gabc1234-dirty",
	"v5.0.0", "v4.5.0-rc2", "0.1.0", "v0.0.1",
}

func TestDevReleases_NeverOfferedAsUpdate(t *testing.T) {
	lists := map[string][]githubRelease{
		"only dev builds (prerelease)":     devReleasesAsPublished(true),
		"only dev builds (not prerelease)": devReleasesAsPublished(false),
		"dev builds above older releases": append(devReleasesAsPublished(true),
			safetyRelease("v4.4.3-beta", true), safetyRelease("v3.5.0", false)),
	}
	for name, releases := range lists {
		for _, version := range fieldVersions {
			for _, channel := range []string{"", "stable", "pre-release"} {
				svc := safetyService(t, version, channel, releases)
				info, err := svc.CheckForUpdate()
				if err != nil {
					t.Fatalf("%s / %s / channel %q: %v", name, version, channel, err)
				}
				if info.IsAvailable && strings.HasPrefix(info.LatestVersion, "0.0.0") {
					t.Errorf("%s / %s / channel %q: offered dev build %q as an update", name, version, channel, info.LatestVersion)
				}
				if info.IsAvailable && strings.Contains(info.ReleaseURL, "v0.0.0-") {
					t.Errorf("%s / %s / channel %q: would download dev build %q", name, version, channel, info.ReleaseURL)
				}
			}
		}
	}
}

// A real newer release next to dev builds is still found and offered: the dev
// builds must not hide an update, only never be one.
func TestDevReleases_DoNotHideARealUpdate(t *testing.T) {
	releases := append(devReleasesAsPublished(true), safetyRelease("v4.5.0-beta", true), safetyRelease("v4.4.3-beta", true))
	for _, channel := range []string{"", "pre-release"} {
		svc := safetyService(t, "v4.4.3-beta", channel, releases)
		info, err := svc.CheckForUpdate()
		if err != nil {
			t.Fatalf("channel %q: %v", channel, err)
		}
		if !info.IsAvailable || info.LatestVersion != "4.5.0-beta" {
			t.Errorf("channel %q: got available=%v latest=%q, want 4.5.0-beta", channel, info.IsAvailable, info.LatestVersion)
		}
	}
}
