package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig_HasExpectedValues(t *testing.T) {
	cfg := DefaultConfig()

	// Project defaults
	if cfg.Project.Type != "ios" {
		t.Errorf("DefaultConfig().Project.Type = %q, want %q", cfg.Project.Type, "ios")
	}

	// Supabase defaults
	if cfg.Supabase.ProjectRef != "" {
		t.Errorf("DefaultConfig().Supabase.ProjectRef = %q, want empty", cfg.Supabase.ProjectRef)
	}
	if cfg.Supabase.FunctionsDir != "supabase/functions" {
		t.Errorf("DefaultConfig().Supabase.FunctionsDir = %q, want %q", cfg.Supabase.FunctionsDir, "supabase/functions")
	}
	if cfg.Supabase.MigrationsDir != "supabase/migrations" {
		t.Errorf("DefaultConfig().Supabase.MigrationsDir = %q, want %q", cfg.Supabase.MigrationsDir, "supabase/migrations")
	}
	if len(cfg.Supabase.ProtectedBranches) != 2 {
		t.Errorf("DefaultConfig().Supabase.ProtectedBranches length = %d, want 2", len(cfg.Supabase.ProtectedBranches))
	}

	// Apple defaults
	if cfg.Apple.PushKeyPattern != "AuthKey_*.p8" {
		t.Errorf("DefaultConfig().Apple.PushKeyPattern = %q, want %q", cfg.Apple.PushKeyPattern, "AuthKey_*.p8")
	}
	if cfg.Apple.PushEnvironment != "development" {
		t.Errorf("DefaultConfig().Apple.PushEnvironment = %q, want %q", cfg.Apple.PushEnvironment, "development")
	}

	// Xcode defaults
	if cfg.Xcode.XcconfigOutput != "Config.xcconfig" {
		t.Errorf("DefaultConfig().Xcode.XcconfigOutput = %q, want %q", cfg.Xcode.XcconfigOutput, "Config.xcconfig")
	}
	if cfg.Xcode.VersionFile != "Version.xcconfig" {
		t.Errorf("DefaultConfig().Xcode.VersionFile = %q, want %q", cfg.Xcode.VersionFile, "Version.xcconfig")
	}

	// Database defaults
	if cfg.Database.PoolerPort != 6543 {
		t.Errorf("DefaultConfig().Database.PoolerPort = %d, want 6543", cfg.Database.PoolerPort)
	}
	if cfg.Database.DirectPort != 5432 {
		t.Errorf("DefaultConfig().Database.DirectPort = %d, want 5432", cfg.Database.DirectPort)
	}
	if cfg.Database.DumpFormat != "custom" {
		t.Errorf("DefaultConfig().Database.DumpFormat = %q, want %q", cfg.Database.DumpFormat, "custom")
	}
	if !cfg.Database.RequireSSL {
		t.Error("DefaultConfig().Database.RequireSSL = false, want true")
	}

	// Backup defaults
	if cfg.Backup.Provider != "supabase" {
		t.Errorf("DefaultConfig().Backup.Provider = %q, want %q", cfg.Backup.Provider, "supabase")
	}
	if cfg.Backup.RetentionDays != 30 {
		t.Errorf("DefaultConfig().Backup.RetentionDays = %d, want 30", cfg.Backup.RetentionDays)
	}

	// Worktree defaults
	if cfg.Worktree.NamingPattern != "{project}-{branch}" {
		t.Errorf("DefaultConfig().Worktree.NamingPattern = %q, want %q", cfg.Worktree.NamingPattern, "{project}-{branch}")
	}
	if !cfg.Worktree.AutoSetupXcconfig {
		t.Error("DefaultConfig().Worktree.AutoSetupXcconfig = false, want true")
	}
}

