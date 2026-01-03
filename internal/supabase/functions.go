package supabase

import (
	"fmt"
	"os"
	"path/filepath"

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

// DeployFunction deploys a single Edge Function.
func (c *Client) DeployFunction(name, projectRef string) error {
	args := []string{"functions", "deploy", name}
	if projectRef != "" {
		args = append(args, "--project-ref", projectRef)
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

// GetFunctionLogs retrieves logs for a function.
func (c *Client) GetFunctionLogs(name, projectRef string) (*shell.Result, error) {
	args := []string{"functions", "logs", name}
	if projectRef != "" {
		args = append(args, "--project-ref", projectRef)
	}

	return shell.Run("supabase", args...)
}

