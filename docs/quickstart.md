# Quick Start

This guide walks you through setting up Drift in an existing iOS/macOS project with Supabase.

## 1. Initialize Drift

Navigate to your project root and run:

```bash
cd /path/to/your/ios-project
drift init
```

This creates a `.drift.yaml` configuration file with sensible defaults.

## 2. Configure Your Project

Edit `.drift.yaml` to match your project:

```yaml
project:
  name: MyApp

supabase:
  functions_path: supabase/functions
  migrations_path: supabase/migrations

xcode:
  xcconfig_path: Config.xcconfig
  version_file: Version.xcconfig

git:
  protected_branches:
    - main
    - master
    - production
```

## 3. Generate Xcode Configuration

Generate `Config.xcconfig` for your current branch:

```bash
drift env setup
```

This will:
1. Detect your current git branch
2. Find the matching Supabase branch
3. Fetch API keys
4. Generate `Config.xcconfig` with credentials

## 4. Add to Xcode

In Xcode, add `Config.xcconfig` to your project and reference it in your build settings:

1. Drag `Config.xcconfig` into your Xcode project
2. Go to Project → Info → Configurations
3. Set the configuration file for each build configuration

## 5. Use in Swift

Access the configuration values in your Swift code:

```swift
// In your Info.plist, add:
// SUPABASE_URL = $(SUPABASE_URL)
// SUPABASE_ANON_KEY = $(SUPABASE_ANON_KEY)

let supabaseURL = Bundle.main.infoDictionary?["SUPABASE_URL"] as? String
let anonKey = Bundle.main.infoDictionary?["SUPABASE_ANON_KEY"] as? String
```

## 6. Deploy Edge Functions

When ready to deploy:

```bash
# Check deployment status
drift deploy status

# Deploy functions only
drift deploy functions

# Deploy functions and set secrets
drift deploy all
```

## Next Steps

- [Environment Management](commands/env.md) - Learn about branch-aware configuration
- [Git Worktrees](commands/worktree.md) - Work on multiple branches simultaneously
- [Database Backups](guides/backups.md) - Set up automated backups
