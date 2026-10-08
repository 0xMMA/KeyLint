package updater

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// The dev channel: installable builds of open pull requests and of main.
//
// CI publishes them as GitHub prereleases (build-linux.yml, job
// publish-dev-build), because a release asset downloads without a login and
// a workflow artifact does not. Tags and versions:
//
//	tag v0.0.0-pr.<N>   version 0.0.0-pr.<N>+<shortsha>   one per open same-repo PR
//	tag v0.0.0-main     version 0.0.0-main+<shortsha>     the latest push to main
//
// Version 0.0.0 is what keeps them away from every client: parseVersion reads
// "pr.N" and "main" as an unknown pre-release suffix on 0.0.0, which is older
// than any real release, so no client — including v4.4.3-beta and older,
// which have no idea this channel exists — ever offers one as an update.
// bestRelease also drops them by tag. See devbuild_safety_test.go.

// DevBuild is one installable dev-channel build.
type DevBuild struct {
	Tag         string `json:"tag"`         // v0.0.0-pr.12 or v0.0.0-main
	Kind        string `json:"kind"`        // "pr" or "main"
	PR          int    `json:"pr"`          // 0 for main
	Title       string `json:"title"`       // PR title; empty for main
	PRURL       string `json:"pr_url"`      // empty for main
	Commit      string `json:"commit"`      // short SHA the build was made from
	Date        string `json:"date"`        // RFC 3339, when this build was uploaded
	Installable bool   `json:"installable"` // has an asset for this platform
	Installed   bool   `json:"installed"`   // this is the build that is running
	NewerBuild  bool   `json:"newer_build"` // same PR or main as the running build, newer commit
}

// BuildIdentity is what the running build knows about itself from its version.
type BuildIdentity struct {
	IsDevBuild bool   `json:"is_dev_build"`
	Kind       string `json:"kind"`   // "pr", "main", or "" for a normal build
	PR         int    `json:"pr"`     // 0 unless Kind is "pr"
	Commit     string `json:"commit"` // short SHA, when the version carries one
	Tag        string `json:"tag"`    // the release this build came from, for dev builds
}

// DevChannel is everything the dev-channel view shows. It never comes back as
// an error: GitHub being unreachable or rate-limited is reported in Error, next
// to the last list fetched this session (if any), so the screen degrades
// instead of breaking.
type DevChannel struct {
	Current BuildIdentity `json:"current"`
	Builds  []DevBuild    `json:"builds"`
	// Orphaned is true when the running build is a PR build whose release no
	// longer exists: the PR was merged or closed. Only set on a definite answer
	// from GitHub, never because GitHub could not be asked.
	Orphaned bool `json:"orphaned"`
	// LatestRelease is the newest real release on the effective update
	// channel, without the "v" — the second choice in the return offer.
	LatestRelease string `json:"latest_release"`
	Error         string `json:"error"`
}

var (
	devTagRe     = regexp.MustCompile(`^v0\.0\.0-(?:pr\.([1-9][0-9]{0,8})|main)$`)
	devVersionRe = regexp.MustCompile(`^v?0\.0\.0-(?:pr\.([1-9][0-9]{0,8})|main)(?:\+([0-9a-f]{7,40}))?$`)
	commitLineRe = regexp.MustCompile(`(?m)^Commit: ([0-9a-f]{7,40})\s*$`)
	titleLineRe  = regexp.MustCompile(`(?m)^Title: (.*?)\s*$`)
	prLineRe     = regexp.MustCompile(`(?m)^PR: (https://github\.com/\S+)\s*$`)
)

// isDevTag reports whether a release tag belongs to the dev channel. Anything
// at v0.0.0 counts, not only the two shapes CI publishes today: no real
// release will ever be 0.0.0, and a normal channel should not have to know
// which dev tags exist to stay clear of them.
func isDevTag(tag string) bool {
	return strings.HasPrefix(tag, "v0.0.0-") || strings.HasPrefix(tag, "0.0.0-")
}

