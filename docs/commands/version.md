# drift version

Manage version and build numbers.

## Usage

```bash
drift version <subcommand> [flags]
```

## Subcommands

| Subcommand | Description |
|------------|-------------|
| `show` | Show current version and build number |
| `bump` | Increment build number |
| `set` | Set version or build number |

## drift version show

Display current version information from `Version.xcconfig`.

```bash
$ drift version show

╔══════════════════════════════════════════════════════════════╗
║  Version Info                                                ║
╚══════════════════════════════════════════════════════════════╝

  Marketing Version:  1.2.3
  Build Number:       42
  Version File:       Version.xcconfig
```

## drift version bump

Increment the build number.

```bash
drift version bump [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--commit` | Commit the change |
| `--push` | Push after commit |

**Example:**

```bash
$ drift version bump

  Current build: 42
  New build: 43

✓ Build number incremented to 43
```

**With commit:**

```bash
drift version bump --commit --push
```

## drift version set

Set a specific version or build number.

```bash
drift version set [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--marketing`, `-m` | Set marketing version (e.g., "1.2.3") |
| `--build`, `-b` | Set build number |

**Examples:**

```bash
# Set marketing version
drift version set --marketing 2.0.0

# Set build number
drift version set --build 100

# Set both
drift version set --marketing 2.0.0 --build 1
```

## Version.xcconfig

Drift manages version information in `Version.xcconfig`:

```
// Version.xcconfig
// Managed by drift

MARKETING_VERSION = 1.2.3
CURRENT_PROJECT_VERSION = 42
```

## Xcode Integration

In your Xcode project:

1. Add `Version.xcconfig` to your project
2. In your target's build settings, set:
   - `MARKETING_VERSION` = `$(MARKETING_VERSION)`
   - `CURRENT_PROJECT_VERSION` = `$(CURRENT_PROJECT_VERSION)`

Or include it in your main xcconfig:

```
#include "Version.xcconfig"
```

## CI/CD Usage

Automatically increment build number in CI:

```bash
# In your CI script
drift version bump --commit --push
drift deploy all
```

## See Also

- [Xcode Integration](../guides/xcode.md)
- [CI/CD Setup](../guides/cicd.md)
