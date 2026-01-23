package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/undrift/drift/internal/config"
	"github.com/undrift/drift/internal/ui"
	"github.com/undrift/drift/pkg/shell"
)

var deviceCmd = &cobra.Command{
	Use:   "device",
	Short: "Mobile device automation and builds",
	Long: `Manage mobile device connections, builds, and automation.

This command provides tools for:
  - Listing and selecting connected iOS devices
  - Building and installing apps to devices
  - Setting up WebDriverAgent (WDA) for MCP automation
  - Managing iOS tunnels and port forwarding`,
}

var deviceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List connected and configured devices",
	Long:  `List all connected iOS devices and show which ones are configured in .drift.yaml.`,
	RunE:  runDeviceList,
}

var deviceStartCmd = &cobra.Command{
	Use:   "start [device]",
	Short: "Start WebDriverAgent for MCP automation",
	Long: `Start WebDriverAgent on a device for MCP automation and testing.

If no device is specified, shows an interactive picker of connected devices.

This sets up:
  1. iOS tunnel (required for iOS 17+)
  2. Port forwarding (localhost:8100 -> device:8100)
  3. WebDriverAgent build and launch

Examples:
  drift device start                     # Interactive device picker
  drift device start "Test dummy"        # Start on named device
  drift device start 00008120-xxx        # Start on device by UDID
  drift device start --quick             # Skip rebuild if WDA running`,
	Args: cobra.MaximumNArgs(1),
	RunE: runDeviceStart,
}

var deviceStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop WebDriverAgent and cleanup",
	Long:  `Stop WebDriverAgent and cleanup all related processes.`,
	RunE:  runDeviceStop,
}

var deviceStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check WebDriverAgent status",
	Long:  `Check if WebDriverAgent is running and show connection status.`,
	RunE:  runDeviceStatus,
}

var deviceBuildCmd = &cobra.Command{
	Use:   "build [device]",
	Short: "Build and install app to device",
	Long: `Build the app and install it to a connected device.

If no device is specified, shows an interactive picker.
You can also select which scheme to build.

Examples:
  drift device build                          # Interactive picker
  drift device build "Test dummy"             # Build to named device
  drift device build --scheme "App (Debug)"   # Use specific scheme
  drift device build --run                    # Build, install, and run`,
	Args: cobra.MaximumNArgs(1),
	RunE: runDeviceBuild,
}

var deviceRunCmd = &cobra.Command{
	Use:   "run [device]",
	Short: "Build, install, and run app on device",
	Long: `Build the app, install it to a device, and launch it.

Shorthand for: drift device build --run

Examples:
  drift device run                     # Interactive picker
  drift device run "Test dummy"        # Run on named device`,
	Args: cobra.MaximumNArgs(1),
	RunE: runDeviceRun,
}

var deviceSimulatorsCmd = &cobra.Command{
	Use:   "simulators",
	Short: "List available iOS simulators",
	Long:  `List all available iOS simulators that can be used for building and testing.`,
	RunE:  runDeviceSimulators,
}

var (
	deviceQuickFlag     bool
	deviceSchemeFlag    string
	deviceRunFlag       bool
	deviceSimulatorFlag string
)

func init() {
	deviceStartCmd.Flags().BoolVarP(&deviceQuickFlag, "quick", "q", false, "Skip rebuild if WDA is already running")
	deviceBuildCmd.Flags().StringVarP(&deviceSchemeFlag, "scheme", "s", "", "Xcode scheme to build")
	deviceBuildCmd.Flags().BoolVarP(&deviceRunFlag, "run", "r", false, "Run app after installing")
	deviceBuildCmd.Flags().StringVar(&deviceSimulatorFlag, "simulator", "", "Build for simulator (use device name or 'default' for iPhone 16 Pro)")
	deviceRunCmd.Flags().StringVar(&deviceSimulatorFlag, "simulator", "", "Build and run on simulator (use device name or 'default')")

	deviceCmd.AddCommand(deviceListCmd)
	deviceCmd.AddCommand(deviceStartCmd)
	deviceCmd.AddCommand(deviceStopCmd)
	deviceCmd.AddCommand(deviceStatusCmd)
	deviceCmd.AddCommand(deviceBuildCmd)
	deviceCmd.AddCommand(deviceRunCmd)
	deviceCmd.AddCommand(deviceSimulatorsCmd)
	rootCmd.AddCommand(deviceCmd)
}

// ConnectedDevice represents a device from ios list
type ConnectedDevice struct {
	UDID       string
	Name       string
	Model      string
	OS         string
	Configured bool // Whether this device is in our config
	ConfigName string
}