// parseBuildIdentity reads a version string as stamped by CI.
func parseBuildIdentity(version string) BuildIdentity {
	m := devVersionRe.FindStringSubmatch(version)
	if m == nil {
		return BuildIdentity{}
	}
	id := BuildIdentity{IsDevBuild: true, Kind: "main", Tag: "v0.0.0-main"}
	if m[1] != "" {
		id.Kind = "pr"
		id.PR, _ = strconv.Atoi(m[1])
		id.Tag = "v0.0.0-pr." + m[1]
	}
	id.Commit = shortSHA(m[2])
	return id
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// GetBuildIdentity tells the frontend whether it is running a dev build and
// which one. No network call.
func (s *Service) GetBuildIdentity() BuildIdentity {
	return parseBuildIdentity(s.currentVersion)
}

// devBuildFrom turns a dev-channel release into a DevBuild, or reports false
// for any other release.
func devBuildFrom(r githubRelease) (DevBuild, bool) {
	if r.Draft {
		return DevBuild{}, false
	}
	m := devTagRe.FindStringSubmatch(r.TagName)
	if m == nil {
		return DevBuild{}, false
	}
	b := DevBuild{Tag: r.TagName, Kind: "main"}
	if m[1] != "" {
		b.Kind = "pr"
		b.PR, _ = strconv.Atoi(m[1])
		if t := titleLineRe.FindStringSubmatch(r.Body); t != nil {
			b.Title = t[1]
		}
		if u := prLineRe.FindStringSubmatch(r.Body); u != nil {
			b.PRURL = u[1]
		}
	}
	if c := commitLineRe.FindStringSubmatch(r.Body); c != nil {
		b.Commit = shortSHA(c[1])
	}
	asset := matchPlatformAsset(r.Assets)
	b.Installable = asset.BrowserDownloadURL != ""
	b.Date = asset.UpdatedAt
	if b.Date == "" {
		b.Date = r.PublishedAt
	}
	return b, true
}

// devChannelAllowed gates the dev channel's install methods, so a stray call
// cannot install a test build for someone who never unlocked the channel. It
// is not a defence against the webview itself: that can call the exported
// SetDeveloperOptions first. What limits the exposure is that only dev tags
// are accepted, and only the owner's branches publish them. A dev build is
// always allowed: with the normal update check silenced, the dev channel is
// its only way off.
func (s *Service) devChannelAllowed() bool {
	if parseBuildIdentity(s.currentVersion).IsDevBuild {
		return true
	}
	return s.settingsSvc != nil && s.settingsSvc.Get().DeveloperOptions
}

var errDevChannelLocked = errors.New("the dev channel is only available with developer options turned on")

// ListDevBuilds lists the dev-channel builds — main first, then open PRs,
// newest PR first — and says whether the running PR build has been orphaned.
// force skips the one-minute cache, as a Refresh button wants; it never
// skips a rate-limit back-off.
func (s *Service) ListDevBuilds(force bool) DevChannel {
	out := DevChannel{Current: s.GetBuildIdentity(), Builds: []DevBuild{}}

	releases, err := s.releases(force)
	if err != nil {
		out.Error = friendlyError(err)
		// Show what was last known, marked by the error, rather than nothing.
		// Orphan detection stays off: it needs a definite answer from GitHub.
		if releases = s.lastKnownReleases(); releases == nil {
			return out
		}
	}

	for _, r := range releases {
		b, ok := devBuildFrom(r)
		if !ok {
			continue
		}
		s.markAgainstCurrent(&b, out.Current)
		out.Builds = append(out.Builds, b)
	}
	sort.SliceStable(out.Builds, func(i, j int) bool {
		a, b := out.Builds[i], out.Builds[j]
		if (a.Kind == "main") != (b.Kind == "main") {
			return a.Kind == "main"
		}
		return a.PR > b.PR
	})

	if best := bestRelease(releases, s.resolveChannel()); best != nil {
		out.LatestRelease = strings.TrimPrefix(best.TagName, "v")
	}

	if err == nil && out.Current.Kind == "pr" && !hasTag(out.Builds, out.Current.Tag) {
		// Not in the list. The list is one page, so ask for the tag itself
		// before declaring the build orphaned: only a 404 says it is gone.
		gone, err := s.tagGone(out.Current.Tag, force)
		switch {
		case err != nil:
			out.Error = friendlyError(err)
		case gone:
			out.Orphaned = true
		}
	}
	return out
}

func (s *Service) markAgainstCurrent(b *DevBuild, cur BuildIdentity) {
	if !cur.IsDevBuild || b.Tag != cur.Tag {
		return
	}
	// A build without a commit on either side cannot be told apart from its
	// successor, so it counts as the running one rather than nagging.
	if cur.Commit == "" || b.Commit == "" || strings.HasPrefix(b.Commit, cur.Commit) || strings.HasPrefix(cur.Commit, b.Commit) {
		b.Installed = true
		return
	}
	b.NewerBuild = true
}

func hasTag(builds []DevBuild, tag string) bool {
	for _, b := range builds {
		if b.Tag == tag {
			return true
		}
	}
	return false
}

// tagGone asks GitHub whether a release tag is gone, remembering a "gone"
// for as long as the list itself is cached so the shell and the About tab
// do not each spend a call on the same answer.
func (s *Service) tagGone(tag string, force bool) (bool, error) {
	s.cacheMu.Lock()
	if !force && s.goneTag == tag && s.now().Sub(s.goneAt) < releasesTTL {
		s.cacheMu.Unlock()
		return true, nil
	}
	s.cacheMu.Unlock()

	_, err := s.releaseByTag(tag)
	if errors.Is(err, errNotFound) {
		s.cacheMu.Lock()
		s.goneTag, s.goneAt = tag, s.now()
		s.cacheMu.Unlock()
		return true, nil
	}
	return false, err
}

// releaseByTag fetches one release, bypassing the list cache: an install
// wants the asset that is there now, not the one from a minute ago.
func (s *Service) releaseByTag(tag string) (githubRelease, error) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()

	if now := s.now(); now.Before(s.blockedUntil) {
		return githubRelease{}, rateLimitError{reset: s.blockedUntil}
	}
	var r githubRelease
	err := s.getJSON(releaseTagURL(s.releasesAPIURL, tag), &r)
	return r, err
}

