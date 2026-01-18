// Package supabase provides utilities for interacting with Supabase.
package supabase

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	managementAPIBaseURL = "https://api.supabase.com"
)

// ManagementClient provides access to Supabase Management API.
type ManagementClient struct {
	accessToken string
	httpClient  *http.Client
}

// NewManagementClient creates a new Management API client.
// It attempts to get the access token from environment or file.
func NewManagementClient() (*ManagementClient, error) {
	token, err := getAccessToken()
	if err != nil {
		return nil, err
	}

	return &ManagementClient{
		accessToken: token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// getAccessToken retrieves the Supabase access token from various sources.
func getAccessToken() (string, error) {
	// 1. Check environment variable first (for CI/CD)
	if token := os.Getenv("SUPABASE_ACCESS_TOKEN"); token != "" {
		return token, nil
	}

	// 2. Check fallback file location
	homeDir, err := os.UserHomeDir()
	if err == nil {
		tokenFile := filepath.Join(homeDir, ".supabase", "access-token")
		if data, err := os.ReadFile(tokenFile); err == nil {
			token := strings.TrimSpace(string(data))
			if token != "" {
				return token, nil
			}
		}
	}

	return "", fmt.Errorf("could not find Supabase access token\n\n" +
		"Set SUPABASE_ACCESS_TOKEN environment variable or run 'supabase login'")
}

// GetSecrets retrieves all secrets with their values for a project.
func (c *ManagementClient) GetSecrets(projectRef string) ([]Secret, error) {
	url := fmt.Sprintf("%s/v1/projects/%s/secrets", managementAPIBaseURL, projectRef)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch secrets: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var secrets []Secret
	if err := json.Unmarshal(body, &secrets); err != nil {
		return nil, fmt.Errorf("failed to parse secrets: %w", err)
	}

	return secrets, nil
}

// SetSecrets sets multiple secrets on a project (creates or updates).
func (c *ManagementClient) SetSecrets(projectRef string, secrets []Secret) error {
	url := fmt.Sprintf("%s/v1/projects/%s/secrets", managementAPIBaseURL, projectRef)

	jsonData, err := json.Marshal(secrets)
	if err != nil {
		return fmt.Errorf("failed to marshal secrets: %w", err)
	}

	req, err := http.NewRequest("POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to set secrets: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// DeleteSecret deletes a secret from a project.
func (c *ManagementClient) DeleteSecret(projectRef string, secretName string) error {
	url := fmt.Sprintf("%s/v1/projects/%s/secrets", managementAPIBaseURL, projectRef)

	// The delete endpoint uses a body with the names to delete
	jsonData, err := json.Marshal([]string{secretName})
	if err != nil {
		return fmt.Errorf("failed to marshal secret name: %w", err)
	}

	req, err := http.NewRequest("DELETE", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete secret: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// IsAccessTokenAvailable checks if a Supabase access token is available.
func IsAccessTokenAvailable() bool {
	_, err := getAccessToken()
	return err == nil
}
