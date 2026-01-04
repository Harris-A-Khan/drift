// Package supabase provides utilities for interacting with the Supabase CLI.
package supabase

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/undrift/drift/pkg/shell"
)

// Client wraps Supabase CLI operations.
type Client struct {
	// ProjectRef is the Supabase project reference (optional, can be auto-detected)
	ProjectRef string
}

// NewClient creates a new Supabase client.
func NewClient() *Client {
	return &Client{}
}

// NewClientWithRef creates a new Supabase client with a specific project reference.
func NewClientWithRef(projectRef string) *Client {
	return &Client{ProjectRef: projectRef}
}

// IsLinked checks if the current directory is linked to a Supabase project.
func (c *Client) IsLinked() bool {
	result, _ := shell.Run("supabase", "status")
	return result != nil && result.ExitCode == 0
}

// GetProjectRef returns the linked project reference.
func (c *Client) GetProjectRef() (string, error) {
	if c.ProjectRef != "" {
		return c.ProjectRef, nil
	}

	// Try to get from .supabase directory
	gitRoot, err := shell.Run("git", "rev-parse", "--show-toplevel")
	if err == nil {
		refFile := filepath.Join(gitRoot.Stdout, ".supabase", "project-ref")
		if data, err := os.ReadFile(refFile); err == nil {
			return strings.TrimSpace(string(data)), nil
		}
	}

	// Try to get from environment
	if ref := os.Getenv("SUPABASE_PROJECT_REF"); ref != "" {
		return ref, nil
	}

	// Try to parse from supabase status
	result, err := shell.Run("supabase", "status")
	if err != nil {
		return "", fmt.Errorf("could not determine project ref: %w", err)
	}

	// Parse output for project ID
	for _, line := range strings.Split(result.Stdout, "\n") {
		if strings.Contains(line, "Project ID") || strings.Contains(line, "project_id") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				return strings.TrimSpace(parts[1]), nil
			}
		}
	}

	return "", fmt.Errorf("could not determine project ref from supabase status")
}

// GetAPIURL returns the API URL for the project.
func (c *Client) GetAPIURL() (string, error) {
	result, err := shell.Run("supabase", "status")
	if err != nil {
		return "", fmt.Errorf("failed to get supabase status: %w", err)
	}

	for _, line := range strings.Split(result.Stdout, "\n") {
		if strings.Contains(line, "API URL") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				// Rejoin in case there are colons in the URL
				url := strings.TrimSpace(strings.Join(parts[1:], ":"))
				return url, nil
			}
		}
	}

	return "", fmt.Errorf("could not find API URL in supabase status")
}

// GetAnonKey returns the anon key for a project reference.
func (c *Client) GetAnonKey(projectRef string) (string, error) {
	if projectRef == "" {
		var err error
		projectRef, err = c.GetProjectRef()
		if err != nil {
			return "", err
		}
	}

	result, err := shell.Run("supabase", "projects", "api-keys", "--project-ref", projectRef, "--output", "json")
	if err != nil {
		return "", fmt.Errorf("failed to get API keys: %w", err)
	}

	var keys []struct {
		Name   string `json:"name"`
		APIKey string `json:"api_key"`
	}

	if err := json.Unmarshal([]byte(result.Stdout), &keys); err != nil {
		return "", fmt.Errorf("failed to parse API keys: %w", err)
	}

	for _, key := range keys {
		if key.Name == "anon" || key.Name == "anon key" {
			return key.APIKey, nil
		}
	}

	return "", fmt.Errorf("anon key not found in project %s", projectRef)
}

// GetServiceKey returns the service key for a project reference.
func (c *Client) GetServiceKey(projectRef string) (string, error) {
	if projectRef == "" {
		var err error
		projectRef, err = c.GetProjectRef()
		if err != nil {
			return "", err
		}
	}

	result, err := shell.Run("supabase", "projects", "api-keys", "--project-ref", projectRef, "--output", "json")
	if err != nil {
		return "", fmt.Errorf("failed to get API keys: %w", err)
	}

	var keys []struct {
		Name   string `json:"name"`
		APIKey string `json:"api_key"`
	}

	if err := json.Unmarshal([]byte(result.Stdout), &keys); err != nil {
		return "", fmt.Errorf("failed to parse API keys: %w", err)
	}

	for _, key := range keys {
		if key.Name == "service_role" || key.Name == "service role" {
			return key.APIKey, nil
		}
	}

	return "", fmt.Errorf("service role key not found in project %s", projectRef)
}

// Status returns the current Supabase project status.
func (c *Client) Status() (string, error) {
	result, err := shell.Run("supabase", "status")
	if err != nil {
		return "", fmt.Errorf("failed to get supabase status: %w", err)
	}
	return result.Stdout, nil
}

// Link links the current directory to a Supabase project.
func (c *Client) Link(projectRef string) error {
	args := []string{"link", "--project-ref", projectRef}

	result, err := shell.Run("supabase", args...)
	if err != nil {
		errMsg := result.Stderr
		if errMsg == "" {
			errMsg = err.Error()
		}
		return fmt.Errorf("failed to link project: %s", errMsg)
	}

	return nil
}

// Unlink unlinks the current directory from a Supabase project.
func (c *Client) Unlink() error {
	_, err := shell.Run("supabase", "unlink")
	if err != nil {
		return fmt.Errorf("failed to unlink project: %w", err)
	}
	return nil
}

// IsCLIInstalled checks if the Supabase CLI is installed.
func IsCLIInstalled() bool {
	return shell.CommandExists("supabase")
}

// RunCommand runs an arbitrary command and returns combined output.
func RunCommand(name string, args ...string) (string, error) {
	result, err := shell.Run(name, args...)
	if err != nil {
		if result != nil {
			return result.Stderr, err
		}
		return "", err
	}
	return result.Stdout, nil
}

