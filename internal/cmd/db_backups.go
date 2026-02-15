package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/undrift/drift/internal/config"
)

type localBackupFile struct {
	Name      string
	Path      string
	Directory string
	SizeBytes int64
	ModTime   time.Time
}

func backupSearchDirs(cfg *config.Config) []string {
	seen := make(map[string]bool)
	dirs := make([]string, 0, 2)

	addDir := func(dir string) {
		if strings.TrimSpace(dir) == "" {
			return
		}
		abs, err := filepath.Abs(dir)
		if err != nil {
			abs = filepath.Clean(dir)
		}
		if seen[abs] {
			return
		}
		seen[abs] = true
		dirs = append(dirs, abs)
	}

	if cfg != nil {
		addDir(cfg.GetBackupPath())
		addDir(cfg.ProjectRoot())
		return dirs
	}

	if cwd, err := os.Getwd(); err == nil {
		addDir(cwd)
	}
	return dirs
}

func discoverLocalBackups(cfg *config.Config) ([]localBackupFile, error) {
	dirs := backupSearchDirs(cfg)
	backups := make([]localBackupFile, 0)
	seen := make(map[string]bool)

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("failed to read backup directory %s: %w", dir, err)
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			name := entry.Name()
			if !strings.HasSuffix(strings.ToLower(name), ".backup") {
				continue
			}

			path := filepath.Join(dir, name)
			absPath, err := filepath.Abs(path)
			if err == nil {
				path = absPath
			}
			if seen[path] {
				continue
			}

			info, err := entry.Info()
			if err != nil {
				continue
			}

			backups = append(backups, localBackupFile{
				Name:      name,
				Path:      path,
				Directory: dir,
				SizeBytes: info.Size(),
				ModTime:   info.ModTime(),
			})
			seen[path] = true
		}
	}

	sort.Slice(backups, func(i, j int) bool {
		if backups[i].ModTime.Equal(backups[j].ModTime) {
			return backups[i].Name < backups[j].Name
		}
		return backups[i].ModTime.After(backups[j].ModTime)
	})

	return backups, nil
}

func findLocalBackupByName(backups []localBackupFile, name string) *localBackupFile {
	for i := range backups {
		if strings.EqualFold(backups[i].Name, name) {
			return &backups[i]
		}
	}
	return nil
}

func suggestLocalBackup(backups []localBackupFile, exactName, envPrefix string) *localBackupFile {
	if len(backups) == 0 {
		return nil
	}

	if strings.TrimSpace(exactName) != "" {
		if exact := findLocalBackupByName(backups, exactName); exact != nil {
			return exact
		}
	}

	prefix := strings.ToLower(strings.TrimSpace(envPrefix))
	if prefix != "" {
		for i := range backups {
			if strings.HasPrefix(strings.ToLower(backups[i].Name), prefix) {
				return &backups[i]
			}
		}
	}

	return &backups[0]
}

func filterLocalBackups(backups []localBackupFile, filter string) []localBackupFile {
	trimmedFilter := strings.TrimSpace(filter)
	query := strings.ToLower(trimmedFilter)
	if query == "" {
		return append([]localBackupFile(nil), backups...)
	}

	filtered := make([]localBackupFile, 0, len(backups))
	for _, backup := range backups {
		name := strings.ToLower(backup.Name)

		switch query {
		case "prod", "production":
			if strings.HasPrefix(name, "prod") {
				filtered = append(filtered, backup)
			}
		case "dev", "development":
			if strings.HasPrefix(name, "dev") {
				filtered = append(filtered, backup)
			}
		default:
			if strings.HasSuffix(query, ".backup") {
				if strings.EqualFold(backup.Name, trimmedFilter) {
					filtered = append(filtered, backup)
				}
				continue
			}
			if strings.HasPrefix(name, query) {
				filtered = append(filtered, backup)
			}
		}
	}

	return filtered
}

func resolveBackupInputPath(input string, backups []localBackupFile) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("backup file not provided")
	}

	if _, err := os.Stat(input); err == nil {
		return input, nil
	}

	if !filepath.IsAbs(input) && filepath.Base(input) == input {
		if match := findLocalBackupByName(backups, input); match != nil {
			return match.Path, nil
		}
	}

	return "", fmt.Errorf("backup file not found: %s", input)
}

func backupDisplayPath(path, projectRoot string) string {
	rel, err := filepath.Rel(projectRoot, path)
	if err != nil {
		return path
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return path
	}
	return rel
}

func formatBackupAge(modTime time.Time) string {
	age := time.Since(modTime)
	if age < 0 {
		age = 0
	}
	if age < time.Hour {
		return fmt.Sprintf("%.0f min ago", age.Minutes())
	}
	if age < 24*time.Hour {
		return fmt.Sprintf("%.0f hours ago", age.Hours())
	}
	return fmt.Sprintf("%.1f days ago", age.Hours()/24)
}

func timestampedBackupFilename(prefix string, now time.Time) string {
	normalizedPrefix := strings.TrimSpace(strings.ToLower(prefix))
	if normalizedPrefix == "" {
		normalizedPrefix = "backup"
	}
	return fmt.Sprintf("%s_%s.backup", normalizedPrefix, now.Format("20060102_150405"))
}
