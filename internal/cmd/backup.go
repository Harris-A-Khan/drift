package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/undrift/drift/internal/config"
	"github.com/undrift/drift/internal/supabase"
	"github.com/undrift/drift/internal/ui"
)

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Cloud backup management",
	Long: `Manage database backups in cloud storage.

The backup command helps you upload, download, and manage database 
backups stored in Supabase Storage (or other configured providers).`,
}

var backupUploadCmd = &cobra.Command{
	Use:   "upload <file> <env>",
	Short: "Upload backup to cloud",
	Long: `Upload a database backup file to cloud storage.

Examples:
  drift backup upload prod.backup prod
  drift backup upload dev.backup dev`,
	Args: cobra.ExactArgs(2),
	RunE: runBackupUpload,
}

var backupDownloadCmd = &cobra.Command{
	Use:   "download <env>",
	Short: "Download backup from cloud",
	Long: `Download the latest database backup from cloud storage.

Examples:
  drift backup download prod    # Downloads latest production backup
  drift backup download dev     # Downloads latest development backup`,
	Args: cobra.ExactArgs(1),
	RunE: runBackupDownload,
}

var backupListCmd = &cobra.Command{
	Use:   "list <env>",
	Short: "List available backups",
	Long:  `List all available backups for an environment.`,
	Example: `  drift backup list prod    # List all production backups
  drift backup list dev     # List all development backups`,
	Args: cobra.ExactArgs(1),
	RunE: runBackupList,
}

var backupDeleteCmd = &cobra.Command{
	Use:   "delete <env> <filename>",
	Short: "Delete a specific backup",
	Long:  `Delete a specific backup from cloud storage.`,
	Example: `  drift backup delete prod 20240101_120000_prod.backup
  drift backup delete dev 20240115_093000_dev.backup`,
	Args: cobra.ExactArgs(2),
	RunE: runBackupDelete,
}

var (
	backupOutputFlag string
)

func init() {
	backupDownloadCmd.Flags().StringVarP(&backupOutputFlag, "output", "o", "", "Output file path")

	backupCmd.AddCommand(backupUploadCmd)
	backupCmd.AddCommand(backupDownloadCmd)
	backupCmd.AddCommand(backupListCmd)
	backupCmd.AddCommand(backupDeleteCmd)
	rootCmd.AddCommand(backupCmd)
}

func runBackupUpload(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}
	localFile := args[0]
	env := args[1]
	cfg := config.LoadOrDefault()

	// Validate environment
	if env != "prod" && env != "production" && env != "dev" && env != "development" {
		return fmt.Errorf("invalid environment: %s (use prod or dev)", env)
	}

	// Normalize environment name
	switch env {
	case "production":
		env = "prod"
	case "development":
		env = "dev"
	}

	// Check file exists
	if _, err := os.Stat(localFile); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", localFile)
	}

	ui.Header("Upload Backup")
	ui.KeyValue("File", localFile)
	ui.KeyValue("Environment", env)

	// Get file size
	info, err := os.Stat(localFile)
	if err == nil {
		sizeMB := float64(info.Size()) / 1024 / 1024
		ui.KeyValue("Size", fmt.Sprintf("%.2f MB", sizeMB))
	}

	ui.NewLine()

	// Create storage client
	storage := supabase.NewStorageClient(cfg.Backup.Bucket)

	// Upload
	sp := ui.NewSpinner("Uploading backup")
	sp.Start()

	meta, err := storage.UploadBackup(localFile, env)
	if err != nil {
		sp.Fail("Upload failed")
		return err
	}

	sp.Success(fmt.Sprintf("Uploaded as %s", meta.Filename))

	return nil
}

