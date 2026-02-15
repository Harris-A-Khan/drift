package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/undrift/drift/internal/config"
)

func loadConfigWithBackupDir(t *testing.T, root, backupDir string) *config.Config {
	t.Helper()

	configPath := filepath.Join(root, ".drift.yaml")
	content := fmt.Sprintf(`project:
  name: test
database:
  backup_dir: %s
`, backupDir)

	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := config.LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	return cfg
}

func createBackupFile(t *testing.T, path string, modTime time.Time) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("failed to create directory for %s: %v", path, err)
	}

	if err := os.WriteFile(path, []byte("backup"), 0644); err != nil {
		t.Fatalf("failed to write backup %s: %v", path, err)
	}

	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("failed to set modtime for %s: %v", path, err)
	}
}

func TestBackupSearchDirs_PrioritizesBackupDirThenRoot(t *testing.T) {
	root := t.TempDir()
	cfg := loadConfigWithBackupDir(t, root, "backups")

	dirs := backupSearchDirs(cfg)
	if len(dirs) != 2 {
		t.Fatalf("backupSearchDirs length = %d, want 2 (%v)", len(dirs), dirs)
	}

	wantBackupDir := filepath.Join(root, "backups")
	if dirs[0] != wantBackupDir {
		t.Fatalf("backupSearchDirs[0] = %q, want %q", dirs[0], wantBackupDir)
	}
	if dirs[1] != root {
		t.Fatalf("backupSearchDirs[1] = %q, want %q", dirs[1], root)
	}
}

func TestBackupSearchDirs_DedupesWhenBackupDirIsProjectRoot(t *testing.T) {
	root := t.TempDir()
	cfg := loadConfigWithBackupDir(t, root, ".")

	dirs := backupSearchDirs(cfg)
	if len(dirs) != 1 {
		t.Fatalf("backupSearchDirs length = %d, want 1 (%v)", len(dirs), dirs)
	}
	if dirs[0] != root {
		t.Fatalf("backupSearchDirs[0] = %q, want %q", dirs[0], root)
	}
}

func TestDiscoverLocalBackups_SortsNewestFirst(t *testing.T) {
	root := t.TempDir()
	cfg := loadConfigWithBackupDir(t, root, "backups")
	backupDir := filepath.Join(root, "backups")

	now := time.Now().Add(-1 * time.Minute)
	oldPath := filepath.Join(root, "dev_20260101_090000.backup")
	newPath := filepath.Join(backupDir, "prod_20260102_090000.backup")
	createBackupFile(t, oldPath, now.Add(-2*time.Hour))
	createBackupFile(t, newPath, now)
	if err := os.WriteFile(filepath.Join(backupDir, "ignore.txt"), []byte("x"), 0644); err != nil {
		t.Fatalf("failed to write non-backup file: %v", err)
	}

	backups, err := discoverLocalBackups(cfg)
	if err != nil {
		t.Fatalf("discoverLocalBackups() error = %v", err)
	}
	if len(backups) != 2 {
		t.Fatalf("discoverLocalBackups() length = %d, want 2", len(backups))
	}
	if backups[0].Path != newPath {
		t.Fatalf("discoverLocalBackups()[0] = %q, want newest %q", backups[0].Path, newPath)
	}
	if backups[1].Path != oldPath {
		t.Fatalf("discoverLocalBackups()[1] = %q, want %q", backups[1].Path, oldPath)
	}
}

func TestSuggestLocalBackup_Precedence(t *testing.T) {
	backups := []localBackupFile{
		{Name: "dev_20260215_140000.backup", Path: "/tmp/dev_20260215_140000.backup"},
		{Name: "prod_20260215_130000.backup", Path: "/tmp/prod_20260215_130000.backup"},
		{Name: "dev.backup", Path: "/tmp/dev.backup"},
	}

	exact := suggestLocalBackup(backups, "dev.backup", "dev")
	if exact == nil || exact.Path != "/tmp/dev.backup" {
		t.Fatalf("exact suggestion = %#v, want /tmp/dev.backup", exact)
	}

	prefixed := suggestLocalBackup(backups, "missing.backup", "prod")
	if prefixed == nil || prefixed.Path != "/tmp/prod_20260215_130000.backup" {
		t.Fatalf("prefixed suggestion = %#v, want /tmp/prod_20260215_130000.backup", prefixed)
	}

	fallback := suggestLocalBackup(backups, "missing.backup", "staging")
	if fallback == nil || fallback.Path != "/tmp/dev_20260215_140000.backup" {
		t.Fatalf("fallback suggestion = %#v, want newest overall", fallback)
	}
}

func TestFilterLocalBackups_SupportsProdDevAndExact(t *testing.T) {
	backups := []localBackupFile{
		{Name: "prod.backup"},
		{Name: "prod_20260215_143000.backup"},
		{Name: "dev_20260215_143000.backup"},
		{Name: "random.backup"},
	}

	prodOnly := filterLocalBackups(backups, "prod")
	if len(prodOnly) != 2 {
		t.Fatalf("filterLocalBackups(prod) = %d, want 2", len(prodOnly))
	}

	devOnly := filterLocalBackups(backups, "development")
	if len(devOnly) != 1 || devOnly[0].Name != "dev_20260215_143000.backup" {
		t.Fatalf("filterLocalBackups(development) = %#v, want only dev backup", devOnly)
	}

	exact := filterLocalBackups(backups, "PROD.BACKUP")
	if len(exact) != 1 || exact[0].Name != "prod.backup" {
		t.Fatalf("filterLocalBackups(exact) = %#v, want prod.backup", exact)
	}
}

func TestTimestampedBackupFilename(t *testing.T) {
	ts := time.Date(2026, time.February, 15, 14, 30, 0, 0, time.Local)
	got := timestampedBackupFilename("prod", ts)
	want := "prod_20260215_143000.backup"
	if got != want {
		t.Fatalf("timestampedBackupFilename() = %q, want %q", got, want)
	}
}

func TestResolveBackupInputPath_UsesBackupInventoryForBareFilename(t *testing.T) {
	root := t.TempDir()
	cfg := loadConfigWithBackupDir(t, root, "backups")
	backupPath := filepath.Join(root, "backups", "prod_20260215_143000.backup")
	createBackupFile(t, backupPath, time.Now())

	backups, err := discoverLocalBackups(cfg)
	if err != nil {
		t.Fatalf("discoverLocalBackups() error = %v", err)
	}

	resolved, err := resolveBackupInputPath("prod_20260215_143000.backup", backups)
	if err != nil {
		t.Fatalf("resolveBackupInputPath() error = %v", err)
	}
	if resolved != backupPath {
		t.Fatalf("resolveBackupInputPath() = %q, want %q", resolved, backupPath)
	}

	direct, err := resolveBackupInputPath(backupPath, backups)
	if err != nil {
		t.Fatalf("resolveBackupInputPath(direct) error = %v", err)
	}
	if direct != backupPath {
		t.Fatalf("resolveBackupInputPath(direct) = %q, want %q", direct, backupPath)
	}
}
