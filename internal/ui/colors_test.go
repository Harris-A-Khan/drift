package ui

import (
	"testing"
)

func TestColorFunctions(t *testing.T) {
	// Test that color functions don't panic and return non-empty strings
	tests := []struct {
		name    string
		colorFn func(...interface{}) string
		input   string
	}{
		{"Green", Green, "test"},
		{"Yellow", Yellow, "test"},
		{"Red", Red, "test"},
		{"Blue", Blue, "test"},
		{"Cyan", Cyan, "test"},
		{"Magenta", Magenta, "test"},
		{"White", White, "test"},
		{"Bold", Bold, "test"},
		{"Dim", Dim, "test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.colorFn(tt.input)
			// Result should contain the input text
			if result == "" {
				t.Errorf("%s() returned empty string", tt.name)
			}
			// The colored output should contain the original text
			if !containsText(result, tt.input) {
				t.Errorf("%s() result should contain '%s', got '%s'", tt.name, tt.input, result)
			}
		})
	}
}

func TestBadge(t *testing.T) {
	tests := []struct {
		label    string
		color    string
		expected string
	}{
		{"OK", "green", "[OK]"},
		{"WARN", "yellow", "[WARN]"},
		{"ERROR", "red", "[ERROR]"},
		{"INFO", "blue", "[INFO]"},
		{"NOTE", "cyan", "[NOTE]"},
		{"PLAIN", "unknown", "[PLAIN]"},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			result := Badge(tt.label, tt.color)
			// Should contain the label in brackets
			if !containsText(result, tt.label) {
				t.Errorf("Badge(%s, %s) should contain '%s', got '%s'", tt.label, tt.color, tt.label, result)
			}
			if !containsText(result, "[") || !containsText(result, "]") {
				t.Errorf("Badge(%s, %s) should contain brackets, got '%s'", tt.label, tt.color, result)
			}
		})
	}
}

func TestEnvColor(t *testing.T) {
	// Test internal envColor function behavior
	tests := []struct {
		env      string
		expected string // The original text should be present
	}{
		{"production", "production"},
		{"Production", "Production"},
		{"development", "development"},
		{"Development", "Development"},
		{"feature", "feature"},
		{"Feature", "Feature"},
	}

	for _, tt := range tests {
		t.Run(tt.env, func(t *testing.T) {
			result := envColor(tt.env)
			if !containsText(result, tt.expected) {
				t.Errorf("envColor(%s) should contain '%s', got '%s'", tt.env, tt.expected, result)
			}
		})
	}
}

func containsText(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
