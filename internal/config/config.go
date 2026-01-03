// Package config handles loading and managing drift configuration.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config represents the .drift.yaml configuration file.
type Config struct {
	Project  ProjectConfig  `yaml:"project" mapstructure:"project"`
	Supabase SupabaseConfig `yaml:"supabase" mapstructure:"supabase"`
	APNS     APNSConfig     `yaml:"apns" mapstructure:"apns"`
	Xcode    XcodeConfig    `yaml:"xcode" mapstructure:"xcode"`
	Database DatabaseConfig `yaml:"database" mapstructure:"database"`
	Backup   BackupConfig   `yaml:"backup" mapstructure:"backup"`
	Worktree WorktreeConfig `yaml:"worktree" mapstructure:"worktree"`

	// Internal: path to the config file
	configPath string
}

// ProjectConfig holds project-level configuration.
type ProjectConfig struct {
	Name string `yaml:"name" mapstructure:"name"`
	Type string `yaml:"type" mapstructure:"type"` // ios, macos, multiplatform
}

// SupabaseConfig holds Supabase-related configuration.
type SupabaseConfig struct {
	ProjectRefFile    string   `yaml:"project_ref_file" mapstructure:"project_ref_file"`
	FunctionsDir      string   `yaml:"functions_dir" mapstructure:"functions_dir"`
	MigrationsDir     string   `yaml:"migrations_dir" mapstructure:"migrations_dir"`
	ProtectedBranches []string `yaml:"protected_branches" mapstructure:"protected_branches"`
}

// APNSConfig holds Apple Push Notification configuration.
type APNSConfig struct {
	TeamID      string `yaml:"team_id" mapstructure:"team_id"`
	BundleID    string `yaml:"bundle_id" mapstructure:"bundle_id"`
	KeyPattern  string `yaml:"key_pattern" mapstructure:"key_pattern"`
	Environment string `yaml:"environment" mapstructure:"environment"` // development, production
}

// XcodeConfig holds Xcode-related configuration.
type XcodeConfig struct {
	XcconfigOutput string            `yaml:"xcconfig_output" mapstructure:"xcconfig_output"`
	VersionFile    string            `yaml:"version_file" mapstructure:"version_file"`
	Schemes        map[string]string `yaml:"schemes" mapstructure:"schemes"`
}

// DatabaseConfig holds database connection configuration.
type DatabaseConfig struct {
	PGBin       string `yaml:"pg_bin" mapstructure:"pg_bin"`
	PoolerHost  string `yaml:"pooler_host" mapstructure:"pooler_host"`
	PoolerPort  int    `yaml:"pooler_port" mapstructure:"pooler_port"`
	DirectPort  int    `yaml:"direct_port" mapstructure:"direct_port"`
	RequireSSL  bool   `yaml:"require_ssl" mapstructure:"require_ssl"`
	DumpFormat  string `yaml:"dump_format" mapstructure:"dump_format"`   // custom, plain, directory, tar
	BackupDir   string `yaml:"backup_dir" mapstructure:"backup_dir"`     // local backup directory
	PromptFresh bool   `yaml:"prompt_fresh" mapstructure:"prompt_fresh"` // prompt when backup is stale
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
	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	cfg.configPath = configPath
	return MergeWithDefaults(&cfg), nil
}

// LoadOrDefault tries to load config, returns defaults if not found.
func LoadOrDefault() *Config {
	cfg, err := Load()
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

// Exists checks if a .drift.yaml file exists in the current or parent directories.
func Exists() bool {
	_, err := FindConfigFile()
	return err == nil
}

