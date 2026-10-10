package updater

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"keylint/internal/features/settings"
)

func TestParseBuildIdentity(t *testing.T) {
	tests := []struct {
		version string
		want    BuildIdentity
	}{
		{"0.0.0-pr.12+abc1234", BuildIdentity{IsDevBuild: true, Kind: "pr", PR: 12, Commit: "abc1234", Tag: "v0.0.0-pr.12"}},
		{"v0.0.0-pr.106+0123456789abcdef0123456789abcdef01234567", BuildIdentity{IsDevBuild: true, Kind: "pr", PR: 106, Commit: "0123456", Tag: "v0.0.0-pr.106"}},
		{"0.0.0-main+deadbee", BuildIdentity{IsDevBuild: true, Kind: "main", Commit: "deadbee", Tag: "v0.0.0-main"}},
		{"0.0.0-main", BuildIdentity{IsDevBuild: true, Kind: "main", Tag: "v0.0.0-main"}},
		{"v4.4.3-beta", BuildIdentity{}},
		{"v4.4.3-beta-12-gabc1234", BuildIdentity{}},
		{"dev", BuildIdentity{}},
		{"", BuildIdentity{}},
		{"0.0.0-pr.0+abc1234", BuildIdentity{}},  // no PR zero
		{"0.0.0-pr.12+ABC1234", BuildIdentity{}}, // CI writes lowercase hex
		{"0.0.0-pr.12-evil", BuildIdentity{}},
		{"1.0.0-pr.12+abc1234", BuildIdentity{}},
	}
	for _, tt := range tests {
		if got := parseBuildIdentity(tt.version); got != tt.want {
			t.Errorf("parseBuildIdentity(%q) = %+v, want %+v", tt.version, got, tt.want)
		}
	}
}

// What parseVersion makes of a dev version decides how every client in the
// field treats it, so pin it: 0.0.0 with the lowest pre-release rank.
func TestParseVersion_DevVersionsAreZero(t *testing.T) {
	for _, v := range []string{"0.0.0-pr.12+abc1234", "v0.0.0-pr.12", "0.0.0-main+abc1234", "v0.0.0-main"} {
		pv := parseVersion(v)
		if pv != (parsedVersion{preType: 0, preNum: 0}) {
			t.Errorf("parseVersion(%q) = %+v, want all zero", v, pv)
		}
		if isNewer(v, "0.0.1") || isNewer(v, "v4.4.3-beta") {
			t.Errorf("%q compares newer than a real version", v)
		}
	}
}

func TestBestRelease_DropsDevTags(t *testing.T) {
	releases := []githubRelease{
		{TagName: "v0.0.0-main"},
		{TagName: "v0.0.0-pr.5", Prerelease: true},
		{TagName: "v0.0.0-something-else"},
	}
	if best := bestRelease(releases, "pre-release"); best != nil {
		t.Errorf("bestRelease picked %q from dev tags alone", best.TagName)
	}
	releases = append(releases, githubRelease{TagName: "v4.4.3-beta", Prerelease: true})
	if best := bestRelease(releases, "pre-release"); best == nil || best.TagName != "v4.4.3-beta" {
		t.Errorf("bestRelease = %v, want v4.4.3-beta", best)
	}
}

// fakeGitHub serves a releases list, single releases by tag and downloads,
// and counts the API calls it answers.
type fakeGitHub struct {
	srv       *httptest.Server
	releases  []githubRelease
	apiCalls  atomic.Int32
	rateLimit atomic.Bool
	tagStatus map[string]int // forced status for /releases/tags/<tag>
}

