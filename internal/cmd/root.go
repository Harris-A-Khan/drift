// Package cmd implements the drift CLI commands.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version   = "dev"
	cfgFile   string
	verbose   bool
	noColor   bool
	yesFlag   bool
)

// SetVersion sets the version string (called from main).
func SetVersion(v string) {
	version = v
	rootCmd.Version = v
}

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "drift",
	Short: "Opinionated development workflow CLI for Supabase projects",
	Long: `drift standardizes and abstracts opinionated development workflows 
for Supabase-backed iOS/macOS projects.

It wraps common operations like environment switching, git worktree management, 
database operations, edge function deployment, and migration pushing into a 
single, cohesive CLI.

Get started:
  drift init          Initialize drift in your project
  drift doctor        Check system dependencies
  drift env show      Show current environment info
  drift env setup     Generate xcconfig for current branch`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is .drift.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output")
	rootCmd.PersistentFlags().BoolVarP(&yesFlag, "yes", "y", false, "skip confirmation prompts")

	// Version flag
	rootCmd.Version = version
	rootCmd.SetVersionTemplate("drift version {{.Version}}\n")
}

func initConfig() {
	if noColor {
		os.Setenv("NO_COLOR", "1")
	}
}

// IsVerbose returns whether verbose mode is enabled.
func IsVerbose() bool {
	return verbose
}

// IsYes returns whether the --yes flag is set (skip confirmations).
func IsYes() bool {
	return yesFlag
}

// GetConfigFile returns the config file path if specified.
func GetConfigFile() string {
	return cfgFile
}

// AddCommand adds a command to the root command.
func AddCommand(cmd *cobra.Command) {
	rootCmd.AddCommand(cmd)
}

// PrintVersion prints the version information.
func PrintVersion() {
	fmt.Printf("drift version %s\n", version)
}

