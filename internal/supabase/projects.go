package supabase

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/undrift/drift/pkg/shell"
)

// Project represents a Supabase project.
type Project struct {
	ID               string          `json:"id"`
	Name             string          `json:"name"`
	Ref              string          `json:"ref"`
	OrganizationID   string          `json:"organization_id"`
	OrganizationSlug string          `json:"organization_slug"`
	Region           string          `json:"region"`
	Status           string          `json:"status"`
	CreatedAt        string          `json:"created_at"`
	Database         ProjectDatabase `json:"database"`
}

// ProjectDatabase contains database info for a project.
type ProjectDatabase struct {
	Host           string `json:"host"`
	PostgresEngine string `json:"postgres_engine"`
	Version        string `json:"version"`
}

// ListProjects returns all Supabase projects accessible to the user.
func (c *Client) ListProjects() ([]Project, error) {
	result, err := shell.Run("supabase", "projects", "list", "--output", "json")
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}

	var projects []Project
	if err := json.Unmarshal([]byte(result.Stdout), &projects); err != nil {
		return nil, fmt.Errorf("failed to parse projects: %w", err)
	}

	return projects, nil
}

// FindProjectsByName finds projects matching the given name (case-insensitive).
func (c *Client) FindProjectsByName(name string) ([]Project, error) {
	projects, err := c.ListProjects()
	if err != nil {
		return nil, err
	}

	var matches []Project
	nameLower := strings.ToLower(name)

	for _, p := range projects {
		if strings.ToLower(p.Name) == nameLower {
			matches = append(matches, p)
		}
	}

	return matches, nil
}

// FindProjectByRef finds a project by its reference ID.
func (c *Client) FindProjectByRef(ref string) (*Project, error) {
	projects, err := c.ListProjects()
	if err != nil {
		return nil, err
	}

	for _, p := range projects {
		if p.Ref == ref {
			return &p, nil
		}
	}

	return nil, fmt.Errorf("project with ref '%s' not found", ref)
}

// LinkProject links the current directory to a Supabase project.
func (c *Client) LinkProject(projectRef string) error {
	result, err := shell.Run("supabase", "link", "--project-ref", projectRef)
	if err != nil {
		errMsg := ""
		if result != nil && result.Stderr != "" {
			errMsg = result.Stderr
		}
		return fmt.Errorf("failed to link project: %s", errMsg)
	}
	return nil
}

// UnlinkProject unlinks the current directory from a Supabase project.
func (c *Client) UnlinkProject() error {
	_, err := shell.Run("supabase", "unlink")
	return err
}

// GetLinkedProject returns the currently linked project, if any.
func (c *Client) GetLinkedProject() (*Project, error) {
	ref, err := c.GetProjectRef()
	if err != nil {
		return nil, err
	}

	return c.FindProjectByRef(ref)
}

// FormatProjectDisplay returns a formatted string for displaying a project.
func (p *Project) FormatProjectDisplay() string {
	status := ""
	switch p.Status {
	case "ACTIVE_HEALTHY":
		status = "active"
	case "INACTIVE":
		status = "inactive"
	default:
		status = strings.ToLower(p.Status)
	}

	return fmt.Sprintf("%s [%s, %s, %s]", p.Name, p.Ref, p.Region, status)
}
