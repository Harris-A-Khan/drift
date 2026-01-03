package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/undrift/drift/pkg/shell"
)

// Worktree represents a git worktree.
type Worktree struct {
	Path     string
	Commit   string
	Branch   string
	IsBare   bool
	IsLocked bool
	IsCurrent bool
}

// ListWorktrees returns all worktrees in the repository.
func ListWorktrees() ([]Worktree, error) {
	result, err := shell.Run("git", "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	currentDir, _ := os.Getwd()
	worktrees := []Worktree{}
	var current *Worktree

	lines := strings.Split(result.Stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if current != nil {
				current.IsCurrent = current.Path == currentDir
				worktrees = append(worktrees, *current)
				current = nil
			}
			continue
		}

		if strings.HasPrefix(line, "worktree ") {
			current = &Worktree{
				Path: strings.TrimPrefix(line, "worktree "),
			}
		} else if strings.HasPrefix(line, "HEAD ") {
			if current != nil {
				current.Commit = strings.TrimPrefix(line, "HEAD ")
			}
		} else if strings.HasPrefix(line, "branch ") {
			if current != nil {
				// refs/heads/branch-name -> branch-name
				branch := strings.TrimPrefix(line, "branch ")
				branch = strings.TrimPrefix(branch, "refs/heads/")
				current.Branch = branch
			}
		} else if line == "bare" {
			if current != nil {
				current.IsBare = true
			}
		} else if line == "locked" {
			if current != nil {
				current.IsLocked = true
			}
		} else if line == "detached" {
			if current != nil {
				current.Branch = "(detached)"
			}
		}
	}

	// Don't forget the last one
	if current != nil {
		current.IsCurrent = current.Path == currentDir
		worktrees = append(worktrees, *current)
	}

	return worktrees, nil
}

// GetWorktree returns the worktree for the given branch.
func GetWorktree(branch string) (*Worktree, error) {
	worktrees, err := ListWorktrees()
	if err != nil {
		return nil, err
	}

	for _, wt := range worktrees {
		if wt.Branch == branch {
			return &wt, nil
		}
	}

	return nil, fmt.Errorf("no worktree found for branch '%s'", branch)
}

// GetWorktreeByPath returns the worktree at the given path.
func GetWorktreeByPath(path string) (*Worktree, error) {
	worktrees, err := ListWorktrees()
	if err != nil {
		return nil, err
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	for _, wt := range worktrees {
		if wt.Path == absPath {
			return &wt, nil
		}
	}

	return nil, fmt.Errorf("no worktree found at path '%s'", path)
}

// CreateWorktree creates a new worktree for the given branch at the specified path.
func CreateWorktree(path, branch string, createBranch bool, baseBranch string) error {
	args := []string{"worktree", "add"}

	if createBranch {
		args = append(args, "-b", branch)
		args = append(args, path)
		if baseBranch != "" {
			args = append(args, baseBranch)
		}
	} else {
		args = append(args, path, branch)
	}

	result, err := shell.Run("git", args...)
	if err != nil {
		errMsg := result.Stderr
		if errMsg == "" {
			errMsg = err.Error()
		}
		return fmt.Errorf("failed to create worktree: %s", errMsg)
	}

	return nil
}

// CreateWorktreeFromRemote creates a new worktree tracking a remote branch.
func CreateWorktreeFromRemote(path, remoteBranch, localBranch string) error {
	// Create worktree with new branch tracking the remote
	args := []string{"worktree", "add", "-b", localBranch, path, "origin/" + remoteBranch}

	result, err := shell.Run("git", args...)
	if err != nil {
		errMsg := result.Stderr
		if errMsg == "" {
			errMsg = err.Error()
		}
		return fmt.Errorf("failed to create worktree: %s", errMsg)
	}

	// Set up tracking
	_, err = shell.RunInDir(path, "git", "branch", "--set-upstream-to", "origin/"+remoteBranch)
	if err != nil {
		// Non-fatal, tracking can be set up later
	}

	return nil
}

// RemoveWorktree removes a worktree.
func RemoveWorktree(path string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)

	result, err := shell.Run("git", args...)
	if err != nil {
		errMsg := result.Stderr
		if errMsg == "" {
			errMsg = err.Error()
		}
		return fmt.Errorf("failed to remove worktree: %s", errMsg)
	}

	return nil
}

// PruneWorktrees removes stale worktree entries.
func PruneWorktrees() error {
	_, err := shell.Run("git", "worktree", "prune")
	if err != nil {
		return fmt.Errorf("failed to prune worktrees: %w", err)
	}
	return nil
}

// LockWorktree locks a worktree with an optional reason.
func LockWorktree(path, reason string) error {
	args := []string{"worktree", "lock"}
	if reason != "" {
		args = append(args, "--reason", reason)
	}
	args = append(args, path)

	_, err := shell.Run("git", args...)
	if err != nil {
		return fmt.Errorf("failed to lock worktree: %w", err)
	}
	return nil
}

// UnlockWorktree unlocks a worktree.
func UnlockWorktree(path string) error {
	_, err := shell.Run("git", "worktree", "unlock", path)
	if err != nil {
		return fmt.Errorf("failed to unlock worktree: %w", err)
	}
	return nil
}

// MoveWorktree moves a worktree to a new path.
func MoveWorktree(oldPath, newPath string) error {
	_, err := shell.Run("git", "worktree", "move", oldPath, newPath)
	if err != nil {
		return fmt.Errorf("failed to move worktree: %w", err)
	}
	return nil
}

// WorktreeExists checks if a worktree exists for the given branch.
func WorktreeExists(branch string) bool {
	wt, _ := GetWorktree(branch)
	return wt != nil
}

// WorktreePathExists checks if a worktree exists at the given path.
func WorktreePathExists(path string) bool {
	wt, _ := GetWorktreeByPath(path)
	return wt != nil
}

// CurrentWorktree returns the worktree for the current directory.
func CurrentWorktree() (*Worktree, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return GetWorktreeByPath(cwd)
}

// GetWorktreePath generates a worktree path based on naming pattern.
func GetWorktreePath(projectName, branch, pattern string) string {
	// Get the parent directory of the main worktree
	mainPath, err := GetMainWorktreePath()
	if err != nil {
		mainPath, _ = os.Getwd()
	}

	parentDir := filepath.Dir(mainPath)

	// Apply naming pattern
	name := pattern
	name = strings.ReplaceAll(name, "{project}", projectName)
	name = strings.ReplaceAll(name, "{branch}", sanitizeBranchName(branch))

	return filepath.Join(parentDir, name)
}

// sanitizeBranchName converts a branch name to a safe directory name.
func sanitizeBranchName(branch string) string {
	// Replace slashes with hyphens
	name := strings.ReplaceAll(branch, "/", "-")
	// Remove any other problematic characters
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			return r
		}
		return '-'
	}, name)
	return name
}

