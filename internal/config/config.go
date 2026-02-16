// Package config handles loading and managing drift configuration.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the .drift.yaml configuration file.
type Config struct {
	Project      ProjectConfig                `yaml:"project" mapstructure:"project"`
	Supabase     SupabaseConfig               `yaml:"supabase" mapstructure:"supabase"`
	Apple        AppleConfig                  `yaml:"apple" mapstructure:"apple"`
	Xcode        XcodeConfig                  `yaml:"xcode" mapstructure:"xcode"`
	Web          WebConfig                    `yaml:"web" mapstructure:"web"`
	Database     DatabaseConfig               `yaml:"database" mapstructure:"database"`
	Backup       BackupConfig                 `yaml:"backup" mapstructure:"backup"`
	Worktree     WorktreeConfig               `yaml:"worktree" mapstructure:"worktree"`
	Device       DeviceConfig                 `yaml:"device" mapstructure:"device"`
	Environments map[string]EnvironmentConfig `yaml:"environments" mapstructure:"environments"`

	// Preferences from .drift.local.yaml (merged at runtime)
	Preferences PreferencesConfig `yaml:"-" mapstructure:"-"`

	// Internal: path to the config file
	configPath string
}

// ProjectType constants.
const (
	ProjectTypeIOS           = "ios"
	ProjectTypeMacOS         = "macos"
	ProjectTypeMultiplatform = "multiplatform"
	ProjectTypeWeb           = "web"
)

const (
	// DefaultDatabasePoolerHost is the fallback pooler host when no branch-specific host is configured.
	DefaultDatabasePoolerHost = "aws-0-us-east-1.pooler.supabase.com"
	// DefaultDatabasePoolerPort is the default transaction pooler port.
	DefaultDatabasePoolerPort = 6543
	// DefaultDatabaseDirectPort is the default direct/session database port.
	DefaultDatabaseDirectPort = 5432
)

// ProjectConfig holds project-level configuration.
type ProjectConfig struct {
	Name string `yaml:"name" mapstructure:"name"`
	Type string `yaml:"type" mapstructure:"type"` // ios, macos, multiplatform, web
}

// IsApplePlatform returns true if this is an iOS, macOS, or multiplatform project.
func (p *ProjectConfig) IsApplePlatform() bool {
	return p.Type == ProjectTypeIOS || p.Type == ProjectTypeMacOS || p.Type == ProjectTypeMultiplatform
}

// IsWebPlatform returns true if this is a web project.
func (p *ProjectConfig) IsWebPlatform() bool {
	return p.Type == ProjectTypeWeb
}

// SupabaseConfig holds Supabase-related configuration.
type SupabaseConfig struct {
	ProjectRef        string            `yaml:"project_ref" mapstructure:"project_ref"`
	ProjectName       string            `yaml:"project_name" mapstructure:"project_name"`
	FunctionsDir      string            `yaml:"functions_dir" mapstructure:"functions_dir"`
	MigrationsDir     string            `yaml:"migrations_dir" mapstructure:"migrations_dir"`
	ProtectedBranches []string          `yaml:"protected_branches" mapstructure:"protected_branches"`
	OverrideBranch    string            `yaml:"override_branch" mapstructure:"override_branch"`
	FallbackBranch    string            `yaml:"fallback_branch" mapstructure:"fallback_branch"`
	SecretsToPush     []string          `yaml:"secrets_to_push" mapstructure:"secrets_to_push"`
	DefaultSecrets    map[string]string `yaml:"default_secrets" mapstructure:"default_secrets"`
	Functions         FunctionsConfig   `yaml:"functions" mapstructure:"functions"`
}

// FunctionsConfig holds Edge Functions configuration.
type FunctionsConfig struct {
	Restricted []FunctionRestriction `yaml:"restricted" mapstructure:"restricted"`
}

// FunctionRestriction defines a function that should be restricted in certain environments.
type FunctionRestriction struct {
	Name         string   `yaml:"name" mapstructure:"name"`
	Environments []string `yaml:"environments" mapstructure:"environments"` // Blocked in these envs
}