func newFakeGitHub(t *testing.T, releases []githubRelease) *fakeGitHub {
	t.Helper()
	f := &fakeGitHub{releases: releases, tagStatus: map[string]int{}}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/download/") {
			w.Write([]byte("installer bytes"))
			return
		}
		f.apiCalls.Add(1)
		if f.rateLimit.Load() {
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(30*time.Minute).Unix(), 10))
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if tag, ok := strings.CutPrefix(r.URL.Path, "/releases/tags/"); ok {
			if code := f.tagStatus[tag]; code != 0 {
				w.WriteHeader(code)
				return
			}
			for _, rel := range f.releases {
				if rel.TagName == tag {
					json.NewEncoder(w).Encode(rel)
					return
				}
			}
			w.WriteHeader(http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(f.releases)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

// devRelease builds a dev-channel release the way CI publishes it.
func (f *fakeGitHub) devRelease(tag, title, commit string) githubRelease {
	body := "Commit: " + commit + "\n"
	if title != "" {
		n := strings.TrimPrefix(tag, "v0.0.0-pr.")
		body = "Installable test build.\n\nTitle: " + title + "\nPR: https://github.com/0xMMA/KeyLint/pull/" + n + "\n" + body
	}
	return githubRelease{
		TagName:     tag,
		Name:        tag,
		Body:        body,
		Prerelease:  true,
		PublishedAt: "2026-10-01T10:00:00Z",
		Assets: []githubAsset{
			{Name: "KeyLint-" + tag + "-windows-amd64-setup.exe", BrowserDownloadURL: f.srv.URL + "/download/" + tag + "-setup.exe", UpdatedAt: "2026-10-02T10:00:00Z"},
			{Name: "KeyLint-" + tag + "-linux-amd64", BrowserDownloadURL: f.srv.URL + "/download/" + tag + "-linux", UpdatedAt: "2026-10-02T10:00:00Z"},
		},
	}
}

func (f *fakeGitHub) realRelease(tag string, prerelease bool) githubRelease {
	return githubRelease{
		TagName:    tag,
		Prerelease: prerelease,
		Assets: []githubAsset{
			{Name: "KeyLint-" + tag + "-windows-amd64-setup.exe", BrowserDownloadURL: f.srv.URL + "/download/" + tag + "-setup.exe"},
			{Name: "KeyLint-" + tag + "-linux-amd64", BrowserDownloadURL: f.srv.URL + "/download/" + tag + "-linux"},
		},
	}
}

func devService(f *fakeGitHub, version string, developerOptions bool) *Service {
	cfg := settings.Default()
	cfg.DeveloperOptions = developerOptions
	s := NewService(version, settings.NewServiceFrom(cfg, settings.EnvOnlyKeys))
	s.releasesAPIURL = f.srv.URL + "/releases?per_page=100"
	return s
}

func standardReleases(f *fakeGitHub) []githubRelease {
	return []githubRelease{
		f.devRelease("v0.0.0-pr.7", "Older change", "7777777aaaaaaa"),
		f.realRelease("v4.4.3-beta", true),
		f.devRelease("v0.0.0-main", "", "1111111bbbbbbb"),
		f.devRelease("v0.0.0-pr.12", "Add dev channel", "abc1234ffffffff"),
		f.realRelease("v3.5.0", false),
	}
}

func TestListDevBuilds_ListsMainFirstThenPRsNewestFirst(t *testing.T) {
	f := newFakeGitHub(t, nil)
	f.releases = standardReleases(f)
	svc := devService(f, "v4.4.3-beta", true)

	got := svc.ListDevBuilds(false)
	if got.Error != "" {
		t.Fatalf("Error = %q", got.Error)
	}
	var tags []string
	for _, b := range got.Builds {
		tags = append(tags, b.Tag)
	}
	if strings.Join(tags, ",") != "v0.0.0-main,v0.0.0-pr.12,v0.0.0-pr.7" {
		t.Fatalf("builds = %v", tags)
	}
	pr := got.Builds[1]
	if pr.Kind != "pr" || pr.PR != 12 || pr.Title != "Add dev channel" || pr.Commit != "abc1234" ||
		pr.PRURL != "https://github.com/0xMMA/KeyLint/pull/12" || !pr.Installable || pr.Date != "2026-10-02T10:00:00Z" {
		t.Errorf("PR build = %+v", pr)
	}
	if got.Builds[0].Kind != "main" || got.Builds[0].Commit != "1111111" {
		t.Errorf("main build = %+v", got.Builds[0])
	}
	if got.LatestRelease != "4.4.3-beta" {
		t.Errorf("LatestRelease = %q, want 4.4.3-beta", got.LatestRelease)
	}
	if got.Current.IsDevBuild || got.Orphaned {
		t.Errorf("a release build reads as dev=%v orphaned=%v", got.Current.IsDevBuild, got.Orphaned)
	}
	for _, b := range got.Builds {
		if b.Installed || b.NewerBuild {
			t.Errorf("%s marked installed/newer on a release build", b.Tag)
		}
	}
}

func TestListDevBuilds_MarksTheRunningBuild(t *testing.T) {
	f := newFakeGitHub(t, nil)
	f.releases = standardReleases(f)

	same := devService(f, "0.0.0-pr.12+abc1234", false).ListDevBuilds(false)
	if b := same.Builds[1]; !b.Installed || b.NewerBuild {
		t.Errorf("same commit: installed=%v newer=%v", b.Installed, b.NewerBuild)
	}
	if same.Builds[0].Installed || same.Builds[2].Installed {
		t.Error("another build marked installed")
	}

	older := devService(f, "0.0.0-pr.12+0000000", false).ListDevBuilds(false)
	if b := older.Builds[1]; b.Installed || !b.NewerBuild {
		t.Errorf("older commit: installed=%v newer=%v", b.Installed, b.NewerBuild)
	}

	mainBuild := devService(f, "0.0.0-main+1111111", false).ListDevBuilds(false)
	if !mainBuild.Builds[0].Installed {
		t.Error("main build not marked installed")
	}
}

func TestListDevBuilds_OrphanedWhenTheReleaseIsGone(t *testing.T) {
	f := newFakeGitHub(t, nil)
	f.releases = standardReleases(f)
	svc := devService(f, "0.0.0-pr.99+abc1234", false)

	got := svc.ListDevBuilds(false)
	if !got.Orphaned {
		t.Fatal("PR #99 has no release but the build is not reported orphaned")
	}
	if got.Error != "" {
		t.Errorf("Error = %q", got.Error)
	}
	if got.LatestRelease != "4.4.3-beta" || got.Builds[0].Kind != "main" {
		t.Error("the return offer's targets are missing")
	}

	// The answer is remembered with the list: asking again costs nothing.
	calls := f.apiCalls.Load()
	if again := svc.ListDevBuilds(false); !again.Orphaned {
		t.Error("orphaned answer lost on the second ask")
	}
	if f.apiCalls.Load() != calls {
		t.Errorf("second ask made %d more API calls, want 0", f.apiCalls.Load()-calls)
	}
}

func TestListDevBuilds_NotOrphanedWhenOnlyMissingFromThePage(t *testing.T) {
	f := newFakeGitHub(t, nil)
	f.releases = standardReleases(f)
	hidden := f.devRelease("v0.0.0-pr.3", "Off the first page", "3333333")
	svc := devService(f, "0.0.0-pr.3+3333333", false)
	// Served by tag but not in the list, as on a crowded first page.
	f.srv.Config.Handler = wrapTag(f.srv.Config.Handler, hidden)

	if got := svc.ListDevBuilds(false); got.Orphaned {
		t.Error("a release that exists was reported orphaned")
	}
}

func wrapTag(next http.Handler, rel githubRelease) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/releases/tags/"+rel.TagName {
			json.NewEncoder(w).Encode(rel)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func TestListDevBuilds_NotOrphanedWhenGitHubCannotBeAsked(t *testing.T) {
	f := newFakeGitHub(t, nil)
	f.releases = standardReleases(f)
	f.tagStatus["v0.0.0-pr.99"] = http.StatusBadGateway
	got := devService(f, "0.0.0-pr.99+abc1234", false).ListDevBuilds(false)
	if got.Orphaned {
		t.Error("orphaned on a failed lookup; only a 404 may say that")
	}
	if got.Error == "" {
		t.Error("the failed lookup is not reported")
	}
}

func TestListDevBuilds_RateLimitIsAMessageAndStopsAsking(t *testing.T) {
	f := newFakeGitHub(t, nil)
	f.releases = standardReleases(f)
	f.rateLimit.Store(true)
	svc := devService(f, "0.0.0-pr.12+abc1234", true)

	got := svc.ListDevBuilds(true)
	if !strings.Contains(got.Error, "limit") {
		t.Errorf("Error = %q, want the rate limit named", got.Error)
	}
	if got.Orphaned {
		t.Error("orphaned while rate-limited")
	}
	if got.Builds == nil {
		t.Error("Builds is nil; the frontend expects an empty list")
	}
	if !got.Current.IsDevBuild {
		t.Error("identity lost on error")
	}

	calls := f.apiCalls.Load()
	svc.ListDevBuilds(true)
	if _, err := svc.InstallDevBuild("v0.0.0-main"); err == nil || !strings.Contains(err.Error(), "limit") {
		t.Errorf("InstallDevBuild while rate-limited: %v", err)
	}
	if f.apiCalls.Load() != calls {
		t.Errorf("%d API calls after GitHub said stop, want 0", f.apiCalls.Load()-calls)
	}
}

func TestReleases_CachedForAMinuteUnlessForced(t *testing.T) {
	f := newFakeGitHub(t, nil)
	f.releases = standardReleases(f)
	svc := devService(f, "v4.4.3-beta", true)
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	svc.ListDevBuilds(false)
	svc.ListDevBuilds(false)
	svc.CheckForUpdate()
	if n := f.apiCalls.Load(); n != 1 {
		t.Errorf("%d API calls within the TTL, want 1", n)
	}
	svc.ListDevBuilds(true)
	if n := f.apiCalls.Load(); n != 2 {
		t.Errorf("force did not refetch: %d calls", n)
	}
	now = now.Add(2 * time.Minute)
	svc.ListDevBuilds(false)
	if n := f.apiCalls.Load(); n != 3 {
		t.Errorf("expired cache not refetched: %d calls", n)
	}
}

func TestCheckForUpdate_DevBuildIsNeverOfferedAReleaseAndAsksNothing(t *testing.T) {
	f := newFakeGitHub(t, nil)
	f.releases = standardReleases(f)
	for _, v := range []string{"0.0.0-pr.12+abc1234", "0.0.0-main+1111111"} {
		info, err := devService(f, v, true).CheckForUpdate()
		if err != nil {
			t.Fatalf("%s: %v", v, err)
		}
		if info.IsAvailable || info.Channel != "dev" {
			t.Errorf("%s: available=%v channel=%q", v, info.IsAvailable, info.Channel)
		}
	}
	if n := f.apiCalls.Load(); n != 0 {
		t.Errorf("dev build update check made %d API calls", n)
	}
}

func TestInstallDevBuild_LockedWithoutDeveloperOptions(t *testing.T) {
	f := newFakeGitHub(t, nil)
	f.releases = standardReleases(f)
	svc := devService(f, "v4.4.3-beta", false)
	svc.applyFunc = func(*Service, string) (InstallResult, error) {
		t.Fatal("installed while locked")
		return InstallResult{}, nil
	}
	if _, err := svc.InstallDevBuild("v0.0.0-main"); err == nil {
		t.Error("InstallDevBuild worked with developer options off")
	}
	if _, err := svc.InstallLatestRelease(); err == nil {
		t.Error("InstallLatestRelease worked with developer options off")
	}
	if n := f.apiCalls.Load(); n != 0 {
		t.Errorf("%d API calls while locked", n)
	}
}

func TestInstallDevBuild_InstallsTheTaggedAsset(t *testing.T) {
	f := newFakeGitHub(t, nil)
	f.releases = standardReleases(f)
	svc := devService(f, "v4.4.3-beta", true)
	var applied string
	svc.applyFunc = func(_ *Service, tmp string) (InstallResult, error) {
		applied = tmp
		return InstallResult{RestartRequired: true}, nil
	}
	res, err := svc.InstallDevBuild("v0.0.0-pr.12")
	if err != nil {
		t.Fatalf("InstallDevBuild: %v", err)
	}
	if !res.RestartRequired || applied == "" {
		t.Errorf("result=%+v applied=%q", res, applied)
	}
}

func TestInstallDevBuild_RejectsWhatIsNotADevBuild(t *testing.T) {
	f := newFakeGitHub(t, nil)
	f.releases = standardReleases(f)
	svc := devService(f, "v4.4.3-beta", true)
	svc.applyFunc = func(*Service, string) (InstallResult, error) {
		t.Fatal("installed a non-dev tag")
		return InstallResult{}, nil
	}
	for _, tag := range []string{"v4.4.3-beta", "", "v0.0.0-pr.12/../../x", "v0.0.0-pr.0"} {
		if _, err := svc.InstallDevBuild(tag); err == nil {
			t.Errorf("InstallDevBuild(%q) accepted", tag)
		}
	}
	if _, err := svc.InstallDevBuild("v0.0.0-pr.99"); err == nil || !strings.Contains(err.Error(), "no longer exists") {
		t.Errorf("gone build: %v", err)
	}
}

func TestInstallLatestRelease_FromADevBuildSkipsDevTags(t *testing.T) {
	f := newFakeGitHub(t, nil)
	f.releases = standardReleases(f)
	svc := devService(f, "0.0.0-pr.99+abc1234", false) // a dev build needs no unlock
	var applied bool
	svc.applyFunc = func(_ *Service, _ string) (InstallResult, error) {
		applied = true
		return InstallResult{}, nil
	}
	var downloaded string
	f.srv.Config.Handler = func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/download/") {
				downloaded = r.URL.Path
			}
			next.ServeHTTP(w, r)
		})
	}(f.srv.Config.Handler)

	if _, err := svc.InstallLatestRelease(); err != nil {
		t.Fatalf("InstallLatestRelease: %v", err)
	}
	if !applied || !strings.Contains(downloaded, "v4.4.3-beta") {
		t.Errorf("applied=%v downloaded=%q, want v4.4.3-beta", applied, downloaded)
	}
}

