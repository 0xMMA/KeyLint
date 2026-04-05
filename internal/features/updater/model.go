package updater

// UpdateInfo is returned to the frontend to describe whether an update is available.
type UpdateInfo struct {
	IsAvailable    bool   `json:"is_available"`
	LatestVersion  string `json:"latest_version"`
	CurrentVersion string `json:"current_version"`
	ReleaseURL     string `json:"release_url"`
	Notes          string `json:"notes"`
	Channel        string `json:"channel"`
}

// LatestJSON mirrors the structure of the latest.json file published with each release.
type LatestJSON struct {
	Version   string                   `json:"version"`
	Notes     string                   `json:"notes"`
	PubDate   string                   `json:"pub_date"`
	Platforms map[string]PlatformAsset `json:"platforms"`
}

// PlatformAsset holds the download URL and optional signature for a single platform binary.
type PlatformAsset struct {
	URL       string `json:"url"`
	Signature string `json:"signature"`
}

// InstallResult is returned by DownloadAndInstall to indicate the outcome.
// On Windows, RestartRequired is true because the NSIS installer needs the app to exit.
type InstallResult struct {
	RestartRequired bool `json:"restart_required"`
}

// githubRelease represents a single release from the GitHub Releases API.
type githubRelease struct {
	TagName    string        `json:"tag_name"`
	Name       string        `json:"name"`
	Body       string        `json:"body"`
	Draft      bool          `json:"draft"`
	Prerelease bool          `json:"prerelease"`
	Assets     []githubAsset `json:"assets"`
}

// githubAsset represents a single downloadable file attached to a GitHub release.
type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}
