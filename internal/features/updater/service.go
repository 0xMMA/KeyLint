package updater

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"keylint/internal/features/settings"
)

// defaultReleasesAPIURL lists the newest releases. 100 rather than the 20 that
// v4.4.3-beta and older ask for: the dev channel publishes one prerelease per
// open pull request plus one for main, and those sort above real releases.
const defaultReleasesAPIURL = "https://api.github.com/repos/0xMMA/KeyLint/releases?per_page=100"

// apiTimeout bounds one call to the GitHub API. Downloads are not bounded by
// it: an installer on a slow line takes longer, and the user is watching it.
const apiTimeout = 15 * time.Second

// releasesTTL is how long a fetched release list is reused. Unauthenticated
// clients get 60 API calls an hour per IP; the shell, the About tab and an
// install each want the list, so without this one session spends a dozen.
const releasesTTL = time.Minute

// Service checks for updates and can apply them using platform-specific strategies.
type Service struct {
	currentVersion string
	releasesAPIURL string
	client         *http.Client
	settingsSvc    *settings.Service
	quitFunc       func()                                                    // called after launching installer on Windows; set via SetQuitFunc
	applyFunc      func(svc *Service, tmpPath string) (InstallResult, error) // override for testing; nil uses applyPlatformUpdate
	now            func() time.Time                                          // override for testing

	// cacheMu guards the release list and the rate-limit back-off below. Wails
	// serves every RPC on its own goroutine, so the shell's startup check and
	// the About tab can ask at the same moment.
	cacheMu      sync.Mutex
	cached       []githubRelease
	cachedAt     time.Time
	blockedUntil time.Time // GitHub said the anonymous quota is spent until then
	goneTag      string    // a dev tag GitHub answered 404 for…
	goneAt       time.Time // …and when
}

// NewService creates an updater Service with the given current version string.
// The version is typically injected at build time via -ldflags "-X main.AppVersion=x.y.z".
func NewService(version string, settingsSvc *settings.Service) *Service {
	return &Service{
		currentVersion: version,
		releasesAPIURL: defaultReleasesAPIURL,
		client:         &http.Client{},
		settingsSvc:    settingsSvc,
		now:            time.Now,
	}
}

// GetVersion returns the current application version.
func (s *Service) GetVersion() string {
	return s.currentVersion
}

// SetQuitFunc sets the callback invoked after launching the installer on Windows.
// The callback should wait briefly (for the frontend to display a message) then quit the app.
func (s *Service) SetQuitFunc(fn func()) {
	s.quitFunc = fn
}

// CheckForUpdate fetches the GitHub Releases API and finds the best available update.
//
// Dev builds (0.0.0-pr.N, 0.0.0-main) never get an answer here: at version
// 0.0.0 every release would look like an update, forever. They are served by
// ListDevBuilds instead, which knows where they came from.
func (s *Service) CheckForUpdate() (UpdateInfo, error) {
	info := UpdateInfo{CurrentVersion: s.currentVersion}

	// Skip update check for dev builds.
	if s.currentVersion == "dev" || s.currentVersion == "" {
		return info, nil
	}
	if parseBuildIdentity(s.currentVersion).IsDevBuild {
		info.Channel = "dev"
		return info, nil
	}

	// Resolve effective channel from settings.
	channel := s.resolveChannel()
	info.Channel = channel

	releases, err := s.releases(false)
	if err != nil {
		return info, err
	}

	best := bestRelease(releases, channel)
	if best == nil {
		return info, nil
	}

	info.LatestVersion = strings.TrimPrefix(best.TagName, "v")
	info.Notes = best.Body

	// Match platform asset by filename substring.
	info.ReleaseURL = matchPlatformAsset(best.Assets).BrowserDownloadURL

	info.IsAvailable = isNewer(best.TagName, s.currentVersion)
	return info, nil
}

// bestRelease picks the highest-versioned release a normal channel may offer:
// no drafts, no prereleases on stable, and never a dev-channel build — those
// are published as prereleases tagged v0.0.0-*, which only the dev channel
// lists. Their version is 0.0.0 and so could never win on version alone; the
// explicit filter is there so that stays true whatever they are tagged later.
func bestRelease(releases []githubRelease, channel string) *githubRelease {
	var best *githubRelease
	for i := range releases {
		r := &releases[i]
		if r.Draft || isDevTag(r.TagName) {
			continue
		}
		if channel == "stable" && r.Prerelease {
			continue
		}
		if best == nil || isNewer(r.TagName, best.TagName) {
			best = r
		}
	}
	return best
}

// rateLimitError is GitHub refusing anonymous requests until reset.
type rateLimitError struct{ reset time.Time }

func (e rateLimitError) Error() string {
	return fmt.Sprintf("GitHub's limit for anonymous requests is used up — try again after %s", e.reset.Local().Format("15:04"))
}

// releases returns the release list, from cache when it is younger than
// releasesTTL unless force is set. Once GitHub has said the anonymous quota is
// spent, nothing is sent until it resets, force or not: asking again only
// costs time, and an empty-handed retry loop is what turns a rate limit into
// a broken screen.
func (s *Service) releases(force bool) ([]githubRelease, error) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()

	now := s.now()
	if now.Before(s.blockedUntil) {
		return nil, rateLimitError{reset: s.blockedUntil}
	}
	if !force && s.cached != nil && now.Sub(s.cachedAt) < releasesTTL {
		return s.cached, nil
	}

	var releases []githubRelease
	if err := s.getJSON(s.releasesAPIURL, &releases); err != nil {
		return nil, err
	}
	s.cached = releases
	s.cachedAt = now
	return releases, nil
}

