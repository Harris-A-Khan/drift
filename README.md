# drift

Opinionated development workflow CLI for Supabase-backed iOS, macOS, and web projects.

`drift` standardizes and abstracts common development operations like environment switching, git worktree management, database operations, edge function deployment, and migration pushing into a single, cohesive CLI.

## Installation

### Homebrew (recommended)

```bash
brew install Harris-A-Khan/tap/drift
```

### From Source (recommended for development)

```bash
git clone https://github.com/Harris-A-Khan/drift.git
cd drift
make install    # Installs to /usr/local/bin
```

### Go Install

```bash
go install github.com/Harris-A-Khan/drift/cmd/drift@latest
```

> **Note:** Requires `$GOPATH/bin` in your PATH. Add to `~/.zshrc`:
> ```bash
> export PATH="$PATH:$(go env GOPATH)/bin"
> ```

### Manual Build

```bash
git clone https://github.com/Harris-A-Khan/drift.git
cd drift
make build              # Creates bin/drift
./bin/drift --version   # Run directly, or copy to a PATH location
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
drift worktree ready feat/my-feature --open

# Deploy edge functions
drift deploy all

# Push migrations
drift migrate push
```

## Commands

### Environment Management (`drift env`)

Manage Supabase environment configuration and xcconfig generation.

```bash
drift env show              # Show current environment info
drift env setup             # Generate Config.xcconfig for current branch
drift env setup --branch X  # Generate for a specific branch
drift env switch <branch>   # Switch to a different environment
```

### Worktree Management (`drift worktree` / `drift wt`)

Manage git worktrees for parallel development.

```bash
drift wt list                     # List all worktrees
drift wt create                   # Interactive: create + setup
drift wt create <branch>          # Create worktree with full setup
drift wt create <branch> --open   # Create, setup, and open in VS Code
drift wt create <branch> --no-setup  # Just create (no file copying/env setup)
drift wt open [branch]            # Open worktree in VS Code
drift wt delete [branch]          # Delete a worktree
drift wt path <branch>            # Print worktree path
drift wt prune                    # Clean stale entries
```

### Deployment (`drift deploy`)

Deploy Edge Functions and manage secrets.

```bash
drift deploy all           # Deploy functions + set secrets
drift deploy functions     # Deploy edge functions only
drift deploy secrets       # Set APNs secrets only
drift deploy status        # Show deployment status
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

Create a `.drift.yaml` in your project root:

```yaml
project:
  name: my-ios-app
  type: ios

supabase:
  functions_dir: supabase/functions
  migrations_dir: supabase/migrations
  protected_branches:
    - main
    - master

apns:
  team_id: "XXXXXXXXXX"
  bundle_id: "com.example.myapp"
  key_pattern: "AuthKey_*.p8"
  environment: development

xcode:
  xcconfig_output: Config.xcconfig
  version_file: Version.xcconfig

database:
  pooler_host: aws-0-us-east-1.pooler.supabase.com
  pooler_port: 6543
  direct_port: 5432

backup:
  provider: supabase
  bucket: database-backups
  retention_days: 30

worktree:
  naming_pattern: "{project}-{branch}"
  copy_on_create:
    - .env
    - "*.p8"
  auto_setup_xcconfig: true
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `PROD_PASSWORD` | Production database password |
| `DEV_PASSWORD` | Development database password |
| `PROD_PROJECT_REF` | Production Supabase project ref |
| `DEV_PROJECT_REF` | Development Supabase project ref |
| `APNS_KEY_ID` | APNs key ID |
| `APNS_TEAM_ID` | APNs team ID |
| `APNS_BUNDLE_ID` | APNs bundle ID |
| `APNS_ENVIRONMENT` | APNs environment (development/production) |
| `DRIFT_DEBUG` | Enable debug output |

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
# Using Python (built-in)
cd docs && python3 -m http.server 3000

# Using npx (no install required)
npx serve docs

# Using docsify-cli
npm i -g docsify-cli
docsify serve docs
```

Then open http://localhost:3000 in your browser.

## License

MIT License - see [LICENSE](LICENSE) for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

