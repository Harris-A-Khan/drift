package xcode

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadVersionFile(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	versionFile := filepath.Join(tmpDir, "Version.xcconfig")

	// Test 1: Read valid version file
	content := `// Version.xcconfig
MARKETING_VERSION = 1.2.3
CURRENT_PROJECT_VERSION = 42
`
	if err := os.WriteFile(versionFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	info, err := ReadVersionFile(versionFile)
	if err != nil {
		t.Errorf("ReadVersionFile failed: %v", err)
	}
	if info.MarketingVersion != "1.2.3" {
		t.Errorf("expected MarketingVersion '1.2.3', got '%s'", info.MarketingVersion)
	}
	if info.BuildNumber != 42 {
		t.Errorf("expected BuildNumber 42, got %d", info.BuildNumber)
	}

	// Test 2: Read file with different spacing
	content = `MARKETING_VERSION=2.0.0
CURRENT_PROJECT_VERSION=100`
	if err := os.WriteFile(versionFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	info, err = ReadVersionFile(versionFile)
	if err != nil {
		t.Errorf("ReadVersionFile failed: %v", err)
	}
	if info.MarketingVersion != "2.0.0" {
		t.Errorf("expected MarketingVersion '2.0.0', got '%s'", info.MarketingVersion)
	}
	if info.BuildNumber != 100 {
		t.Errorf("expected BuildNumber 100, got %d", info.BuildNumber)
	}

	// Test 3: Read non-existent file
	_, err = ReadVersionFile(filepath.Join(tmpDir, "nonexistent.xcconfig"))
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestWriteVersionFile(t *testing.T) {
	tmpDir := t.TempDir()
	versionFile := filepath.Join(tmpDir, "Version.xcconfig")

	// Test 1: Write to new file
	info := &VersionInfo{
		MarketingVersion: "1.0.0",
		BuildNumber:      1,
	}

	if err := WriteVersionFile(versionFile, info); err != nil {
		t.Errorf("WriteVersionFile failed: %v", err)
	}

	// Verify written content
	readInfo, err := ReadVersionFile(versionFile)
	if err != nil {
		t.Errorf("failed to read written file: %v", err)
	}
	if readInfo.MarketingVersion != "1.0.0" {
		t.Errorf("expected MarketingVersion '1.0.0', got '%s'", readInfo.MarketingVersion)
	}
	if readInfo.BuildNumber != 1 {
		t.Errorf("expected BuildNumber 1, got %d", readInfo.BuildNumber)
	}

	// Test 2: Update existing file
	info.BuildNumber = 2
	info.MarketingVersion = "1.1.0"
	if err := WriteVersionFile(versionFile, info); err != nil {
		t.Errorf("WriteVersionFile failed on update: %v", err)
	}

	readInfo, err = ReadVersionFile(versionFile)
	if err != nil {
		t.Errorf("failed to read updated file: %v", err)
	}
	if readInfo.MarketingVersion != "1.1.0" {
		t.Errorf("expected updated MarketingVersion '1.1.0', got '%s'", readInfo.MarketingVersion)
	}
	if readInfo.BuildNumber != 2 {
		t.Errorf("expected updated BuildNumber 2, got %d", readInfo.BuildNumber)
	}
}

func TestIncrementBuildNumber(t *testing.T) {
	tmpDir := t.TempDir()
	versionFile := filepath.Join(tmpDir, "Version.xcconfig")

	// Test 1: Increment from non-existent file (should start at 1)
	newBuild, err := IncrementBuildNumber(versionFile)
	if err != nil {
		t.Errorf("IncrementBuildNumber failed: %v", err)
	}
	if newBuild != 1 {
		t.Errorf("expected new build 1, got %d", newBuild)
	}

	// Test 2: Increment existing build number
	newBuild, err = IncrementBuildNumber(versionFile)
	if err != nil {
		t.Errorf("IncrementBuildNumber failed: %v", err)
	}
	if newBuild != 2 {
		t.Errorf("expected new build 2, got %d", newBuild)
	}

	// Test 3: Increment again
	newBuild, err = IncrementBuildNumber(versionFile)
	if err != nil {
		t.Errorf("IncrementBuildNumber failed: %v", err)
	}
	if newBuild != 3 {
		t.Errorf("expected new build 3, got %d", newBuild)
	}
}

func TestSetBuildNumber(t *testing.T) {
	tmpDir := t.TempDir()
	versionFile := filepath.Join(tmpDir, "Version.xcconfig")

	// Set build number to specific value
	if err := SetBuildNumber(versionFile, 100); err != nil {
		t.Errorf("SetBuildNumber failed: %v", err)
	}

	build, err := GetBuildNumber(versionFile)
	if err != nil {
		t.Errorf("GetBuildNumber failed: %v", err)
	}
	if build != 100 {
		t.Errorf("expected build 100, got %d", build)
	}
}

func TestGetBuildNumber(t *testing.T) {
	tmpDir := t.TempDir()
	versionFile := filepath.Join(tmpDir, "Version.xcconfig")

	// Test 1: Non-existent file
	_, err := GetBuildNumber(versionFile)
	if err == nil {
		t.Error("expected error for non-existent file")
	}

	// Test 2: Valid file
	content := `CURRENT_PROJECT_VERSION = 55`
	if err := os.WriteFile(versionFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	build, err := GetBuildNumber(versionFile)
	if err != nil {
		t.Errorf("GetBuildNumber failed: %v", err)
	}
	if build != 55 {
		t.Errorf("expected build 55, got %d", build)
	}
}

func TestSetMarketingVersion(t *testing.T) {
	tmpDir := t.TempDir()
	versionFile := filepath.Join(tmpDir, "Version.xcconfig")

	// Set marketing version
	if err := SetMarketingVersion(versionFile, "2.5.0"); err != nil {
		t.Errorf("SetMarketingVersion failed: %v", err)
	}

	version, err := GetMarketingVersion(versionFile)
	if err != nil {
		t.Errorf("GetMarketingVersion failed: %v", err)
	}
	if version != "2.5.0" {
		t.Errorf("expected version '2.5.0', got '%s'", version)
	}
}

func TestGetMarketingVersion(t *testing.T) {
	tmpDir := t.TempDir()
	versionFile := filepath.Join(tmpDir, "Version.xcconfig")

	// Test 1: Non-existent file
	_, err := GetMarketingVersion(versionFile)
	if err == nil {
		t.Error("expected error for non-existent file")
	}

	// Test 2: Valid file
	content := `MARKETING_VERSION = 3.0.0-beta`
	if err := os.WriteFile(versionFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	version, err := GetMarketingVersion(versionFile)
	if err != nil {
		t.Errorf("GetMarketingVersion failed: %v", err)
	}
	if version != "3.0.0-beta" {
		t.Errorf("expected version '3.0.0-beta', got '%s'", version)
	}
}

func TestCreateVersionFile(t *testing.T) {
	tmpDir := t.TempDir()
	versionFile := filepath.Join(tmpDir, "Version.xcconfig")

	// Create version file
	if err := CreateVersionFile(versionFile, "1.0.0", 1); err != nil {
		t.Errorf("CreateVersionFile failed: %v", err)
	}

	// Verify content
	data, err := os.ReadFile(versionFile)
	if err != nil {
		t.Errorf("failed to read created file: %v", err)
	}

	content := string(data)
	if !contains(content, "MARKETING_VERSION = 1.0.0") {
		t.Error("expected MARKETING_VERSION in created file")
	}
	if !contains(content, "CURRENT_PROJECT_VERSION = 1") {
		t.Error("expected CURRENT_PROJECT_VERSION in created file")
	}
	if !contains(content, "Managed by drift") {
		t.Error("expected drift comment in created file")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsImpl(s, substr))
}

func containsImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
