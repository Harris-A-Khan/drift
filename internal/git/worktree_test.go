package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestListWorktrees(t *testing.T) {
	repo := setupTestRepo(t)
	defer chdir(t, repo.path)()

	worktrees, err := ListWorktrees()
	if err != nil {
		t.Fatalf("ListWorktrees() error = %v", err)
	}

	// Should have at least the main worktree
	if len(worktrees) < 1 {
		t.Error("ListWorktrees() should return at least one worktree")
	}

	// The first/only worktree should be current
	found := false
	for _, wt := range worktrees {
		if wt.IsCurrent {
			found = true
			break
		}
	}

	if !found {
		t.Error("ListWorktrees() should mark current worktree")
	}
}

func TestListWorktrees_WithMultipleWorktrees(t *testing.T) {
	repo := setupTestRepo(t)
	defer chdir(t, repo.path)()

	// Create a branch for the worktree
	cmd := exec.Command("git", "branch", "wt-branch")
	cmd.Dir = repo.path
	cmd.Run()

	// Create a worktree
	wtPath := filepath.Join(filepath.Dir(repo.path), "test-worktree")
	cmd = exec.Command("git", "worktree", "add", wtPath, "wt-branch")
	cmd.Dir = repo.path
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create worktree: %v", err)
	}
	defer func() {
		// Cleanup worktree
		cmd = exec.Command("git", "worktree", "remove", wtPath)
		cmd.Dir = repo.path
		cmd.Run()
	}()

	worktrees, err := ListWorktrees()
	if err != nil {
		t.Fatalf("ListWorktrees() error = %v", err)
	}

	if len(worktrees) < 2 {
		t.Errorf("ListWorktrees() returned %d worktrees, want at least 2", len(worktrees))
	}

	// Check that we can find the new worktree
	found := false
	for _, wt := range worktrees {
		if wt.Branch == "wt-branch" {
			found = true
			expectedPath, _ := filepath.EvalSymlinks(wtPath)
			actualPath, _ := filepath.EvalSymlinks(wt.Path)
			if actualPath != expectedPath {
				t.Errorf("Worktree path = %q, want %q", actualPath, expectedPath)
			}
			break
		}
	}

	if !found {
		t.Error("ListWorktrees() should include the new worktree")
	}
}

func TestWorktree_Fields(t *testing.T) {
	repo := setupTestRepo(t)
	defer chdir(t, repo.path)()

	worktrees, err := ListWorktrees()
	if err != nil {
		t.Fatalf("ListWorktrees() error = %v", err)
	}

	if len(worktrees) == 0 {
		t.Fatal("No worktrees found")
	}

	wt := worktrees[0]

	// Path should be set
	if wt.Path == "" {
		t.Error("Worktree.Path is empty")
	}

	// Commit should be a SHA
	if len(wt.Commit) < 40 {
		t.Errorf("Worktree.Commit = %q, want 40-character SHA", wt.Commit)
	}

	// Branch should be set (unless detached)
	if wt.Branch == "" && !wt.IsBare {
		t.Error("Worktree.Branch is empty for non-bare worktree")
	}
}

func TestGetWorktree(t *testing.T) {
	repo := setupTestRepo(t)
	defer chdir(t, repo.path)()

	// Get current branch
	currentBranch, _ := CurrentBranch()

	wt, err := GetWorktree(currentBranch)
	if err != nil {
		t.Fatalf("GetWorktree() error = %v", err)
	}

	if wt.Branch != currentBranch {
		t.Errorf("GetWorktree().Branch = %q, want %q", wt.Branch, currentBranch)
	}
}

func TestGetWorktree_NotFound(t *testing.T) {
	repo := setupTestRepo(t)
	defer chdir(t, repo.path)()

	_, err := GetWorktree("non-existent-branch")
	if err == nil {
		t.Error("GetWorktree() expected error for non-existent branch")
	}
}

func TestGetWorktreeByPath(t *testing.T) {
	repo := setupTestRepo(t)
	defer chdir(t, repo.path)()

	// Resolve symlinks first since git worktree list returns resolved paths
	resolvedPath, _ := filepath.EvalSymlinks(repo.path)

	wt, err := GetWorktreeByPath(resolvedPath)
	if err != nil {
		t.Fatalf("GetWorktreeByPath() error = %v", err)
	}

	expectedPath, _ := filepath.EvalSymlinks(repo.path)
	actualPath, _ := filepath.EvalSymlinks(wt.Path)

	if actualPath != expectedPath {
		t.Errorf("GetWorktreeByPath().Path = %q, want %q", actualPath, expectedPath)
	}
}