// getConnectedDevices returns all connected iOS devices
func getConnectedDevices(cfg *config.Config) ([]ConnectedDevice, error) {
	// First get list of UDIDs
	listResult, err := shell.Run("ios", "list")
	if err != nil {
		return nil, fmt.Errorf("failed to list devices (is go-ios installed?): %w", err)
	}

	// Parse JSON: {"deviceList":["udid1", "udid2"]}
	var deviceList struct {
		DeviceList []string `json:"deviceList"`
	}
	if err := json.Unmarshal([]byte(listResult.Stdout), &deviceList); err != nil {
		return nil, fmt.Errorf("failed to parse device list: %w", err)
	}

	var devices []ConnectedDevice
	for _, udid := range deviceList.DeviceList {
		device := ConnectedDevice{
			UDID: udid,
			Name: "Unknown Device",
		}

		// Try to get device info
		infoResult, _ := shell.Run("ios", "info", fmt.Sprintf("--udid=%s", udid))
		if infoResult != nil && infoResult.ExitCode == 0 {
			var info map[string]interface{}
			if json.Unmarshal([]byte(infoResult.Stdout), &info) == nil {
				if name, ok := info["DeviceName"].(string); ok {
					device.Name = name
				}
				if model, ok := info["ProductType"].(string); ok {
					device.Model = model
				}
				if os, ok := info["ProductVersion"].(string); ok {
					device.OS = os
				}
			}
		}

		// Check if device is in config
		if cfg != nil {
			for _, d := range cfg.Device.Devices {
				if d.UDID == udid {
					device.Configured = true
					device.ConfigName = d.Name
					if device.Name == "Unknown Device" {
						device.Name = d.Name
					}
					break
				}
			}
		}

		devices = append(devices, device)
	}

	return devices, nil
}

// selectDevice shows an interactive picker for connected devices
func selectDevice(cfg *config.Config, prompt string) (*ConnectedDevice, error) {
	devices, err := getConnectedDevices(cfg)
	if err != nil {
		return nil, err
	}

	if len(devices) == 0 {
		return nil, fmt.Errorf("no devices connected\n\nConnect an iOS device via USB and try again")
	}

	if len(devices) == 1 {
		// Only one device, use it
		ui.Infof("Using device: %s", devices[0].Name)
		return &devices[0], nil
	}

	// Multiple devices - show picker
	options := make([]string, len(devices))
	for i, d := range devices {
		configured := ""
		if d.Configured {
			configured = ui.Green(" (configured)")
		}
		osInfo := ""
		if d.OS != "" {
			osInfo = fmt.Sprintf(" - iOS %s", d.OS)
		}
		options[i] = fmt.Sprintf("%s%s%s", d.Name, osInfo, configured)
	}

	idx, _, err := ui.PromptSelectWithIndex(prompt, options)
	if err != nil {
		return nil, err
	}

	return &devices[idx], nil
}

// findDeviceByNameOrUDID finds a device by name or UDID
func findDeviceByNameOrUDID(cfg *config.Config, query string) (*ConnectedDevice, error) {
	devices, err := getConnectedDevices(cfg)
	if err != nil {
		return nil, err
	}

	// First try exact UDID match
	for i, d := range devices {
		if d.UDID == query {
			return &devices[i], nil
		}
	}

	// Then try name match (case insensitive)
	queryLower := strings.ToLower(query)
	for i, d := range devices {
		if strings.ToLower(d.Name) == queryLower || strings.ToLower(d.ConfigName) == queryLower {
			return &devices[i], nil
		}
	}

	// Try partial UDID match
	for i, d := range devices {
		if strings.HasPrefix(d.UDID, query) {
			return &devices[i], nil
		}
	}

	return nil, fmt.Errorf("device '%s' not found or not connected", query)
}