// errNotFound is a 404 from the GitHub API.
var errNotFound = errors.New("not found")

// getJSON performs one bounded GitHub API call. The caller holds cacheMu, so
// the rate-limit state it records cannot race with another call.
func (s *Service) getJSON(url string, out any) error {
	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "KeyLint")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetching releases: %w", err)
	}
	defer resp.Body.Close()

	if limited, reset := rateLimited(resp, s.now()); limited {
		s.blockedUntil = reset
		return rateLimitError{reset: reset}
	}
	if resp.StatusCode == http.StatusNotFound {
		return errNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("releases API returned status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("parsing releases: %w", err)
	}
	return nil
}

// rateLimited reports whether resp is GitHub's rate-limit answer, and until
// when. Primary limits come as 403 or 429 with X-RateLimit-Remaining: 0 and a
// reset epoch; secondary limits as 403 or 429 with Retry-After. A 403 with
// neither is some other refusal and is reported as a plain status error.
func rateLimited(resp *http.Response, now time.Time) (bool, time.Time) {
	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusTooManyRequests {
		return false, time.Time{}
	}
	if secs, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && secs > 0 {
		return true, now.Add(time.Duration(secs) * time.Second)
	}
	if resp.Header.Get("X-RateLimit-Remaining") == "0" {
		if epoch, err := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
			if reset := time.Unix(epoch, 0); reset.After(now) {
				return true, reset
			}
		}
		// Spent, but no usable reset time: back off for GitHub's window.
		return true, now.Add(time.Hour)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return true, now.Add(time.Minute)
	}
	return false, time.Time{}
}

// resolveChannel determines the effective update channel.
func (s *Service) resolveChannel() string {
	if s.settingsSvc != nil {
		cfg := s.settingsSvc.Get()
		if cfg.UpdateChannel == "stable" || cfg.UpdateChannel == "pre-release" {
			return cfg.UpdateChannel
		}
	}
	// Auto-detect from current version.
	pv := parseVersion(s.currentVersion)
	if pv.preType < 3 {
		return "pre-release"
	}
	return "stable"
}

// matchPlatformAsset finds the asset for the current platform, or the zero
// asset when there is none.
func matchPlatformAsset(assets []githubAsset) githubAsset {
	var substring string
	switch runtime.GOOS {
	case "windows":
		substring = "windows-amd64-setup"
	default:
		substring = "linux-amd64"
	}
	if runtime.GOARCH == "arm64" {
		substring = strings.Replace(substring, "amd64", "arm64", 1)
	}

	for _, a := range assets {
		if strings.Contains(a.Name, substring) {
			return a
		}
	}
	return githubAsset{}
}

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

	return s.downloadAndApply(updateInfo.ReleaseURL)
}

// downloadAndApply fetches an installer or binary, saves it to a temp file and
// hands it to the platform-specific installer. Shared by the normal update
// path and the dev channel, so both install the same way.
func (s *Service) downloadAndApply(url string) (InstallResult, error) {
	resp, err := s.client.Get(url)
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

// parsedVersion holds the decomposed parts of a semver string with optional pre-release suffix.
type parsedVersion struct {
	major   int
	minor   int
	patch   int
	preType int // 0=alpha, 1=beta, 2=rc, 3=stable (no suffix)
	preNum  int // trailing number from suffix (e.g. rc2 → 2, alpha → 0)
}

// parseVersion parses a version string like "4.1.8-alpha", "v4.1.8-rc2", "4.1.8".
func parseVersion(s string) parsedVersion {
	s = strings.TrimPrefix(s, "v")

	var pv parsedVersion
	pv.preType = 3 // stable by default

	// Split off pre-release suffix at first hyphen.
	base := s
	suffix := ""
	if idx := strings.IndexByte(s, '-'); idx >= 0 {
		base = s[:idx]
		suffix = s[idx+1:]
	}

	// Parse major.minor.patch
	parts := strings.Split(base, ".")
	if len(parts) >= 1 {
		pv.major, _ = strconv.Atoi(parts[0])
	}
	if len(parts) >= 2 {
		pv.minor, _ = strconv.Atoi(parts[1])
	}
	if len(parts) >= 3 {
		pv.patch, _ = strconv.Atoi(parts[2])
	}

	// Parse pre-release suffix.
	if suffix != "" {
		lower := strings.ToLower(suffix)
		switch {
		case strings.HasPrefix(lower, "alpha"):
			pv.preType = 0
			pv.preNum = parseTrailingInt(lower[5:])
		case strings.HasPrefix(lower, "beta"):
			pv.preType = 1
			pv.preNum = parseTrailingInt(lower[4:])
		case strings.HasPrefix(lower, "rc"):
			pv.preType = 2
			pv.preNum = parseTrailingInt(lower[2:])
		default:
			// Unknown suffix — treat as pre-release with lowest priority.
			pv.preType = 0
			pv.preNum = 0
		}
	}

	return pv
}

// parseTrailingInt extracts a number from a string like "2" or "" (returns 0 for empty).
func parseTrailingInt(s string) int {
	if s == "" {
		return 0
	}
	n, _ := strconv.Atoi(s)
	return n
}

// isNewer returns true when latestVer is strictly newer than currentVer.
// Both versions may have a leading 'v' prefix and optional pre-release suffixes.
func isNewer(latestVer, currentVer string) bool {
	l := parseVersion(latestVer)
	c := parseVersion(currentVer)

	if l.major != c.major {
		return l.major > c.major
	}
	if l.minor != c.minor {
		return l.minor > c.minor
	}
	if l.patch != c.patch {
		return l.patch > c.patch
	}
	if l.preType != c.preType {
		return l.preType > c.preType
	}
	return l.preNum > c.preNum
}
