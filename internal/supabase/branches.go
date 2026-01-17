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

// BranchSecrets contains all secrets for a Supabase branch.
// Available for non-production branches via `supabase branches get`.
type BranchSecrets struct {
	PostgresURL           string `json:"POSTGRES_URL"`
	PostgresURLNonPooling string `json:"POSTGRES_URL_NON_POOLING"`
	SupabaseAnonKey       string `json:"SUPABASE_ANON_KEY"`
	SupabaseJWTSecret     string `json:"SUPABASE_JWT_SECRET"`
	SupabaseServiceRoleKey string `json:"SUPABASE_SERVICE_ROLE_KEY"`
	SupabaseURL           string `json:"SUPABASE_URL"`
}

// BranchConnectionInfo contains parsed connection information from the experimental API.
type BranchConnectionInfo struct {
	PostgresURL           string // Full pooler connection URL (transaction mode, port 6543)
	PostgresURLNonPooling string // Direct connection URL (IPv6 only)
	PoolerHost            string // e.g., aws-1-us-east-2.pooler.supabase.com
	PoolerPort            int    // 6543 for transaction mode, 5432 for session mode
	ProjectRef            string // e.g., gkhvtzjajeykbashnavj
	SupabaseURL           string // e.g., https://gkhvtzjajeykbashnavj.supabase.co
	SupabaseAnonKey       string
	SupabaseServiceRoleKey string
}

// GetBranchSecrets retrieves all secrets for a non-production branch.
// This only works for preview/development branches, not production.
func (c *Client) GetBranchSecrets(branchName string) (*BranchSecrets, error) {
	result, err := shell.Run("supabase", "branches", "get", branchName, "--output", "json")
	if err != nil {
		return nil, fmt.Errorf("failed to get branch secrets: %w - %s", err, result.Stderr)
	}

	var secrets BranchSecrets
	if err := json.Unmarshal([]byte(result.Stdout), &secrets); err != nil {
		return nil, fmt.Errorf("failed to parse branch secrets: %w", err)
	}

	return &secrets, nil
}

// GetBranchConnectionInfo retrieves connection info using the experimental API.
// This returns the full pooler URL including the correct regional host.
func (c *Client) GetBranchConnectionInfo(branchName, projectRef string) (*BranchConnectionInfo, error) {
	args := []string{"branches", "get", branchName, "--experimental", "--output", "env"}
	if projectRef != "" {
		args = append(args, "--project-ref", projectRef)
	}

	result, err := shell.Run("supabase", args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get branch connection info: %w - %s", err, result.Stderr)
	}

	return ParseBranchEnvOutput(result.Stdout)
}

// ParseBranchEnvOutput parses the env output from `supabase branches get --experimental --output env`.
func ParseBranchEnvOutput(output string) (*BranchConnectionInfo, error) {
	info := &BranchConnectionInfo{}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "=") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := strings.Trim(parts[1], "\"")

		switch key {
		case "POSTGRES_URL":
			info.PostgresURL = value
		case "POSTGRES_URL_NON_POOLING":
			info.PostgresURLNonPooling = value
		case "SUPABASE_URL":
			info.SupabaseURL = value
		case "SUPABASE_ANON_KEY":
			info.SupabaseAnonKey = value
		case "SUPABASE_SERVICE_ROLE_KEY":
			info.SupabaseServiceRoleKey = value
		}
	}

	// Parse pooler info from POSTGRES_URL
	if info.PostgresURL != "" {
		host, port := ExtractHostPortFromURL(info.PostgresURL)
		info.PoolerHost = host
		info.PoolerPort = port

		// Extract project ref from username (postgres.{ref})
		user := ExtractUserFromURL(info.PostgresURL)
		if strings.HasPrefix(user, "postgres.") {
			info.ProjectRef = strings.TrimPrefix(user, "postgres.")
		}
	}

	return info, nil
}

// ExtractHostPortFromURL extracts host and port from a PostgreSQL URL.
// URL format: postgresql://user:password@host:port/database
func ExtractHostPortFromURL(url string) (string, int) {
	// Find @ symbol
	atIdx := strings.Index(url, "@")
	if atIdx == -1 {
		return "", 0
	}

	rest := url[atIdx+1:] // After @

	// Find / for database
	slashIdx := strings.Index(rest, "/")
	if slashIdx == -1 {
		slashIdx = len(rest)
	}
	hostPort := rest[:slashIdx]

	// Handle query params
	if qIdx := strings.Index(hostPort, "?"); qIdx != -1 {
		hostPort = hostPort[:qIdx]
	}

	// Split host:port
	colonIdx := strings.LastIndex(hostPort, ":")
	if colonIdx == -1 {
		return hostPort, 5432 // Default port
	}

	host := hostPort[:colonIdx]
	portStr := hostPort[colonIdx+1:]

	port := 5432
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		port = 5432
	}

	return host, port
}

// ExtractUserFromURL extracts the username from a PostgreSQL URL.
func ExtractUserFromURL(url string) string {
	// Find ://
	colonIdx := strings.Index(url, "://")
	if colonIdx == -1 {
		return ""
	}
	rest := url[colonIdx+3:] // Skip "://"

	// Find @ symbol
	atIdx := strings.Index(rest, "@")
	if atIdx == -1 {
		return ""
	}
	userPass := rest[:atIdx]

	// Find : in user:pass
	colonIdx = strings.Index(userPass, ":")
	if colonIdx == -1 {
		return userPass
	}

	return userPass[:colonIdx]
}

// ExtractPasswordFromURL extracts the password from a PostgreSQL connection URL.
// URL format: postgresql://user:password@host:port/database
// Returns empty string if password is masked (e.g., "******" from API).
func ExtractPasswordFromURL(url string) string {
	// Find the password between : and @
	// postgresql://postgres.ref:PASSWORD@host:port/database
	colonIdx := strings.Index(url, "://")
	if colonIdx == -1 {
		return ""
	}
	rest := url[colonIdx+3:] // Skip "://"

	// Find the @ symbol
	atIdx := strings.Index(rest, "@")
	if atIdx == -1 {
		return ""
	}
	userPass := rest[:atIdx]

	// Find the colon in user:pass
	colonIdx = strings.Index(userPass, ":")
	if colonIdx == -1 {
		return ""
	}

	password := userPass[colonIdx+1:]

	// URL-decode the password (e.g., %2A -> *)
	if decoded, err := urlDecode(password); err == nil {
		password = decoded
	}

	// Check if password is masked (API returns ****** for production)
	if isMaskedPassword(password) {
		return ""
	}

	return password
}

// urlDecode decodes a URL-encoded string.
func urlDecode(s string) (string, error) {
	// Simple URL decoding for common cases
	result := strings.ReplaceAll(s, "%2A", "*")
	result = strings.ReplaceAll(result, "%2a", "*")
	result = strings.ReplaceAll(result, "%40", "@")
	result = strings.ReplaceAll(result, "%3A", ":")
	result = strings.ReplaceAll(result, "%3a", ":")
	result = strings.ReplaceAll(result, "%2F", "/")
	result = strings.ReplaceAll(result, "%2f", "/")
	return result, nil
}

// isMaskedPassword checks if a password is masked (all asterisks).
func isMaskedPassword(password string) bool {
	if password == "" {
		return false
	}
	for _, c := range password {
		if c != '*' {
			return false
		}
	}
	return true
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
	Region         string // AWS region (e.g., us-east-1)
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

	// Get region from project info
	if project, err := c.FindProjectByRef(branch.ProjectRef); err == nil && project != nil {
		info.Region = project.Region
	}

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