func TestRateLimited(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	mk := func(code int, h map[string]string) *http.Response {
		r := &http.Response{StatusCode: code, Header: http.Header{}}
		for k, v := range h {
			r.Header.Set(k, v)
		}
		return r
	}
	if ok, _ := rateLimited(mk(200, nil), now); ok {
		t.Error("200 read as rate limit")
	}
	if ok, _ := rateLimited(mk(403, nil), now); ok {
		t.Error("plain 403 read as rate limit")
	}
	if ok, until := rateLimited(mk(403, map[string]string{"X-RateLimit-Remaining": "0", "X-RateLimit-Reset": "1000600"}), now); !ok || until.Unix() != 1000600 {
		t.Errorf("primary limit: %v %v", ok, until)
	}
	// A fast local clock sees the reset as past: a short wait, not an hour.
	if ok, until := rateLimited(mk(403, map[string]string{"X-RateLimit-Remaining": "0", "X-RateLimit-Reset": "999000"}), now); !ok || until.Sub(now) != time.Minute {
		t.Errorf("past reset: %v %v", ok, until.Sub(now))
	}
	if ok, until := rateLimited(mk(429, map[string]string{"Retry-After": "30"}), now); !ok || until.Sub(now) != 30*time.Second {
		t.Errorf("secondary limit: %v %v", ok, until)
	}
}