// EnvironmentConfig holds per-environment configuration.
type EnvironmentConfig struct {
	Secrets     map[string]string `yaml:"secrets" mapstructure:"secrets"`
	PushKey     string            `yaml:"push_key" mapstructure:"push_key"`
	SkipSecrets []string          `yaml:"skip_secrets" mapstructure:"skip_secrets"`
}

// GetTargetBranch returns the Supabase branch to use for a git branch.
// Priority: override_branch > git branch name mapping
// If override is set, always use it. Otherwise, use the git branch name directly.
func (c *SupabaseConfig) GetTargetBranch(gitBranch string) string {
	if c.OverrideBranch != "" {
		return c.OverrideBranch
	}
	return gitBranch
}

// HasOverride returns true if an override branch is configured.
func (c *SupabaseConfig) HasOverride() bool {
	return c.OverrideBranch != ""
}

// AppleConfig holds Apple Developer configuration for iOS/macOS projects.
type AppleConfig struct {
	TeamID          string   `yaml:"team_id" mapstructure:"team_id"`
	BundleID        string   `yaml:"bundle_id" mapstructure:"bundle_id"`
	PushKeyPattern  string   `yaml:"push_key_pattern" mapstructure:"push_key_pattern"`
	PushEnvironment string   `yaml:"push_environment" mapstructure:"push_environment"` // development, production
	SecretsDir      string   `yaml:"secrets_dir" mapstructure:"secrets_dir"`           // directory for API keys, certs (default: secrets)
	KeySearchPaths  []string `yaml:"key_search_paths" mapstructure:"key_search_paths"` // search paths for APNs key files
}

// XcodeConfig holds Xcode-related configuration.
type XcodeConfig struct {
	XcconfigOutput string            `yaml:"xcconfig_output" mapstructure:"xcconfig_output"`
	VersionFile    string            `yaml:"version_file" mapstructure:"version_file"`
	Schemes        map[string]string `yaml:"schemes" mapstructure:"schemes"`
}

// WebConfig holds web project configuration.
type WebConfig struct {
	EnvOutput string `yaml:"env_output" mapstructure:"env_output"` // .env.local by default
}

// DatabaseConfig holds database connection configuration.
type DatabaseConfig struct {
	PoolerHost        string            `yaml:"pooler_host" mapstructure:"pooler_host"`
	BranchPoolerHosts map[string]string `yaml:"branch_pooler_hosts" mapstructure:"branch_pooler_hosts"`
	PoolerPort        int               `yaml:"pooler_port" mapstructure:"pooler_port"`
	DirectPort        int               `yaml:"direct_port" mapstructure:"direct_port"`
	RequireSSL        bool              `yaml:"require_ssl" mapstructure:"require_ssl"`
	DumpFormat        string            `yaml:"dump_format" mapstructure:"dump_format"`   // custom, plain, directory, tar
	BackupDir         string            `yaml:"backup_dir" mapstructure:"backup_dir"`     // local backup directory
	PromptFresh       bool              `yaml:"prompt_fresh" mapstructure:"prompt_fresh"` // prompt when backup is stale
}

// GetPoolerHostForBranch resolves the pooler host for a git branch/environment label.
// Lookup order:
// 1) exact branch key
// 2) lower-cased branch key
// 3) prefix aliases (e.g. feature/foo -> feature)
// 4) environment aliases (production/main, development/dev, feature/preview)
// 5) database.pooler_host default
func (d *DatabaseConfig) GetPoolerHostForBranch(branch string) string {
	defaultHost := DefaultDatabasePoolerHost
	if d != nil {
		if configured := strings.TrimSpace(d.PoolerHost); configured != "" {
			defaultHost = configured
		}
	}

	if d == nil || len(d.BranchPoolerHosts) == 0 {
		return defaultHost
	}

	for _, key := range poolerHostLookupKeys(branch) {
		if host, ok := lookupMapValueCaseInsensitive(d.BranchPoolerHosts, key); ok {
			host = strings.TrimSpace(host)
			if host != "" {
				return host
			}
		}
	}

	return defaultHost
}