func runBackupDownload(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}
	env := args[0]
	cfg := config.LoadOrDefault()

	// Validate environment
	if env != "prod" && env != "production" && env != "dev" && env != "development" {
		return fmt.Errorf("invalid environment: %s (use prod or dev)", env)
	}

	// Normalize environment name
	switch env {
	case "production":
		env = "prod"
	case "development":
		env = "dev"
	}

	// Determine output path
	outputPath := backupOutputFlag
	if outputPath == "" {
		outputPath = fmt.Sprintf("%s.backup", env)
	}

	ui.Header("Download Backup")
	ui.KeyValue("Environment", env)
	ui.KeyValue("Output", outputPath)

	ui.NewLine()

	// Create storage client
	storage := supabase.NewStorageClient(cfg.Backup.Bucket)

	// Download
	sp := ui.NewSpinner("Downloading latest backup")
	sp.Start()

	meta, err := storage.DownloadLatestBackup(env, outputPath)
	if err != nil {
		sp.Fail("Download failed")
		return err
	}

	sp.Success(fmt.Sprintf("Downloaded %s", meta.Filename))

	// Show file size
	if info, err := os.Stat(outputPath); err == nil {
		sizeMB := float64(info.Size()) / 1024 / 1024
		ui.KeyValue("Size", fmt.Sprintf("%.2f MB", sizeMB))
	}

	// Next steps
	ui.NewLine()
	ui.SubHeader("Next Steps")
	ui.List(fmt.Sprintf("drift db restore %s  - Restore this backup to database", outputPath))
	ui.List("drift db status        - Check current database status")

	return nil
}

func runBackupList(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}
	env := args[0]
	cfg := config.LoadOrDefault()

	// Validate environment
	if env != "prod" && env != "production" && env != "dev" && env != "development" {
		return fmt.Errorf("invalid environment: %s (use prod or dev)", env)
	}

	// Normalize environment name
	switch env {
	case "production":
		env = "prod"
	case "development":
		env = "dev"
	}

	ui.Header(fmt.Sprintf("Cloud Backups - %s", env))

	// Create storage client
	storage := supabase.NewStorageClient(cfg.Backup.Bucket)

	// List backups
	sp := ui.NewSpinner("Fetching backup list")
	sp.Start()

	backups, err := storage.ListBackups(env)
	if err != nil {
		sp.Fail("Failed to list backups")
		return err
	}
	sp.Stop()

	if len(backups) == 0 {
		ui.Info("No backups found")
		return nil
	}

	for _, b := range backups {
		// Extract timestamp from filename if possible
		name := filepath.Base(b.Filename)
		fmt.Printf("  %s\n", ui.Cyan(name))
	}

	ui.NewLine()
	ui.Infof("Total: %d backups", len(backups))

	// Next steps
	ui.NewLine()
	ui.SubHeader("Next Steps")
	ui.List(fmt.Sprintf("drift backup download %s   - Download latest backup", env))
	ui.List(fmt.Sprintf("drift backup delete %s <filename>  - Delete a backup", env))

	return nil
}

func runBackupDelete(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}
	env := args[0]
	filename := args[1]
	cfg := config.LoadOrDefault()

	// Validate environment
	if env != "prod" && env != "production" && env != "dev" && env != "development" {
		return fmt.Errorf("invalid environment: %s (use prod or dev)", env)
	}

	// Normalize environment name
	switch env {
	case "production":
		env = "prod"
	case "development":
		env = "dev"
	}

	ui.Header("Delete Backup")
	ui.KeyValue("Environment", env)
	ui.KeyValue("Filename", filename)

	// Confirm
	if !IsYes() {
		ui.NewLine()
		ui.Warning("This will permanently delete the backup!")
		confirmed, err := ui.PromptYesNo("Are you sure?", false)
		if err != nil || !confirmed {
			ui.Info("Cancelled")
			return nil
		}
	}

	ui.NewLine()

	// Create storage client
	storage := supabase.NewStorageClient(cfg.Backup.Bucket)

	// Delete
	sp := ui.NewSpinner("Deleting backup")
	sp.Start()

	if err := storage.DeleteBackup(env, filename); err != nil {
		sp.Fail("Delete failed")
		return err
	}

	sp.Success("Backup deleted")

	// Next steps
	ui.NewLine()
	ui.SubHeader("Next Steps")
	ui.List(fmt.Sprintf("drift backup list %s  - Verify backup was deleted", env))

	return nil
}

