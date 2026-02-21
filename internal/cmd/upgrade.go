package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/undrift/drift/internal/ui"
	"github.com/undrift/drift/pkg/shell"
)

var upgradeCmd = &cobra.Command{
	Use:     "upgrade",
	Aliases: []string{"update"},
	Short:   "Check for and install drift updates",
	Long: `Check if a newer version of drift is available and optionally upgrade.

Checks the latest release on GitHub and compares it with the installed version.
If installed via Homebrew, upgrades using brew. Otherwise shows manual instructions.`,
	RunE: runUpgrade,
}

var upgradeCheckOnly bool

func init() {
	upgradeCmd.Flags().BoolVar(&upgradeCheckOnly, "check", false, "Only check for updates, don't install")
	rootCmd.AddCommand(upgradeCmd)
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

func runUpgrade(cmd *cobra.Command, args []string) error {
	currentVersion := strings.TrimPrefix(version, "v")

	ui.Infof("Current version: %s", version)

	// Fetch latest release from GitHub
	spinner := ui.NewSpinner("Checking for updates...")
	spinner.Start()

	latest, err := fetchLatestRelease()
	spinner.Stop()
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	latestVersion := strings.TrimPrefix(latest.TagName, "v")

	if currentVersion == latestVersion {
		ui.Success("You're on the latest version!")
		return nil
	}

	// Check if current version looks like a dev build
	isDev := currentVersion == "dev" || strings.Contains(currentVersion, "-g")

	if !isDev && !isNewer(latestVersion, currentVersion) {
		ui.Success("You're on the latest version!")
		return nil
	}

	ui.Successf("New version available: %s -> %s", version, latest.TagName)

	if upgradeCheckOnly {
		return nil
	}

	// Detect install method and upgrade
	if isBrewInstall() {
		ui.Info("Upgrading via Homebrew...")
		result, err := shell.Run("brew", "upgrade", "Harris-A-Khan/tap/drift")
		if err != nil || result.ExitCode != 0 {
			// Try update tap first, then upgrade
			shell.Run("brew", "tap", "Harris-A-Khan/tap")
			result, err = shell.Run("brew", "upgrade", "Harris-A-Khan/tap/drift")
			if err != nil || result.ExitCode != 0 {
				ui.Warning("Brew upgrade failed, trying reinstall...")
				result, err = shell.Run("brew", "reinstall", "Harris-A-Khan/tap/drift")
				if err != nil || result.ExitCode != 0 {
					return fmt.Errorf("brew upgrade failed: %s", result.Stderr)
				}
			}
		}
		ui.Successf("Upgraded to %s", latest.TagName)
	} else {
		ui.Info("Not installed via Homebrew.")
		fmt.Println()
		ui.Info("To upgrade via Homebrew:")
		fmt.Printf("  brew install Harris-A-Khan/tap/drift\n\n")
		ui.Info("Or download the latest release:")
		fmt.Printf("  %s\n\n", latest.HTMLURL)
		ui.Info("Or build from source:")
		fmt.Println("  sudo ./dev-install.sh")
	}

	return nil
}

func fetchLatestRelease() (*githubRelease, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://api.github.com/repos/Harris-A-Khan/drift/releases/latest")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}
	return &release, nil
}

func isBrewInstall() bool {
	result, err := shell.Run("brew", "list", "Harris-A-Khan/tap/drift")
	return err == nil && result.ExitCode == 0
}

// isNewer returns true if version a is newer than version b.
// Compares semver-style versions (e.g., "1.6.0" > "1.5.0").
func isNewer(a, b string) bool {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")

	for i := 0; i < len(aParts) && i < len(bParts); i++ {
		var aNum, bNum int
		fmt.Sscanf(aParts[i], "%d", &aNum)
		fmt.Sscanf(bParts[i], "%d", &bNum)
		if aNum > bNum {
			return true
		}
		if aNum < bNum {
			return false
		}
	}
	return len(aParts) > len(bParts)
}
