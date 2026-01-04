package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/undrift/drift/internal/config"
	"github.com/undrift/drift/internal/ui"
	"github.com/undrift/drift/internal/xcode"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build helpers",
	Long:  `Utilities for managing Xcode build numbers and versions.`,
}

var buildBumpCmd = &cobra.Command{
	Use:   "bump",
	Short: "Increment build number",
	Long:  `Increment the CURRENT_PROJECT_VERSION in Version.xcconfig.`,
	RunE:  runBuildBump,
}

var buildSetCmd = &cobra.Command{
	Use:   "set <number>",
	Short: "Set build number to specific value",
	Long:  `Set the CURRENT_PROJECT_VERSION to a specific value.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runBuildSet,
}

var buildVersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show current version info",
	Long:  `Display the current marketing version and build number.`,
	RunE:  runBuildVersion,
}

var buildSetVersionCmd = &cobra.Command{
	Use:   "set-version <version>",
	Short: "Set marketing version",
	Long:  `Set the MARKETING_VERSION in Version.xcconfig.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runBuildSetVersion,
}

func init() {
	buildCmd.AddCommand(buildBumpCmd)
	buildCmd.AddCommand(buildSetCmd)
	buildCmd.AddCommand(buildVersionCmd)
	buildCmd.AddCommand(buildSetVersionCmd)
	rootCmd.AddCommand(buildCmd)
}

func runBuildBump(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}
	cfg := config.LoadOrDefault()
	versionFile := cfg.GetVersionFilePath()

	// Get current build number for display
	currentInfo, _ := xcode.ReadVersionFile(versionFile)
	currentBuild := 0
	if currentInfo != nil {
		currentBuild = currentInfo.BuildNumber
	}

	// Increment
	newBuild, err := xcode.IncrementBuildNumber(versionFile)
	if err != nil {
		return fmt.Errorf("failed to increment build number: %w", err)
	}

	ui.Successf("Build number incremented: %d → %d", currentBuild, newBuild)
	ui.KeyValue("File", versionFile)

	return nil
}

func runBuildSet(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}
	cfg := config.LoadOrDefault()
	versionFile := cfg.GetVersionFilePath()

	// Parse build number
	var buildNumber int
	if _, err := fmt.Sscanf(args[0], "%d", &buildNumber); err != nil {
		return fmt.Errorf("invalid build number: %s", args[0])
	}

	if buildNumber < 0 {
		return fmt.Errorf("build number must be non-negative")
	}

	// Set build number
	if err := xcode.SetBuildNumber(versionFile, buildNumber); err != nil {
		return fmt.Errorf("failed to set build number: %w", err)
	}

	ui.Successf("Build number set to %d", buildNumber)
	ui.KeyValue("File", versionFile)

	return nil
}

func runBuildVersion(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}
	cfg := config.LoadOrDefault()
	versionFile := cfg.GetVersionFilePath()

	info, err := xcode.ReadVersionFile(versionFile)
	if err != nil {
		return fmt.Errorf("failed to read version file: %w", err)
	}

	ui.Header("Version Info")
	ui.KeyValue("File", versionFile)

	if info.MarketingVersion != "" {
		ui.KeyValue("Marketing Version", ui.Cyan(info.MarketingVersion))
	} else {
		ui.KeyValue("Marketing Version", ui.Dim("(not set)"))
	}

	ui.KeyValue("Build Number", ui.Cyan(fmt.Sprintf("%d", info.BuildNumber)))

	return nil
}

func runBuildSetVersion(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}
	cfg := config.LoadOrDefault()
	versionFile := cfg.GetVersionFilePath()
	version := args[0]

	// Set marketing version
	if err := xcode.SetMarketingVersion(versionFile, version); err != nil {
		return fmt.Errorf("failed to set marketing version: %w", err)
	}

	ui.Successf("Marketing version set to %s", version)
	ui.KeyValue("File", versionFile)

	return nil
}