func TestMergeWithDefaults_FillsMissingValues(t *testing.T) {
	// Start with mostly empty config
	cfg := &Config{
		Project: ProjectConfig{
			Name: "my-app",
			// Type is empty, should be filled
		},
		Database: DatabaseConfig{
			PoolerHost: "custom.pooler.host",
			// Other fields empty, should be filled
		},
	}

	merged := MergeWithDefaults(cfg)

	// Should preserve custom values
	if merged.Project.Name != "my-app" {
		t.Errorf("MergeWithDefaults() should preserve Project.Name, got %q", merged.Project.Name)
	}
	if merged.Database.PoolerHost != "custom.pooler.host" {
		t.Errorf("MergeWithDefaults() should preserve Database.PoolerHost, got %q", merged.Database.PoolerHost)
	}

	// Should fill defaults
	if merged.Project.Type != "ios" {
		t.Errorf("MergeWithDefaults() should fill Project.Type, got %q", merged.Project.Type)
	}
	if merged.Supabase.FunctionsDir != "supabase/functions" {
		t.Errorf("MergeWithDefaults() should fill Supabase.FunctionsDir, got %q", merged.Supabase.FunctionsDir)
	}
	if merged.Database.PoolerPort != 6543 {
		t.Errorf("MergeWithDefaults() should fill Database.PoolerPort, got %d", merged.Database.PoolerPort)
	}
}

func TestMergeWithDefaults_PreservesExistingValues(t *testing.T) {
	cfg := &Config{
		Project: ProjectConfig{
			Name: "my-app",
			Type: "macos",
		},
		Supabase: SupabaseConfig{
			FunctionsDir:      "custom/functions",
			MigrationsDir:     "custom/migrations",
			ProtectedBranches: []string{"production", "staging"},
		},
		Database: DatabaseConfig{
			PoolerPort: 9999,
			DirectPort: 8888,
		},
		Backup: BackupConfig{
			Provider:      "s3",
			RetentionDays: 60,
		},
	}

	merged := MergeWithDefaults(cfg)

	// All custom values should be preserved
	if merged.Project.Type != "macos" {
		t.Errorf("MergeWithDefaults() should preserve Type, got %q", merged.Project.Type)
	}
	if merged.Supabase.FunctionsDir != "custom/functions" {
		t.Errorf("MergeWithDefaults() should preserve FunctionsDir, got %q", merged.Supabase.FunctionsDir)
	}
	if len(merged.Supabase.ProtectedBranches) != 2 || merged.Supabase.ProtectedBranches[0] != "production" {
		t.Errorf("MergeWithDefaults() should preserve ProtectedBranches, got %v", merged.Supabase.ProtectedBranches)
	}
	if merged.Database.PoolerPort != 9999 {
		t.Errorf("MergeWithDefaults() should preserve PoolerPort, got %d", merged.Database.PoolerPort)
	}
	if merged.Backup.Provider != "s3" {
		t.Errorf("MergeWithDefaults() should preserve Provider, got %q", merged.Backup.Provider)
	}
	if merged.Backup.RetentionDays != 60 {
		t.Errorf("MergeWithDefaults() should preserve RetentionDays, got %d", merged.Backup.RetentionDays)
	}
}

func TestLoadFromPath_ValidConfig(t *testing.T) {
	// Create a temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".drift.yaml")

	content := `
project:
  name: test-app
  type: macos

supabase:
  functions_dir: my/functions
  protected_branches:
    - main
    - production

database:
  pooler_port: 7777
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("LoadFromPath() error = %v", err)
	}

	if cfg.Project.Name != "test-app" {
		t.Errorf("LoadFromPath() Project.Name = %q, want %q", cfg.Project.Name, "test-app")
	}
	if cfg.Project.Type != "macos" {
		t.Errorf("LoadFromPath() Project.Type = %q, want %q", cfg.Project.Type, "macos")
	}
	if cfg.Supabase.FunctionsDir != "my/functions" {
		t.Errorf("LoadFromPath() Supabase.FunctionsDir = %q, want %q", cfg.Supabase.FunctionsDir, "my/functions")
	}
	if cfg.Database.PoolerPort != 7777 {
		t.Errorf("LoadFromPath() Database.PoolerPort = %d, want 7777", cfg.Database.PoolerPort)
	}
	// Should still have defaults merged
	if cfg.Database.DirectPort != 5432 {
		t.Errorf("LoadFromPath() should merge defaults, Database.DirectPort = %d, want 5432", cfg.Database.DirectPort)
	}
}

func TestLoadFromPath_NonExistent(t *testing.T) {
	_, err := LoadFromPath("/this/path/does/not/exist/.drift.yaml")
	if err == nil {
		t.Error("LoadFromPath() expected error for non-existent file")
	}
}

func TestLoadFromPath_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".drift.yaml")

	// Invalid YAML
	content := `
project:
  name: [invalid yaml
  type: :::
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	_, err := LoadFromPath(configPath)
	if err == nil {
		t.Error("LoadFromPath() expected error for invalid YAML")
	}
}

func TestLoadFromPath_SetsConfigPath(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".drift.yaml")

	content := `project:
  name: test
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("LoadFromPath() error = %v", err)
	}

	if cfg.ConfigPath() != configPath {
		t.Errorf("ConfigPath() = %q, want %q", cfg.ConfigPath(), configPath)
	}
}

func TestConfig_ProjectRoot(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".drift.yaml")

	content := `project:
  name: test
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("LoadFromPath() error = %v", err)
	}

	expectedRoot, _ := filepath.EvalSymlinks(tmpDir)
	actualRoot, _ := filepath.EvalSymlinks(cfg.ProjectRoot())

	if actualRoot != expectedRoot {
		t.Errorf("ProjectRoot() = %q, want %q", actualRoot, expectedRoot)
	}
}

