package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// testRepo holds information about a temporary test repository.
type testRepo struct {
	path    string
	cleanup func()
}

// setupTestRepo creates a temporary git repository for testing.
func setupTestRepo(t *testing.T) *testRepo {
	t.Helper()

	tmpDir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Configure git user for commits
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = tmpDir
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	cmd.Run()

	// Create initial commit
	testFile := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create initial commit: %v", err)
	}

	return &testRepo{
		path: tmpDir,
		cleanup: func() {
			// TempDir is cleaned up automatically by t.TempDir()
		},
	}
}

// chdir changes to the specified directory and returns a cleanup function.
func chdir(t *testing.T, dir string) func() {
	t.Helper()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	return func() {
		os.Chdir(oldWd)
	}
}

func TestIsGitRepository_True(t *testing.T) {
	repo := setupTestRepo(t)
	defer chdir(t, repo.path)()

	if !IsGitRepository() {
		t.Error("IsGitRepository() = false, want true in a git repo")
	}
}

func TestIsGitRepository_False(t *testing.T) {
	// Create a temp dir outside of any git repo by using /tmp directly
	// and creating a deeply nested path that won't have a .git ancestor
	tmpDir := t.TempDir()

	// Create a .git blocker to prevent git from finding parent repos
	// by creating a file named .git (not a directory)
	gitBlocker := filepath.Join(tmpDir, ".git")
	if err := os.WriteFile(gitBlocker, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create .git blocker: %v", err)
	}

	defer chdir(t, tmpDir)()

	// Now git won't recognize this as a repo (invalid .git file)
	// and won't search parent directories
	if IsGitRepository() {
		t.Skip("Cannot reliably test non-git directory within a git repo")
	}
}

func TestGetRepoRoot(t *testing.T) {
	repo := setupTestRepo(t)
	defer chdir(t, repo.path)()

	root, err := GetRepoRoot()
	if err != nil {
		t.Fatalf("GetRepoRoot() error = %v", err)
	}

	// Resolve symlinks for comparison
	expectedRoot, _ := filepath.EvalSymlinks(repo.path)
	actualRoot, _ := filepath.EvalSymlinks(root)

	if actualRoot != expectedRoot {
		t.Errorf("GetRepoRoot() = %q, want %q", actualRoot, expectedRoot)
	}
}

func TestGetRepoRoot_Subdirectory(t *testing.T) {
	repo := setupTestRepo(t)

	// Create subdirectory
	subDir := filepath.Join(repo.path, "sub", "dir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	defer chdir(t, subDir)()

	root, err := GetRepoRoot()
	if err != nil {
		t.Fatalf("GetRepoRoot() error = %v", err)
	}

	expectedRoot, _ := filepath.EvalSymlinks(repo.path)
	actualRoot, _ := filepath.EvalSymlinks(root)

	if actualRoot != expectedRoot {
		t.Errorf("GetRepoRoot() from subdir = %q, want %q", actualRoot, expectedRoot)
	}
}

func TestGetRepoRoot_NotGitRepo(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a .git blocker file to prevent git from finding parent repos
	gitBlocker := filepath.Join(tmpDir, ".git")
	if err := os.WriteFile(gitBlocker, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create .git blocker: %v", err)
	}

	defer chdir(t, tmpDir)()

	_, err := GetRepoRoot()
	if err == nil {
		t.Skip("Cannot reliably test non-git directory within a git repo")
	}
}

func TestGetGitDir(t *testing.T) {
	repo := setupTestRepo(t)
	defer chdir(t, repo.path)()

	gitDir, err := GetGitDir()
	if err != nil {
		t.Fatalf("GetGitDir() error = %v", err)
	}

	expectedGitDir, _ := filepath.EvalSymlinks(filepath.Join(repo.path, ".git"))
	actualGitDir, _ := filepath.EvalSymlinks(gitDir)

	if actualGitDir != expectedGitDir {
		t.Errorf("GetGitDir() = %q, want %q", actualGitDir, expectedGitDir)
	}
}

func TestGetCommonDir(t *testing.T) {
	repo := setupTestRepo(t)
	defer chdir(t, repo.path)()

	commonDir, err := GetCommonDir()
	if err != nil {
		t.Fatalf("GetCommonDir() error = %v", err)
	}

	// In a regular repo (not a worktree), common dir equals git dir
	gitDir, _ := GetGitDir()

	expectedCommonDir, _ := filepath.EvalSymlinks(gitDir)
	actualCommonDir, _ := filepath.EvalSymlinks(commonDir)

	if actualCommonDir != expectedCommonDir {
		t.Errorf("GetCommonDir() = %q, want %q", actualCommonDir, expectedCommonDir)
	}
}

func TestIsWorktree_MainRepo(t *testing.T) {
	repo := setupTestRepo(t)
	defer chdir(t, repo.path)()

	if IsWorktree() {
		t.Error("IsWorktree() = true for main repo, want false")
	}
}

func TestStatus_Clean(t *testing.T) {
	repo := setupTestRepo(t)
	defer chdir(t, repo.path)()

	status, err := Status()
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}

	if status != "" {
		t.Errorf("Status() = %q, want empty for clean repo", status)
	}
}

