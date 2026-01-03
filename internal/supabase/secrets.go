package supabase

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/undrift/drift/pkg/shell"
)

// Secret represents a Supabase secret.
type Secret struct {
	Name  string
	Value string
}

// SetSecret sets a secret on the specified project/branch.
func (c *Client) SetSecret(projectRef, name, value string) error {
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

	return nil
}

// SetSecrets sets multiple secrets at once.
func (c *Client) SetSecrets(projectRef string, secrets []Secret) error {
	if len(secrets) == 0 {
		return nil
	}

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

	return nil
}

// ListSecrets lists all secrets for a project.
func (c *Client) ListSecrets(projectRef string) ([]string, error) {
	args := []string{"secrets", "list"}
	if projectRef != "" {
		args = append(args, "--project-ref", projectRef)
	}

	result, err := shell.Run("supabase", args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets: %w", err)
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
	secrets := &APNSSecrets{
		TeamID:      teamID,
		BundleID:    bundleID,
		Environment: environment,
	}

	// Try to find the .p8 key file
	keyFiles, err := filepath.Glob(filepath.Join(projectRoot, keyPattern))
	if err != nil || len(keyFiles) == 0 {
		// Try in parent directory
		keyFiles, err = filepath.Glob(filepath.Join(filepath.Dir(projectRoot), keyPattern))
	}

	if err == nil && len(keyFiles) > 0 {
		keyFile := keyFiles[0]
		
		// Extract Key ID from filename (AuthKey_XXXXXXXXXX.p8)
		baseName := filepath.Base(keyFile)
		if strings.HasPrefix(baseName, "AuthKey_") {
			keyID := strings.TrimSuffix(strings.TrimPrefix(baseName, "AuthKey_"), ".p8")
			secrets.KeyID = keyID
		}

		// Read the private key
		keyData, err := os.ReadFile(keyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read APNs key file: %w", err)
		}
		secrets.PrivateKey = string(keyData)
	}

	// Override from environment variables
	if keyID := os.Getenv("APNS_KEY_ID"); keyID != "" {
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
		secrets.PrivateKey = pk
	}

	// Validate
	if secrets.KeyID == "" {
		return nil, fmt.Errorf("APNS_KEY_ID not found")
	}
	if secrets.PrivateKey == "" {
		return nil, fmt.Errorf("APNs private key not found")
	}

	return secrets, nil
}

// SetDebugSwitch sets the ENABLE_DEBUG_SWITCH secret (only for non-production).
func (c *Client) SetDebugSwitch(projectRef string, enabled bool) error {
	value := "false"
	if enabled {
		value = "true"
	}
	return c.SetSecret(projectRef, "ENABLE_DEBUG_SWITCH", value)
}