func TestConfig_ProjectRoot_NoConfigPath(t *testing.T) {
	cfg := &Config{}

	// Should fall back to current directory
	root := cfg.ProjectRoot()
	cwd, _ := os.Getwd()

	if root != cwd {
		t.Errorf("ProjectRoot() with no config path = %q, want cwd %q", root, cwd)
	}
}

func TestConfig_IsProtectedBranch(t *testing.T) {
	cfg := &Config{
		Supabase: SupabaseConfig{
			ProtectedBranches: []string{"main", "master", "production"},
		},
	}

	tests := []struct {
		branch   string
		expected bool
	}{
		{"main", true},
		{"master", true},
		{"production", true},
		{"feature/test", false},
		{"develop", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.branch, func(t *testing.T) {
			result := cfg.IsProtectedBranch(tt.branch)
			if result != tt.expected {
				t.Errorf("IsProtectedBranch(%q) = %v, want %v", tt.branch, result, tt.expected)
			}
		})
	}
}

func TestConfig_GetFunctionsPath(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".drift.yaml")

	content := `project:
  name: test
supabase:
  functions_dir: supabase/functions
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("LoadFromPath() error = %v", err)
	}

	expected := filepath.Join(tmpDir, "supabase/functions")
	if cfg.GetFunctionsPath() != expected {
		t.Errorf("GetFunctionsPath() = %q, want %q", cfg.GetFunctionsPath(), expected)
	}
}

func TestConfig_GetMigrationsPath(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".drift.yaml")

	content := `project:
  name: test
supabase:
  migrations_dir: db/migrations
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("LoadFromPath() error = %v", err)
	}

	expected := filepath.Join(tmpDir, "db/migrations")
	if cfg.GetMigrationsPath() != expected {
		t.Errorf("GetMigrationsPath() = %q, want %q", cfg.GetMigrationsPath(), expected)
	}
}

func TestConfig_GetXcconfigPath(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".drift.yaml")

	content := `project:
  name: test
xcode:
  xcconfig_output: Build/Config.xcconfig
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("LoadFromPath() error = %v", err)
	}

	expected := filepath.Join(tmpDir, "Build/Config.xcconfig")
	if cfg.GetXcconfigPath() != expected {
		t.Errorf("GetXcconfigPath() = %q, want %q", cfg.GetXcconfigPath(), expected)
	}
}

func TestConfig_GetVersionFilePath(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".drift.yaml")

	content := `project:
  name: test
xcode:
  version_file: Build/Version.xcconfig
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("LoadFromPath() error = %v", err)
	}

	expected := filepath.Join(tmpDir, "Build/Version.xcconfig")
	if cfg.GetVersionFilePath() != expected {
		t.Errorf("GetVersionFilePath() = %q, want %q", cfg.GetVersionFilePath(), expected)
	}
}

func TestFindConfigFile_InCurrentDir(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".drift.yaml")

	if err := os.WriteFile(configPath, []byte("project:\n  name: test\n"), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Change to temp directory
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	found, err := FindConfigFile()
	if err != nil {
		t.Fatalf("FindConfigFile() error = %v", err)
	}

	// Resolve symlinks for comparison (macOS /var -> /private/var)
	foundResolved, _ := filepath.EvalSymlinks(found)
	expectedResolved, _ := filepath.EvalSymlinks(configPath)

	if foundResolved != expectedResolved {
		t.Errorf("FindConfigFile() = %q, want %q", foundResolved, expectedResolved)
	}
}

