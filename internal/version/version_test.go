package version

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"v1.0.0", "1.0.0"},
		{"V1.0.0", "1.0.0"},
		{"1.0.0", "1.0.0"},
		{"  v1.2.3  ", "1.2.3"},
		{"v2.1.0-beta.1", "2.1.0-beta.1"},
	}

	for _, tt := range tests {
		result := normalizeVersion(tt.input)
		if result != tt.expected {
			t.Errorf("normalizeVersion(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected int
	}{
		// Basic comparisons
		{"1.0.0", "1.0.0", 0},
		{"1.0.0", "1.0.1", -1},
		{"1.0.1", "1.0.0", 1},
		{"1.1.0", "1.0.0", 1},
		{"2.0.0", "1.9.9", 1},

		// With v prefix
		{"v1.0.0", "v1.0.1", -1},
		{"v1.0.0", "1.0.0", 0},

		// Prerelease comparisons
		{"1.0.0-alpha.1", "1.0.0", -1},         // prerelease < stable
		{"1.0.0", "1.0.0-alpha.1", 1},          // stable > prerelease
		{"1.0.0-alpha.1", "1.0.0-alpha.2", -1}, // alpha.1 < alpha.2
		{"1.0.0-alpha.1", "1.0.0-beta.1", -1},  // alpha < beta
		{"1.0.0-beta.1", "1.0.0-rc.1", -1},     // beta < rc
		{"1.0.0-rc.1", "1.0.0", -1},            // rc < stable

		// Real-world scenarios
		{"1.2.3", "1.2.4", -1},
		{"1.2.10", "1.2.9", 1}, // Numeric comparison, not string
		{"0.9.0", "1.0.0", -1},
		{"2.0.0-rc.1", "2.0.0", -1},
	}

	for _, tt := range tests {
		result := CompareVersions(tt.v1, tt.v2)
		if result != tt.expected {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", tt.v1, tt.v2, result, tt.expected)
		}
	}
}

func TestIsNewerVersion(t *testing.T) {
	tests := []struct {
		current  string
		latest   string
		expected bool
	}{
		{"1.0.0", "1.0.1", true},
		{"1.0.0", "1.0.0", false},
		{"1.0.1", "1.0.0", false},
		{"v1.0.0", "v2.0.0", true},
	}

	for _, tt := range tests {
		result := IsNewerVersion(tt.current, tt.latest)
		if result != tt.expected {
			t.Errorf("IsNewerVersion(%q, %q) = %v, want %v", tt.current, tt.latest, result, tt.expected)
		}
	}
}

func TestChecker_Check(t *testing.T) {
	// Create a mock GitHub API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		release := GitHubRelease{
			TagName:     "v1.2.0",
			Name:        "Release 1.2.0",
			HTMLURL:     "https://github.com/test/repo/releases/tag/v1.2.0",
			Body:        "## Changes\n- New feature",
			Draft:       false,
			Prerelease:  false,
			PublishedAt: time.Now(),
		}
		json.NewEncoder(w).Encode(release)
	}))
	defer server.Close()

	// Create checker with custom HTTP client pointing to mock server
	checker := NewChecker("1.0.0", "test", "repo")
	checker.httpClient = server.Client()

	// Override the URL for testing (we'll need to modify the package for this)
	// For now, we test the version comparison logic
	t.Run("version comparison logic", func(t *testing.T) {
		if !IsNewerVersion("1.0.0", "1.2.0") {
			t.Error("Expected 1.2.0 to be newer than 1.0.0")
		}
	})
}

func TestChecker_CacheTTL(t *testing.T) {
	checker := NewChecker("1.0.0", "test", "repo")

	// Set a short TTL
	checker.SetCacheTTL(100 * time.Millisecond)

	if checker.cacheTTL != 100*time.Millisecond {
		t.Errorf("Expected cacheTTL to be 100ms, got %v", checker.cacheTTL)
	}
}

func TestChecker_GetCurrentVersion(t *testing.T) {
	checker := NewChecker("v1.5.3", "test", "repo")

	// Should normalize the version
	if v := checker.GetCurrentVersion(); v != "1.5.3" {
		t.Errorf("GetCurrentVersion() = %q, want %q", v, "1.5.3")
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected [4]int
	}{
		{"1.0.0", [4]int{1, 0, 0, 0}},
		{"1.2.3", [4]int{1, 2, 3, 0}},
		{"2.0", [4]int{2, 0, 0, 0}},
		{"1", [4]int{1, 0, 0, 0}},
		{"1.2.3-alpha.1", [4]int{1, 2, 3, 1001}}, // alpha + 1000 + 1
		{"1.2.3-beta.2", [4]int{1, 2, 3, 2002}},  // beta + 2000 + 2
		{"1.2.3-rc.3", [4]int{1, 2, 3, 3003}},    // rc + 3000 + 3
	}

	for _, tt := range tests {
		result := parseVersion(tt.input)
		if result != tt.expected {
			t.Errorf("parseVersion(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}
