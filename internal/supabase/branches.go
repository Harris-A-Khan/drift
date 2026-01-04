package supabase

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/undrift/drift/pkg/shell"
)

// Branch represents a Supabase branch.
type Branch struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	GitBranch  string `json:"git_branch"`
	ProjectRef string `json:"project_ref"`
	IsDefault  bool   `json:"is_default"`
	Persistent bool   `json:"persistent"`
	Status     string `json:"status"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// Environment represents the deployment environment type.
type Environment string

const (
	// EnvProduction is the production environment (main branch).
	EnvProduction Environment = "Production"
	// EnvDevelopment is the development environment (persistent branch).
	EnvDevelopment Environment = "Development"
	// EnvFeature is a feature branch environment (preview branch).
	EnvFeature Environment = "Feature"
)

// GetBranches fetches all Supabase branches for the linked project.
func (c *Client) GetBranches() ([]Branch, error) {
	result, err := shell.Run("supabase", "branches", "list", "--output", "json")
	if err != nil {
		// Check if branching is not enabled
		if strings.Contains(result.Stderr, "not enabled") || strings.Contains(result.Stdout, "not enabled") {
			return nil, fmt.Errorf("branching is not enabled for this project")
		}
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	var branches []Branch
	if err := json.Unmarshal([]byte(result.Stdout), &branches); err != nil {
		return nil, fmt.Errorf("failed to parse branches: %w", err)
	}

	return branches, nil
}

// GetBranch gets a specific branch by name.
func (c *Client) GetBranch(name string) (*Branch, error) {
	branches, err := c.GetBranches()
	if err != nil {
		return nil, err
	}

	for _, b := range branches {
		if b.Name == name || b.GitBranch == name {
			return &b, nil
		}
	}

	return nil, fmt.Errorf("branch '%s' not found", name)
}

// ResolveBranch finds the Supabase branch for a git branch and returns the environment type.
func (c *Client) ResolveBranch(gitBranch string) (*Branch, Environment, error) {
	branches, err := c.GetBranches()
	if err != nil {
		return nil, "", err
	}

	// First, try to find an exact match by git_branch
	for _, b := range branches {
		if b.GitBranch == gitBranch {
			env := c.determineEnvironment(&b)
			return &b, env, nil
		}
	}

	// Try to find by name
	for _, b := range branches {
		if b.Name == gitBranch {
			env := c.determineEnvironment(&b)
			return &b, env, nil
		}
	}

	// No match found - return nil but not an error (caller can decide to fallback)
	return nil, "", nil
}

// ResolveBranchWithFallback finds the Supabase branch for a git branch,
// falling back to development if not found.
func (c *Client) ResolveBranchWithFallback(gitBranch string) (*Branch, Environment, error) {
	branch, env, err := c.ResolveBranch(gitBranch)
	if err != nil {
		return nil, "", err
	}

	if branch != nil {
		return branch, env, nil
	}

	// Fallback to development
	branch, env, err = c.ResolveBranch("development")
	if err != nil {
		return nil, "", err
	}

	if branch != nil {
		return branch, env, nil
	}

	return nil, "", fmt.Errorf("no matching Supabase branch found for '%s' and no development branch available", gitBranch)
}

// determineEnvironment determines the environment type for a branch.
func (c *Client) determineEnvironment(b *Branch) Environment {
	if b.IsDefault {
		return EnvProduction
	}
	if b.Persistent {
		return EnvDevelopment
	}
	return EnvFeature
}

// GetProductionBranch returns the production (default) branch.
func (c *Client) GetProductionBranch() (*Branch, error) {
	branches, err := c.GetBranches()
	if err != nil {
		return nil, err
	}

	for _, b := range branches {
		if b.IsDefault {
			return &b, nil
		}
	}

	return nil, fmt.Errorf("no production branch found")
}

// GetDevelopmentBranch returns the development (persistent non-default) branch.
func (c *Client) GetDevelopmentBranch() (*Branch, error) {
	branches, err := c.GetBranches()
	if err != nil {
		return nil, err
	}

	for _, b := range branches {
		if b.Persistent && !b.IsDefault {
			return &b, nil
		}
		// Also check by name
		if b.GitBranch == "development" || b.Name == "development" {
			return &b, nil
		}
	}

	return nil, fmt.Errorf("no development branch found")
}

// CreateBranch creates a new Supabase preview branch.
func (c *Client) CreateBranch(gitBranch string) (*Branch, error) {
	result, err := shell.Run("supabase", "branches", "create", gitBranch, "--output", "json")
	if err != nil {
		errMsg := result.Stderr
		if errMsg == "" {
			errMsg = err.Error()
		}
		return nil, fmt.Errorf("failed to create branch: %s", errMsg)
	}

	var branch Branch
	if err := json.Unmarshal([]byte(result.Stdout), &branch); err != nil {
		// Try to get the branch after creation
		return c.GetBranch(gitBranch)
	}

	return &branch, nil
}

// DeleteBranch deletes a Supabase preview branch.
func (c *Client) DeleteBranch(branchName string) error {
	result, err := shell.Run("supabase", "branches", "delete", branchName)
	if err != nil {
		errMsg := result.Stderr
		if errMsg == "" {
			errMsg = err.Error()
		}
		return fmt.Errorf("failed to delete branch: %s", errMsg)
	}
	return nil
}

// GetBranchURL returns the API URL for a specific branch.
func (c *Client) GetBranchURL(projectRef string) string {
	return fmt.Sprintf("https://%s.supabase.co", projectRef)
}

// BranchInfo holds resolved branch information for display.
type BranchInfo struct {
	GitBranch      string
	SupabaseBranch *Branch
	Environment    Environment
	ProjectRef     string
	APIURL         string
	IsFallback     bool
	IsOverride     bool   // True if using override_branch from config
	OverrideFrom   string // Original git branch if overridden
}

// GetBranchInfo resolves full branch information for a git branch.
// Logic:
//   - main → production Supabase branch
//   - development → development Supabase branch
//   - other → find matching Supabase branch, fallback to development
func (c *Client) GetBranchInfo(gitBranch string) (*BranchInfo, error) {
	return c.GetBranchInfoWithOverride(gitBranch, "")
}

// GetBranchInfoWithOverride resolves branch info, optionally using an override branch.
// If overrideBranch is set, it takes priority over the git branch.
func (c *Client) GetBranchInfoWithOverride(gitBranch, overrideBranch string) (*BranchInfo, error) {
	info := &BranchInfo{
		GitBranch: gitBranch,
	}

	// Determine target branch
	targetBranch := gitBranch
	if overrideBranch != "" {
		targetBranch = overrideBranch
		info.IsOverride = true
		info.OverrideFrom = gitBranch
	}

	// First try exact match with target branch
	branch, env, err := c.ResolveBranch(targetBranch)
	if err != nil {
		return nil, err
	}

	if branch == nil {
		// Try fallback to development
		branch, env, err = c.ResolveBranch("development")
		if err != nil {
			return nil, err
		}
		if branch == nil {
			return nil, fmt.Errorf("no Supabase branch found for '%s'", targetBranch)
		}
		info.IsFallback = true
	}

	info.SupabaseBranch = branch
	info.Environment = env
	info.ProjectRef = branch.ProjectRef
	info.APIURL = c.GetBranchURL(branch.ProjectRef)

	return info, nil
}

// FindSimilarBranches returns Supabase branches with names similar to the query.
// Uses simple substring matching for now.
func (c *Client) FindSimilarBranches(query string) ([]Branch, error) {
	branches, err := c.GetBranches()
	if err != nil {
		return nil, err
	}

	var similar []Branch
	queryLower := strings.ToLower(query)

	for _, b := range branches {
		nameLower := strings.ToLower(b.Name)
		gitBranchLower := strings.ToLower(b.GitBranch)

		// Check if query is a substring of branch name or git branch
		if strings.Contains(nameLower, queryLower) || strings.Contains(gitBranchLower, queryLower) {
			similar = append(similar, b)
			continue
		}

		// Check if branch name/git branch contains parts of the query (split by / or -)
		queryParts := strings.FieldsFunc(queryLower, func(r rune) bool {
			return r == '/' || r == '-' || r == '_'
		})
		for _, part := range queryParts {
			if len(part) > 2 && (strings.Contains(nameLower, part) || strings.Contains(gitBranchLower, part)) {
				similar = append(similar, b)
				break
			}
		}
	}

	return similar, nil
}