// GetPoolerPort returns the configured pooler port or the default.
func (d *DatabaseConfig) GetPoolerPort() int {
	if d == nil || d.PoolerPort == 0 {
		return DefaultDatabasePoolerPort
	}
	return d.PoolerPort
}

// BackupConfig holds backup storage configuration.
type BackupConfig struct {
	Provider      string `yaml:"provider" mapstructure:"provider"` // supabase, s3, backblaze
	Bucket        string `yaml:"bucket" mapstructure:"bucket"`
	RetentionDays int    `yaml:"retention_days" mapstructure:"retention_days"`
}

// WorktreeConfig holds git worktree configuration.
type WorktreeConfig struct {
	NamingPattern     string   `yaml:"naming_pattern" mapstructure:"naming_pattern"`
	CopyOnCreate      []string `yaml:"copy_on_create" mapstructure:"copy_on_create"`
	AutoSetupXcconfig bool     `yaml:"auto_setup_xcconfig" mapstructure:"auto_setup_xcconfig"`
}

// DeviceConfig holds mobile device automation configuration.
type DeviceConfig struct {
	WDAPath       string        `yaml:"wda_path" mapstructure:"wda_path"`
	WDAPort       int           `yaml:"wda_port" mapstructure:"wda_port"`
	DefaultDevice string        `yaml:"default_device" mapstructure:"default_device"`
	Devices       []DeviceEntry `yaml:"devices" mapstructure:"devices"`
}

// DeviceEntry represents a configured test device.
type DeviceEntry struct {
	Name    string `yaml:"name" mapstructure:"name"`
	UDID    string `yaml:"udid" mapstructure:"udid"`
	Model   string `yaml:"model" mapstructure:"model"`
	OS      string `yaml:"os" mapstructure:"os"`
	Primary bool   `yaml:"primary" mapstructure:"primary"`
	Notes   string `yaml:"notes" mapstructure:"notes"`
}

// Load finds and loads .drift.yaml from current or parent directories.
func Load() (*Config, error) {
	configPath, err := FindConfigFile()
	if err != nil {
		return nil, err
	}

	return LoadFromPath(configPath)
}

// LoadFromPath loads configuration from a specific path.
func LoadFromPath(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	var cfg Config
	if strings.TrimSpace(string(data)) != "" {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	cfg.configPath = configPath
	return MergeWithDefaults(&cfg), nil
}

// LoadOrDefault tries to load config, returns defaults if not found.
func LoadOrDefault() *Config {
	cfg, err := LoadWithLocal()
	if err != nil {
		return DefaultConfig()
	}
	return cfg
}

// FindConfigFile walks up the directory tree looking for .drift.yaml.
func FindConfigFile() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	for {
		configPath := filepath.Join(dir, ".drift.yaml")
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		}

		// Also check for .drift.yml
		configPath = filepath.Join(dir, ".drift.yml")
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf(".drift.yaml not found (searched from current directory to root)")
		}
		dir = parent
	}
}

// ConfigPath returns the path to the loaded config file.
func (c *Config) ConfigPath() string {
	return c.configPath
}

// ProjectRoot returns the directory containing .drift.yaml.
func (c *Config) ProjectRoot() string {
	if c.configPath == "" {
		// Fall back to current directory
		dir, _ := os.Getwd()
		return dir
	}
	return filepath.Dir(c.configPath)
}

// IsProtectedBranch checks if a branch is in the protected branches list.
func (c *Config) IsProtectedBranch(branch string) bool {
	for _, protected := range c.Supabase.ProtectedBranches {
		if protected == branch {
			return true
		}
	}
	return false
}

// GetFunctionsPath returns the absolute path to the functions directory.
func (c *Config) GetFunctionsPath() string {
	return filepath.Join(c.ProjectRoot(), c.Supabase.FunctionsDir)
}

