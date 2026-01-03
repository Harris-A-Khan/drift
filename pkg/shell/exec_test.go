package shell

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRun_EchoCommand(t *testing.T) {
	result, err := Run("echo", "hello world")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.Stdout != "hello world" {
		t.Errorf("Run() stdout = %q, want %q", result.Stdout, "hello world")
	}

	if result.ExitCode != 0 {
		t.Errorf("Run() exitCode = %d, want 0", result.ExitCode)
	}

	if result.Duration <= 0 {
		t.Errorf("Run() duration = %v, want > 0", result.Duration)
	}
}

func TestRun_NonExistentCommand(t *testing.T) {
	result, err := Run("this-command-does-not-exist-12345")
	if err == nil {
		t.Error("Run() expected error for non-existent command")
	}

	if result.ExitCode != -1 {
		t.Errorf("Run() exitCode = %d, want -1 for non-existent command", result.ExitCode)
	}
}

func TestRun_CommandWithExitCode(t *testing.T) {
	// 'false' command always exits with code 1
	result, err := Run("false")
	if err != nil {
		t.Fatalf("Run() error = %v, want nil (exit codes are not errors)", err)
	}

	if result.ExitCode != 1 {
		t.Errorf("Run() exitCode = %d, want 1", result.ExitCode)
	}
}

func TestRun_CapturesStderr(t *testing.T) {
	// Use sh -c to write to stderr
	result, err := Run("sh", "-c", "echo error >&2")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.Stderr != "error" {
		t.Errorf("Run() stderr = %q, want %q", result.Stderr, "error")
	}
}

func TestRun_TrimsWhitespace(t *testing.T) {
	result, err := Run("echo", "  spaced  ")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// echo adds its own formatting, but shell.Run trims the result
	if strings.TrimSpace(result.Stdout) != "spaced" {
		t.Errorf("Run() stdout should be trimmed, got %q", result.Stdout)
	}
}

func TestRunWithEnv(t *testing.T) {
	env := map[string]string{
		"TEST_VAR": "test_value",
	}

	result, err := RunWithEnv(env, "sh", "-c", "echo $TEST_VAR")
	if err != nil {
		t.Fatalf("RunWithEnv() error = %v", err)
	}

	if result.Stdout != "test_value" {
		t.Errorf("RunWithEnv() stdout = %q, want %q", result.Stdout, "test_value")
	}
}

func TestRunWithEnv_MultipleVars(t *testing.T) {
	env := map[string]string{
		"VAR1": "value1",
		"VAR2": "value2",
	}

	result, err := RunWithEnv(env, "sh", "-c", "echo $VAR1-$VAR2")
	if err != nil {
		t.Fatalf("RunWithEnv() error = %v", err)
	}

	if result.Stdout != "value1-value2" {
		t.Errorf("RunWithEnv() stdout = %q, want %q", result.Stdout, "value1-value2")
	}
}

func TestRunInDir(t *testing.T) {
	// Create a temp directory
	tmpDir := t.TempDir()

	result, err := RunInDir(tmpDir, "pwd")
	if err != nil {
		t.Fatalf("RunInDir() error = %v", err)
	}

	// Resolve symlinks for comparison (macOS /tmp is symlinked)
	expectedDir, _ := filepath.EvalSymlinks(tmpDir)
	actualDir, _ := filepath.EvalSymlinks(result.Stdout)

	if actualDir != expectedDir {
		t.Errorf("RunInDir() ran in %q, want %q", actualDir, expectedDir)
	}
}

func TestRunInDir_NonExistentDir(t *testing.T) {
	_, err := RunInDir("/this/path/does/not/exist", "pwd")
	if err == nil {
		t.Error("RunInDir() expected error for non-existent directory")
	}
}

func TestRunWithInput(t *testing.T) {
	result, err := RunWithInput("hello from stdin", "cat")
	if err != nil {
		t.Fatalf("RunWithInput() error = %v", err)
	}

	if result.Stdout != "hello from stdin" {
		t.Errorf("RunWithInput() stdout = %q, want %q", result.Stdout, "hello from stdin")
	}
}

func TestRunWithInput_MultipleLines(t *testing.T) {
	input := "line1\nline2\nline3"
	result, err := RunWithInput(input, "cat")
	if err != nil {
		t.Fatalf("RunWithInput() error = %v", err)
	}

	if result.Stdout != input {
		t.Errorf("RunWithInput() stdout = %q, want %q", result.Stdout, input)
	}
}

func TestRunWithTimeout_Succeeds(t *testing.T) {
	result, err := RunWithTimeout(5*time.Second, "echo", "fast")
	if err != nil {
		t.Fatalf("RunWithTimeout() error = %v", err)
	}

	if result.Stdout != "fast" {
		t.Errorf("RunWithTimeout() stdout = %q, want %q", result.Stdout, "fast")
	}
}

func TestRunWithTimeout_TimesOut(t *testing.T) {
	// sleep for 10 seconds but timeout after 100ms
	result, err := RunWithTimeout(100*time.Millisecond, "sleep", "10")

	// When a command times out, it should either return an error or a non-zero exit code
	// The behavior depends on how the context cancellation is handled
	if err == nil && result != nil && result.ExitCode == 0 {
		t.Error("RunWithTimeout() expected non-zero exit or error for timed out command")
	}

	// If there's an error, log it for debugging
	if err != nil {
		t.Logf("RunWithTimeout() error = %v (expected for timeout)", err)
	}

	// If there's a result with non-zero exit, that's also acceptable
	if result != nil && result.ExitCode != 0 {
		t.Logf("RunWithTimeout() exit code = %d (expected for timeout)", result.ExitCode)
	}
}