func runDeviceList(cmd *cobra.Command, args []string) error {
	cfg := config.LoadOrDefault()

	ui.Header("iOS Devices")

	devices, err := getConnectedDevices(cfg)
	if err != nil {
		ui.Warning(fmt.Sprintf("Could not list devices: %v", err))
		ui.NewLine()
		ui.Info("Make sure go-ios is installed: brew install go-ios")
		return nil
	}

	if len(devices) == 0 {
		ui.Info("No devices connected")
		ui.NewLine()
		ui.Info("Connect an iOS device via USB to get started")
		return nil
	}

	ui.SubHeader("Connected Devices")
	for _, d := range devices {
		configured := ""
		if d.Configured {
			configured = ui.Green(" (configured)")
		}
		fmt.Printf("  %s%s\n", ui.Cyan(d.Name), configured)
		fmt.Printf("    UDID:  %s\n", d.UDID)
		if d.Model != "" {
			fmt.Printf("    Model: %s\n", d.Model)
		}
		if d.OS != "" {
			fmt.Printf("    iOS:   %s\n", d.OS)
		}
		fmt.Println()
	}

	// Show configured but not connected
	if len(cfg.Device.Devices) > 0 {
		var notConnected []config.DeviceEntry
		for _, cfgDevice := range cfg.Device.Devices {
			found := false
			for _, connected := range devices {
				if connected.UDID == cfgDevice.UDID {
					found = true
					break
				}
			}
			if !found {
				notConnected = append(notConnected, cfgDevice)
			}
		}

		if len(notConnected) > 0 {
			ui.SubHeader("Configured (Not Connected)")
			for _, d := range notConnected {
				fmt.Printf("  %s\n", ui.Dim(d.Name))
				fmt.Printf("    UDID:  %s\n", ui.Dim(d.UDID))
				fmt.Println()
			}
		}
	}

	return nil
}

func runDeviceStart(cmd *cobra.Command, args []string) error {
	cfg := config.LoadOrDefault()

	// Check dependencies
	if err := checkDeviceDependencies(); err != nil {
		return err
	}

	wdaPort := cfg.Device.WDAPort
	if wdaPort == 0 {
		wdaPort = 8100
	}

	// Quick mode: check if WDA is already running
	if deviceQuickFlag && checkWDAStatus(wdaPort) {
		ui.Success(fmt.Sprintf("WDA is already running at http://localhost:%d", wdaPort))
		return nil
	}

	ui.Header("Start WebDriverAgent")

	// Select device
	var device *ConnectedDevice
	var err error

	if len(args) > 0 {
		device, err = findDeviceByNameOrUDID(cfg, args[0])
	} else {
		device, err = selectDevice(cfg, "Select device for WDA")
	}

	if err != nil {
		return err
	}

	ui.NewLine()
	ui.KeyValue("Device", ui.Cyan(device.Name))
	ui.KeyValue("UDID", device.UDID)
	if device.OS != "" {
		ui.KeyValue("iOS", device.OS)
	}
	ui.KeyValue("WDA Port", fmt.Sprintf("%d", wdaPort))
	ui.NewLine()

	// Step 1: Start iOS tunnel
	sp := ui.NewSpinner("Starting iOS tunnel...")
	sp.Start()
	if err := ensureIOSTunnel(); err != nil {
		sp.Fail("Failed to start iOS tunnel")
		return err
	}
	sp.Success("iOS tunnel ready")

	// Step 2: Start port forwarding
	sp = ui.NewSpinner("Setting up port forwarding...")
	sp.Start()
	if err := startPortForwarding(device.UDID, wdaPort); err != nil {
		sp.Fail("Failed to start port forwarding")
		return err
	}
	sp.Success("Port forwarding ready")

	// Step 3: Build and run WDA
	wdaPath := cfg.Device.WDAPath
	if wdaPath == "" {
		wdaPath = "/tmp/WebDriverAgent"
	}
	devTeam := cfg.Apple.TeamID // Use apns.team_id (same Apple Developer Team)

	// Clone WDA if needed
	if _, err := os.Stat(wdaPath); os.IsNotExist(err) {
		sp = ui.NewSpinner("Cloning WebDriverAgent...")
		sp.Start()
		result, err := shell.Run("git", "clone", "https://github.com/appium/WebDriverAgent.git", wdaPath)
		if err != nil {
			sp.Fail("Failed to clone WebDriverAgent")
			return fmt.Errorf("git clone failed: %s", result.Stderr)
		}
		sp.Success("WebDriverAgent cloned")
	}

	ui.NewLine()
	ui.Warning("Building WebDriverAgent - watch your device!")
	ui.Info("You may need to:")
	ui.List("Enter your Mac password for code signing")
	ui.List("Trust the developer certificate on your device")
	ui.List("  Settings > General > VPN & Device Management")
	ui.NewLine()

	// Build WDA interactively so user can see output
	wdaArgs := []string{
		"-project", filepath.Join(wdaPath, "WebDriverAgent.xcodeproj"),
		"-scheme", "WebDriverAgentRunner",
		"-destination", fmt.Sprintf("id=%s", device.UDID),
		"-derivedDataPath", filepath.Join(os.Getenv("HOME"), "Library/Developer/Xcode/DerivedData/WDA-Drift"),
		"-allowProvisioningUpdates",
	}
	if devTeam != "" {
		wdaArgs = append(wdaArgs, fmt.Sprintf("DEVELOPMENT_TEAM=%s", devTeam))
	}
	wdaArgs = append(wdaArgs, "test")

	// Run xcodebuild in background
	wdaCmd := exec.Command("xcodebuild", wdaArgs...)
	wdaCmd.Dir = wdaPath
	wdaCmd.Stdout = os.Stdout
	wdaCmd.Stderr = os.Stderr

	if err := wdaCmd.Start(); err != nil {
		return fmt.Errorf("failed to start xcodebuild: %w", err)
	}

	// Wait for WDA to respond
	ui.NewLine()
	sp = ui.NewSpinner("Waiting for WebDriverAgent to respond...")
	sp.Start()

	for i := 0; i < 90; i++ { // 3 minutes timeout
		if checkWDAStatus(wdaPort) {
			sp.Success("WebDriverAgent is ready!")
			break
		}
		time.Sleep(2 * time.Second)
	}

	if !checkWDAStatus(wdaPort) {
		sp.Fail("Timeout waiting for WebDriverAgent")
		ui.NewLine()
		ui.Warning("WDA may still be building. Check the output above.")
		ui.Info("Common issues:")
		ui.List("Developer certificate not trusted on device")
		ui.List("Device is locked")
		ui.List("Provisioning profile issues")
		return nil
	}

	ui.NewLine()
	ui.Success(fmt.Sprintf("WDA ready at http://localhost:%d", wdaPort))
	ui.NewLine()
	ui.SubHeader("Quick Commands")
	ui.List(fmt.Sprintf("Test:   curl http://localhost:%d/status", wdaPort))
	ui.List("Stop:   drift device stop")
	ui.List("Status: drift device status")
	ui.NewLine()
	ui.Info("Press Ctrl+C to stop WDA")

	// Wait for xcodebuild to finish (or be killed)
	wdaCmd.Wait()

	return nil
}

