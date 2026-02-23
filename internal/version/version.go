// Package version provides functionality to check for new releases on GitHub
// and compare semantic versions.
package version

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// GitHubRelease represents the relevant fields from GitHub's releases API
type GitHubRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	HTMLURL     string    `json:"html_url"`
	Body        string    `json:"body"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
}

// ReleaseInfo contains version comparison results
type ReleaseInfo struct {
	CurrentVersion  string    `json:"current_version"`
	LatestVersion   string    `json:"latest_version"`
	UpdateAvailable bool      `json:"update_available"`
	ReleaseURL      string    `json:"release_url,omitempty"`
	ReleaseNotes    string    `json:"release_notes,omitempty"`
	ReleaseName     string    `json:"release_name,omitempty"`
	PublishedAt     time.Time `json:"published_at,omitempty"`
	CheckedAt       time.Time `json:"checked_at"`
}

// Checker handles version checking with caching
type Checker struct {
	currentVersion string
	owner          string
	repo           string
	httpClient     *http.Client

	mu          sync.RWMutex
	cachedInfo  *ReleaseInfo
	cacheExpiry time.Time
	cacheTTL    time.Duration
}

// Default configuration
const (
	DefaultCacheTTL    = 1 * time.Hour
	DefaultHTTPTimeout = 10 * time.Second
	GitHubAPIURL       = "https://api.github.com/repos/%s/%s/releases/latest"
)

// NewChecker creates a new version checker
func NewChecker(currentVersion, owner, repo string) *Checker {
	return &Checker{
		currentVersion: normalizeVersion(currentVersion),
		owner:          owner,
		repo:           repo,
		httpClient: &http.Client{
			Timeout: DefaultHTTPTimeout,
		},
		cacheTTL: DefaultCacheTTL,
	}
}

// SetCacheTTL allows customizing the cache duration
func (c *Checker) SetCacheTTL(ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cacheTTL = ttl
}

// Check fetches the latest release info, using cache if available
func (c *Checker) Check() (*ReleaseInfo, error) {
	// Check cache first
	c.mu.RLock()
	if c.cachedInfo != nil && time.Now().Before(c.cacheExpiry) {
		info := *c.cachedInfo
		c.mu.RUnlock()
		return &info, nil
	}
	c.mu.RUnlock()

	// Fetch from GitHub
	info, err := c.fetchLatestRelease()
	if err != nil {
		// If fetch fails but we have stale cache, return it
		c.mu.RLock()
		if c.cachedInfo != nil {
			staleInfo := *c.cachedInfo
			staleInfo.CheckedAt = time.Now()
			c.mu.RUnlock()
			return &staleInfo, nil
		}
		c.mu.RUnlock()
		return nil, err
	}

	// Update cache
	c.mu.Lock()
	c.cachedInfo = info
	c.cacheExpiry = time.Now().Add(c.cacheTTL)
	c.mu.Unlock()

	return info, nil
}

// ForceCheck bypasses the cache and fetches fresh data
func (c *Checker) ForceCheck() (*ReleaseInfo, error) {
	info, err := c.fetchLatestRelease()
	if err != nil {
		return nil, err
	}

	// Update cache
	c.mu.Lock()
	c.cachedInfo = info
	c.cacheExpiry = time.Now().Add(c.cacheTTL)
	c.mu.Unlock()

	return info, nil
}

// fetchLatestRelease makes the actual API call to GitHub
func (c *Checker) fetchLatestRelease() (*ReleaseInfo, error) {
	url := fmt.Sprintf(GitHubAPIURL, c.owner, c.repo)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers for GitHub API
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", fmt.Sprintf("Vigil/%s", c.currentVersion))

	resp, err := c.httpClient.Do(req) // #nosec G107 -- URL is constructed from hardcoded GitHub API pattern
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// No releases yet - not an error
		return &ReleaseInfo{
			CurrentVersion:  c.currentVersion,
			LatestVersion:   c.currentVersion,
			UpdateAvailable: false,
			CheckedAt:       time.Now(),
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to decode release info: %w", err)
	}

	// Skip prereleases and drafts
	if release.Draft || release.Prerelease {
		return &ReleaseInfo{
			CurrentVersion:  c.currentVersion,
			LatestVersion:   c.currentVersion,
			UpdateAvailable: false,
			CheckedAt:       time.Now(),
		}, nil
	}

	latestVersion := normalizeVersion(release.TagName)
	updateAvailable := CompareVersions(c.currentVersion, latestVersion) < 0

	// Truncate release notes if too long (for UI display)
	releaseNotes := release.Body
	if len(releaseNotes) > 500 {
		releaseNotes = releaseNotes[:497] + "..."
	}

	return &ReleaseInfo{
		CurrentVersion:  c.currentVersion,
		LatestVersion:   latestVersion,
		UpdateAvailable: updateAvailable,
		ReleaseURL:      release.HTMLURL,
		ReleaseNotes:    releaseNotes,
		ReleaseName:     release.Name,
		PublishedAt:     release.PublishedAt,
		CheckedAt:       time.Now(),
	}, nil
}

// GetCurrentVersion returns the current version string
func (c *Checker) GetCurrentVersion() string {
	return c.currentVersion
}

// CompareVersions compares two semantic versions
// Returns: -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
func CompareVersions(v1, v2 string) int {
	v1 = normalizeVersion(v1)
	v2 = normalizeVersion(v2)

	parts1 := parseVersion(v1)
	parts2 := parseVersion(v2)

	// Compare major, minor, patch
	for i := 0; i < 3; i++ {
		if parts1[i] < parts2[i] {
			return -1
		}
		if parts1[i] > parts2[i] {
			return 1
		}
	}

	// Compare prerelease (empty means stable, which is greater)
	pre1, pre2 := parts1[3], parts2[3]
	if pre1 == 0 && pre2 != 0 {
		return 1 // v1 is stable, v2 is prerelease
	}
	if pre1 != 0 && pre2 == 0 {
		return -1 // v1 is prerelease, v2 is stable
	}
	if pre1 < pre2 {
		return -1
	}
	if pre1 > pre2 {
		return 1
	}

	return 0
}

// normalizeVersion strips 'v' prefix and normalizes the version string
func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	return v
}

// parseVersion extracts major, minor, patch, and prerelease number
// Returns [major, minor, patch, prerelease]
func parseVersion(v string) [4]int {
	var result [4]int

	// Handle prerelease suffix (-alpha.1, -beta.2, -rc.3, etc.)
	prerelease := 0
	if idx := strings.Index(v, "-"); idx != -1 {
		prePart := v[idx+1:]
		v = v[:idx]

		// Extract prerelease number (e.g., "alpha.2" -> 2, "rc3" -> 3)
		re := regexp.MustCompile(`(\d+)`)
		if matches := re.FindStringSubmatch(prePart); len(matches) > 1 {
			prerelease, _ = strconv.Atoi(matches[1])
		}

		// Assign weight based on prerelease type
		preLower := strings.ToLower(prePart)
		switch {
		case strings.HasPrefix(preLower, "alpha"):
			prerelease += 1000 // alpha.1 = 1001
		case strings.HasPrefix(preLower, "beta"):
			prerelease += 2000 // beta.1 = 2001
		case strings.HasPrefix(preLower, "rc"):
			prerelease += 3000 // rc.1 = 3001
		default:
			prerelease += 500 // unknown prerelease
		}
	}
	result[3] = prerelease

	// Parse major.minor.patch
	parts := strings.Split(v, ".")
	for i := 0; i < len(parts) && i < 3; i++ {
		// Strip any non-numeric suffix
		numStr := regexp.MustCompile(`^\d+`).FindString(parts[i])
		if num, err := strconv.Atoi(numStr); err == nil {
			result[i] = num
		}
	}

	return result
}

// IsNewerVersion is a convenience function to check if latest is newer than current
func IsNewerVersion(current, latest string) bool {
	return CompareVersions(current, latest) < 0
}
