package supabase

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/undrift/drift/pkg/shell"
)

// StorageClient handles Supabase Storage operations.
type StorageClient struct {
	client *Client
	bucket string
}

// NewStorageClient creates a new storage client.
func NewStorageClient(bucket string) *StorageClient {
	return &StorageClient{
		client: NewClient(),
		bucket: bucket,
	}
}

// BackupMetadata holds information about a backup.
type BackupMetadata struct {
	Filename    string    `json:"filename"`
	Environment string    `json:"environment"`
	Size        int64     `json:"size"`
	CreatedAt   time.Time `json:"created_at"`
	Checksum    string    `json:"checksum,omitempty"`
}

// BackupIndex holds the index of all backups.
type BackupIndex struct {
	Backups   []BackupMetadata `json:"backups"`
	UpdatedAt time.Time        `json:"updated_at"`
}

// Upload uploads a file to Supabase Storage.
func (s *StorageClient) Upload(localPath, remotePath string) error {
	// Check if file exists
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", localPath)
	}

	args := []string{
		"storage", "cp",
		localPath,
		fmt.Sprintf("ss:///%s/%s", s.bucket, remotePath),
	}

	result, err := shell.Run("supabase", args...)
	if err != nil {
		errMsg := result.Stderr
		if errMsg == "" {
			errMsg = err.Error()
		}
		return fmt.Errorf("upload failed: %s", errMsg)
	}

	return nil
}

// Download downloads a file from Supabase Storage.
func (s *StorageClient) Download(remotePath, localPath string) error {
	// Ensure directory exists
	dir := filepath.Dir(localPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	args := []string{
		"storage", "cp",
		fmt.Sprintf("ss:///%s/%s", s.bucket, remotePath),
		localPath,
	}

	result, err := shell.Run("supabase", args...)
	if err != nil {
		errMsg := result.Stderr
		if errMsg == "" {
			errMsg = err.Error()
		}
		return fmt.Errorf("download failed: %s", errMsg)
	}

	return nil
}

// List lists files in a storage path.
func (s *StorageClient) List(path string) ([]string, error) {
	args := []string{
		"storage", "ls",
		fmt.Sprintf("ss:///%s/%s", s.bucket, path),
	}

	result, err := shell.Run("supabase", args...)
	if err != nil {
		return nil, fmt.Errorf("list failed: %w", err)
	}

	if result.Stdout == "" {
		return []string{}, nil
	}

	// Parse output
	var files []string
	for _, line := range splitLines(result.Stdout) {
		if line != "" {
			files = append(files, line)
		}
	}

	return files, nil
}

// Delete deletes a file from storage.
func (s *StorageClient) Delete(remotePath string) error {
	args := []string{
		"storage", "rm",
		fmt.Sprintf("ss:///%s/%s", s.bucket, remotePath),
	}

	result, err := shell.Run("supabase", args...)
	if err != nil {
		errMsg := result.Stderr
		if errMsg == "" {
			errMsg = err.Error()
		}
		return fmt.Errorf("delete failed: %s", errMsg)
	}

	return nil
}

// UploadBackup uploads a database backup with metadata.
func (s *StorageClient) UploadBackup(localPath, environment string) (*BackupMetadata, error) {
	// Get file info
	info, err := os.Stat(localPath)
	if err != nil {
		return nil, err
	}

	// Generate remote filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("%s-%s.backup", environment, timestamp)
	remotePath := fmt.Sprintf("backups/%s/%s", environment, filename)

	// Upload file
	if err := s.Upload(localPath, remotePath); err != nil {
		return nil, err
	}

	// Create metadata
	meta := &BackupMetadata{
		Filename:    filename,
		Environment: environment,
		Size:        info.Size(),
		CreatedAt:   time.Now(),
	}

	return meta, nil
}

// DownloadLatestBackup downloads the latest backup for an environment.
func (s *StorageClient) DownloadLatestBackup(environment, localPath string) (*BackupMetadata, error) {
	// List backups
	path := fmt.Sprintf("backups/%s", environment)
	files, err := s.List(path)
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no backups found for %s", environment)
	}

	// Find latest (files are named with timestamps, so lexical sort works)
	latest := files[len(files)-1]

	// Download
	remotePath := fmt.Sprintf("%s/%s", path, latest)
	if err := s.Download(remotePath, localPath); err != nil {
		return nil, err
	}

	// Get file info
	info, err := os.Stat(localPath)
	if err != nil {
		return nil, err
	}

	return &BackupMetadata{
		Filename:    latest,
		Environment: environment,
		Size:        info.Size(),
		CreatedAt:   time.Now(),
	}, nil
}

// ListBackups lists all backups for an environment.
func (s *StorageClient) ListBackups(environment string) ([]BackupMetadata, error) {
	path := fmt.Sprintf("backups/%s", environment)
	files, err := s.List(path)
	if err != nil {
		return nil, err
	}

	backups := make([]BackupMetadata, 0, len(files))
	for _, f := range files {
		backups = append(backups, BackupMetadata{
			Filename:    f,
			Environment: environment,
		})
	}

	return backups, nil
}

// DeleteBackup deletes a specific backup.
func (s *StorageClient) DeleteBackup(environment, filename string) error {
	remotePath := fmt.Sprintf("backups/%s/%s", environment, filename)
	return s.Delete(remotePath)
}

// SaveMetadataIndex saves the backup metadata index.
func (s *StorageClient) SaveMetadataIndex(index *BackupIndex) error {
	index.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}

	// Write to temp file
	tmpFile, err := os.CreateTemp("", "metadata-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(data); err != nil {
		return err
	}
	tmpFile.Close()

	// Upload
	return s.Upload(tmpFile.Name(), "backups/metadata.json")
}

// LoadMetadataIndex loads the backup metadata index.
func (s *StorageClient) LoadMetadataIndex() (*BackupIndex, error) {
	tmpFile, err := os.CreateTemp("", "metadata-*.json")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	if err := s.Download("backups/metadata.json", tmpFile.Name()); err != nil {
		// Return empty index if not found
		return &BackupIndex{Backups: []BackupMetadata{}}, nil
	}

	data, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return nil, err
	}

	var index BackupIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, err
	}

	return &index, nil
}

// CompressFile compresses a file using gzip.
func CompressFile(srcPath, dstPath string) error {
	result, err := shell.Run("gzip", "-c", srcPath)
	if err != nil {
		return err
	}

	return os.WriteFile(dstPath, []byte(result.Stdout), 0644)
}

// DecompressFile decompresses a gzip file.
func DecompressFile(srcPath, dstPath string) error {
	result, err := shell.Run("gunzip", "-c", srcPath)
	if err != nil {
		return err
	}

	return os.WriteFile(dstPath, []byte(result.Stdout), 0644)
}

// CopyFile copies a file from src to dst.
func CopyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