func runDeviceStop(cmd *cobra.Command, args []string) error {
	ui.Header("Stop WebDriverAgent")

	// Kill xcodebuild WDA
	shell.Run("pkill", "-f", "xcodebuild.*WebDriverAgent")
	ui.Success("Stopped WebDriverAgent build")

	// Kill port forwarding
	shell.Run("pkill", "-f", "ios forward")
	ui.Success("Stopped port forwarding")

	// Kill tunnel
	shell.Run("pkill", "-f", "ios tunnel")
	ui.Success("Stopped iOS tunnel")

	ui.NewLine()
	ui.Success("All device processes stopped")

	return nil
}

func runDeviceStatus(cmd *cobra.Command, args []string) error {
	cfg := config.LoadOrDefault()

	wdaPort := cfg.Device.WDAPort
	if wdaPort == 0 {
		wdaPort = 8100
	}

	ui.Header("Device Status")
	ui.NewLine()

	// WDA Status
	if checkWDAStatus(wdaPort) {
		ui.KeyValue("WDA", ui.Green("RUNNING"))
		ui.KeyValue("URL", fmt.Sprintf("http://localhost:%d", wdaPort))
	} else {
		ui.KeyValue("WDA", ui.Red("NOT RUNNING"))
	}

	// Tunnel status
	result, _ := shell.Run("pgrep", "-f", "ios tunnel")
	if result != nil && result.ExitCode == 0 {
		// Test if tunnel is healthy
		testResult, _ := shell.Run("ios", "list")
		if testResult != nil && testResult.ExitCode == 0 && strings.Contains(testResult.Stdout, "deviceList") {
			ui.KeyValue("Tunnel", ui.Green("HEALTHY"))
		} else {
			ui.KeyValue("Tunnel", ui.Yellow("STALE"))
		}
	} else {
		ui.KeyValue("Tunnel", ui.Red("NOT RUNNING"))
	}

	// Port forwarding
	result, _ = shell.Run("pgrep", "-f", fmt.Sprintf("ios forward %d", wdaPort))
	if result != nil && result.ExitCode == 0 {
		ui.KeyValue("Forward", ui.Green("ACTIVE"))
	} else {
		ui.KeyValue("Forward", ui.Red("NOT RUNNING"))
	}

	ui.NewLine()

	// Connected devices
	devices, err := getConnectedDevices(cfg)
	if err != nil {
		ui.KeyValue("Devices", ui.Red("ERROR"))
	} else if len(devices) == 0 {
		ui.KeyValue("Devices", ui.Dim("None connected"))
	} else {
		ui.SubHeader("Connected Devices")
		for _, d := range devices {
			marker := ""
			if d.Configured {
				marker = ui.Green(" (configured)")
			}
			fmt.Printf("  %s%s\n", ui.Cyan(d.Name), marker)
		}
	}

	return nil
}

