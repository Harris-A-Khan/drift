package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// LocalConfig represents the .drift.local.yaml configuration file (gitignored).
// This file contains branch/developer-specific overrides.
type LocalConfig struct {
	Supabase    LocalSupabaseConfig    `yaml:"supabase" mapstructure:"supabase"`
	Device      LocalDeviceConfig      `yaml:"device" mapstructure:"device"`
	Preferences PreferencesConfig      `yaml:"preferences" mapstructure:"preferences"`
}

// LocalSupabaseConfig holds local Supabase overrides.
type LocalSupabaseConfig struct {
	OverrideBranch string `yaml:"override_branch" mapstructure:"override_branch"`
}

// LocalDeviceConfig holds local device overrides.
type LocalDeviceConfig struct {
	DefaultDevice string `yaml:"default_device" mapstructure:"default_device"`
}

// PreferencesConfig holds developer preferences.
type PreferencesConfig struct {
	Verbose          bool   `yaml:"verbose" mapstructure:"verbose"`
	Editor           string `yaml:"editor" mapstructure:"editor"`
	AutoOpenWorktree bool   `yaml:"auto_open_worktree" mapstructure:"auto_open_worktree"`
}

// LocalConfigFilename is the name of the local config file.
const LocalConfigFilename = ".drift.local.yaml"

// LoadLocal loads the local config from the same directory as the main config.
func LoadLocal(mainConfigPath string) (*LocalConfig, error) {
	dir := filepath.Dir(mainConfigPath)
	localPath := filepath.Join(dir, LocalConfigFilename)

	return LoadLocalFromPath(localPath)
}

// LoadLocalFromPath loads local configuration from a specific path.
func LoadLocalFromPath(localPath string) (*LocalConfig, error) {
	// Check if file exists
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		// Return empty config if no local config exists
		return &LocalConfig{}, nil
	}

	v := viper.New()
	v.SetConfigFile(localPath)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read local config file: %w", err)
	}

	var cfg LocalConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse local config file: %w", err)
	}

	return &cfg, nil
}

// FindLocalConfigFile finds the local config file in the same directory as the main config.
func FindLocalConfigFile() (string, error) {
	mainConfigPath, err := FindConfigFile()
	if err != nil {
		return "", err
	}

	dir := filepath.Dir(mainConfigPath)
	return filepath.Join(dir, LocalConfigFilename), nil
}

// LocalConfigExists checks if a .drift.local.yaml file exists.
func LocalConfigExists() bool {
	localPath, err := FindLocalConfigFile()
	if err != nil {
		return false
	}
	_, err = os.Stat(localPath)
	return err == nil
}

// MergeLocalConfig merges the local config into the main config.
// Local config values override main config values.
func MergeLocalConfig(main *Config, local *LocalConfig) *Config {
	if local == nil {
		return main
	}

	// Override Supabase settings
	if local.Supabase.OverrideBranch != "" {
		main.Supabase.OverrideBranch = local.Supabase.OverrideBranch
	}

	// Override Device settings
	if local.Device.DefaultDevice != "" {
		main.Device.DefaultDevice = local.Device.DefaultDevice
	}

	// Store preferences in config
	main.Preferences = local.Preferences

	return main
}

// LoadWithLocal loads both the main config and local config, merging them.
func LoadWithLocal() (*Config, error) {
	// Load main config
	mainConfigPath, err := FindConfigFile()
	if err != nil {
		return nil, err
	}

	cfg, err := LoadFromPath(mainConfigPath)
	if err != nil {
		return nil, err
	}

	// Load local config
	local, err := LoadLocal(mainConfigPath)
	if err != nil {
		// Non-fatal: warn but continue with main config only
		return cfg, nil
	}

	// Merge local into main
	return MergeLocalConfig(cfg, local), nil
}

// LoadOrDefaultWithLocal tries to load config with local overrides, returns defaults if not found.
func LoadOrDefaultWithLocal() *Config {
	cfg, err := LoadWithLocal()
	if err != nil {
		return DefaultConfig()
	}
	return cfg
}

// GenerateLocalConfigContent generates the content for .drift.local.yaml.
func GenerateLocalConfigContent() string {
	return `# .drift.local.yaml - Local/developer-specific configuration (GITIGNORED)
# This file contains settings that should NOT be committed to git.
# Each developer can have their own copy with personal preferences.

# Supabase branch overrides
# supabase:
#   override_branch: "feat-my-feature"  # Always use this Supabase branch

# Device preferences
# device:
#   default_device: "My iPhone"  # Your preferred test device

# Developer preferences
preferences:
  verbose: false                 # Show verbose output for all commands
  # editor: "cursor"             # Default editor for open commands
  # auto_open_worktree: true     # Open worktree in editor after create
`
}

// WriteLocalConfig writes a new local config file.
func WriteLocalConfig(path string) error {
	content := GenerateLocalConfigContent()
	return os.WriteFile(path, []byte(content), 0644)
}

// AddToGitignore adds .drift.local.yaml to .gitignore if not already present.
func AddToGitignore(projectRoot string) error {
	gitignorePath := filepath.Join(projectRoot, ".gitignore")

	// Read existing content
	content := ""
	if data, err := os.ReadFile(gitignorePath); err == nil {
		content = string(data)
	}

	// Check if already present
	if contains(content, LocalConfigFilename) {
		return nil // Already in gitignore
	}

	// Append to .gitignore
	newContent := content
	if newContent != "" && !endsWithNewline(newContent) {
		newContent += "\n"
	}
	newContent += "\n# Drift local config (developer-specific)\n"
	newContent += LocalConfigFilename + "\n"

	return os.WriteFile(gitignorePath, []byte(newContent), 0644)
}

// contains checks if a string contains a substring (line-aware).
func contains(content, substr string) bool {
	// Check for exact line match
	lines := splitLines(content)
	for _, line := range lines {
		if line == substr {
			return true
		}
	}
	return false
}

// splitLines splits content into lines.
func splitLines(content string) []string {
	var lines []string
	current := ""
	for _, c := range content {
		if c == '\n' {
			lines = append(lines, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

// endsWithNewline checks if content ends with a newline.
func endsWithNewline(content string) bool {
	return len(content) > 0 && content[len(content)-1] == '\n'
}
