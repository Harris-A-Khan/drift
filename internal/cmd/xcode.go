package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/undrift/drift/internal/config"
	"github.com/undrift/drift/internal/ui"
	"github.com/undrift/drift/internal/xcode"
)

var xcodeCmd = &cobra.Command{
	Use:   "xcode",
	Short: "Xcode project management",
	Long: `Manage Xcode schemes and project configuration.

Commands:
  schemes   - List all available schemes
  validate  - Validate configured schemes exist`,
}

var xcodeSchemesCmd = &cobra.Command{
	Use:   "schemes",
	Short: "List all available Xcode schemes",
	Long: `List all Xcode schemes found in the project.

This scans both .xcodeproj and .xcworkspace files for available schemes.
Shared schemes (in xcshareddata) are shown first.`,
	RunE: runXcodeSchemes,
}

var xcodeValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configured schemes exist",
	Long: `Check that all schemes configured in .drift.yaml actually exist in the Xcode project.

This helps catch configuration errors before deployment or build operations.`,
	RunE: runXcodeValidate,
}

func init() {
	xcodeCmd.AddCommand(xcodeSchemesCmd)
	xcodeCmd.AddCommand(xcodeValidateCmd)
	rootCmd.AddCommand(xcodeCmd)
}

func runXcodeSchemes(cmd *cobra.Command, args []string) error {
	ui.Header("Xcode Schemes")

	// First try the fast method (scanning files)
	schemes, err := xcode.ListSchemes()
	if err != nil || len(schemes) == 0 {
		// Fall back to xcodebuild -list
		ui.Infof("Scanning via xcodebuild...")
		schemeNames, err := xcode.ListSchemesViaXcodebuild()
		if err != nil {
			return fmt.Errorf("could not list schemes: %w", err)
		}

		if len(schemeNames) == 0 {
			ui.Info("No schemes found")
			return nil
		}

		for _, name := range schemeNames {
			ui.List(name)
		}
		ui.NewLine()
		ui.Infof("Total: %d schemes", len(schemeNames))
		return nil
	}

	// Group by shared/personal
	var sharedSchemes, personalSchemes []xcode.Scheme
	for _, s := range schemes {
		if s.IsShared {
			sharedSchemes = append(sharedSchemes, s)
		} else {
			personalSchemes = append(personalSchemes, s)
		}
	}

	if len(sharedSchemes) > 0 {
		ui.SubHeader("Shared Schemes")
		for _, s := range sharedSchemes {
			fmt.Printf("  %s  %s\n", ui.Cyan(s.Name), ui.Dim(s.Container))
		}
	}

	if len(personalSchemes) > 0 {
		ui.NewLine()
		ui.SubHeader("Personal Schemes")
		for _, s := range personalSchemes {
			fmt.Printf("  %s  %s\n", ui.Yellow(s.Name), ui.Dim(s.Container))
		}
	}

	ui.NewLine()
	ui.Infof("Total: %d schemes (%d shared, %d personal)",
		len(schemes), len(sharedSchemes), len(personalSchemes))

	// Show configuration status
	if config.Exists() {
		cfg := config.LoadOrDefault()
		if cfg.Xcode.Schemes != nil && len(cfg.Xcode.Schemes) > 0 {
			ui.NewLine()
			ui.SubHeader("Configured in .drift.yaml")
			for env, scheme := range cfg.Xcode.Schemes {
				if scheme != "" {
					exists := "✓"
					color := ui.Green
					if !xcode.SchemeExists(scheme) {
						exists = "✗"
						color = ui.Red
					}
					fmt.Printf("  %s %s: %s\n", color(exists), env, scheme)
				}
			}
		}
	}

	return nil
}

func runXcodeValidate(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}
	cfg := config.LoadOrDefault()

	ui.Header("Validate Xcode Configuration")

	hasErrors := false
	validCount := 0
	totalCount := 0

	if cfg.Xcode.Schemes == nil || len(cfg.Xcode.Schemes) == 0 {
		ui.Warning("No schemes configured in .drift.yaml")
		ui.NewLine()
		ui.Info("Add schemes to your config:")
		ui.List(`xcode:
  schemes:
    production: "AppName (Production)"
    development: "AppName (Development)"
    feature: "AppName (Debug)"`)
		return nil
	}

	ui.SubHeader("Scheme Validation")

	for env, scheme := range cfg.Xcode.Schemes {
		if scheme == "" {
			continue
		}
		totalCount++

		if xcode.SchemeExists(scheme) {
			fmt.Printf("  %s %s: %s\n", ui.Green("✓"), env, scheme)
			validCount++
		} else {
			fmt.Printf("  %s %s: %s %s\n", ui.Red("✗"), env, scheme, ui.Red("(not found)"))
			hasErrors = true
		}
	}

	ui.NewLine()

	if hasErrors {
		ui.Warningf("%d of %d configured schemes not found", totalCount-validCount, totalCount)
		ui.NewLine()
		ui.Info("Available schemes:")
		schemes, _ := xcode.ListSchemes()
		for _, s := range schemes {
			ui.List(s.Name)
		}
		return fmt.Errorf("scheme validation failed")
	}

	ui.Successf("All %d configured schemes are valid", validCount)
	return nil
}