func runDeviceBuild(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}
	cfg := config.LoadOrDefault()

	// Check this is an iOS project
	if cfg.Project.Type != "ios" && cfg.Project.Type != "multiplatform" {
		return fmt.Errorf("device build is only available for iOS projects")
	}

	// Check if building for simulator
	if deviceSimulatorFlag != "" {
		return runSimulatorBuild(cmd, args, cfg)
	}

	ui.Header("Build to Device")

	// Select device
	var device *ConnectedDevice
	var err error

	if len(args) > 0 {
		device, err = findDeviceByNameOrUDID(cfg, args[0])
	} else {
		device, err = selectDevice(cfg, "Select device to build to")
	}

	if err != nil {
		return err
	}

	ui.NewLine()
	ui.KeyValue("Device", ui.Cyan(device.Name))
	ui.KeyValue("UDID", device.UDID)

	// Find Xcode project/workspace
	projectRoot := cfg.ProjectRoot()
	var xcodeFile string
	var xcodeType string // "workspace" or "project"

	// Prefer workspace
	workspaces, _ := filepath.Glob(filepath.Join(projectRoot, "*.xcworkspace"))
	if len(workspaces) > 0 {
		xcodeFile = workspaces[0]
		xcodeType = "workspace"
	} else {
		projects, _ := filepath.Glob(filepath.Join(projectRoot, "*.xcodeproj"))
		if len(projects) > 0 {
			xcodeFile = projects[0]
			xcodeType = "project"
		}
	}

	if xcodeFile == "" {
		return fmt.Errorf("no Xcode project or workspace found in %s", projectRoot)
	}

	ui.KeyValue("Project", filepath.Base(xcodeFile))

	// Get or select scheme
	scheme := deviceSchemeFlag
	if scheme == "" {
		// List available schemes
		schemes, err := getXcodeSchemes(xcodeFile, xcodeType)
		if err != nil {
			ui.Warning(fmt.Sprintf("Could not list schemes: %v", err))
			scheme, err = ui.PromptString("Enter scheme name", "")
			if err != nil {
				return err
			}
		} else if len(schemes) == 1 {
			scheme = schemes[0]
			ui.KeyValue("Scheme", scheme)
		} else {
			idx, _, err := ui.PromptSelectWithIndex("Select scheme", schemes)
			if err != nil {
				return err
			}
			scheme = schemes[idx]
		}
	} else {
		ui.KeyValue("Scheme", scheme)
	}

	ui.NewLine()

	// Confirm
	if !IsYes() {
		confirmed, _ := ui.PromptYesNo("Build and install to device?", true)
		if !confirmed {
			ui.Info("Cancelled")
			return nil
		}
	}

	ui.NewLine()

	// Build
	buildArgs := []string{
		fmt.Sprintf("-%s", xcodeType), xcodeFile,
		"-scheme", scheme,
		"-destination", fmt.Sprintf("id=%s", device.UDID),
		"-allowProvisioningUpdates",
	}

	if cfg.Apple.TeamID != "" {
		buildArgs = append(buildArgs, fmt.Sprintf("DEVELOPMENT_TEAM=%s", cfg.Apple.TeamID))
	}

	if deviceRunFlag {
		// Build and run
		buildArgs = append(buildArgs, "build")
		ui.Info("Building...")
	} else {
		// Just build and install
		buildArgs = append(buildArgs, "build")
		ui.Info("Building and installing...")
	}

	// Run xcodebuild interactively
	buildCmd := exec.Command("xcodebuild", buildArgs...)
	buildCmd.Dir = projectRoot
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	buildCmd.Stdin = os.Stdin

	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	ui.NewLine()
	ui.Success("Build complete!")

	if deviceRunFlag {
		ui.NewLine()
		ui.Info("Launching app...")
		// Find bundle ID from scheme or ask user
		bundleID, _ := ui.PromptString("Enter bundle ID to launch (or press Enter to skip)", "")
		if bundleID != "" {
			launchResult, err := shell.Run("ios", "launch", bundleID, fmt.Sprintf("--udid=%s", device.UDID))
			if err != nil {
				ui.Warning(fmt.Sprintf("Failed to launch: %s", launchResult.Stderr))
			} else {
				ui.Success("App launched!")
			}
		}
	}

	return nil
}

