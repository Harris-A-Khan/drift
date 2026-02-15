package supabase

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/undrift/drift/pkg/shell"
)

// Secret represents a Supabase secret.
type Secret struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// SetSecret sets a secret on the specified project/branch.
// Uses the Management API for reliable secret setting on both main projects and branches.
func (c *Client) SetSecret(projectRef, name, value string) error {
	// Use Management API - more reliable for branches
	mgmtClient, err := NewManagementClient()
	if err != nil {
		// Fall back to CLI if Management API is not available
		return c.setSecretViaCLI(projectRef, name, value)
	}

	secrets := []Secret{{Name: name, Value: value}}
	if err := mgmtClient.SetSecrets(projectRef, secrets); err != nil {
		return fmt.Errorf("failed to set secret %s: %w", name, err)
	}

	return nil
}

// setSecretViaCLI sets a secret using the Supabase CLI (fallback).
func (c *Client) setSecretViaCLI(projectRef, name, value string) error {
	args := []string{"secrets", "set", name + "=" + value}
	if projectRef != "" {
		args = append(args, "--project-ref", projectRef)
	}

	result, err := shell.Run("supabase", args...)
	if err != nil {
		errMsg := result.Stderr
		if errMsg == "" {
			errMsg = err.Error()
		}
		return fmt.Errorf("failed to set secret %s: %s", name, errMsg)
	}

	// Check exit code - shell.Run returns nil error but non-zero exit code on failure
	if result.ExitCode != 0 {
		errMsg := result.Stderr
		if errMsg == "" {
			errMsg = result.Stdout
		}
		return fmt.Errorf("failed to set secret %s: %s", name, errMsg)
	}

	return nil
}

// SetSecrets sets multiple secrets at once.
// Uses the Management API for reliable secret setting on both main projects and branches.
func (c *Client) SetSecrets(projectRef string, secrets []Secret) error {
	if len(secrets) == 0 {
		return nil
	}

	// Use Management API - more reliable for branches
	mgmtClient, err := NewManagementClient()
	if err != nil {
		// Fall back to CLI if Management API is not available
		return c.setSecretsViaCLI(projectRef, secrets)
	}

	if err := mgmtClient.SetSecrets(projectRef, secrets); err != nil {
		return fmt.Errorf("failed to set secrets: %w", err)
	}

	return nil
}

// setSecretsViaCLI sets multiple secrets using the Supabase CLI (fallback).
func (c *Client) setSecretsViaCLI(projectRef string, secrets []Secret) error {
	args := []string{"secrets", "set"}
	for _, s := range secrets {
		args = append(args, s.Name+"="+s.Value)
	}

	if projectRef != "" {
		args = append(args, "--project-ref", projectRef)
	}

	result, err := shell.Run("supabase", args...)
	if err != nil {
		errMsg := result.Stderr
		if errMsg == "" {
			errMsg = err.Error()
		}
		return fmt.Errorf("failed to set secrets: %s", errMsg)
	}

	// Check exit code - shell.Run returns nil error but non-zero exit code on failure
	if result.ExitCode != 0 {
		errMsg := result.Stderr
		if errMsg == "" {
			errMsg = result.Stdout
		}
		return fmt.Errorf("failed to set secrets: %s", errMsg)
	}

	return nil
}

// ListSecrets lists all secrets for a project.
// Uses the Management API for reliable access on both main projects and branches.
func (c *Client) ListSecrets(projectRef string) ([]string, error) {
	// Use Management API - more reliable for branches
	mgmtClient, err := NewManagementClient()
	if err != nil {
		// Fall back to CLI if Management API is not available
		return c.listSecretsViaCLI(projectRef)
	}

	secrets, err := mgmtClient.GetSecrets(projectRef)
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets: %w", err)
	}

	// Extract just the names
	names := make([]string, len(secrets))
	for i, s := range secrets {
		names[i] = s.Name
	}

	return names, nil
}

// listSecretsViaCLI lists secrets using the Supabase CLI (fallback).
func (c *Client) listSecretsViaCLI(projectRef string) ([]string, error) {
	args := []string{"secrets", "list"}
	if projectRef != "" {
		args = append(args, "--project-ref", projectRef)
	}

	result, err := shell.Run("supabase", args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets: %w", err)
	}

	// Check exit code
	if result.ExitCode != 0 {
		errMsg := result.Stderr
		if errMsg == "" {
			errMsg = result.Stdout
		}
		return nil, fmt.Errorf("failed to list secrets: %s", errMsg)
	}

	// Parse the output (typically a table format)
	secrets := []string{}
	for _, line := range strings.Split(result.Stdout, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "NAME") && !strings.HasPrefix(line, "─") {
			// Extract secret name (first column)
			parts := strings.Fields(line)
			if len(parts) > 0 {
				secrets = append(secrets, parts[0])
			}
		}
	}

	return secrets, nil
}

// UnsetSecret removes a secret from a project.
func (c *Client) UnsetSecret(projectRef, name string) error {
	args := []string{"secrets", "unset", name}
	if projectRef != "" {
		args = append(args, "--project-ref", projectRef)
	}

	result, err := shell.Run("supabase", args...)
	if err != nil {
		errMsg := result.Stderr
		if errMsg == "" {
			errMsg = err.Error()
		}
		return fmt.Errorf("failed to unset secret %s: %s", name, errMsg)
	}

	return nil
}

// APNSSecrets holds the required APNs configuration secrets.
type APNSSecrets struct {
	KeyID       string
	TeamID      string
	BundleID    string
	PrivateKey  string
	Environment string // "development" or "production"
}

