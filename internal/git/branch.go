package git

import (
	"fmt"
	"strings"

	"github.com/undrift/drift/pkg/shell"
)

// CurrentBranch returns the name of the current git branch.
func CurrentBranch() (string, error) {
	result, err := shell.Run("git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}

	branch := result.Stdout
	if branch == "HEAD" {
		// Detached HEAD state - get the commit hash instead
		result, err = shell.Run("git", "rev-parse", "--short", "HEAD")
		if err != nil {
			return "", fmt.Errorf("failed to get current commit: %w", err)
		}
		return result.Stdout, nil
	}

	return branch, nil
}

// BranchExists checks if a branch exists locally.
func BranchExists(branch string) bool {
	result, _ := shell.Run("git", "rev-parse", "--verify", branch)
	return result != nil && result.ExitCode == 0
}

// RemoteBranchExists checks if a branch exists on a remote.
func RemoteBranchExists(remote, branch string) bool {
	if remote == "" {
		remote = "origin"
	}

	ref := fmt.Sprintf("refs/remotes/%s/%s", remote, branch)
	result, _ := shell.Run("git", "rev-parse", "--verify", ref)
	return result != nil && result.ExitCode == 0
}

// ListBranches returns a list of local branch names.
func ListBranches() ([]string, error) {
	result, err := shell.Run("git", "branch", "--format=%(refname:short)")
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	if result.Stdout == "" {
		return []string{}, nil
	}

	branches := strings.Split(result.Stdout, "\n")
	return branches, nil
}

// ListRemoteBranches returns a list of remote branch names.
func ListRemoteBranches(remote string) ([]string, error) {
	if remote == "" {
		remote = "origin"
	}

	result, err := shell.Run("git", "branch", "-r", "--format=%(refname:short)")
	if err != nil {
		return nil, fmt.Errorf("failed to list remote branches: %w", err)
	}

	if result.Stdout == "" {
		return []string{}, nil
	}

	branches := []string{}
	prefix := remote + "/"

	for _, line := range strings.Split(result.Stdout, "\n") {
		if strings.HasPrefix(line, prefix) {
			// Remove the remote prefix
			branch := strings.TrimPrefix(line, prefix)
			// Skip HEAD pointer
			if branch != "HEAD" && !strings.Contains(branch, "->") {
				branches = append(branches, branch)
			}
		}
	}

	return branches, nil
}

// CreateBranch creates a new branch from the specified base.
func CreateBranch(name, base string) error {
	if base == "" {
		base = "HEAD"
	}

	_, err := shell.Run("git", "branch", name, base)
	if err != nil {
		return fmt.Errorf("failed to create branch %s: %w", name, err)
	}

	return nil
}

// DeleteBranch deletes a local branch.
func DeleteBranch(name string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}

	_, err := shell.Run("git", "branch", flag, name)
	if err != nil {
		return fmt.Errorf("failed to delete branch %s: %w", name, err)
	}

	return nil
}

// TrackingBranch returns the tracking branch for the current branch.
func TrackingBranch() (string, error) {
	result, err := shell.Run("git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if err != nil {
		return "", nil // No tracking branch is not an error
	}

	return result.Stdout, nil
}

// SetUpstreamTo sets the upstream tracking branch.
func SetUpstreamTo(remote, branch string) error {
	upstream := fmt.Sprintf("%s/%s", remote, branch)
	_, err := shell.Run("git", "branch", "--set-upstream-to", upstream)
	if err != nil {
		return fmt.Errorf("failed to set upstream to %s: %w", upstream, err)
	}
	return nil
}

// GetDefaultBranch returns the default branch name (main or master).
func GetDefaultBranch() (string, error) {
	// First check if origin/main exists
	if RemoteBranchExists("origin", "main") {
		return "main", nil
	}

	// Check for origin/master
	if RemoteBranchExists("origin", "master") {
		return "master", nil
	}

	// Check local branches
	if BranchExists("main") {
		return "main", nil
	}

	if BranchExists("master") {
		return "master", nil
	}

	// Try to get from remote HEAD
	result, err := shell.Run("git", "symbolic-ref", "refs/remotes/origin/HEAD")
	if err == nil {
		// Returns something like refs/remotes/origin/main
		parts := strings.Split(result.Stdout, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1], nil
		}
	}

	return "main", nil // Default to main
}

// GetCommitHash returns the full commit hash for a ref.
func GetCommitHash(ref string) (string, error) {
	if ref == "" {
		ref = "HEAD"
	}

	result, err := shell.Run("git", "rev-parse", ref)
	if err != nil {
		return "", fmt.Errorf("failed to get commit hash for %s: %w", ref, err)
	}

	return result.Stdout, nil
}

// GetShortCommitHash returns the short commit hash for a ref.
func GetShortCommitHash(ref string) (string, error) {
	if ref == "" {
		ref = "HEAD"
	}

	result, err := shell.Run("git", "rev-parse", "--short", ref)
	if err != nil {
		return "", fmt.Errorf("failed to get short commit hash for %s: %w", ref, err)
	}

	return result.Stdout, nil
}