func runDeviceRun(cmd *cobra.Command, args []string) error {
	deviceRunFlag = true
	return runDeviceBuild(cmd, args)
}

// Helper functions

func checkWDAStatus(port int) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%d/status", port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func checkDeviceDependencies() error {
	// Check for ios (go-ios)
	if _, err := exec.LookPath("ios"); err != nil {
		return fmt.Errorf("go-ios not found\n\nInstall with: brew install go-ios")
	}

	// Check for xcodebuild
	if _, err := exec.LookPath("xcodebuild"); err != nil {
		return fmt.Errorf("xcodebuild not found\n\nPlease install Xcode from the App Store")
	}

	// Check for git (needed to clone WebDriverAgent)
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git not found\n\nInstall with: xcode-select --install")
	}

	return nil
}

func ensureIOSTunnel() error {
	// Check if already running and healthy
	result, _ := shell.Run("pgrep", "-f", "ios tunnel")
	if result != nil && result.ExitCode == 0 {
		// Test if tunnel is healthy
		testResult, _ := shell.Run("ios", "list")
		if testResult != nil && testResult.ExitCode == 0 && strings.Contains(testResult.Stdout, "deviceList") {
			return nil // Already running and healthy
		}
		// Stale tunnel, kill it
		shell.Run("pkill", "-f", "ios tunnel")
		time.Sleep(2 * time.Second)
	}

	// Start tunnel in background
	cmd := exec.Command("ios", "tunnel", "start", "--userspace")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return err
	}

	// Wait for tunnel to initialize
	time.Sleep(3 * time.Second)

	// Verify it started
	testResult, _ := shell.Run("ios", "list")
	if testResult == nil || testResult.ExitCode != 0 {
		return fmt.Errorf("tunnel started but not responding")
	}

	return nil
}

func startPortForwarding(udid string, port int) error {
	// Kill existing
	shell.Run("pkill", "-f", fmt.Sprintf("ios forward %d", port))
	time.Sleep(1 * time.Second)

	// Start forwarding in background
	cmd := exec.Command("ios", "forward", fmt.Sprintf("%d", port), fmt.Sprintf("%d", port), fmt.Sprintf("--udid=%s", udid))
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return err
	}

	time.Sleep(2 * time.Second)
	return nil
}

func getXcodeSchemes(xcodeFile string, xcodeType string) ([]string, error) {
	args := []string{
		fmt.Sprintf("-%s", xcodeType), xcodeFile,
		"-list",
		"-json",
	}

	result, err := shell.Run("xcodebuild", args...)
	if err != nil {
		return nil, err
	}

	// Parse JSON output
	var listOutput map[string]interface{}
	if err := json.Unmarshal([]byte(result.Stdout), &listOutput); err != nil {
		// Fall back to text parsing
		return parseXcodeSchemesText(result.Stdout), nil
	}

	// Extract schemes from JSON
	var schemes []string
	if xcodeType == "workspace" {
		if ws, ok := listOutput["workspace"].(map[string]interface{}); ok {
			if schemeList, ok := ws["schemes"].([]interface{}); ok {
				for _, s := range schemeList {
					if name, ok := s.(string); ok {
						schemes = append(schemes, name)
					}
				}
			}
		}
	} else {
		if proj, ok := listOutput["project"].(map[string]interface{}); ok {
			if schemeList, ok := proj["schemes"].([]interface{}); ok {
				for _, s := range schemeList {
					if name, ok := s.(string); ok {
						schemes = append(schemes, name)
					}
				}
			}
		}
	}

	return schemes, nil
}

func parseXcodeSchemesText(output string) []string {
	var schemes []string
	inSchemes := false

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "Schemes:" {
			inSchemes = true
			continue
		}
		if inSchemes {
			if line == "" || strings.HasSuffix(line, ":") {
				break
			}
			schemes = append(schemes, line)
		}
	}

	return schemes
}

// Simulator represents an iOS simulator.
type Simulator struct {
	UDID        string `json:"udid"`
	Name        string `json:"name"`
	DeviceType  string `json:"deviceTypeIdentifier"`
	State       string `json:"state"`
	IsAvailable bool   `json:"isAvailable"`
	Runtime     string `json:"-"` // Parsed from runtime key
}

