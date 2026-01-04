package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/undrift/drift/internal/config"
	"github.com/undrift/drift/internal/supabase"
)

func TestEnvColorString(t *testing.T) {
	tests := []struct {
		env      string
		expected string
	}{
		{"Production", "Production"},
		{"Development", "Development"},
		{"Feature", "Feature"},
		{"feature-test", "feature-test"},
	}

	for _, tt := range tests {
		t.Run(tt.env, func(t *testing.T) {
			result := envColorString(tt.env)
			// Result should contain the original text
			if !containsString(result, tt.expected) {
				t.Errorf("envColorString(%s) should contain '%s', got '%s'", tt.env, tt.expected, result)
			}
		})
	}
}

func TestGetSchemeForEnvironment(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()

	// Create a fake Xcode project structure
	xcodeProj := filepath.Join(tmpDir, "TestApp.xcodeproj")
	if err := os.MkdirAll(xcodeProj, 0755); err != nil {
		t.Fatalf("failed to create xcodeproj: %v", err)
	}

	// Test 1: Config with explicit scheme mapping
	cfg := &config.Config{
		Xcode: config.XcodeConfig{
			Schemes: map[string]string{
				"production":  "TestApp-Prod",
				"development": "TestApp-Dev",
				"feature":     "TestApp-Feature",
			},
		},
		Project: config.ProjectConfig{
			Name: "TestApp",
		},
	}

	prodInfo := &supabase.BranchInfo{
		Environment: supabase.EnvProduction,
	}

	scheme := getSchemeForEnvironment(cfg, prodInfo)
	if scheme != "TestApp-Prod" {
		t.Errorf("expected 'TestApp-Prod', got '%s'", scheme)
	}

	devInfo := &supabase.BranchInfo{
		Environment: supabase.EnvDevelopment,
	}

	scheme = getSchemeForEnvironment(cfg, devInfo)
	if scheme != "TestApp-Dev" {
		t.Errorf("expected 'TestApp-Dev', got '%s'", scheme)
	}

	featureInfo := &supabase.BranchInfo{
		Environment: supabase.EnvFeature,
	}

	scheme = getSchemeForEnvironment(cfg, featureInfo)
	if scheme != "TestApp-Feature" {
		t.Errorf("expected 'TestApp-Feature', got '%s'", scheme)
	}

	// Test 2: Config without scheme mapping but with project name
	cfg2 := &config.Config{
		Xcode: config.XcodeConfig{},
		Project: config.ProjectConfig{
			Name: "MyApp",
		},
	}

	// Should fall back to project name
	scheme = getSchemeForEnvironment(cfg2, prodInfo)
	if scheme != "MyApp" {
		t.Errorf("expected 'MyApp', got '%s'", scheme)
	}
}

func TestSchemeExists(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// Create a fake xcscheme file
	schemeDir := filepath.Join(tmpDir, "TestApp.xcodeproj", "xcshareddata", "xcschemes")
	if err := os.MkdirAll(schemeDir, 0755); err != nil {
		t.Fatalf("failed to create scheme dir: %v", err)
	}

	schemeFile := filepath.Join(schemeDir, "TestScheme.xcscheme")
	if err := os.WriteFile(schemeFile, []byte("<xml/>"), 0644); err != nil {
		t.Fatalf("failed to create scheme file: %v", err)
	}

	// Test 1: Existing scheme
	if !schemeExists("TestScheme") {
		t.Error("schemeExists should return true for existing scheme")
	}

	// Test 2: Non-existent scheme
	if schemeExists("NonExistentScheme") {
		t.Error("schemeExists should return false for non-existent scheme")
	}
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
