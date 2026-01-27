# drift

Opinionated development workflow CLI for Supabase-backed iOS, macOS, and web projects.

`drift` standardizes and abstracts common development operations like environment switching, git worktree management, database operations, edge function deployment, and migration pushing into a single, cohesive CLI.

## Installation

### Homebrew (recommended for macOS)

```bash
brew tap Harris-A-Khan/tap
brew install drift
```

### Quick Install Script

```bash
curl -fsSL https://raw.githubusercontent.com/Harris-A-Khan/drift/main/install.sh | bash
```

### Direct Download

```bash
# macOS Apple Silicon (M1/M2/M3)
curl -L -o drift https://github.com/Harris-A-Khan/drift/releases/latest/download/drift-darwin-arm64
chmod +x drift && sudo mv drift /usr/local/bin/

# macOS Intel
curl -L -o drift https://github.com/Harris-A-Khan/drift/releases/latest/download/drift-darwin-amd64
chmod +x drift && sudo mv drift /usr/local/bin/

# Linux x86_64
curl -L -o drift https://github.com/Harris-A-Khan/drift/releases/latest/download/drift-linux-amd64
chmod +x drift && sudo mv drift /usr/local/bin/

# Linux ARM64
curl -L -o drift https://github.com/Harris-A-Khan/drift/releases/latest/download/drift-linux-arm64
chmod +x drift && sudo mv drift /usr/local/bin/
```

### Go Install

```bash
go install github.com/Harris-A-Khan/drift/cmd/drift@latest
```

> **Note:** Requires `$GOPATH/bin` in your PATH. Add to `~/.zshrc`:
> ```bash
> export PATH="$PATH:$(go env GOPATH)/bin"
> ```

### From Source

```bash
git clone https://github.com/Harris-A-Khan/drift.git
cd drift
make install    # Builds and installs to /usr/local/bin
```

## Quick Start

```bash
# Initialize drift in your project
drift init

# Check dependencies
drift doctor

# Set up xcconfig for current branch
drift env setup

# Create a feature worktree with full setup
drift worktree create feat/my-feature --open

# Deploy edge functions
drift deploy all

# Push migrations
drift migrate push
```

## Commands

### Environment Management (`drift env`)

Manage Supabase environment configuration and config file generation.

```bash
drift env show              # Show current environment info
drift env setup             # Generate config for current branch
drift env setup --branch X  # Generate for a specific Supabase branch
drift env setup --copy-env  # Copy custom variables from another worktree
drift env switch <branch>   # Switch to a different environment
drift env validate          # Validate environment configuration
drift env diff <b1> <b2>    # Compare environments between branches
```

For iOS/macOS projects, generates `Config.xcconfig`. For web projects, generates `.env.local`.

### Configuration (`drift config`)

View and modify drift configuration.

```bash
drift config show           # Show current configuration
drift config set-branch     # Interactive: set Supabase branch override
drift config set-branch X   # Force use of Supabase branch X
drift config clear-branch   # Clear the override, use auto-detection
```

### Worktree Management (`drift worktree` / `drift wt`)

Manage git worktrees for parallel development with full environment setup.

```bash
drift wt list                        # List all worktrees
drift wt create                      # Interactive: create + setup
drift wt create <branch>             # Create worktree with full setup
drift wt create <branch> --open      # Create, setup, and open in VS Code
drift wt create <branch> --no-setup  # Just create (no file copying/env setup)
drift wt open [branch]               # Open worktree in VS Code
drift wt delete [branch]             # Delete a worktree
drift wt path <branch>               # Print worktree path
drift wt prune                       # Clean stale entries
drift wt info [branch]               # Show detailed worktree info
drift wt cleanup                     # Clean up merged worktrees
drift wt sync                        # Interactive sync across worktrees
```

The `create` command automatically copies configured files (`.env`, `.p8` keys) and generates environment config.

### Device Management (`drift device`)

Build, run, and manage iOS device and simulator workflows.

```bash
drift device build               # Build for connected device
drift device build --simulator   # Build for default simulator
drift device build --simulator "iPhone 16 Pro"
drift device run --simulator     # Build, install, and run on simulator
drift device simulators          # List available simulators
drift device list                # List connected devices
drift device start               # Start WebDriverAgent for automation
drift device status              # Check device/WDA status
```

### Xcode Management (`drift xcode`)

Manage Xcode schemes.

```bash
drift xcode schemes              # List all schemes
drift xcode validate             # Validate configured schemes exist
```

