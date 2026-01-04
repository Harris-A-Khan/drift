package supabase

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListFunctions(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()
	functionsDir := filepath.Join(tmpDir, "supabase", "functions")

	// Create functions directory
	if err := os.MkdirAll(functionsDir, 0755); err != nil {
		t.Fatalf("failed to create functions dir: %v", err)
	}

	// Test 1: Empty functions directory
	functions, err := ListFunctions(functionsDir)
	if err != nil {
		t.Errorf("ListFunctions failed on empty dir: %v", err)
	}
	if len(functions) != 0 {
		t.Errorf("expected 0 functions, got %d", len(functions))
	}

	// Test 2: Create a valid function
	func1Dir := filepath.Join(functionsDir, "hello-world")
	if err := os.MkdirAll(func1Dir, 0755); err != nil {
		t.Fatalf("failed to create function dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(func1Dir, "index.ts"), []byte("export default {}"), 0644); err != nil {
		t.Fatalf("failed to create index.ts: %v", err)
	}

	functions, err = ListFunctions(functionsDir)
	if err != nil {
		t.Errorf("ListFunctions failed: %v", err)
	}
	if len(functions) != 1 {
		t.Errorf("expected 1 function, got %d", len(functions))
	}
	if len(functions) > 0 && functions[0].Name != "hello-world" {
		t.Errorf("expected function name 'hello-world', got '%s'", functions[0].Name)
	}

	// Test 3: Create a _shared directory (should be skipped)
	sharedDir := filepath.Join(functionsDir, "_shared")
	if err := os.MkdirAll(sharedDir, 0755); err != nil {
		t.Fatalf("failed to create _shared dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sharedDir, "index.ts"), []byte("export default {}"), 0644); err != nil {
		t.Fatalf("failed to create index.ts: %v", err)
	}

	functions, err = ListFunctions(functionsDir)
	if err != nil {
		t.Errorf("ListFunctions failed: %v", err)
	}
	if len(functions) != 1 {
		t.Errorf("expected 1 function (skipping _shared), got %d", len(functions))
	}

	// Test 4: Create another valid function
	func2Dir := filepath.Join(functionsDir, "send-email")
	if err := os.MkdirAll(func2Dir, 0755); err != nil {
		t.Fatalf("failed to create function dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(func2Dir, "index.ts"), []byte("export default {}"), 0644); err != nil {
		t.Fatalf("failed to create index.ts: %v", err)
	}

	functions, err = ListFunctions(functionsDir)
	if err != nil {
		t.Errorf("ListFunctions failed: %v", err)
	}
	if len(functions) != 2 {
		t.Errorf("expected 2 functions, got %d", len(functions))
	}

	// Test 5: Directory without index.ts should be skipped
	func3Dir := filepath.Join(functionsDir, "incomplete")
	if err := os.MkdirAll(func3Dir, 0755); err != nil {
		t.Fatalf("failed to create function dir: %v", err)
	}

	functions, err = ListFunctions(functionsDir)
	if err != nil {
		t.Errorf("ListFunctions failed: %v", err)
	}
	if len(functions) != 2 {
		t.Errorf("expected 2 functions (skipping incomplete), got %d", len(functions))
	}
}

func TestListFunctionsNonExistent(t *testing.T) {
	_, err := ListFunctions("/nonexistent/path")
	if err == nil {
		t.Error("expected error for non-existent path")
	}
}

func TestDetermineEnvironment(t *testing.T) {
	client := NewClient()

	tests := []struct {
		name     string
		branch   Branch
		expected Environment
	}{
		{
			name:     "Production branch",
			branch:   Branch{IsDefault: true, Persistent: false},
			expected: EnvProduction,
		},
		{
			name:     "Development branch",
			branch:   Branch{IsDefault: false, Persistent: true},
			expected: EnvDevelopment,
		},
		{
			name:     "Feature branch",
			branch:   Branch{IsDefault: false, Persistent: false},
			expected: EnvFeature,
		},
		{
			name:     "Production takes precedence",
			branch:   Branch{IsDefault: true, Persistent: true},
			expected: EnvProduction,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.determineEnvironment(&tt.branch)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestGetBranchURL(t *testing.T) {
	client := NewClient()

	tests := []struct {
		projectRef string
		expected   string
	}{
		{"abcdefghij", "https://abcdefghij.supabase.co"},
		{"xyz123", "https://xyz123.supabase.co"},
	}

	for _, tt := range tests {
		result := client.GetBranchURL(tt.projectRef)
		if result != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, result)
		}
	}
}

func TestEnvironmentConstants(t *testing.T) {
	// Test that constants have expected values
	if EnvProduction != "Production" {
		t.Errorf("EnvProduction should be 'Production', got '%s'", EnvProduction)
	}
	if EnvDevelopment != "Development" {
		t.Errorf("EnvDevelopment should be 'Development', got '%s'", EnvDevelopment)
	}
	if EnvFeature != "Feature" {
		t.Errorf("EnvFeature should be 'Feature', got '%s'", EnvFeature)
	}
}

func TestNewClient(t *testing.T) {
	client := NewClient()
	if client == nil {
		t.Error("NewClient returned nil")
	}
	if client.ProjectRef != "" {
		t.Error("NewClient should have empty ProjectRef")
	}
}

func TestNewClientWithRef(t *testing.T) {
	client := NewClientWithRef("test-ref")
	if client == nil {
		t.Error("NewClientWithRef returned nil")
	}
	if client.ProjectRef != "test-ref" {
		t.Errorf("expected ProjectRef 'test-ref', got '%s'", client.ProjectRef)
	}
}
