package supabase

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/undrift/drift/pkg/shell"
)

// Function represents an Edge Function.
type Function struct {
	Name string
	Path string
}

// ListFunctions returns all Edge Functions in the functions directory.
func ListFunctions(functionsDir string) ([]Function, error) {
	entries, err := os.ReadDir(functionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("functions directory not found: %s", functionsDir)
		}
		return nil, err
	}

	functions := []Function{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Skip directories starting with _ (like _shared)
		if entry.Name()[0] == '_' {
			continue
		}

		funcPath := filepath.Join(functionsDir, entry.Name())
		
		// Check if index.ts exists
		indexPath := filepath.Join(funcPath, "index.ts")
		if _, err := os.Stat(indexPath); os.IsNotExist(err) {
			continue
		}

		functions = append(functions, Function{
			Name: entry.Name(),
			Path: funcPath,
		})
	}

	return functions, nil
}

// DeployOptions holds options for function deployment.
type DeployOptions struct {
	NoVerifyJWT bool
}

// DeployFunction deploys a single Edge Function.
func (c *Client) DeployFunction(name, projectRef string) error {
	return c.DeployFunctionWithOptions(name, projectRef, DeployOptions{})
}

// DeployFunctionWithOptions deploys a single Edge Function with options.
func (c *Client) DeployFunctionWithOptions(name, projectRef string, opts DeployOptions) error {
	args := []string{"functions", "deploy", name}
	if projectRef != "" {
		args = append(args, "--project-ref", projectRef)
	}
	if opts.NoVerifyJWT {
		args = append(args, "--no-verify-jwt")
	}

	result, err := shell.Run("supabase", args...)
	if err != nil {
		errMsg := result.Stderr
		if errMsg == "" {
			errMsg = err.Error()
		}
		return fmt.Errorf("failed to deploy function '%s': %s", name, errMsg)
	}

	return nil
}

// DeployAllFunctions deploys all Edge Functions in the directory.
func (c *Client) DeployAllFunctions(functionsDir, projectRef string) error {
	functions, err := ListFunctions(functionsDir)
	if err != nil {
		return err
	}

	if len(functions) == 0 {
		return fmt.Errorf("no functions found in %s", functionsDir)
	}

	for _, fn := range functions {
		if err := c.DeployFunction(fn.Name, projectRef); err != nil {
			return err
		}
	}

	return nil
}

// ServeFunction starts a local function server.
func (c *Client) ServeFunction(name string, envFile string) error {
	args := []string{"functions", "serve"}
	if name != "" {
		args = append(args, name)
	}
	if envFile != "" {
		args = append(args, "--env-file", envFile)
	}

	return shell.RunInteractive("supabase", args...)
}

// InvokeFunction invokes a function locally.
func (c *Client) InvokeFunction(name string, data string) (*shell.Result, error) {
	args := []string{"functions", "invoke", name}
	if data != "" {
		args = append(args, "--body", data)
	}

	return shell.Run("supabase", args...)
}

// GetFunctionLogs retrieves logs for a function via Management API.
func (c *Client) GetFunctionLogs(name, projectRef string) ([]FunctionLogEntry, error) {
	mgmtClient, err := NewManagementClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create management client: %w", err)
	}

	return mgmtClient.GetFunctionLogs(projectRef, name)
}

// DeployedFunction represents a function deployed on Supabase.
type DeployedFunction struct {
	Name      string
	Status    string
	Version   string
	CreatedAt string
	UpdatedAt string
}

// ListDeployedFunctions returns all Edge Functions deployed on a project.
func (c *Client) ListDeployedFunctions(projectRef string) ([]DeployedFunction, error) {
	args := []string{"functions", "list", "--project-ref", projectRef}

	result, err := shell.Run("supabase", args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list deployed functions: %w", err)
	}

	var functions []DeployedFunction
	lines := strings.Split(result.Stdout, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip header and separator lines
		// Format: ID | NAME | SLUG | STATUS | VERSION | UPDATED_AT
		if line == "" || strings.HasPrefix(line, "ID") || strings.HasPrefix(line, "-") || strings.HasPrefix(line, "│") || strings.HasPrefix(line, "┌") || strings.HasPrefix(line, "└") || strings.HasPrefix(line, "├") {
			continue
		}

		// Parse pipe-separated format: ID | NAME | SLUG | STATUS | VERSION | UPDATED_AT
		// Split by | and trim each part
		parts := strings.Split(line, "|")
		if len(parts) >= 2 {
			// NAME is the second column (index 1)
			name := strings.TrimSpace(parts[1])
			if name == "" || name == "NAME" {
				continue
			}

			fn := DeployedFunction{
				Name:   name,
				Status: "active",
			}
			if len(parts) > 3 {
				fn.Status = strings.TrimSpace(parts[3])
			}
			if len(parts) > 4 {
				fn.Version = strings.TrimSpace(parts[4])
			}
			if len(parts) > 5 {
				fn.UpdatedAt = strings.TrimSpace(parts[5])
			}
			functions = append(functions, fn)
		}
	}

	return functions, nil
}

// DeleteFunction deletes a deployed Edge Function.
func (c *Client) DeleteFunction(name, projectRef string) error {
	args := []string{"functions", "delete", name, "--project-ref", projectRef}

	result, err := shell.Run("supabase", args...)
	if err != nil {
		errMsg := result.Stderr
		if errMsg == "" {
			errMsg = err.Error()
		}
		return fmt.Errorf("failed to delete function '%s': %s", name, errMsg)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("failed to delete function '%s': %s", name, result.Stderr)
	}

	return nil
}

// DownloadFunction downloads a deployed Edge Function's source code.
func (c *Client) DownloadFunction(name, projectRef, outputDir string) error {
	args := []string{"functions", "download", name, "--project-ref", projectRef}

	// Create temp dir if output dir not specified
	if outputDir == "" {
		var err error
		outputDir, err = os.MkdirTemp("", "drift-func-*")
		if err != nil {
			return fmt.Errorf("failed to create temp directory: %w", err)
		}
	}

	result, err := shell.RunInDir(outputDir, "supabase", args...)
	if err != nil {
		errMsg := result.Stderr
		if errMsg == "" {
			errMsg = err.Error()
		}
		return fmt.Errorf("failed to download function '%s': %s", name, errMsg)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("failed to download function '%s': %s", name, result.Stderr)
	}

	return nil
}

// DownloadFunctionToTemp downloads a function to a temporary directory and returns the path.
func (c *Client) DownloadFunctionToTemp(name, projectRef string) (string, error) {
	tempDir, err := os.MkdirTemp("", "drift-func-download-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	args := []string{"functions", "download", name, "--project-ref", projectRef}

	result, err := shell.RunInDir(tempDir, "supabase", args...)
	if err != nil {
		os.RemoveAll(tempDir)
		errMsg := result.Stderr
		if errMsg == "" {
			errMsg = err.Error()
		}
		return "", fmt.Errorf("failed to download function '%s': %s", name, errMsg)
	}

	if result.ExitCode != 0 {
		os.RemoveAll(tempDir)
		return "", fmt.Errorf("failed to download function '%s': %s", name, result.Stderr)
	}

	return tempDir, nil
}

// NewFunction creates a new Edge Function locally.
func (c *Client) NewFunction(name string) error {
	args := []string{"functions", "new", name}

	result, err := shell.Run("supabase", args...)
	if err != nil {
		errMsg := result.Stderr
		if errMsg == "" {
			errMsg = err.Error()
		}
		return fmt.Errorf("failed to create function '%s': %s", name, errMsg)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("failed to create function '%s': %s", name, result.Stderr)
	}

	return nil
}