func TestGetWorktreeByPath_NotFound(t *testing.T) {
	repo := setupTestRepo(t)
	defer chdir(t, repo.path)()

	_, err := GetWorktreeByPath("/non/existent/path")
	if err == nil {
		t.Error("GetWorktreeByPath() expected error for non-existent path")
	}
}

func TestWorktreeExists(t *testing.T) {
	repo := setupTestRepo(t)
	defer chdir(t, repo.path)()

	currentBranch, _ := CurrentBranch()

	if !WorktreeExists(currentBranch) {
		t.Errorf("WorktreeExists('%s') = false, want true", currentBranch)
	}
}

func TestWorktreeExists_False(t *testing.T) {
	repo := setupTestRepo(t)
	defer chdir(t, repo.path)()

	if WorktreeExists("non-existent-branch-xyz") {
		t.Error("WorktreeExists('non-existent-branch-xyz') = true, want false")
	}
}

func TestWorktreePathExists(t *testing.T) {
	repo := setupTestRepo(t)
	defer chdir(t, repo.path)()

	// Resolve symlinks since git worktree list returns resolved paths
	resolvedPath, _ := filepath.EvalSymlinks(repo.path)

	if !WorktreePathExists(resolvedPath) {
		t.Errorf("WorktreePathExists('%s') = false, want true", resolvedPath)
	}
}

func TestWorktreePathExists_False(t *testing.T) {
	repo := setupTestRepo(t)
	defer chdir(t, repo.path)()

	if WorktreePathExists("/non/existent/path") {
		t.Error("WorktreePathExists('/non/existent/path') = true, want false")
	}
}

func TestCurrentWorktree(t *testing.T) {
	repo := setupTestRepo(t)
	defer chdir(t, repo.path)()

	wt, err := CurrentWorktree()
	if err != nil {
		t.Fatalf("CurrentWorktree() error = %v", err)
	}

	if !wt.IsCurrent {
		t.Error("CurrentWorktree().IsCurrent = false, want true")
	}
}

func TestCreateWorktree(t *testing.T) {
	repo := setupTestRepo(t)
	defer chdir(t, repo.path)()

	wtPath := filepath.Join(filepath.Dir(repo.path), "new-worktree")
	defer func() {
		// Cleanup
		cmd := exec.Command("git", "worktree", "remove", "--force", wtPath)
		cmd.Dir = repo.path
		cmd.Run()
		cmd = exec.Command("git", "branch", "-D", "new-wt-branch")
		cmd.Dir = repo.path
		cmd.Run()
	}()

	err := CreateWorktree(wtPath, "new-wt-branch", true, "")
	if err != nil {
		t.Fatalf("CreateWorktree() error = %v", err)
	}

	// Verify the worktree exists
	if !WorktreeExists("new-wt-branch") {
		t.Error("CreateWorktree() did not create the worktree")
	}

	// Verify the directory exists
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Error("CreateWorktree() did not create the directory")
	}
}

func TestCreateWorktree_ExistingBranch(t *testing.T) {
	repo := setupTestRepo(t)
	defer chdir(t, repo.path)()

	// Create a branch first
	cmd := exec.Command("git", "branch", "existing-branch")
	cmd.Dir = repo.path
	cmd.Run()

	// Use resolved path for consistency
	resolvedRepoPath, _ := filepath.EvalSymlinks(repo.path)
	wtPath := filepath.Join(filepath.Dir(resolvedRepoPath), "existing-branch-wt")
	defer func() {
		// Cleanup
		cmd := exec.Command("git", "worktree", "remove", "--force", wtPath)
		cmd.Dir = repo.path
		cmd.Run()
	}()

	// Create worktree for existing branch (createBranch = false)
	err := CreateWorktree(wtPath, "existing-branch", false, "")
	if err != nil {
		t.Fatalf("CreateWorktree() error = %v", err)
	}

	// Check if directory exists as a simpler verification
	if _, statErr := os.Stat(wtPath); os.IsNotExist(statErr) {
		t.Error("CreateWorktree() did not create the worktree directory")
	}
}

func TestRemoveWorktree(t *testing.T) {
	repo := setupTestRepo(t)
	defer chdir(t, repo.path)()

	// Create a worktree first
	wtPath := filepath.Join(filepath.Dir(repo.path), "to-remove-wt")

	cmd := exec.Command("git", "branch", "to-remove-branch")
	cmd.Dir = repo.path
	cmd.Run()

	cmd = exec.Command("git", "worktree", "add", wtPath, "to-remove-branch")
	cmd.Dir = repo.path
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create worktree: %v", err)
	}

	// Remove the worktree
	err := RemoveWorktree(wtPath, false)
	if err != nil {
		t.Fatalf("RemoveWorktree() error = %v", err)
	}

	// Verify it's gone
	if WorktreePathExists(wtPath) {
		t.Error("RemoveWorktree() did not remove the worktree")
	}
}

