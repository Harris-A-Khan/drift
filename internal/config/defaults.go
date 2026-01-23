package config

// DefaultConfig returns the default configuration values.
func DefaultConfig() *Config {
	return &Config{
		Project: ProjectConfig{
			Name: "",
			Type: "ios",
		},
		Supabase: SupabaseConfig{
			ProjectRef:        "",
			ProjectName:       "",
			FunctionsDir:      "supabase/functions",
			MigrationsDir:     "supabase/migrations",
			ProtectedBranches: []string{"main", "master"},
			Functions:         FunctionsConfig{Restricted: []FunctionRestriction{}},
		},
		Environments: map[string]EnvironmentConfig{},
		Apple: AppleConfig{
			TeamID:          "",
			BundleID:        "",
			PushKeyPattern:  "AuthKey_*.p8",
			PushEnvironment: "development",
		},
		Xcode: XcodeConfig{
			XcconfigOutput: "Config.xcconfig",
			VersionFile:    "Version.xcconfig",
			Schemes: map[string]string{
				"production":  "",
				"development": "",
				"feature":     "",
			},
		},
		Web: WebConfig{
			EnvOutput: ".env.local",
		},
		Database: DatabaseConfig{
			PoolerHost:  "aws-0-us-east-1.pooler.supabase.com",
			PoolerPort:  6543,
			DirectPort:  5432,
			RequireSSL:  true,
			DumpFormat:  "custom",
			BackupDir:   "backups",
			PromptFresh: true,
		},
		Backup: BackupConfig{
			Provider:      "supabase",
			Bucket:        "database-backups",
			RetentionDays: 30,
		},
		Worktree: WorktreeConfig{
			NamingPattern:     "{project}-{branch}",
			CopyOnCreate:      []string{".env", "*.p8"},
			AutoSetupXcconfig: true,
		},
		Device: DeviceConfig{
			WDAPath:       "/tmp/WebDriverAgent",
			WDAPort:       8100,
			DefaultDevice: "",
			Devices:       []DeviceEntry{},
		},
	}
}

// MergeWithDefaults merges a loaded config with defaults for any missing values.
func MergeWithDefaults(cfg *Config) *Config {
	defaults := DefaultConfig()

	// Project defaults
	if cfg.Project.Type == "" {
		cfg.Project.Type = defaults.Project.Type
	}

	// Supabase defaults
	if cfg.Supabase.FunctionsDir == "" {
		cfg.Supabase.FunctionsDir = defaults.Supabase.FunctionsDir
	}
	if cfg.Supabase.MigrationsDir == "" {
		cfg.Supabase.MigrationsDir = defaults.Supabase.MigrationsDir
	}
	if len(cfg.Supabase.ProtectedBranches) == 0 {
		cfg.Supabase.ProtectedBranches = defaults.Supabase.ProtectedBranches
	}

	// Apple defaults
	if cfg.Apple.PushKeyPattern == "" {
		cfg.Apple.PushKeyPattern = defaults.Apple.PushKeyPattern
	}
	if cfg.Apple.PushEnvironment == "" {
		cfg.Apple.PushEnvironment = defaults.Apple.PushEnvironment
	}

	// Xcode defaults
	if cfg.Xcode.XcconfigOutput == "" {
		cfg.Xcode.XcconfigOutput = defaults.Xcode.XcconfigOutput
	}
	if cfg.Xcode.VersionFile == "" {
		cfg.Xcode.VersionFile = defaults.Xcode.VersionFile
	}

	// Web defaults
	if cfg.Web.EnvOutput == "" {
		cfg.Web.EnvOutput = defaults.Web.EnvOutput
	}

	// Database defaults
	if cfg.Database.PoolerHost == "" {
		cfg.Database.PoolerHost = defaults.Database.PoolerHost
	}
	if cfg.Database.PoolerPort == 0 {
		cfg.Database.PoolerPort = defaults.Database.PoolerPort
	}
	if cfg.Database.DirectPort == 0 {
		cfg.Database.DirectPort = defaults.Database.DirectPort
	}
	if cfg.Database.DumpFormat == "" {
		cfg.Database.DumpFormat = defaults.Database.DumpFormat
	}
	if cfg.Database.BackupDir == "" {
		cfg.Database.BackupDir = defaults.Database.BackupDir
	}

	// Backup defaults
	if cfg.Backup.Provider == "" {
		cfg.Backup.Provider = defaults.Backup.Provider
	}
	if cfg.Backup.Bucket == "" {
		cfg.Backup.Bucket = defaults.Backup.Bucket
	}
	if cfg.Backup.RetentionDays == 0 {
		cfg.Backup.RetentionDays = defaults.Backup.RetentionDays
	}

	// Worktree defaults
	if cfg.Worktree.NamingPattern == "" {
		cfg.Worktree.NamingPattern = defaults.Worktree.NamingPattern
	}
	if len(cfg.Worktree.CopyOnCreate) == 0 {
		cfg.Worktree.CopyOnCreate = defaults.Worktree.CopyOnCreate
	}

	// Device defaults
	if cfg.Device.WDAPath == "" {
		cfg.Device.WDAPath = defaults.Device.WDAPath
	}
	if cfg.Device.WDAPort == 0 {
		cfg.Device.WDAPort = defaults.Device.WDAPort
	}

	return cfg
}