// releaseTagURL turns the list URL (…/releases?per_page=100) into the URL of
// one release by tag (…/releases/tags/<tag>).
func releaseTagURL(listURL, tag string) string {
	base, _, _ := strings.Cut(listURL, "?")
	return strings.TrimSuffix(base, "/") + "/tags/" + url.PathEscape(tag)
}

// InstallDevBuild downloads and installs a dev-channel build by tag, through
// the same path as a normal update. Only dev tags are accepted, and only
// with developer options on or from a dev build.
func (s *Service) InstallDevBuild(tag string) (InstallResult, error) {
	if !s.devChannelAllowed() {
		return InstallResult{}, errDevChannelLocked
	}
	if !devTagRe.MatchString(tag) {
		return InstallResult{}, fmt.Errorf("%q is not a dev-channel build", tag)
	}
	r, err := s.releaseByTag(tag)
	if errors.Is(err, errNotFound) {
		return InstallResult{}, fmt.Errorf("the build %s no longer exists", tag)
	}
	if err != nil {
		return InstallResult{}, errors.New(friendlyError(err))
	}
	asset := matchPlatformAsset(r.Assets)
	if asset.BrowserDownloadURL == "" {
		return InstallResult{}, fmt.Errorf("the build %s has nothing to install on this platform", tag)
	}
	return s.downloadAndApply(asset.BrowserDownloadURL)
}

// InstallLatestRelease installs the newest real release on the effective
// update channel, whatever version is running — the way back from a dev
// build, which the normal update check deliberately ignores.
func (s *Service) InstallLatestRelease() (InstallResult, error) {
	if !s.devChannelAllowed() {
		return InstallResult{}, errDevChannelLocked
	}
	releases, err := s.releases(false)
	if err != nil {
		return InstallResult{}, errors.New(friendlyError(err))
	}
	best := bestRelease(releases, s.resolveChannel())
	if best == nil {
		return InstallResult{}, errors.New("no release found")
	}
	asset := matchPlatformAsset(best.Assets)
	if asset.BrowserDownloadURL == "" {
		return InstallResult{}, fmt.Errorf("release %s has nothing to install on this platform", best.TagName)
	}
	return s.downloadAndApply(asset.BrowserDownloadURL)
}

// friendlyError words a fetch failure for the screen.
func friendlyError(err error) string {
	var rl rateLimitError
	if errors.As(err, &rl) {
		return rl.Error()
	}
	return "Could not reach GitHub: " + err.Error()
}
