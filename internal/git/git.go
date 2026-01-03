// Package git provides utilities for git operations.
package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/undrift/drift/pkg/shell"
)

// IsGitRepository checks if the current directory is in a git repository.
func IsGitRepository() bool {
	_, err := shell.Run("git", "rev-parse", "--git-dir")
	return err == nil
}

// GetRepoRoot returns the root directory of the git repository.
func GetRepoRoot() (string, error) {
	result, err := shell.Run("git", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("not a git repository: %w", err)
	}
	return result.Stdout, nil
}

// GetGitDir returns the path to the .git directory.
func GetGitDir() (string, error) {
	result, err := shell.Run("git", "rev-parse", "--git-dir")
	if err != nil {
		return "", fmt.Errorf("failed to get git dir: %w", err)
	}

	gitDir := result.Stdout
	if !filepath.IsAbs(gitDir) {
		cwd, _ := os.Getwd()
		gitDir = filepath.Join(cwd, gitDir)
	}

	return gitDir, nil
}

// GetCommonDir returns the path to the common git directory (for worktrees).
func GetCommonDir() (string, error) {
	result, err := shell.Run("git", "rev-parse", "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("failed to get git common dir: %w", err)
	}

	commonDir := result.Stdout
	if !filepath.IsAbs(commonDir) {
		cwd, _ := os.Getwd()
		commonDir = filepath.Join(cwd, commonDir)
	}

	return commonDir, nil
}

// IsWorktree returns true if the current directory is a git worktree.
func IsWorktree() bool {
	gitDir, err := GetGitDir()
	if err != nil {
		return false
	}

	commonDir, err := GetCommonDir()
	if err != nil {
		return false
	}

	// If git-dir and common-dir are different, we're in a worktree
	return gitDir != commonDir
}

// GetMainWorktreePath returns the path to the main worktree.
func GetMainWorktreePath() (string, error) {
	commonDir, err := GetCommonDir()
	if err != nil {
		return "", err
	}

	// The common dir is typically the .git folder of the main worktree
	// The main worktree is its parent
	return filepath.Dir(commonDir), nil
}

// GetRemoteURL returns the URL of the specified remote (default: origin).
func GetRemoteURL(remote string) (string, error) {
	if remote == "" {
		remote = "origin"
	}

	result, err := shell.Run("git", "remote", "get-url", remote)
	if err != nil {
		return "", fmt.Errorf("failed to get remote URL: %w", err)
	}

	return result.Stdout, nil
}

// Fetch fetches from the specified remote.
func Fetch(remote string) error {
	if remote == "" {
		remote = "origin"
	}

	_, err := shell.Run("git", "fetch", remote)
	if err != nil {
		return fmt.Errorf("failed to fetch from %s: %w", remote, err)
	}

	return nil
}

// FetchPrune fetches with pruning of deleted remote branches.
func FetchPrune(remote string) error {
	if remote == "" {
		remote = "origin"
	}

	_, err := shell.Run("git", "fetch", "--prune", remote)
	if err != nil {
		return fmt.Errorf("failed to fetch from %s: %w", remote, err)
	}

	return nil
}

// Status returns the current git status.
func Status() (string, error) {
	result, err := shell.Run("git", "status", "--porcelain")
	if err != nil {
		return "", fmt.Errorf("failed to get git status: %w", err)
	}
	return result.Stdout, nil
}

// IsDirty returns true if there are uncommitted changes.
func IsDirty() (bool, error) {
	status, err := Status()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(status) != "", nil
}

// Stash stashes current changes.
func Stash(message string) error {
	args := []string{"stash", "push"}
	if message != "" {
		args = append(args, "-m", message)
	}

	_, err := shell.Run("git", args...)
	if err != nil {
		return fmt.Errorf("failed to stash changes: %w", err)
	}

	return nil
}

// StashPop pops the last stash.
func StashPop() error {
	_, err := shell.Run("git", "stash", "pop")
	if err != nil {
		return fmt.Errorf("failed to pop stash: %w", err)
	}
	return nil
}

// Checkout checks out the specified ref (branch, tag, or commit).
func Checkout(ref string) error {
	_, err := shell.Run("git", "checkout", ref)
	if err != nil {
		return fmt.Errorf("failed to checkout %s: %w", ref, err)
	}
	return nil
}

// Pull pulls from the current tracking branch.
func Pull() error {
	_, err := shell.Run("git", "pull")
	if err != nil {
		return fmt.Errorf("failed to pull: %w", err)
	}
	return nil
}