func TestRunInDirWithEnv(t *testing.T) {
	tmpDir := t.TempDir()
	env := map[string]string{
		"MY_VAR": "my_value",
	}

	result, err := RunInDirWithEnv(tmpDir, env, "sh", "-c", "echo $MY_VAR && pwd")
	if err != nil {
		t.Fatalf("RunInDirWithEnv() error = %v", err)
	}

	if !strings.Contains(result.Stdout, "my_value") {
		t.Errorf("RunInDirWithEnv() stdout should contain env var, got %q", result.Stdout)
	}

	// Resolve symlinks for comparison
	expectedDir, _ := filepath.EvalSymlinks(tmpDir)
	if !strings.Contains(result.Stdout, expectedDir) && !strings.Contains(result.Stdout, tmpDir) {
		t.Errorf("RunInDirWithEnv() should run in %q, stdout = %q", tmpDir, result.Stdout)
	}
}

func TestCommandExists_True(t *testing.T) {
	// 'echo' should exist on all Unix systems
	if !CommandExists("echo") {
		t.Error("CommandExists(echo) = false, want true")
	}
}

func TestCommandExists_False(t *testing.T) {
	if CommandExists("this-command-definitely-does-not-exist-xyz") {
		t.Error("CommandExists(nonexistent) = true, want false")
	}
}

func TestWhich_Found(t *testing.T) {
	path := Which("echo")
	if path == "" {
		t.Error("Which(echo) = empty, want path")
	}

	// Path should be absolute
	if !filepath.IsAbs(path) {
		t.Errorf("Which(echo) = %q, want absolute path", path)
	}
}

func TestWhich_NotFound(t *testing.T) {
	path := Which("this-command-definitely-does-not-exist-xyz")
	if path != "" {
		t.Errorf("Which(nonexistent) = %q, want empty", path)
	}
}

func TestRunSilent_Success(t *testing.T) {
	err := RunSilent("echo", "should not appear")
	if err != nil {
		t.Errorf("RunSilent() error = %v, want nil", err)
	}
}

func TestRunSilent_Failure(t *testing.T) {
	err := RunSilent("false")
	if err == nil {
		t.Error("RunSilent(false) expected error")
	}
}

func TestDefaultRunner_Run(t *testing.T) {
	runner := NewRunner()
	ctx := context.Background()

	result, err := runner.Run(ctx, "echo", "test")
	if err != nil {
		t.Fatalf("DefaultRunner.Run() error = %v", err)
	}

	if result.Stdout != "test" {
		t.Errorf("DefaultRunner.Run() stdout = %q, want %q", result.Stdout, "test")
	}
}

func TestDefaultRunner_RunInDir(t *testing.T) {
	runner := NewRunner()
	ctx := context.Background()
	tmpDir := t.TempDir()

	result, err := runner.RunInDir(ctx, tmpDir, "pwd")
	if err != nil {
		t.Fatalf("DefaultRunner.RunInDir() error = %v", err)
	}

	expectedDir, _ := filepath.EvalSymlinks(tmpDir)
	actualDir, _ := filepath.EvalSymlinks(result.Stdout)

	if actualDir != expectedDir {
		t.Errorf("DefaultRunner.RunInDir() ran in %q, want %q", actualDir, expectedDir)
	}
}

func TestDefaultRunner_WithCancelledContext(t *testing.T) {
	runner := NewRunner()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := runner.Run(ctx, "sleep", "10")
	if err == nil {
		t.Error("DefaultRunner.Run() with cancelled context should return error")
	}
}

func TestResult_Fields(t *testing.T) {
	result, err := Run("sh", "-c", "echo out; echo err >&2; exit 42")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.Stdout != "out" {
		t.Errorf("Result.Stdout = %q, want %q", result.Stdout, "out")
	}

	if result.Stderr != "err" {
		t.Errorf("Result.Stderr = %q, want %q", result.Stderr, "err")
	}

	if result.ExitCode != 42 {
		t.Errorf("Result.ExitCode = %d, want 42", result.ExitCode)
	}

	if result.Duration <= 0 {
		t.Errorf("Result.Duration = %v, want > 0", result.Duration)
	}
}

func TestRun_WithArguments(t *testing.T) {
	// Test multiple arguments
	result, err := Run("echo", "one", "two", "three")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.Stdout != "one two three" {
		t.Errorf("Run() stdout = %q, want %q", result.Stdout, "one two three")
	}
}

func TestRun_SpecialCharacters(t *testing.T) {
	// Test that arguments with special characters are handled properly
	result, err := Run("echo", "hello $world", "'quoted'", "\"double\"")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// echo should output the literal strings
	expected := "hello $world 'quoted' \"double\""
	if result.Stdout != expected {
		t.Errorf("Run() stdout = %q, want %q", result.Stdout, expected)
	}
}

func TestRunWithEnv_InheritsEnvironment(t *testing.T) {
	// Set an env var in the current process
	os.Setenv("DRIFT_TEST_INHERIT", "inherited")
	defer os.Unsetenv("DRIFT_TEST_INHERIT")

	env := map[string]string{
		"NEW_VAR": "new",
	}

	result, err := RunWithEnv(env, "sh", "-c", "echo $DRIFT_TEST_INHERIT-$NEW_VAR")
	if err != nil {
		t.Fatalf("RunWithEnv() error = %v", err)
	}

	if result.Stdout != "inherited-new" {
		t.Errorf("RunWithEnv() should inherit existing env, got %q", result.Stdout)
	}
}
