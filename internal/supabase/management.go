// Package supabase provides utilities for interacting with Supabase.
package supabase

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

	// 2. Check macOS Keychain (where supabase CLI stores credentials after login)
	if runtime.GOOS == "darwin" {
		if token := getTokenFromKeychain(); token != "" {
			return token, nil
		}
	}

	// 3. Check Linux secret service / keyring
	if runtime.GOOS == "linux" {
		if token := getTokenFromLinuxKeyring(); token != "" {
			return token, nil
		}
	}

	// 4. Check fallback file location
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

// getTokenFromKeychain retrieves the Supabase access token from macOS Keychain.
func getTokenFromKeychain() string {
	// The Supabase CLI stores tokens with service name "Supabase CLI"
	// The account name is the profile name - "supabase" is the default profile
	// Try the default profile first, then fall back to "access-token"
	accountNames := []string{"supabase", "access-token"}

	for _, account := range accountNames {
		cmd := exec.Command("security", "find-generic-password", "-s", "Supabase CLI", "-a", account, "-w")
		output, err := cmd.Output()
		if err != nil {
			continue
		}

		tokenData := strings.TrimSpace(string(output))
		if tokenData == "" {
			continue
		}

		// The token is stored with "go-keyring-base64:" prefix and base64 encoded
		if strings.HasPrefix(tokenData, "go-keyring-base64:") {
			encoded := strings.TrimPrefix(tokenData, "go-keyring-base64:")
			decoded, err := base64.StdEncoding.DecodeString(encoded)
			if err != nil {
				continue
			}
			return string(decoded)
		}

		// If no prefix, return as-is
		return tokenData
	}

	return ""
}

// getTokenFromLinuxKeyring retrieves the Supabase access token from Linux secret service.
func getTokenFromLinuxKeyring() string {
	// Try using secret-tool (common on Linux with GNOME/KDE)
	cmd := exec.Command("secret-tool", "lookup", "service", "Supabase CLI")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	tokenData := strings.TrimSpace(string(output))
	if tokenData == "" {
		return ""
	}

	// Handle base64 encoding if present
	if strings.HasPrefix(tokenData, "go-keyring-base64:") {
		encoded := strings.TrimPrefix(tokenData, "go-keyring-base64:")
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return ""
		}
		return string(decoded)
	}

	return tokenData
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

// ProjectStatusResponse represents the project status from the API.
type ProjectStatusResponse struct {
	Status string `json:"status"`
}

// GetProjectStatus returns the health status of a Supabase project.
// Returns values like "ACTIVE_HEALTHY", "ACTIVE_UNHEALTHY", "INACTIVE", "COMING_UP", etc.
func (c *ManagementClient) GetProjectStatus(projectRef string) (string, error) {
	url := fmt.Sprintf("%s/v1/projects/%s/health", managementAPIBaseURL, projectRef)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch project status: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Fall back to getting project info if health endpoint doesn't exist
		return c.getProjectStatusFromInfo(projectRef)
	}

	// Try to parse health response
	var healthResp []struct {
		Name    string `json:"name"`
		Healthy bool   `json:"healthy"`
		Status  string `json:"status"`
	}
	if err := json.Unmarshal(body, &healthResp); err != nil {
		// Try alternate format
		return c.getProjectStatusFromInfo(projectRef)
	}

	// Check database health specifically
	for _, h := range healthResp {
		if h.Name == "database" || h.Name == "db" {
			if h.Healthy {
				return "ACTIVE_HEALTHY", nil
			}
			return "ACTIVE_UNHEALTHY", nil
		}
	}

	// If all healthy, return healthy
	allHealthy := true
	for _, h := range healthResp {
		if !h.Healthy {
			allHealthy = false
			break
		}
	}
	if allHealthy {
		return "ACTIVE_HEALTHY", nil
	}
	return "ACTIVE_UNHEALTHY", nil
}

// getProjectStatusFromInfo gets status from the project info endpoint.
func (c *ManagementClient) getProjectStatusFromInfo(projectRef string) (string, error) {
	url := fmt.Sprintf("%s/v1/projects/%s", managementAPIBaseURL, projectRef)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch project info: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var projectInfo struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &projectInfo); err != nil {
		return "", fmt.Errorf("failed to parse project info: %w", err)
	}

	if projectInfo.Status == "" {
		return "UNKNOWN", nil
	}
	return projectInfo.Status, nil
}
