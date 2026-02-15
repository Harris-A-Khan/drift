package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// LocalConfig represents the .drift.local.yaml configuration file (gitignored).
// This file contains branch/developer-specific overrides.
type LocalConfig struct {
	Supabase     LocalSupabaseConfig          `yaml:"supabase" mapstructure:"supabase"`
	Apple        LocalAppleConfig             `yaml:"apple" mapstructure:"apple"`
	Device       LocalDeviceConfig            `yaml:"device" mapstructure:"device"`
	Environments map[string]EnvironmentConfig `yaml:"environments" mapstructure:"environments"`
	Preferences  PreferencesConfig            `yaml:"preferences" mapstructure:"preferences"`
}

// LocalSupabaseConfig holds local Supabase overrides.
type LocalSupabaseConfig struct {
	OverrideBranch string `yaml:"override_branch" mapstructure:"override_branch"`
	FallbackBranch string `yaml:"fallback_branch" mapstructure:"fallback_branch"`
}

// LocalAppleConfig holds local Apple/APNs overrides.
type LocalAppleConfig struct {
	KeySearchPaths []string `yaml:"key_search_paths" mapstructure:"key_search_paths"`
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
	if local.Supabase.FallbackBranch != "" {
		main.Supabase.FallbackBranch = local.Supabase.FallbackBranch
	}

	// Override Apple settings
	if len(local.Apple.KeySearchPaths) > 0 {
		main.Apple.KeySearchPaths = local.Apple.KeySearchPaths
	}

	// Override Device settings
	if local.Device.DefaultDevice != "" {
		main.Device.DefaultDevice = local.Device.DefaultDevice
	}

	// Merge per-environment local secrets and push key overrides
	if len(local.Environments) > 0 {
		if main.Environments == nil {
			main.Environments = make(map[string]EnvironmentConfig)
		}

		for envName, localEnv := range local.Environments {
			mainEnv := main.Environments[envName]
			if localEnv.PushKey != "" {
				mainEnv.PushKey = localEnv.PushKey
			}
			if len(localEnv.Secrets) > 0 {
				if mainEnv.Secrets == nil {
					mainEnv.Secrets = make(map[string]string)
				}
				for key, value := range localEnv.Secrets {
					mainEnv.Secrets[key] = value
				}
			}
			if len(localEnv.SkipSecrets) > 0 {
				seen := make(map[string]bool)
				merged := make([]string, 0, len(mainEnv.SkipSecrets)+len(localEnv.SkipSecrets))
				for _, key := range mainEnv.SkipSecrets {
					if key == "" || seen[key] {
						continue
					}
					seen[key] = true
					merged = append(merged, key)
				}
				for _, key := range localEnv.SkipSecrets {
					if key == "" || seen[key] {
						continue
					}
					seen[key] = true
					merged = append(merged, key)
				}
				mainEnv.SkipSecrets = merged
			}
			main.Environments[envName] = mainEnv
		}
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
#   fallback_branch: "development"       # Used when no matching branch exists

# Local APNs key search overrides
# apple:
#   key_search_paths:
#     - "secrets"        # Relative to project root
#     - "../shared-keys" # Relative path outside project
#     - "/abs/path/to/keys" # Absolute path

# Local secret values (keep sensitive values out of .drift.yaml)
# environments:
#   development:
#     secrets:
#       ENABLE_DEBUG_SWITCH: "true"
#       API_BASE_URL: "https://dev-api.example.com"
#   production:
#     skip_secrets:
#       - ENABLE_DEBUG_SWITCH

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

// UpdateLocalSupabaseOverrides updates Supabase override fields in .drift.local.yaml.
func UpdateLocalSupabaseOverrides(localPath, overrideBranch, fallbackBranch string) error {
	cfg := make(map[string]interface{})

	// Read existing content if present.
	if data, err := os.ReadFile(localPath); err == nil && strings.TrimSpace(string(data)) != "" {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return err
		}
	}

	supabaseSection, ok := cfg["supabase"].(map[string]interface{})
	if !ok {
		supabaseSection = make(map[string]interface{})
		cfg["supabase"] = supabaseSection
	}

	if overrideBranch != "" {
		supabaseSection["override_branch"] = overrideBranch
	}
	if fallbackBranch != "" {
		supabaseSection["fallback_branch"] = fallbackBranch
	}

	newData, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(localPath, newData, 0644)
}

// ClearLocalSupabaseOverride removes supabase.override_branch from .drift.local.yaml.
func ClearLocalSupabaseOverride(localPath string) error {
	cfg := make(map[string]interface{})

	if data, err := os.ReadFile(localPath); err == nil && strings.TrimSpace(string(data)) != "" {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return err
		}
	}

	supabaseSection, ok := cfg["supabase"].(map[string]interface{})
	if !ok {
		return nil
	}

	delete(supabaseSection, "override_branch")
	if len(supabaseSection) == 0 {
		delete(cfg, "supabase")
	}

	newData, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(localPath, newData, 0644)
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
