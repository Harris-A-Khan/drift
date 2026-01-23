# drift device

Manage iOS device builds, runs, and simulators.

## Usage

```bash
drift device <subcommand> [flags]
```

## Subcommands

| Subcommand | Description |
|------------|-------------|
| `list` | List connected iOS devices |
| `build` | Build and install app to device or simulator |
| `run` | Build, install, and run app |
| `simulators` | List available iOS simulators |
| `start` | Start WebDriverAgent for MCP automation |
| `stop` | Stop WebDriverAgent and cleanup |
| `status` | Check WebDriverAgent status |

## drift device build

Build the app and install it to a connected device or simulator.

```bash
drift device build [device] [flags]
```

If no device is specified, shows an interactive picker.

**Flags:**

| Flag | Description |
|------|-------------|
| `--scheme`, `-s` | Xcode scheme to build |
| `--run`, `-r` | Run app after installing |
| `--simulator` | Build for simulator instead of device |

**Examples:**

```bash
# Interactive device picker
drift device build

# Build to a named device
drift device build "My iPhone"

# Build with specific scheme
drift device build --scheme "App (Debug)"

# Build and run
drift device build --run
```

### Simulator Builds

Use the `--simulator` flag to build for iOS Simulator instead of a physical device.

```bash
# Build for default simulator (iPhone 16 Pro or similar)
drift device build --simulator

# Build for a specific simulator
drift device build --simulator "iPhone 16 Pro"

# Build with partial name match
drift device build --simulator "iPhone 15"
```

The simulator will be booted automatically if not already running.

## drift device run

Build, install, and run the app on a device or simulator.

```bash
drift device run [device] [flags]
```

This is shorthand for `drift device build --run`.

**Flags:**

| Flag | Description |
|------|-------------|
| `--simulator` | Run on simulator instead of device |

**Examples:**

```bash
# Interactive picker, build and run
drift device run

# Run on named device
drift device run "My iPhone"

# Run on simulator
drift device run --simulator
drift device run --simulator "iPhone 16 Pro"
```

## drift device simulators

List all available iOS simulators.

```bash
drift device simulators
```

**Example Output:**

```
╔══════════════════════════════════════════════════════════════╗
║  iOS Simulators                                              ║
╚══════════════════════════════════════════════════════════════╝

───── iOS 18.0
  ● iPhone 16 Pro (running)
  ○ iPhone 16
  ○ iPad Pro 13-inch

───── iOS 17.5
  ○ iPhone 15 Pro
  ○ iPhone 15

→ Total: 5 simulators available

───── Usage
  • drift device build --simulator                    # Build for default simulator
  • drift device build --simulator "iPhone 16 Pro"    # Build for specific simulator
  • drift device run --simulator                      # Build, install, and run
```

## drift device list

List all connected iOS devices.

```bash
drift device list
```

Shows connected devices and their configuration status:

```
╔══════════════════════════════════════════════════════════════╗
║  iOS Devices                                                 ║
╚══════════════════════════════════════════════════════════════╝

───── Connected Devices
  My iPhone (configured)
    UDID:  00008120-001234567890
    Model: iPhone16,1
    iOS:   18.0

  Test Device
    UDID:  00008120-009876543210
    Model: iPhone15,2
    iOS:   17.5

───── Configured (Not Connected)
  Development iPhone
    UDID:  00008120-001111111111
```

## drift device start

Start WebDriverAgent on a device for MCP automation and testing.

```bash
drift device start [device] [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--quick`, `-q` | Skip rebuild if WDA is already running |

This command sets up:
1. iOS tunnel (required for iOS 17+)
2. Port forwarding (localhost:8100 → device:8100)
3. WebDriverAgent build and launch

## drift device stop

Stop WebDriverAgent and cleanup all related processes.

```bash
drift device stop
```

## drift device status

Check WebDriverAgent and tunnel status.

```bash
drift device status
```

**Example Output:**

```
╔══════════════════════════════════════════════════════════════╗
║  Device Status                                               ║
╚══════════════════════════════════════════════════════════════╝

  WDA:     RUNNING
  URL:     http://localhost:8100
  Tunnel:  HEALTHY
  Forward: ACTIVE

───── Connected Devices
  My iPhone (configured)
```

## Requirements

- **For physical devices:** [go-ios](https://github.com/danielpaulus/go-ios) (`brew install go-ios`)
- **For simulators:** Xcode with iOS Simulator installed
- **For all builds:** Xcode and valid code signing setup

## See Also

- [Xcode Management](xcode.md)
- [Configuration](../config/drift-yaml.md)