func TestListDevBuilds_KeepsTheLastListWhenGitHubStopsAnswering(t *testing.T) {
	f := newFakeGitHub(t, nil)
	f.releases = standardReleases(f)
	svc := devService(f, "0.0.0-pr.99+abc1234", false)
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	if first := svc.ListDevBuilds(false); len(first.Builds) != 3 || !first.Orphaned {
		t.Fatalf("first list: %d builds, orphaned=%v", len(first.Builds), first.Orphaned)
	}
	f.rateLimit.Store(true)
	now = now.Add(5 * time.Minute)

	got := svc.ListDevBuilds(true)
	if got.Error == "" {
		t.Error("the rate limit is not reported")
	}
	if len(got.Builds) != 3 {
		t.Errorf("%d builds, want the 3 last known", len(got.Builds))
	}
	if got.Orphaned {
		t.Error("orphaned claimed without a fresh answer from GitHub")
	}
}

func TestListDevBuilds_TellsADevBuildAboutANewerRelease(t *testing.T) {
	f := newFakeGitHub(t, nil)
	f.releases = standardReleases(f) // dev builds uploaded 2026-10-02T10:00:00Z
	for i := range f.releases {
		if f.releases[i].TagName == "v4.4.3-beta" {
			f.releases[i].PublishedAt = "2026-10-05T09:00:00Z"
		}
	}

	got := devService(f, "0.0.0-pr.12+abc1234", false).ListDevBuilds(false)
	if got.LatestRelease != "4.4.3-beta" || got.LatestReleaseDate != "2026-10-05T09:00:00Z" {
		t.Errorf("latest release = %q %q", got.LatestRelease, got.LatestReleaseDate)
	}
	if !got.NewReleaseSinceBuild {
		t.Error("a release published after the running build is not reported as news")
	}

	for i := range f.releases {
		if f.releases[i].TagName == "v4.4.3-beta" {
			f.releases[i].PublishedAt = "2026-09-01T09:00:00Z"
		}
	}
	if older := devService(f, "0.0.0-pr.12+abc1234", false).ListDevBuilds(false); older.NewReleaseSinceBuild {
		t.Error("an older release reported as news")
	}
	if rel := devService(f, "v4.4.3-beta", true).ListDevBuilds(false); rel.NewReleaseSinceBuild {
		t.Error("news reported to a release build, which has its normal update check")
	}
}