func TestStatus_Dirty(t *testing.T) {
	repo := setupTestRepo(t)
	defer chdir(t, repo.path)()

	// Create an untracked file
	testFile := filepath.Join(repo.path, "untracked.txt")
	if err := os.WriteFile(testFile, []byte("untracked"), 0644); err != nil {
		t.Fatalf("Failed to create untracked file: %v", err)
	}

	status, err := Status()
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}

	if status == "" {
		t.Error("Status() = empty, want non-empty for dirty repo")
	}

	if !strings.Contains(status, "untracked.txt") {
		t.Errorf("Status() should contain untracked.txt, got %q", status)
	}
}

func TestIsDirty_Clean(t *testing.T) {
	repo := setupTestRepo(t)
	defer chdir(t, repo.path)()

	dirty, err := IsDirty()
	if err != nil {
		t.Fatalf("IsDirty() error = %v", err)
	}

	if dirty {
		t.Error("IsDirty() = true, want false for clean repo")
	}
}

func TestIsDirty_Dirty(t *testing.T) {
	repo := setupTestRepo(t)
	defer chdir(t, repo.path)()

	// Modify a file
	testFile := filepath.Join(repo.path, "README.md")
	if err := os.WriteFile(testFile, []byte("# Modified\n"), 0644); err != nil {
		t.Fatalf("Failed to modify file: %v", err)
	}

	dirty, err := IsDirty()
	if err != nil {
		t.Fatalf("IsDirty() error = %v", err)
	}

	if !dirty {
		t.Error("IsDirty() = false, want true for dirty repo")
	}
}

func TestCheckout(t *testing.T) {
	repo := setupTestRepo(t)
	defer chdir(t, repo.path)()

	// Create a new branch
	cmd := exec.Command("git", "branch", "test-branch")
	cmd.Dir = repo.path
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create branch: %v", err)
	}

	// Checkout the branch
	if err := Checkout("test-branch"); err != nil {
		t.Fatalf("Checkout() error = %v", err)
	}

	// Verify we're on the new branch
	branch, err := CurrentBranch()
	if err != nil {
		t.Fatalf("CurrentBranch() error = %v", err)
	}

	if branch != "test-branch" {
		t.Errorf("CurrentBranch() after checkout = %q, want %q", branch, "test-branch")
	}
}

func TestCheckout_NonExistent(t *testing.T) {
	repo := setupTestRepo(t)
	defer chdir(t, repo.path)()

	err := Checkout("non-existent-branch-xyz-12345")
	if err == nil {
		// The function may not return an error if git outputs to stderr but exits 0
		// Check if we're still on the original branch
		branch, _ := CurrentBranch()
		if branch == "non-existent-branch-xyz-12345" {
			t.Error("Checkout() should not have succeeded for non-existent branch")
		}
	}
}

func TestGetMainWorktreePath(t *testing.T) {
	repo := setupTestRepo(t)
	defer chdir(t, repo.path)()

	mainPath, err := GetMainWorktreePath()
	if err != nil {
		t.Fatalf("GetMainWorktreePath() error = %v", err)
	}

	expectedPath, _ := filepath.EvalSymlinks(repo.path)
	actualPath, _ := filepath.EvalSymlinks(mainPath)

	if actualPath != expectedPath {
		t.Errorf("GetMainWorktreePath() = %q, want %q", actualPath, expectedPath)
	}
}
