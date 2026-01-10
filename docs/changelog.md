# Changelog

All notable changes to Drift are documented here.

## [0.3.0] - 2025-01-10

### Added
- **Interactive worktree creation** - `drift worktree create` without arguments now prompts for branch name and base branch
- **`--copy-env` flag** - Interactive picker to copy custom variables from another worktree's config file
- **`--scheme` flag** - Specify Xcode scheme for buildServer.json generation (`drift env setup --build-server --scheme "MyApp"`)
- **`--open` flag** - Open worktree in VS Code after creation (`drift worktree create feat/x --open`)
- **Auto-link Supabase** - `drift env setup` now automatically links Supabase if not already linked
- Web project support for `.env.local` generation

### Changed
- **Consolidated worktree commands** - `drift worktree create` now does full setup by default (previously required `ready`)
- **Removed `drift worktree ready`** - Use `drift worktree create` instead (add `--no-setup` for bare worktree)
- Improved error messages when Supabase CLI is not available or not authenticated

### Fixed
- Silent error bug where failed commands showed no output
- Fixed environment variable copying to work for both web (`.env.local`) and iOS/macOS (`Config.xcconfig`) projects

## [0.2.1] - 2024-01-15

### Added
- Comprehensive test suite (182 tests)
  - Tests for `internal/supabase` package
  - Tests for `internal/xcode` package
  - Tests for `internal/ui` package
  - Tests for `internal/cmd` helpers

### Fixed
- Fixed table header color bug that caused panic with dynamic column counts

## [0.2.0] - 2024-01-15

### Added
- `drift deploy list-secrets` - List secrets on target environment
- `drift storage setup` - Create storage bucket with RLS policies
- `drift storage status` - Show storage bucket status
- `--build-server` flag for `drift env setup` - Generate buildServer.json for sourcekit-lsp
- `RunCommand` helper for executing external commands

### Changed
- Improved storage bucket creation with proper RLS policy setup
- Enhanced scheme detection for buildServer.json generation

## [0.1.1] - 2024-01-14

### Added
- `RequireInit()` guard for commands requiring initialization
- Better error messages when drift is not initialized

### Fixed
- Commands now properly check for `drift.yaml` before executing

## [0.1.0] - 2024-01-14

### Added
- Initial release
- Core commands:
  - `drift init` - Initialize drift in a project
  - `drift env show/setup/switch` - Environment management
  - `drift deploy functions/secrets/all/status` - Deployment
  - `drift branch list/create/delete/status` - Branch management
  - `drift worktree list/create/remove/goto` - Worktree management
  - `drift backup create/restore/list/upload/download` - Backups
  - `drift version show/bump/set` - Version management
- Configuration via `drift.yaml`
- Xcode xcconfig generation
- Supabase CLI integration
- Git worktree support
- APNs secret management
- Database backup and restore

## Versioning

Drift follows [Semantic Versioning](https://semver.org/):

- **MAJOR**: Breaking changes
- **MINOR**: New features, backward compatible
- **PATCH**: Bug fixes, backward compatible