// getSimulators returns available iOS simulators.
func getSimulators() ([]Simulator, error) {
	result, err := shell.Run("xcrun", "simctl", "list", "devices", "-j")
	if err != nil {
		return nil, fmt.Errorf("failed to list simulators: %w", err)
	}

	var output struct {
		Devices map[string][]Simulator `json:"devices"`
	}

	if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
		return nil, fmt.Errorf("failed to parse simulator list: %w", err)
	}

	var simulators []Simulator
	for runtime, devices := range output.Devices {
		// Only include iOS simulators
		if !strings.Contains(runtime, "iOS") {
			continue
		}

		// Extract iOS version from runtime
		// Format: com.apple.CoreSimulator.SimRuntime.iOS-17-5
		iosVersion := ""
		if idx := strings.LastIndex(runtime, "iOS-"); idx != -1 {
			iosVersion = strings.ReplaceAll(runtime[idx+4:], "-", ".")
		}

		for _, sim := range devices {
			if sim.IsAvailable {
				sim.Runtime = iosVersion
				simulators = append(simulators, sim)
			}
		}
	}

	return simulators, nil
}

// selectSimulator shows an interactive picker for available simulators.
func selectSimulator(prompt string) (*Simulator, error) {
	simulators, err := getSimulators()
	if err != nil {
		return nil, err
	}

	if len(simulators) == 0 {
		return nil, fmt.Errorf("no simulators available\n\nCreate one in Xcode: Window > Devices and Simulators")
	}

	// Group by common device types for easier selection
	options := make([]string, len(simulators))
	for i, sim := range simulators {
		stateIcon := ""
		if sim.State == "Booted" {
			stateIcon = ui.Green(" (running)")
		}
		options[i] = fmt.Sprintf("%s - iOS %s%s", sim.Name, sim.Runtime, stateIcon)
	}

	idx, _, err := ui.PromptSelectWithIndex(prompt, options)
	if err != nil {
		return nil, err
	}

	return &simulators[idx], nil
}

// findSimulatorByName finds a simulator by name (partial match supported).
func findSimulatorByName(name string) (*Simulator, error) {
	simulators, err := getSimulators()
	if err != nil {
		return nil, err
	}

	// Handle default
	if name == "default" || name == "" {
		// Find iPhone 16 Pro or similar
		defaultNames := []string{"iPhone 16 Pro", "iPhone 15 Pro", "iPhone 14 Pro", "iPhone"}
		for _, defaultName := range defaultNames {
			for i, sim := range simulators {
				if strings.Contains(sim.Name, defaultName) {
					return &simulators[i], nil
				}
			}
		}
		// Just return the first one
		if len(simulators) > 0 {
			return &simulators[0], nil
		}
		return nil, fmt.Errorf("no simulators available")
	}

	// Exact match first
	for i, sim := range simulators {
		if sim.Name == name {
			return &simulators[i], nil
		}
	}

	// Partial match
	nameLower := strings.ToLower(name)
	for i, sim := range simulators {
		if strings.Contains(strings.ToLower(sim.Name), nameLower) {
			return &simulators[i], nil
		}
	}

	return nil, fmt.Errorf("simulator '%s' not found", name)
}

// bootSimulatorIfNeeded boots a simulator if it's not already running.
func bootSimulatorIfNeeded(sim *Simulator) error {
	if sim.State == "Booted" {
		return nil
	}

	ui.Infof("Booting simulator: %s", sim.Name)
	_, err := shell.Run("xcrun", "simctl", "boot", sim.UDID)
	if err != nil {
		return fmt.Errorf("failed to boot simulator: %w", err)
	}

	// Give it a moment to start
	time.Sleep(2 * time.Second)
	return nil
}

