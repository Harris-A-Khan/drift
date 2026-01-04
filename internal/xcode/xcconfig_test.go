package xcode

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/undrift/drift/internal/supabase"
)

func TestNewXcconfigGenerator(t *testing.T) {
	gen := NewXcconfigGenerator("/path/to/Config.xcconfig")
	if gen == nil {
		t.Error("NewXcconfigGenerator returned nil")
	}
	if gen.OutputPath != "/path/to/Config.xcconfig" {
		t.Errorf("expected OutputPath '/path/to/Config.xcconfig', got '%s'", gen.OutputPath)
	}
	if gen.BuildServerDir != "/path/to" {
		t.Errorf("expected BuildServerDir '/path/to', got '%s'", gen.BuildServerDir)
	}
}

func TestGenerate(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "Config.xcconfig")

	gen := NewXcconfigGenerator(configPath)

	data := XcconfigData{
		GitBranch:      "feature/test",
		Environment:    "Feature",
		SupabaseBranch: "feature-test",
		ProjectRef:     "abcdefghij",
		APIURL:         "https://abcdefghij.supabase.co",
		AnonKey:        "test-anon-key-123",
		IsFallback:     false,
		GeneratedAt:    time.Now(),
	}

	if err := gen.Generate(data); err != nil {
		t.Errorf("Generate failed: %v", err)
	}

	// Verify file was created
	if !XcconfigExists(configPath) {
		t.Error("Config file was not created")
	}

	// Verify content
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Errorf("failed to read generated file: %v", err)
	}

	contentStr := string(content)
	expectedStrings := []string{
		"SUPABASE_URL = https://abcdefghij.supabase.co",
		"SUPABASE_ANON_KEY = test-anon-key-123",
		"SUPABASE_PROJECT_REF = abcdefghij",
		"DRIFT_ENVIRONMENT = Feature",
		"DRIFT_GIT_BRANCH = feature/test",
		"DRIFT_SUPABASE_BRANCH = feature-test",
	}

	for _, expected := range expectedStrings {
		if !containsString(contentStr, expected) {
			t.Errorf("expected '%s' in generated config", expected)
		}
	}
}

func TestGenerateWithFallback(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "Config.xcconfig")

	gen := NewXcconfigGenerator(configPath)

	data := XcconfigData{
		GitBranch:      "some-branch",
		Environment:    "Development",
		SupabaseBranch: "development",
		ProjectRef:     "xyz123",
		APIURL:         "https://xyz123.supabase.co",
		AnonKey:        "anon-key",
		IsFallback:     true,
		GeneratedAt:    time.Now(),
	}

	if err := gen.Generate(data); err != nil {
		t.Errorf("Generate failed: %v", err)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Errorf("failed to read generated file: %v", err)
	}

	if !containsString(string(content), "Using fallback") {
		t.Error("expected fallback note in generated config")
	}
}

func TestGenerateFromBranchInfo(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "Config.xcconfig")

	gen := NewXcconfigGenerator(configPath)

	info := &supabase.BranchInfo{
		GitBranch: "main",
		SupabaseBranch: &supabase.Branch{
			Name:      "main",
			IsDefault: true,
		},
		ProjectRef:  "prodref123",
		APIURL:      "https://prodref123.supabase.co",
		Environment: supabase.EnvProduction,
		IsFallback:  false,
	}

	if err := gen.GenerateFromBranchInfo(info, "prod-anon-key"); err != nil {
		t.Errorf("GenerateFromBranchInfo failed: %v", err)
	}

	// Verify file was created
	if !XcconfigExists(configPath) {
		t.Error("Config file was not created")
	}

	env, err := GetCurrentEnvironment(configPath)
	if err != nil {
		t.Errorf("GetCurrentEnvironment failed: %v", err)
	}
	if env != "Production" {
		t.Errorf("expected environment 'Production', got '%s'", env)
	}
}

func TestReadXcconfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "Config.xcconfig")

	content := `// This is a comment
SUPABASE_URL = https://example.supabase.co
SUPABASE_ANON_KEY = my-anon-key
DRIFT_ENVIRONMENT = Development

// Another comment
CUSTOM_KEY = custom value
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	values, err := ReadXcconfig(configPath)
	if err != nil {
		t.Errorf("ReadXcconfig failed: %v", err)
	}

	if values["SUPABASE_URL"] != "https://example.supabase.co" {
		t.Errorf("expected SUPABASE_URL 'https://example.supabase.co', got '%s'", values["SUPABASE_URL"])
	}
	if values["SUPABASE_ANON_KEY"] != "my-anon-key" {
		t.Errorf("expected SUPABASE_ANON_KEY 'my-anon-key', got '%s'", values["SUPABASE_ANON_KEY"])
	}
	if values["DRIFT_ENVIRONMENT"] != "Development" {
		t.Errorf("expected DRIFT_ENVIRONMENT 'Development', got '%s'", values["DRIFT_ENVIRONMENT"])
	}
	if values["CUSTOM_KEY"] != "custom value" {
		t.Errorf("expected CUSTOM_KEY 'custom value', got '%s'", values["CUSTOM_KEY"])
	}
}

func TestReadXcconfigNonExistent(t *testing.T) {
	_, err := ReadXcconfig("/nonexistent/path/Config.xcconfig")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestGetCurrentEnvironment(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "Config.xcconfig")

	// Test 1: File with DRIFT_ENVIRONMENT
	content := `DRIFT_ENVIRONMENT = Production`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	env, err := GetCurrentEnvironment(configPath)
	if err != nil {
		t.Errorf("GetCurrentEnvironment failed: %v", err)
	}
	if env != "Production" {
		t.Errorf("expected 'Production', got '%s'", env)
	}

	// Test 2: File without DRIFT_ENVIRONMENT
	content = `SUPABASE_URL = https://example.supabase.co`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err = GetCurrentEnvironment(configPath)
	if err == nil {
		t.Error("expected error when DRIFT_ENVIRONMENT is missing")
	}
}

func TestXcconfigExists(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "Config.xcconfig")

	// Test 1: Non-existent file
	if XcconfigExists(configPath) {
		t.Error("expected false for non-existent file")
	}

	// Test 2: Create file
	if err := os.WriteFile(configPath, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	if !XcconfigExists(configPath) {
		t.Error("expected true for existing file")
	}
}

func TestGenerateCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	nestedPath := filepath.Join(tmpDir, "subdir", "nested", "Config.xcconfig")

	gen := NewXcconfigGenerator(nestedPath)

	data := XcconfigData{
		GitBranch:      "main",
		Environment:    "Production",
		SupabaseBranch: "main",
		ProjectRef:     "ref123",
		APIURL:         "https://ref123.supabase.co",
		AnonKey:        "key",
		GeneratedAt:    time.Now(),
	}

	if err := gen.Generate(data); err != nil {
		t.Errorf("Generate failed: %v", err)
	}

	// Verify nested directories were created
	if !XcconfigExists(nestedPath) {
		t.Error("Config file was not created in nested directory")
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
