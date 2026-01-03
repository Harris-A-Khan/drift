package xcode

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// VersionInfo holds version information from Version.xcconfig.
type VersionInfo struct {
	MarketingVersion string // e.g., "1.2.3"
	BuildNumber      int    // e.g., 42
}

// ReadVersionFile reads version information from Version.xcconfig.
func ReadVersionFile(path string) (*VersionInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read version file: %w", err)
	}

	info := &VersionInfo{}
	content := string(data)

	// Parse CURRENT_PROJECT_VERSION (build number)
	buildRe := regexp.MustCompile(`CURRENT_PROJECT_VERSION\s*=\s*(\d+)`)
	if matches := buildRe.FindStringSubmatch(content); len(matches) > 1 {
		info.BuildNumber, _ = strconv.Atoi(matches[1])
	}

	// Parse MARKETING_VERSION
	versionRe := regexp.MustCompile(`MARKETING_VERSION\s*=\s*([^\s]+)`)
	if matches := versionRe.FindStringSubmatch(content); len(matches) > 1 {
		info.MarketingVersion = matches[1]
	}

	return info, nil
}

// WriteVersionFile writes version information to Version.xcconfig.
func WriteVersionFile(path string, info *VersionInfo) error {
	// Read existing content to preserve any additional settings
	existingContent := ""
	if data, err := os.ReadFile(path); err == nil {
		existingContent = string(data)
	}

	// Update or add CURRENT_PROJECT_VERSION
	buildRe := regexp.MustCompile(`CURRENT_PROJECT_VERSION\s*=\s*\d+`)
	newBuild := fmt.Sprintf("CURRENT_PROJECT_VERSION = %d", info.BuildNumber)

	if buildRe.MatchString(existingContent) {
		existingContent = buildRe.ReplaceAllString(existingContent, newBuild)
	} else {
		existingContent = strings.TrimSpace(existingContent) + "\n" + newBuild + "\n"
	}

	// Update or add MARKETING_VERSION if provided
	if info.MarketingVersion != "" {
		versionRe := regexp.MustCompile(`MARKETING_VERSION\s*=\s*[^\s]+`)
		newVersion := fmt.Sprintf("MARKETING_VERSION = %s", info.MarketingVersion)

		if versionRe.MatchString(existingContent) {
			existingContent = versionRe.ReplaceAllString(existingContent, newVersion)
		} else {
			existingContent = strings.TrimSpace(existingContent) + "\n" + newVersion + "\n"
		}
	}

	return os.WriteFile(path, []byte(existingContent), 0644)
}

// IncrementBuildNumber reads the version file, increments the build number, and writes it back.
func IncrementBuildNumber(path string) (int, error) {
	info, err := ReadVersionFile(path)
	if err != nil {
		// File might not exist, start from 1
		info = &VersionInfo{BuildNumber: 0}
	}

	info.BuildNumber++

	if err := WriteVersionFile(path, info); err != nil {
		return 0, err
	}

	return info.BuildNumber, nil
}

// SetBuildNumber sets the build number to a specific value.
func SetBuildNumber(path string, buildNumber int) error {
	info, err := ReadVersionFile(path)
	if err != nil {
		info = &VersionInfo{}
	}

	info.BuildNumber = buildNumber
	return WriteVersionFile(path, info)
}

// GetBuildNumber returns the current build number.
func GetBuildNumber(path string) (int, error) {
	info, err := ReadVersionFile(path)
	if err != nil {
		return 0, err
	}
	return info.BuildNumber, nil
}

// SetMarketingVersion sets the marketing version.
func SetMarketingVersion(path, version string) error {
	info, err := ReadVersionFile(path)
	if err != nil {
		info = &VersionInfo{}
	}

	info.MarketingVersion = version
	return WriteVersionFile(path, info)
}

// GetMarketingVersion returns the current marketing version.
func GetMarketingVersion(path string) (string, error) {
	info, err := ReadVersionFile(path)
	if err != nil {
		return "", err
	}
	return info.MarketingVersion, nil
}

// CreateVersionFile creates a new Version.xcconfig with initial values.
func CreateVersionFile(path string, marketingVersion string, buildNumber int) error {
	content := fmt.Sprintf(`// Version.xcconfig
// Managed by drift

MARKETING_VERSION = %s
CURRENT_PROJECT_VERSION = %d
`, marketingVersion, buildNumber)

	return os.WriteFile(path, []byte(content), 0644)
}

