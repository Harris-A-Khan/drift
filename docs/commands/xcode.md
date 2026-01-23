# drift xcode

Manage Xcode schemes and project configuration.

## Usage

```bash
drift xcode <subcommand> [flags]
```

## Subcommands

| Subcommand | Description |
|------------|-------------|
| `schemes` | List all available Xcode schemes |
| `validate` | Validate configured schemes exist |

## drift xcode schemes

List all Xcode schemes found in the project.

```bash
drift xcode schemes
```

This scans both `.xcodeproj` and `.xcworkspace` files for available schemes. Shared schemes (in `xcshareddata`) are shown first.

**Example Output:**

```
╔══════════════════════════════════════════════════════════════╗
║  Xcode Schemes                                               ║
╚══════════════════════════════════════════════════════════════╝

───── Shared Schemes
  MyApp (Production)  MyApp.xcodeproj
  MyApp (Development)  MyApp.xcodeproj
  MyApp (Debug)  MyApp.xcodeproj

───── Personal Schemes
  MyApp-Testing  MyApp.xcodeproj

→ Total: 4 schemes (3 shared, 1 personal)

───── Configured in .drift.yaml
  ✓ production: MyApp (Production)
  ✓ development: MyApp (Development)
  ✓ feature: MyApp (Debug)
```

## drift xcode validate

Check that all schemes configured in `.drift.yaml` actually exist in the Xcode project.

```bash
drift xcode validate
```

This helps catch configuration errors before deployment or build operations.

**Example Output (Success):**

```
╔══════════════════════════════════════════════════════════════╗
║  Validate Xcode Configuration                                ║
╚══════════════════════════════════════════════════════════════╝

───── Scheme Validation
  ✓ production: MyApp (Production)
  ✓ development: MyApp (Development)
  ✓ feature: MyApp (Debug)

✓ All 3 configured schemes are valid
```

**Example Output (Error):**

```
╔══════════════════════════════════════════════════════════════╗
║  Validate Xcode Configuration                                ║
╚══════════════════════════════════════════════════════════════╝

───── Scheme Validation
  ✓ production: MyApp (Production)
  ✗ development: MyApp (Dev) (not found)
  ✓ feature: MyApp (Debug)

⚠ 1 of 3 configured schemes not found

Available schemes:
  • MyApp (Production)
  • MyApp (Development)
  • MyApp (Debug)
```

## Configuration

Configure environment-to-scheme mappings in `.drift.yaml`:

```yaml
xcode:
  schemes:
    production: "MyApp (Production)"
    development: "MyApp (Development)"
    feature: "MyApp (Debug)"
```

When running `drift env setup --build-server`, the appropriate scheme is automatically selected based on the current environment.

## Auto-Detection

During `drift init`, Drift scans for available Xcode schemes and can auto-configure the scheme mappings:

1. Looks for schemes containing "Production", "Release", or "Prod" for production
2. Looks for schemes containing "Development", "Dev", or "Staging" for development
3. Uses default/Debug schemes for feature branches

## See Also

- [Xcode Integration Guide](../guides/xcode.md)
- [Environment Management](env.md)
- [Configuration](../config/drift-yaml.md)
