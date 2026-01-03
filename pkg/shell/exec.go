// Package shell provides utilities for executing shell commands.
package shell

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Result holds the output and exit code of a command execution.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
}

// Runner is an interface for executing shell commands.
// This allows for mocking in tests.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) (*Result, error)
	RunInDir(ctx context.Context, dir, name string, args ...string) (*Result, error)
	RunInteractive(ctx context.Context, name string, args ...string) error
}

// DefaultRunner implements the Runner interface using real shell execution.
type DefaultRunner struct{}

// NewRunner creates a new DefaultRunner.
func NewRunner() Runner {
	return &DefaultRunner{}
}

// Run executes a command with context support.
func (r *DefaultRunner) Run(ctx context.Context, name string, args ...string) (*Result, error) {
	return runCmd(ctx, "", nil, false, name, args...)
}

// RunInDir runs a command in a specific directory with context support.
func (r *DefaultRunner) RunInDir(ctx context.Context, dir, name string, args ...string) (*Result, error) {
	return runCmd(ctx, dir, nil, false, name, args...)
}

// RunInteractive runs a command with stdin/stdout/stderr attached.
func (r *DefaultRunner) RunInteractive(ctx context.Context, name string, args ...string) error {
	_, err := runCmd(ctx, "", nil, true, name, args...)
	return err
}

// runCmd is the internal function that executes commands.
func runCmd(ctx context.Context, dir string, env map[string]string, interactive bool, name string, args ...string) (*Result, error) {
	start := time.Now()
	cmd := exec.CommandContext(ctx, name, args...)

	if dir != "" {
		cmd.Dir = dir
	}

	if env != nil {
		cmd.Env = os.Environ()
		for k, v := range env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	var stdout, stderr bytes.Buffer
	if interactive {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
	}

	err := cmd.Run()

	result := &Result{
		Stdout:   strings.TrimSpace(stdout.String()),
		Stderr:   strings.TrimSpace(stderr.String()),
		ExitCode: 0,
		Duration: time.Since(start),
	}

	if err == nil {
		return result, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}

	result.ExitCode = -1
	return result, fmt.Errorf("failed to execute '%s': %w", name, err)
}

// Convenience functions that use background context.
// These maintain backward compatibility with existing code.

// Run executes a command and returns the result.
func Run(name string, args ...string) (*Result, error) {
	return runCmd(context.Background(), "", nil, false, name, args...)
}

// RunWithEnv executes a command with additional environment variables.
func RunWithEnv(env map[string]string, name string, args ...string) (*Result, error) {
	return runCmd(context.Background(), "", env, false, name, args...)
}

// RunInteractive runs a command with stdin/stdout/stderr attached.
func RunInteractive(name string, args ...string) error {
	_, err := runCmd(context.Background(), "", nil, true, name, args...)
	return err
}

// RunInDir runs a command in a specific directory.
func RunInDir(dir, name string, args ...string) (*Result, error) {
	return runCmd(context.Background(), dir, nil, false, name, args...)
}

// RunSilent executes a command without capturing output (discards stdout/stderr).
func RunSilent(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

// CommandExists checks if a command is available in PATH.
func CommandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// Which returns the full path to a command, or empty string if not found.
func Which(name string) string {
	path, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	return path
}

// RunWithInput runs a command with the provided stdin input.
func RunWithInput(input string, name string, args ...string) (*Result, error) {
	cmd := exec.Command(name, args...)
	cmd.Stdin = strings.NewReader(input)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()

	result := &Result{
		Stdout:   strings.TrimSpace(stdout.String()),
		Stderr:   strings.TrimSpace(stderr.String()),
		ExitCode: 0,
		Duration: time.Since(start),
	}

	if err == nil {
		return result, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}

	return result, fmt.Errorf("failed to execute '%s': %w", name, err)
}

// RunWithTimeout runs a command with a timeout.
func RunWithTimeout(timeout time.Duration, name string, args ...string) (*Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return runCmd(ctx, "", nil, false, name, args...)
}

// RunInDirWithEnv runs a command in a specific directory with environment variables.
func RunInDirWithEnv(dir string, env map[string]string, name string, args ...string) (*Result, error) {
	return runCmd(context.Background(), dir, env, false, name, args...)
}