// GetMigrationsPath returns the absolute path to the migrations directory.
func (c *Config) GetMigrationsPath() string {
	return filepath.Join(c.ProjectRoot(), c.Supabase.MigrationsDir)
}

// GetXcconfigPath returns the absolute path to the xcconfig output file.
func (c *Config) GetXcconfigPath() string {
	return filepath.Join(c.ProjectRoot(), c.Xcode.XcconfigOutput)
}

// GetVersionFilePath returns the absolute path to the version file.
func (c *Config) GetVersionFilePath() string {
	return filepath.Join(c.ProjectRoot(), c.Xcode.VersionFile)
}

// GetEnvLocalPath returns the absolute path to the .env.local file for web projects.
func (c *Config) GetEnvLocalPath() string {
	return filepath.Join(c.ProjectRoot(), c.Web.EnvOutput)
}

// GetSecretsPath returns the absolute path to the secrets directory.
func (c *Config) GetSecretsPath() string {
	return filepath.Join(c.ProjectRoot(), c.Apple.SecretsDir)
}

// GetBackupPath returns the absolute path to the backup directory.
func (c *Config) GetBackupPath() string {
	return filepath.Join(c.ProjectRoot(), c.Database.BackupDir)
}

// GetPrimaryDevice returns the primary device from the config, or the first device if no primary is set.
func (c *Config) GetPrimaryDevice() *DeviceEntry {
	for i, d := range c.Device.Devices {
		if d.Primary {
			return &c.Device.Devices[i]
		}
	}
	// Fall back to default device by name
	if c.Device.DefaultDevice != "" {
		for i, d := range c.Device.Devices {
			if d.Name == c.Device.DefaultDevice {
				return &c.Device.Devices[i]
			}
		}
	}
	// Fall back to first device
	if len(c.Device.Devices) > 0 {
		return &c.Device.Devices[0]
	}
	return nil
}

// GetDeviceByUDID returns a device by its UDID.
func (c *Config) GetDeviceByUDID(udid string) *DeviceEntry {
	for i, d := range c.Device.Devices {
		if d.UDID == udid {
			return &c.Device.Devices[i]
		}
	}
	return nil
}

// GetDeviceByName returns a device by its name.
func (c *Config) GetDeviceByName(name string) *DeviceEntry {
	for i, d := range c.Device.Devices {
		if d.Name == name {
			return &c.Device.Devices[i]
		}
	}
	return nil
}

// Exists checks if a .drift.yaml file exists in the current or parent directories.
func Exists() bool {
	_, err := FindConfigFile()
	return err == nil
}

// IsFunctionRestricted checks if a function is restricted in the given environment.
func (c *Config) IsFunctionRestricted(functionName, environment string) bool {
	for _, r := range c.Supabase.Functions.Restricted {
		if r.Name == functionName {
			for _, env := range r.Environments {
				if env == environment {
					return true
				}
			}
		}
	}
	return false
}

// GetRestrictedFunctions returns a list of function names restricted in the given environment.
func (c *Config) GetRestrictedFunctions(environment string) []string {
	var restricted []string
	for _, r := range c.Supabase.Functions.Restricted {
		for _, env := range r.Environments {
			if env == environment {
				restricted = append(restricted, r.Name)
				break
			}
		}
	}
	return restricted
}

// GetEnvironmentConfig returns the configuration for a specific environment.
func (c *Config) GetEnvironmentConfig(environment string) *EnvironmentConfig {
	if c.Environments == nil {
		return nil
	}
	keys := environmentLookupKeys(environment)
	if len(keys) == 0 {
		return nil
	}

	var merged EnvironmentConfig
	found := false
	skipSet := make(map[string]bool)

	// Merge from least-specific to most-specific so specific keys override.
	for i := len(keys) - 1; i >= 0; i-- {
		key := keys[i]
		env, ok := c.Environments[key]
		if !ok {
			continue
		}
		found = true

		if env.PushKey != "" {
			merged.PushKey = env.PushKey
		}
		if len(env.Secrets) > 0 {
			if merged.Secrets == nil {
				merged.Secrets = make(map[string]string)
			}
			for secretKey, secretValue := range env.Secrets {
				merged.Secrets[secretKey] = secretValue
			}
		}
		if len(env.SkipSecrets) > 0 {
			for _, secretKey := range env.SkipSecrets {
				secretKey = strings.TrimSpace(secretKey)
				if secretKey == "" || skipSet[secretKey] {
					continue
				}
				skipSet[secretKey] = true
				merged.SkipSecrets = append(merged.SkipSecrets, secretKey)
			}
		}
	}

	if !found {
		return nil
	}
	return &merged
}