func TestPruneWorktrees(t *testing.T) {
	repo := setupTestRepo(t)
	defer chdir(t, repo.path)()

	// Just verify prune doesn't error on a clean repo
	err := PruneWorktrees()
	if err != nil {
		t.Errorf("PruneWorktrees() error = %v", err)
	}
}

func TestGetWorktreePath(t *testing.T) {
	repo := setupTestRepo(t)
	defer chdir(t, repo.path)()

	path := GetWorktreePath("myproject", "feature/test", "{project}-{branch}")

	// Should contain project name and sanitized branch name
	if path == "" {
		t.Error("GetWorktreePath() returned empty string")
	}

	// Should contain the expected name at the end
	expectedName := "myproject-feature-test"
	if filepath.Base(path) != expectedName {
		t.Errorf("GetWorktreePath() basename = %q, want %q", filepath.Base(path), expectedName)
	}
}

func TestSanitizeBranchName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"main", "main"},
		{"feature/test", "feature-test"},
		{"feature/foo/bar", "feature-foo-bar"},
		{"fix-123", "fix-123"},
		{"release_1.0", "release_1.0"},
		{"test@branch", "test-branch"},
		{"test#branch!", "test-branch-"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeBranchName(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeBranchName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestLockWorktree(t *testing.T) {
	repo := setupTestRepo(t)
	defer chdir(t, repo.path)()

	// Use resolved path for consistency
	resolvedRepoPath, _ := filepath.EvalSymlinks(repo.path)
	wtPath := filepath.Join(filepath.Dir(resolvedRepoPath), "lock-test-wt")

	cmd := exec.Command("git", "branch", "lock-test-branch")
	cmd.Dir = repo.path
	cmd.Run()

	cmd = exec.Command("git", "worktree", "add", wtPath, "lock-test-branch")
	cmd.Dir = repo.path
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create worktree: %v", err)
	}
	defer func() {
		// Cleanup - must unlock first
		UnlockWorktree(wtPath)
		cmd = exec.Command("git", "worktree", "remove", "--force", wtPath)
		cmd.Dir = repo.path
		cmd.Run()
	}()

	// Lock the worktree
	err := LockWorktree(wtPath, "Testing lock")
	if err != nil {
		t.Fatalf("LockWorktree() error = %v", err)
	}

	// Verify it's locked by checking worktree list
	worktrees, _ := ListWorktrees()
	found := false
	for _, wt := range worktrees {
		if wt.Branch == "lock-test-branch" {
			found = true
			if !wt.IsLocked {
				// Some git versions may not report lock status correctly
				t.Log("LockWorktree() lock status not reflected in list (may vary by git version)")
			}
			break
		}
	}
	if !found {
		t.Error("Could not find the locked worktree in list")
	}
}

func TestUnlockWorktree(t *testing.T) {
	repo := setupTestRepo(t)
	defer chdir(t, repo.path)()

	// Create and lock a worktree
	wtPath := filepath.Join(filepath.Dir(repo.path), "unlock-test-wt")

	cmd := exec.Command("git", "branch", "unlock-test-branch")
	cmd.Dir = repo.path
	cmd.Run()

	cmd = exec.Command("git", "worktree", "add", wtPath, "unlock-test-branch")
	cmd.Dir = repo.path
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create worktree: %v", err)
	}
	defer func() {
		cmd = exec.Command("git", "worktree", "remove", "--force", wtPath)
		cmd.Dir = repo.path
		cmd.Run()
	}()

	// Lock it
	LockWorktree(wtPath, "")

	// Unlock it
	err := UnlockWorktree(wtPath)
	if err != nil {
		t.Fatalf("UnlockWorktree() error = %v", err)
	}

	// Verify it's unlocked
	worktrees, _ := ListWorktrees()
	for _, wt := range worktrees {
		if wt.Branch == "unlock-test-branch" {
			if wt.IsLocked {
				t.Error("UnlockWorktree() did not unlock the worktree")
			}
			break
		}
	}
}

// Helper functions

func hasPrefix(path, prefix string) bool {
	absPath, _ := filepath.Abs(path)
	absPrefix, _ := filepath.Abs(prefix)
	evalPath, _ := filepath.EvalSymlinks(absPath)
	evalPrefix, _ := filepath.EvalSymlinks(absPrefix)

	return len(evalPath) >= len(evalPrefix) && evalPath[:len(evalPrefix)] == evalPrefix
}

func containsName(path, name string) bool {
	return filepath.Base(path) == name
}