func TestFindConfigFile_InParentDir(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "sub", "dir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	configPath := filepath.Join(tmpDir, ".drift.yaml")
	if err := os.WriteFile(configPath, []byte("project:\n  name: test\n"), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Change to subdirectory
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(subDir)

	found, err := FindConfigFile()
	if err != nil {
		t.Fatalf("FindConfigFile() error = %v", err)
	}

	foundAbs, _ := filepath.Abs(found)
	expectedAbs, _ := filepath.Abs(configPath)
	foundResolved, _ := filepath.EvalSymlinks(foundAbs)
	expectedResolved, _ := filepath.EvalSymlinks(expectedAbs)

	if foundResolved != expectedResolved {
		t.Errorf("FindConfigFile() = %q, want %q", foundResolved, expectedResolved)
	}
}

func TestFindConfigFile_YmlExtension(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".drift.yml")

	if err := os.WriteFile(configPath, []byte("project:\n  name: test\n"), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Change to temp directory
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	found, err := FindConfigFile()
	if err != nil {
		t.Fatalf("FindConfigFile() error = %v", err)
	}

	// Resolve symlinks for comparison (macOS /var -> /private/var)
	foundResolved, _ := filepath.EvalSymlinks(found)
	expectedResolved, _ := filepath.EvalSymlinks(configPath)

	if foundResolved != expectedResolved {
		t.Errorf("FindConfigFile() with .yml = %q, want %q", foundResolved, expectedResolved)
	}
}

func TestFindConfigFile_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	// Change to empty temp directory
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	_, err := FindConfigFile()
	if err == nil {
		t.Error("FindConfigFile() expected error when no config exists")
	}
}

func TestExists_True(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".drift.yaml")

	if err := os.WriteFile(configPath, []byte("project:\n  name: test\n"), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	if !Exists() {
		t.Error("Exists() = false, want true")
	}
}

func TestExists_False(t *testing.T) {
	tmpDir := t.TempDir()

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	if Exists() {
		t.Error("Exists() = true, want false")
	}
}

func TestLoadOrDefault_WithConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".drift.yaml")

	content := `project:
  name: my-custom-app
  type: macos
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	cfg := LoadOrDefault()

	if cfg.Project.Name != "my-custom-app" {
		t.Errorf("LoadOrDefault() should load config, got Project.Name = %q", cfg.Project.Name)
	}
	if cfg.Project.Type != "macos" {
		t.Errorf("LoadOrDefault() should load config, got Project.Type = %q", cfg.Project.Type)
	}
}

func TestLoadOrDefault_WithoutConfig(t *testing.T) {
	tmpDir := t.TempDir()

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	cfg := LoadOrDefault()

	// Should return defaults
	if cfg.Project.Type != "ios" {
		t.Errorf("LoadOrDefault() without config should return defaults, got Project.Type = %q", cfg.Project.Type)
	}
	if cfg.Supabase.FunctionsDir != "supabase/functions" {
		t.Errorf("LoadOrDefault() without config should return defaults, got FunctionsDir = %q", cfg.Supabase.FunctionsDir)
	}
}

func TestConfig_NestedStructures(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".drift.yaml")

	content := `
project:
  name: nested-test
xcode:
  schemes:
    production: MyApp-Prod
    development: MyApp-Dev
    feature: MyApp-Feature
worktree:
  copy_on_create:
    - .env
    - .env.local
    - "*.p8"
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("LoadFromPath() error = %v", err)
	}

	// Check map
	if cfg.Xcode.Schemes["production"] != "MyApp-Prod" {
		t.Errorf("Schemes[production] = %q, want %q", cfg.Xcode.Schemes["production"], "MyApp-Prod")
	}
	if cfg.Xcode.Schemes["development"] != "MyApp-Dev" {
		t.Errorf("Schemes[development] = %q, want %q", cfg.Xcode.Schemes["development"], "MyApp-Dev")
	}

	// Check slice
	if len(cfg.Worktree.CopyOnCreate) != 3 {
		t.Errorf("CopyOnCreate length = %d, want 3", len(cfg.Worktree.CopyOnCreate))
	}
	if cfg.Worktree.CopyOnCreate[0] != ".env" {
		t.Errorf("CopyOnCreate[0] = %q, want %q", cfg.Worktree.CopyOnCreate[0], ".env")
	}
}