// APNSLookupInfo describes how Drift searched for an APNs key.
type APNSLookupInfo struct {
	Pattern      string
	SearchDirs   []string
	MatchedFile  string
	UsedEnvKeyID bool
	UsedEnvKey   bool
}

// SetAPNSSecrets sets all APNs-related secrets.
func (c *Client) SetAPNSSecrets(projectRef string, apns APNSSecrets) error {
	secrets := []Secret{
		{Name: "APNS_KEY_ID", Value: apns.KeyID},
		{Name: "APNS_TEAM_ID", Value: apns.TeamID},
		{Name: "APNS_BUNDLE_ID", Value: apns.BundleID},
		{Name: "APNS_PRIVATE_KEY", Value: apns.PrivateKey},
		{Name: "APNS_ENVIRONMENT", Value: apns.Environment},
	}

	return c.SetSecrets(projectRef, secrets)
}

// LoadAPNSSecretsFromConfig loads APNs secrets from config and environment.
func LoadAPNSSecretsFromConfig(teamID, bundleID, keyPattern, environment, projectRoot string) (*APNSSecrets, error) {
	return LoadAPNSSecretsFromConfigWithSecretsDir(teamID, bundleID, keyPattern, environment, projectRoot, "secrets")
}

// LoadAPNSSecretsFromConfigWithSecretsDir loads APNs secrets with a custom secrets directory.
func LoadAPNSSecretsFromConfigWithSecretsDir(teamID, bundleID, keyPattern, environment, projectRoot, secretsDir string) (*APNSSecrets, error) {
	secrets, _, err := LoadAPNSSecretsFromConfigWithSearchPaths(
		teamID,
		bundleID,
		keyPattern,
		environment,
		projectRoot,
		secretsDir,
		nil,
	)
	return secrets, err
}

// LoadAPNSSecretsFromConfigWithSearchPaths loads APNs secrets with custom key search paths.
// If searchPaths is empty, defaults are used: secretsDir, project root, then project parent.
func LoadAPNSSecretsFromConfigWithSearchPaths(teamID, bundleID, keyPattern, environment, projectRoot, secretsDir string, searchPaths []string) (*APNSSecrets, *APNSLookupInfo, error) {
	secrets := &APNSSecrets{
		TeamID:      teamID,
		BundleID:    bundleID,
		Environment: environment,
	}
	lookup := &APNSLookupInfo{
		Pattern: keyPattern,
	}

	// Determine search order.
	paths := searchPaths
	if len(paths) == 0 {
		if secretsDir != "" {
			paths = append(paths, secretsDir)
		}
		paths = append(paths, ".", "..")
	}

	seen := make(map[string]bool)
	searchDirs := make([]string, 0, len(paths))
	for _, raw := range paths {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		abs := raw
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(projectRoot, raw)
		}
		abs = filepath.Clean(abs)
		if seen[abs] {
			continue
		}
		seen[abs] = true
		searchDirs = append(searchDirs, abs)
	}
	lookup.SearchDirs = searchDirs

	var matched string
	for _, dir := range searchDirs {
		keyFiles, err := filepath.Glob(filepath.Join(dir, keyPattern))
		if err != nil {
			return nil, lookup, fmt.Errorf("invalid APNs key pattern %q: %w", keyPattern, err)
		}
		if len(keyFiles) == 0 {
			continue
		}
		sort.Strings(keyFiles)
		matched = keyFiles[0]
		break
	}

	if matched != "" {
		lookup.MatchedFile = matched
		// Extract Key ID from filename (AuthKey_XXXXXXXXXX.p8)
		baseName := filepath.Base(matched)
		if strings.HasPrefix(baseName, "AuthKey_") {
			keyID := strings.TrimSuffix(strings.TrimPrefix(baseName, "AuthKey_"), ".p8")
			secrets.KeyID = keyID
		}

		// Read the private key
		keyData, err := os.ReadFile(matched)
		if err != nil {
			return nil, lookup, fmt.Errorf("failed to read APNs key file %q: %w", matched, err)
		}
		secrets.PrivateKey = string(keyData)
	}

	// Override from environment variables
	if keyID := os.Getenv("APNS_KEY_ID"); keyID != "" {
		lookup.UsedEnvKeyID = true
		secrets.KeyID = keyID
	}
	if teamID := os.Getenv("APNS_TEAM_ID"); teamID != "" {
		secrets.TeamID = teamID
	}
	if bundleID := os.Getenv("APNS_BUNDLE_ID"); bundleID != "" {
		secrets.BundleID = bundleID
	}
	if env := os.Getenv("APNS_ENVIRONMENT"); env != "" {
		secrets.Environment = env
	}
	if pk := os.Getenv("APNS_PRIVATE_KEY"); pk != "" {
		lookup.UsedEnvKey = true
		secrets.PrivateKey = pk
	}

	// Validate
	if secrets.KeyID == "" {
		return nil, lookup, fmt.Errorf("APNS_KEY_ID not found (searched: %s)", strings.Join(searchDirs, ", "))
	}
	if secrets.PrivateKey == "" {
		return nil, lookup, fmt.Errorf("APNs private key not found (searched: %s)", strings.Join(searchDirs, ", "))
	}

	return secrets, lookup, nil
}

// SetDebugSwitch sets the ENABLE_DEBUG_SWITCH secret (only for non-production).
func (c *Client) SetDebugSwitch(projectRef string, enabled bool) error {
	value := "false"
	if enabled {
		value = "true"
	}
	return c.SetSecret(projectRef, "ENABLE_DEBUG_SWITCH", value)
}