### Edge Functions (`drift functions`)

Manage and compare Edge Functions.

```bash
drift functions list       # Compare local vs deployed functions
drift functions logs <fn>  # View function logs
drift functions diff <fn>  # Compare local vs deployed code
drift functions delete <fn> # Delete a deployed function
drift functions serve      # Run functions locally
drift functions new <name> # Create a new function
```

### Secrets Management (`drift secrets`)

Manage Edge Function secrets.

```bash
drift secrets list         # List secrets for current branch
drift secrets diff dev prod # Compare secrets between branches
drift secrets copy dev     # Copy secrets from dev to current branch
```

### Deployment (`drift deploy`)

Deploy Edge Functions and set environment secrets.

```bash
drift deploy all           # Deploy functions + set secrets
drift deploy functions     # Deploy edge functions only
drift deploy secrets       # Set environment secrets
drift deploy status        # Show deployment status
drift deploy list-secrets  # List configured secrets
```

### Database Operations (`drift db`)

Manage database dumps and restores.

```bash
drift db dump prod         # Dump production database
drift db dump dev          # Dump development database
drift db push dev          # Push prod backup to development
drift db push feature      # Push dev backup to feature branch
drift db list              # List local backups
```

### Migrations (`drift migrate`)

Push database migrations.

```bash
drift migrate push         # Push migrations to current branch
drift migrate push <branch> # Push to specific branch
drift migrate status       # Show migration status
drift migrate new <name>   # Create new migration
```

### Backup Management (`drift backup`)

Manage cloud backups.

```bash
drift backup upload <file> <env>  # Upload backup
drift backup download <env>       # Download latest backup
drift backup list <env>           # List cloud backups
drift backup delete <env> <file>  # Delete a backup
```

### Build Helpers (`drift build`)

Manage Xcode version numbers.

```bash
drift build version        # Show current version info
drift build bump           # Increment build number
drift build set <number>   # Set build number
drift build set-version <version>  # Set marketing version
```

### System (`drift doctor`)

Check system dependencies and configuration.

```bash
drift doctor               # Check all dependencies
```

## Configuration

Create a `.drift.yaml` in your project root (or run `drift init`).

### Local Configuration

Create `.drift.local.yaml` for developer-specific settings (automatically gitignored):

```yaml
supabase:
  override_branch: "feat-my-feature"

device:
  default_device: "My iPhone"

preferences:
  verbose: true
  editor: "cursor"
```

### Main Configuration

```yaml
project:
  name: my-app
  type: ios  # or "web" for web projects

supabase:
  project_ref: abcdefghij      # Supabase project reference
  project_name: my-project     # Display name
  functions_dir: supabase/functions
  migrations_dir: supabase/migrations
  protected_branches:
    - main
    - master
  override_branch: v2/migration  # Optional: force specific Supabase branch

apns:  # iOS/macOS only
  team_id: "XXXXXXXXXX"
  bundle_id: "com.example.myapp"
  key_pattern: "AuthKey_*.p8"
  environment: development

xcode:  # iOS/macOS only
  xcconfig_path: Config.xcconfig
  version_file: Version.xcconfig
  schemes:
    production: "MyApp (Production)"
    development: "MyApp (Development)"
    feature: "MyApp (Debug)"

database:
  pooler_host: aws-0-us-east-1.pooler.supabase.com
  pooler_port: 6543

backup:
  bucket: database-backups

worktree:
  naming_pattern: "{project}-{branch}"
  copy_on_create:
    - .env
    - "*.p8"

# Per-environment configuration
environments:
  production:
    secrets:
      APNS_KEY_ID: "KEY_PROD_123"
    push_key: "AuthKey_PROD.p8"
  development:
    secrets:
      APNS_KEY_ID: "KEY_DEV_456"
    push_key: "AuthKey_DEV.p8"

# Function deployment restrictions
functions:
  restricted:
    - name: "dangerous-function"
      environments: ["production"]
```

## Environment Variables

| Variable | Description | Required For |
|----------|-------------|--------------|
| `SUPABASE_ACCESS_TOKEN` | Supabase access token for API access | CI/CD, secrets copy |
| `SUPABASE_DB_PASSWORD` | Database password for migrations | migrate push/history |
| `PROD_PASSWORD` | Production database password | db dump/push to production |
| `PROD_PROJECT_REF` | Production Supabase project ref | db operations |
| `APNS_KEY_ID` | APNs key ID | deploy secrets |
| `APNS_TEAM_ID` | APNs team ID | deploy secrets |
| `APNS_BUNDLE_ID` | APNs bundle ID | deploy secrets |
| `APNS_ENVIRONMENT` | APNs environment (development/production) | deploy secrets |
| `DRIFT_DEBUG` | Enable debug output | debugging |