// runSimulatorBuild handles building and running on iOS simulators.
func runSimulatorBuild(cmd *cobra.Command, args []string, cfg *config.Config) error {
	ui.Header("Build to Simulator")

	// Select or find simulator
	var sim *Simulator
	var err error

	if deviceSimulatorFlag == "default" || deviceSimulatorFlag == "" {
		sim, err = findSimulatorByName("default")
	} else {
		sim, err = findSimulatorByName(deviceSimulatorFlag)
	}

	if err != nil {
		// If not found by name, show picker
		sim, err = selectSimulator("Select simulator")
		if err != nil {
			return err
		}
	}

	ui.NewLine()
	ui.KeyValue("Simulator", ui.Cyan(sim.Name))
	ui.KeyValue("iOS", sim.Runtime)
	ui.KeyValue("State", sim.State)

	// Find Xcode project/workspace
	projectRoot := cfg.ProjectRoot()
	var xcodeFile string
	var xcodeType string

	workspaces, _ := filepath.Glob(filepath.Join(projectRoot, "*.xcworkspace"))
	if len(workspaces) > 0 {
		xcodeFile = workspaces[0]
		xcodeType = "workspace"
	} else {
		projects, _ := filepath.Glob(filepath.Join(projectRoot, "*.xcodeproj"))
		if len(projects) > 0 {
			xcodeFile = projects[0]
			xcodeType = "project"
		}
	}

	if xcodeFile == "" {
		return fmt.Errorf("no Xcode project or workspace found in %s", projectRoot)
	}

	ui.KeyValue("Project", filepath.Base(xcodeFile))

	// Get or select scheme
	scheme := deviceSchemeFlag
	if scheme == "" {
		schemes, err := getXcodeSchemes(xcodeFile, xcodeType)
		if err != nil {
			ui.Warning(fmt.Sprintf("Could not list schemes: %v", err))
			scheme, err = ui.PromptString("Enter scheme name", "")
			if err != nil {
				return err
			}
		} else if len(schemes) == 1 {
			scheme = schemes[0]
			ui.KeyValue("Scheme", scheme)
		} else {
			idx, _, err := ui.PromptSelectWithIndex("Select scheme", schemes)
			if err != nil {
				return err
			}
			scheme = schemes[idx]
		}
	} else {
		ui.KeyValue("Scheme", scheme)
	}

	ui.NewLine()

	// Confirm
	if !IsYes() {
		confirmed, _ := ui.PromptYesNo("Build for simulator?", true)
		if !confirmed {
			ui.Info("Cancelled")
			return nil
		}
	}

	// Boot simulator if needed
	if err := bootSimulatorIfNeeded(sim); err != nil {
		ui.Warning(fmt.Sprintf("Could not boot simulator: %v", err))
	}

	ui.NewLine()

	// Build for simulator
	destination := fmt.Sprintf("platform=iOS Simulator,name=%s", sim.Name)
	buildArgs := []string{
		fmt.Sprintf("-%s", xcodeType), xcodeFile,
		"-scheme", scheme,
		"-destination", destination,
		"-configuration", "Debug",
	}

	if deviceRunFlag {
		buildArgs = append(buildArgs, "build")
		ui.Info("Building for simulator...")
	} else {
		buildArgs = append(buildArgs, "build")
		ui.Info("Building for simulator...")
	}

	// Run xcodebuild interactively
	buildCmd := exec.Command("xcodebuild", buildArgs...)
	buildCmd.Dir = projectRoot
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	buildCmd.Stdin = os.Stdin

	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	ui.NewLine()
	ui.Success("Build complete!")

	if deviceRunFlag {
		ui.NewLine()
		ui.Info("Opening Simulator...")
		shell.RunInteractive("open", "-a", "Simulator")

		ui.Info("The app should launch automatically in the simulator")
	}

	return nil
}

func runDeviceSimulators(cmd *cobra.Command, args []string) error {
	ui.Header("iOS Simulators")

	simulators, err := getSimulators()
	if err != nil {
		return err
	}

	if len(simulators) == 0 {
		ui.Info("No simulators found")
		ui.NewLine()
		ui.Info("Create one in Xcode: Window > Devices and Simulators")
		return nil
	}

	// Group by runtime
	byRuntime := make(map[string][]Simulator)
	for _, sim := range simulators {
		byRuntime[sim.Runtime] = append(byRuntime[sim.Runtime], sim)
	}

	// Get sorted runtime versions
	var runtimes []string
	for r := range byRuntime {
		runtimes = append(runtimes, r)
	}
	// Sort in reverse (newest first)
	for i, j := 0, len(runtimes)-1; i < j; i, j = i+1, j-1 {
		runtimes[i], runtimes[j] = runtimes[j], runtimes[i]
	}

	for _, runtime := range runtimes {
		sims := byRuntime[runtime]
		ui.SubHeader(fmt.Sprintf("iOS %s", runtime))

		for _, sim := range sims {
			stateIcon := ui.Dim("○")
			if sim.State == "Booted" {
				stateIcon = ui.Green("●")
			}
			fmt.Printf("  %s %s\n", stateIcon, sim.Name)
		}
		ui.NewLine()
	}

	ui.Infof("Total: %d simulators available", len(simulators))

	// Show usage hints
	ui.NewLine()
	ui.SubHeader("Usage")
	ui.List("drift device build --simulator                    # Build for default simulator")
	ui.List("drift device build --simulator \"iPhone 16 Pro\"    # Build for specific simulator")
	ui.List("drift device run --simulator                      # Build, install, and run")

	return nil
}
