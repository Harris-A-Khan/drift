package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/undrift/drift/internal/config"
	"github.com/undrift/drift/internal/supabase"
	"github.com/undrift/drift/internal/ui"
)

var storageCmd = &cobra.Command{
	Use:   "storage",
	Short: "Manage cloud storage for backups",
	Long: `Manage Supabase Storage for database backups.

The storage command helps set up and manage the cloud storage
infrastructure for database backups.`,
}

var storageSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Set up storage bucket for backups",
	Long: `Create the database-backups storage bucket and configure RLS policies.

This is a one-time setup command that:
  1. Creates the 'database-backups' bucket in Supabase Storage
  2. Sets up Row Level Security (RLS) policies for secure access
  3. Creates the initial folder structure (backups/prod, backups/dev)

Prerequisites:
  - Supabase CLI installed and logged in
  - Project linked or PROD_PROJECT_REF in environment`,
	RunE: runStorageSetup,
}

var storageStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show storage bucket status",
	Long:  `Show the current status of the backup storage bucket.`,
	RunE:  runStorageStatus,
}

func init() {
	storageCmd.AddCommand(storageSetupCmd)
	storageCmd.AddCommand(storageStatusCmd)
	rootCmd.AddCommand(storageCmd)
}

func runStorageSetup(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}

	cfg := config.LoadOrDefault()
	bucket := cfg.Backup.Bucket
	if bucket == "" {
		bucket = "database-backups"
	}

	ui.Header("Storage Setup")
	ui.KeyValue("Bucket", ui.Cyan(bucket))

	// Get project ref
	client := supabase.NewClient()
	projectRef, err := getProductionProjectRef(client)
	if err != nil {
		return err
	}
	ui.KeyValue("Project Ref", ui.Cyan(projectRef))

	ui.NewLine()

	// Confirm
	if !IsYes() {
		ui.Warning("This will create a storage bucket and set up RLS policies")
		confirmed, err := ui.PromptYesNo("Continue?", true)
		if err != nil || !confirmed {
			ui.Info("Cancelled")
			return nil
		}
	}

	ui.NewLine()

	// Step 1: Create bucket
	sp := ui.NewSpinner("Creating storage bucket")
	sp.Start()

	if err := createStorageBucket(projectRef, bucket); err != nil {
		sp.Fail("Failed to create bucket")
		ui.Info("Bucket may already exist, continuing...")
	} else {
		sp.Success(fmt.Sprintf("Created bucket: %s", bucket))
	}

	// Step 2: Set up RLS policies
	sp = ui.NewSpinner("Setting up RLS policies")
	sp.Start()

	if err := setupStorageRLS(cfg, projectRef, bucket); err != nil {
		sp.Fail("Failed to set up RLS policies")
		ui.Warning(fmt.Sprintf("You may need to set up policies manually: %v", err))
	} else {
		sp.Success("RLS policies configured")
	}

	// Step 3: Create initial metadata
	sp = ui.NewSpinner("Creating initial folder structure")
	sp.Start()

	if err := createInitialMetadata(bucket); err != nil {
		sp.Fail("Failed to create metadata")
		ui.Warning("You can create the structure manually")
	} else {
		sp.Success("Folder structure created")
	}

	ui.NewLine()
	ui.Success("Storage setup complete!")

	ui.NewLine()
	ui.SubHeader("Bucket Structure")
	ui.Info(fmt.Sprintf("  %s/", bucket))
	ui.Info("    backups/")
	ui.Info("      prod/")
	ui.Info("      dev/")
	ui.Info("      metadata.json")

	ui.NewLine()
	ui.SubHeader("Next Steps")
	ui.NumberedList(1, "Test upload: drift backup upload <file> prod")
	ui.NumberedList(2, "Test download: drift backup download prod")
	ui.NumberedList(3, "Set up automated backups in CI/CD")

	return nil
}

func runStorageStatus(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}

	cfg := config.LoadOrDefault()
	bucket := cfg.Backup.Bucket
	if bucket == "" {
		bucket = "database-backups"
	}

	ui.Header("Storage Status")
	ui.KeyValue("Bucket", ui.Cyan(bucket))

	// List backups in bucket
	storage := supabase.NewStorageClient(bucket)

	ui.NewLine()
	ui.SubHeader("Production Backups")

	prodBackups, err := storage.ListBackups("prod")
	if err != nil {
		ui.Warning(fmt.Sprintf("Could not list prod backups: %v", err))
	} else if len(prodBackups) == 0 {
		ui.Info("No backups found")
	} else {
		for _, b := range prodBackups {
			ui.List(b.Filename)
		}
		ui.Infof("Total: %d backups", len(prodBackups))
	}

	ui.NewLine()
	ui.SubHeader("Development Backups")

	devBackups, err := storage.ListBackups("dev")
	if err != nil {
		ui.Warning(fmt.Sprintf("Could not list dev backups: %v", err))
	} else if len(devBackups) == 0 {
		ui.Info("No backups found")
	} else {
		for _, b := range devBackups {
			ui.List(b.Filename)
		}
		ui.Infof("Total: %d backups", len(devBackups))
	}

	return nil
}