> **Note:** Non-production database credentials are automatically retrieved via the Supabase CLI. Only production credentials need to be set manually.

## CI/CD Usage

Drift can be used in GitHub Actions and other CI/CD pipelines.

### GitHub Actions Setup

1. **Get your Supabase access token** from https://supabase.com/dashboard/account/tokens

2. **Add secrets to your GitHub repository** (Settings > Secrets and variables > Actions):
   - `SUPABASE_ACCESS_TOKEN` - Your Supabase access token (required)
   - `SUPABASE_DB_PASSWORD` - Database password (for migration commands)

3. **Add drift to your workflow:**

```yaml
name: Deploy

on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install Supabase CLI
        uses: supabase/setup-cli@v1

      - name: Install Drift CLI
        run: |
          curl -L -o drift $(curl -s https://api.github.com/repos/Harris-A-Khan/drift/releases/latest | grep "browser_download_url.*drift-linux-amd64" | cut -d '"' -f 4)
          chmod +x drift
          sudo mv drift /usr/local/bin/

      - name: Deploy Edge Functions
        env:
          SUPABASE_ACCESS_TOKEN: ${{ secrets.SUPABASE_ACCESS_TOKEN }}
        run: drift deploy functions -y

      - name: Push Migrations
        env:
          SUPABASE_ACCESS_TOKEN: ${{ secrets.SUPABASE_ACCESS_TOKEN }}
          SUPABASE_DB_PASSWORD: ${{ secrets.SUPABASE_DB_PASSWORD }}
        run: drift migrate push -y
```

### Available CI/CD Commands

```bash
# Status and information (read-only)
drift status                    # Show project status
drift functions list            # Compare local vs deployed functions
drift secrets list              # List configured secrets
drift migrate history           # Show applied migrations

# Deployment (requires SUPABASE_ACCESS_TOKEN)
drift deploy functions -y       # Deploy edge functions
drift deploy secrets -y         # Set environment secrets
drift deploy all -y             # Full deployment

# Migrations (requires SUPABASE_ACCESS_TOKEN + SUPABASE_DB_PASSWORD)
drift migrate push -y           # Push pending migrations

# Comparisons
drift functions diff <name>     # Compare local vs deployed code
drift secrets diff main dev     # Compare secrets between branches
```

### Pin to a Specific Version

```yaml
- name: Install Drift CLI
  run: |
    VERSION="v1.0.0"  # Pin to specific version
    curl -L -o drift "https://github.com/Harris-A-Khan/drift/releases/download/${VERSION}/drift-linux-amd64"
    chmod +x drift
    sudo mv drift /usr/local/bin/
```

See `.github/examples/drift-deploy.yml` for a complete example workflow.

## Requirements

- **Required:**
  - Git
  - Supabase CLI (`brew install supabase/tap/supabase`)

- **For database operations:**
  - PostgreSQL (`brew install postgresql@16`)
  - Add to PATH in `~/.zshrc`: `export PATH="/opt/homebrew/opt/postgresql@16/bin:$PATH"`

- **Optional:**
  - fzf (for interactive selection)
  - VS Code (for opening worktrees)
  - xcode-build-server (`brew install xcode-build-server`) for sourcekit-lsp support

## Global Flags

| Flag | Description |
|------|-------------|
| `--config <path>` | Use a specific config file |
| `--verbose, -v` | Verbose output |
| `--yes, -y` | Skip confirmation prompts |
| `--no-color` | Disable colored output |
| `--version` | Show version |

## Documentation

Full documentation is available in the `docs/` directory, including:

- **Command Reference** - Detailed usage for all commands (`env`, `worktree`, `deploy`, `db`, `migrate`, etc.)
- **Configuration Guide** - Complete `.drift.yaml` reference
- **Guides** - Xcode integration, worktree workflows, backup strategies
- **Architecture** - How drift works internally

### Viewing Docs Locally

```bash
# Using drift (recommended)
drift docs              # Serve on localhost:3000 and open browser
drift docs --port 8080  # Custom port
drift docs --no-open    # Don't open browser automatically

# Alternative methods
cd docs && python3 -m http.server 3000
npx serve docs
```

Press Ctrl+C to stop the server.

## License

MIT License - see [LICENSE](LICENSE) for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