func TestReleases_FreshCacheServedDuringRateLimit(t *testing.T) {
	f := newFakeGitHub(t, nil)
	f.releases = standardReleases(f)
	svc := devService(f, "v4.4.3-beta", true)
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	svc.ListDevBuilds(false)
	svc.cacheMu.Lock()
	svc.blockedUntil = now.Add(time.Hour) // a tag lookup hit the limit since
	svc.cacheMu.Unlock()

	if got := svc.ListDevBuilds(false); got.Error != "" || len(got.Builds) != 3 {
		t.Errorf("fresh cache not served while blocked: error=%q builds=%d", got.Error, len(got.Builds))
	}
}

func TestDownloadAndApply_OneInstallAtATime(t *testing.T) {
	f := newFakeGitHub(t, nil)
	f.releases = standardReleases(f)
	svc := devService(f, "v4.4.3-beta", true)
	entered := make(chan struct{})
	release := make(chan struct{})
	svc.applyFunc = func(*Service, string) (InstallResult, error) {
		close(entered)
		<-release
		return InstallResult{RestartRequired: true}, nil
	}

	first := make(chan error, 1)
	go func() {
		_, err := svc.InstallDevBuild("v0.0.0-pr.12")
		first <- err
	}()
	<-entered

	if _, err := svc.InstallLatestRelease(); err == nil || !strings.Contains(err.Error(), "already running") {
		t.Errorf("second install while the first runs: %v", err)
	}
	close(release)
	if err := <-first; err != nil {
		t.Fatalf("first install: %v", err)
	}

	// Free again once the first is done.
	svc.applyFunc = func(*Service, string) (InstallResult, error) { return InstallResult{}, nil }
	if _, err := svc.InstallLatestRelease(); err != nil {
		t.Errorf("install after the first finished: %v", err)
	}
}
