package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/undrift/drift/internal/ui"
	"github.com/undrift/drift/pkg/shell"
)

var fastlaneCmd = &cobra.Command{
	Use:   "fastlane",
	Short: "Fastlane CI/CD operations",
	Long: `Run Fastlane lanes for iOS CI/CD operations.

These commands wrap common Fastlane lanes for building, testing,
and deploying iOS apps.

Commands:
  test         - Run tests via fastlane
  build        - Build for testing
  beta         - Build and upload to TestFlight
  release      - Build and upload to App Store
  screenshots  - Capture App Store screenshots
  match sync   - Sync code signing certificates`,
	Example: `  drift fastlane test          # Run tests
  drift fastlane build         # Build for testing
  drift fastlane beta          # Upload to TestFlight
  drift fastlane release       # Upload to App Store
  drift fastlane screenshots   # Capture screenshots
  drift fastlane match sync    # Sync certificates`,
}

var fastlaneTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Run tests via fastlane",
	Long:  `Run the test lane in Fastlane to execute all tests.`,
	RunE:  runFastlaneTest,
}

var fastlaneBuildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build for testing",
	Long:  `Run the build lane in Fastlane to build the app for testing.`,
	RunE:  runFastlaneBuild,
}

var fastlaneBetaCmd = &cobra.Command{
	Use:   "beta",
	Short: "Build and upload to TestFlight",
	Long:  `Run the beta lane in Fastlane to build and upload to TestFlight.`,
	RunE:  runFastlaneBeta,
}

var fastlaneReleaseCmd = &cobra.Command{
	Use:   "release",
	Short: "Build and upload to App Store",
	Long:  `Run the release lane in Fastlane to build and upload to App Store.`,
	RunE:  runFastlaneRelease,
}

var fastlaneScreenshotsCmd = &cobra.Command{
	Use:   "screenshots",
	Short: "Capture App Store screenshots",
	Long:  `Run the screenshots lane in Fastlane to capture App Store screenshots.`,
	RunE:  runFastlaneScreenshots,
}

var fastlaneMatchCmd = &cobra.Command{
	Use:   "match",
	Short: "Code signing certificate management",
	Long:  `Manage code signing certificates using Fastlane Match.`,
}

var fastlaneMatchSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync code signing certificates",
	Long:  `Run fastlane match to sync code signing certificates from your certificate repository.`,
	RunE:  runFastlaneMatchSync,
}

func init() {
	fastlaneMatchCmd.AddCommand(fastlaneMatchSyncCmd)

	fastlaneCmd.AddCommand(fastlaneTestCmd)
	fastlaneCmd.AddCommand(fastlaneBuildCmd)
	fastlaneCmd.AddCommand(fastlaneBetaCmd)
	fastlaneCmd.AddCommand(fastlaneReleaseCmd)
	fastlaneCmd.AddCommand(fastlaneScreenshotsCmd)
	fastlaneCmd.AddCommand(fastlaneMatchCmd)
	rootCmd.AddCommand(fastlaneCmd)
}

// runFastlaneLane is a helper that runs a fastlane lane with the given name.
func runFastlaneLane(laneName string) error {
	// Check if fastlane is installed
	if !shell.CommandExists("fastlane") {
		ui.Error("Fastlane is not installed")
		ui.Info("Install with: brew install fastlane")
		return fmt.Errorf("fastlane not found")
	}

	ui.Infof("Running fastlane %s...", laneName)
	ui.NewLine()

	// Run fastlane interactively so output is streamed to terminal
	if err := shell.RunInteractive("fastlane", laneName); err != nil {
		return fmt.Errorf("fastlane %s failed: %w", laneName, err)
	}

	ui.NewLine()
	ui.Successf("fastlane %s completed", laneName)
	return nil
}

func runFastlaneTest(cmd *cobra.Command, args []string) error {
	return runFastlaneLane("test")
}

func runFastlaneBuild(cmd *cobra.Command, args []string) error {
	return runFastlaneLane("build")
}

func runFastlaneBeta(cmd *cobra.Command, args []string) error {
	return runFastlaneLane("beta")
}

func runFastlaneRelease(cmd *cobra.Command, args []string) error {
	return runFastlaneLane("release")
}

func runFastlaneScreenshots(cmd *cobra.Command, args []string) error {
	return runFastlaneLane("screenshots")
}

func runFastlaneMatchSync(cmd *cobra.Command, args []string) error {
	// Check if fastlane is installed
	if !shell.CommandExists("fastlane") {
		ui.Error("Fastlane is not installed")
		ui.Info("Install with: brew install fastlane")
		return fmt.Errorf("fastlane not found")
	}

	ui.Info("Syncing code signing certificates...")
	ui.NewLine()

	// Run fastlane match interactively
	if err := shell.RunInteractive("fastlane", "match"); err != nil {
		return fmt.Errorf("fastlane match failed: %w", err)
	}

	ui.NewLine()
	ui.Success("Code signing certificates synced")
	return nil
}
