# Quick Start

This guide walks you through setting up Drift in an existing iOS, macOS, or web project with Supabase.

## 1. Initialize Drift

Navigate to your project root and run:

```bash
cd /path/to/your/project
drift init
```

This creates a `.drift.yaml` configuration file with sensible defaults and links your Supabase project.

## 2. Configure Your Project

Edit `.drift.yaml` to match your project:

```yaml
project:
  name: MyApp
  type: ios  # or "web" for web projects

supabase:
  project_ref: abcdefghij
  functions_dir: supabase/functions
  migrations_dir: supabase/migrations
  protected_branches:
    - main
    - master
```

## 3. Generate Environment Configuration

Generate configuration for your current branch:

```bash
drift env setup
```

This will:
1. Detect your current git branch
2. Find (or create) the matching Supabase branch
3. Fetch API keys
4. Generate the appropriate config file:
   - **iOS/macOS projects:** `Config.xcconfig`
   - **Web projects:** `.env.local`

## 4. Add to Your Project

### For iOS/macOS (Xcode)

In Xcode, add `Config.xcconfig` to your project and reference it in your build settings:

1. Drag `Config.xcconfig` into your Xcode project
2. Go to Project → Info → Configurations
3. Set the configuration file for each build configuration

### For Web Projects

The generated `.env.local` contains your Supabase credentials:

```bash
NEXT_PUBLIC_SUPABASE_URL=https://abcdefghij.supabase.co
NEXT_PUBLIC_SUPABASE_ANON_KEY=eyJ...
```

## 5. Use in Your Code

### Swift (iOS/macOS)

```swift
// In your Info.plist, add:
// SUPABASE_URL = $(SUPABASE_URL)
// SUPABASE_ANON_KEY = $(SUPABASE_ANON_KEY)

let supabaseURL = Bundle.main.infoDictionary?["SUPABASE_URL"] as? String
let anonKey = Bundle.main.infoDictionary?["SUPABASE_ANON_KEY"] as? String
```

### JavaScript/TypeScript (Web)

```typescript
const supabaseUrl = process.env.NEXT_PUBLIC_SUPABASE_URL!
const supabaseAnonKey = process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY!
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