// GetEnvironmentSecrets returns the secrets for a specific environment.
func (c *Config) GetEnvironmentSecrets(environment string) map[string]string {
	envCfg := c.GetEnvironmentConfig(environment)
	if envCfg == nil {
		return nil
	}
	return envCfg.Secrets
}

// GetEnvironmentPushKey returns the push key path for a specific environment.
func (c *Config) GetEnvironmentPushKey(environment string) string {
	envCfg := c.GetEnvironmentConfig(environment)
	if envCfg == nil {
		return ""
	}
	return envCfg.PushKey
}

func environmentLookupKeys(environment string) []string {
	raw := strings.TrimSpace(environment)
	if raw == "" {
		return nil
	}

	lower := strings.ToLower(raw)
	keys := []string{raw}
	if lower != raw {
		keys = append(keys, lower)
	}

	canonical := lower
	switch lower {
	case "prod", "production", "main", "master":
		canonical = "production"
	case "dev", "development":
		canonical = "development"
	case "feature", "preview":
		canonical = "feature"
	}

	if !containsString(keys, canonical) {
		keys = append(keys, canonical)
	}

	// Feature environments commonly reuse development settings unless explicitly overridden.
	if canonical == "feature" && !containsString(keys, "development") {
		keys = append(keys, "development")
	}

	return keys
}

func poolerHostLookupKeys(branch string) []string {
	raw := strings.TrimSpace(branch)
	if raw == "" {
		return nil
	}

	lower := strings.ToLower(raw)
	keys := []string{raw}
	if lower != raw {
		keys = append(keys, lower)
	}

	if idx := strings.Index(lower, "/"); idx > 0 {
		prefix := lower[:idx]
		if !containsString(keys, prefix) {
			keys = append(keys, prefix)
		}
	}

	addAlias := func(alias string) {
		if alias == "" || containsString(keys, alias) {
			return
		}
		keys = append(keys, alias)
	}

	switch {
	case lower == "main" || lower == "master" || lower == "prod" || lower == "production":
		addAlias("production")
		addAlias("main")
		addAlias("master")
		addAlias("prod")
	case lower == "dev" || lower == "development":
		addAlias("development")
		addAlias("dev")
	case lower == "feature" || lower == "preview" || strings.HasPrefix(lower, "feature/"):
		addAlias("feature")
		addAlias("preview")
		// Feature branches often reuse development pooler hosts.
		addAlias("development")
	}

	return keys
}

func lookupMapValueCaseInsensitive(values map[string]string, key string) (string, bool) {
	if values == nil {
		return "", false
	}

	if value, ok := values[key]; ok {
		return value, true
	}

	for existingKey, value := range values {
		if strings.EqualFold(existingKey, key) {
			return value, true
		}
	}

	return "", false
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// IsVerbose returns whether verbose mode is enabled in preferences.
func (c *Config) IsVerbose() bool {
	return c.Preferences.Verbose
}

// GetEditor returns the preferred editor from preferences.
func (c *Config) GetEditor() string {
	if c.Preferences.Editor != "" {
		return c.Preferences.Editor
	}
	return "code" // Default to VS Code
}

// ShouldAutoOpenWorktree returns whether worktrees should auto-open after creation.
func (c *Config) ShouldAutoOpenWorktree() bool {
	return c.Preferences.AutoOpenWorktree
}