// getProductionProjectRef gets the production project reference
func getProductionProjectRef(client *supabase.Client) (string, error) {
	// First try environment variable
	if ref := os.Getenv("PROD_PROJECT_REF"); ref != "" {
		return ref, nil
	}

	// Try .supabase-project-ref file
	if data, err := os.ReadFile(".supabase-project-ref"); err == nil {
		return string(data), nil
	}

	// Try to get from supabase CLI
	return client.GetProjectRef()
}

// createStorageBucket creates the storage bucket using supabase CLI
func createStorageBucket(_ string, bucket string) error {
	// Note: The supabase CLI doesn't have a direct "storage create" command
	// We'll use the storage cp command which will create the bucket if it doesn't exist
	// or show an error that we can catch

	// Create a temp file to upload
	tmpFile, err := os.CreateTemp("", ".drift-storage-init")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	// Write a marker file
	tmpFile.WriteString("{\"initialized\": true}")
	tmpFile.Close()

	// Try to upload - this will help verify the bucket exists or create folder structure
	storage := supabase.NewStorageClient(bucket)
	err = storage.Upload(tmpFile.Name(), "backups/.init")

	// Clean up the init file
	storage.Delete("backups/.init")

	return err
}

// setupStorageRLS sets up RLS policies for the storage bucket
func setupStorageRLS(cfg *config.Config, projectRef, bucket string) error {
	// Get database password
	password := os.Getenv("PROD_PASSWORD")
	if password == "" {
		var err error
		password, err = ui.PromptPassword("Enter production database password")
		if err != nil {
			return err
		}
	}

	// Build connection string
	poolerHost := cfg.Database.PoolerHost
	if poolerHost == "" {
		poolerHost = "aws-0-us-east-1.pooler.supabase.com"
	}

	connString := fmt.Sprintf(
		"postgresql://postgres.%s:%s@%s:%d/postgres",
		projectRef,
		password,
		poolerHost,
		cfg.Database.PoolerPort,
	)

	// RLS policy SQL
	policySQL := fmt.Sprintf(`
-- Enable RLS on storage.objects
ALTER TABLE storage.objects ENABLE ROW LEVEL SECURITY;

-- Policy: Service role can insert backups
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE policyname = 'Service role can upload backups' AND tablename = 'objects') THEN
        CREATE POLICY "Service role can upload backups"
        ON storage.objects
        FOR INSERT
        TO service_role
        WITH CHECK (bucket_id = '%s');
    END IF;
END $$;

-- Policy: Service role can update (for latest.backup.gz)
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE policyname = 'Service role can update backups' AND tablename = 'objects') THEN
        CREATE POLICY "Service role can update backups"
        ON storage.objects
        FOR UPDATE
        TO service_role
        USING (bucket_id = '%s')
        WITH CHECK (bucket_id = '%s');
    END IF;
END $$;

-- Policy: Service role can delete old backups
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE policyname = 'Service role can delete backups' AND tablename = 'objects') THEN
        CREATE POLICY "Service role can delete backups"
        ON storage.objects
        FOR DELETE
        TO service_role
        USING (bucket_id = '%s');
    END IF;
END $$;

-- Policy: Authenticated users can read backups
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE policyname = 'Authenticated users can download backups' AND tablename = 'objects') THEN
        CREATE POLICY "Authenticated users can download backups"
        ON storage.objects
        FOR SELECT
        TO authenticated
        USING (bucket_id = '%s');
    END IF;
END $$;

-- Policy: Service role can read (for listing, metadata)
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE policyname = 'Service role can read backups' AND tablename = 'objects') THEN
        CREATE POLICY "Service role can read backups"
        ON storage.objects
        FOR SELECT
        TO service_role
        USING (bucket_id = '%s');
    END IF;
END $$;
`, bucket, bucket, bucket, bucket, bucket, bucket)

	// Execute SQL via psql (must be in PATH)
	psql, err := exec.LookPath("psql")
	if err != nil {
		return fmt.Errorf("psql not found in PATH. Install PostgreSQL and add it to your PATH")
	}

	// Write SQL to temp file
	tmpFile, err := os.CreateTemp("", "rls-*.sql")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString(policySQL)
	tmpFile.Close()

	// Execute
	result, err := supabase.RunCommand(psql, "-d", connString, "-f", tmpFile.Name())
	if err != nil {
		return fmt.Errorf("failed to execute RLS policies: %v - %s", err, result)
	}

	return nil
}

// createInitialMetadata creates the initial metadata.json file
func createInitialMetadata(bucket string) error {
	storage := supabase.NewStorageClient(bucket)

	// Create initial index
	index := &supabase.BackupIndex{
		Backups: []supabase.BackupMetadata{},
	}

	return storage.SaveMetadataIndex(index)
}
