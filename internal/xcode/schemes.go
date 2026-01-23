package xcode

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/undrift/drift/pkg/shell"
)

// Scheme represents an Xcode scheme.
type Scheme struct {
	Name      string
	Path      string // Path to .xcscheme file
	IsShared  bool   // Whether it's in xcshareddata (shared) or xcuserdata (personal)
	Container string // Project or workspace name
}

// ListSchemes returns all available schemes in the current directory.
// It searches for both .xcodeproj and .xcworkspace files.
func ListSchemes() ([]Scheme, error) {
	var schemes []Scheme

	// Try workspace first
	workspaces, _ := filepath.Glob("*.xcworkspace")
	if len(workspaces) > 0 {
		wsSchemes, err := listSchemesFromWorkspace(workspaces[0])
		if err == nil {
			schemes = append(schemes, wsSchemes...)
		}
	}

	// Also check projects
	projects, _ := filepath.Glob("*.xcodeproj")
	for _, proj := range projects {
		projSchemes, err := listSchemesFromProject(proj)
		if err == nil {
			schemes = append(schemes, projSchemes...)
		}
	}

	// Deduplicate by name (workspace schemes take precedence)
	seen := make(map[string]bool)
	var unique []Scheme
	for _, s := range schemes {
		if !seen[s.Name] {
			seen[s.Name] = true
			unique = append(unique, s)
		}
	}

	return unique, nil
}

// listSchemesFromProject lists schemes from a .xcodeproj directory.
func listSchemesFromProject(projPath string) ([]Scheme, error) {
	var schemes []Scheme

	// Check shared schemes
	sharedPath := filepath.Join(projPath, "xcshareddata", "xcschemes")
	if entries, err := os.ReadDir(sharedPath); err == nil {
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".xcscheme") {
				name := strings.TrimSuffix(entry.Name(), ".xcscheme")
				schemes = append(schemes, Scheme{
					Name:      name,
					Path:      filepath.Join(sharedPath, entry.Name()),
					IsShared:  true,
					Container: filepath.Base(projPath),
				})
			}
		}
	}

	// Check user schemes (less reliable, but include them)
	userPath := filepath.Join(projPath, "xcuserdata")
	if users, err := os.ReadDir(userPath); err == nil {
		for _, user := range users {
			userSchemes := filepath.Join(userPath, user.Name(), "xcschemes")
			if entries, err := os.ReadDir(userSchemes); err == nil {
				for _, entry := range entries {
					if strings.HasSuffix(entry.Name(), ".xcscheme") {
						name := strings.TrimSuffix(entry.Name(), ".xcscheme")
						// Check if we already have this scheme as shared
						found := false
						for _, s := range schemes {
							if s.Name == name {
								found = true
								break
							}
						}
						if !found {
							schemes = append(schemes, Scheme{
								Name:      name,
								Path:      filepath.Join(userSchemes, entry.Name()),
								IsShared:  false,
								Container: filepath.Base(projPath),
							})
						}
					}
				}
			}
		}
	}

	return schemes, nil
}

// listSchemesFromWorkspace lists schemes from a .xcworkspace directory.
func listSchemesFromWorkspace(wsPath string) ([]Scheme, error) {
	var schemes []Scheme

	// Check shared schemes in workspace
	sharedPath := filepath.Join(wsPath, "xcshareddata", "xcschemes")
	if entries, err := os.ReadDir(sharedPath); err == nil {
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".xcscheme") {
				name := strings.TrimSuffix(entry.Name(), ".xcscheme")
				schemes = append(schemes, Scheme{
					Name:      name,
					Path:      filepath.Join(sharedPath, entry.Name()),
					IsShared:  true,
					Container: filepath.Base(wsPath),
				})
			}
		}
	}

	return schemes, nil
}

// ListSchemesViaXcodebuild uses xcodebuild -list to get schemes.
// This is more accurate but slower.
func ListSchemesViaXcodebuild() ([]string, error) {
	// Try workspace first
	workspaces, _ := filepath.Glob("*.xcworkspace")
	if len(workspaces) > 0 {
		return listSchemesViaXcodebuild("-workspace", workspaces[0])
	}

	// Try project
	projects, _ := filepath.Glob("*.xcodeproj")
	if len(projects) > 0 {
		return listSchemesViaXcodebuild("-project", projects[0])
	}

	return nil, fmt.Errorf("no Xcode project or workspace found")
}

// listSchemesViaXcodebuild runs xcodebuild -list and parses the output.
func listSchemesViaXcodebuild(flag, path string) ([]string, error) {
	result, err := shell.Run("xcodebuild", flag, path, "-list", "-json")
	if err != nil {
		return nil, fmt.Errorf("xcodebuild failed: %w", err)
	}

	// Parse JSON output
	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
		// Fall back to text parsing
		return parseSchemesFromText(result.Stdout), nil
	}

	var schemes []string

	// Handle workspace output
	if ws, ok := output["workspace"].(map[string]interface{}); ok {
		if schemeList, ok := ws["schemes"].([]interface{}); ok {
			for _, s := range schemeList {
				if name, ok := s.(string); ok {
					schemes = append(schemes, name)
				}
			}
		}
	}

	// Handle project output
	if proj, ok := output["project"].(map[string]interface{}); ok {
		if schemeList, ok := proj["schemes"].([]interface{}); ok {
			for _, s := range schemeList {
				if name, ok := s.(string); ok {
					schemes = append(schemes, name)
				}
			}
		}
	}

	return schemes, nil
}

// parseSchemesFromText parses xcodebuild -list text output.
func parseSchemesFromText(output string) []string {
	var schemes []string
	inSchemes := false

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "Schemes:" {
			inSchemes = true
			continue
		}
		if inSchemes {
			if line == "" || strings.HasSuffix(line, ":") {
				break
			}
			schemes = append(schemes, line)
		}
	}

	return schemes
}

// SchemeExists checks if a scheme with the given name exists.
func SchemeExists(name string) bool {
	schemes, err := ListSchemes()
	if err != nil {
		return false
	}

	for _, s := range schemes {
		if s.Name == name {
			return true
		}
	}

	return false
}

// ValidateScheme checks if a scheme exists and returns an error if not.
func ValidateScheme(name string) error {
	if !SchemeExists(name) {
		return fmt.Errorf("scheme '%s' not found", name)
	}
	return nil
}

// GetScheme returns a scheme by name.
func GetScheme(name string) (*Scheme, error) {
	schemes, err := ListSchemes()
	if err != nil {
		return nil, err
	}

	for _, s := range schemes {
		if s.Name == name {
			return &s, nil
		}
	}

	return nil, fmt.Errorf("scheme '%s' not found", name)
}

// SuggestSchemes returns scheme suggestions based on environment.
// It looks for common naming patterns like "AppName (Production)", "AppName-Dev", etc.
func SuggestSchemes(projectName string) map[string]string {
	schemes, err := ListSchemes()
	if err != nil {
		return nil
	}

	suggestions := make(map[string]string)

	// Common patterns to look for
	prodPatterns := []string{
		projectName + " (Production)",
		projectName + "-Prod",
		projectName + "-Production",
		projectName + " Production",
		projectName + "Prod",
	}

	devPatterns := []string{
		projectName + " (Development)",
		projectName + "-Dev",
		projectName + "-Development",
		projectName + " Development",
		projectName + "Dev",
	}

	featurePatterns := []string{
		projectName + " (Feature)",
		projectName + "-Feature",
		projectName + " Feature",
		projectName + " (Debug)",
		projectName + "-Debug",
	}

	for _, s := range schemes {
		nameLower := strings.ToLower(s.Name)

		// Check production patterns
		for _, pattern := range prodPatterns {
			if strings.EqualFold(s.Name, pattern) {
				suggestions["production"] = s.Name
				break
			}
		}

		// Check development patterns
		for _, pattern := range devPatterns {
			if strings.EqualFold(s.Name, pattern) {
				suggestions["development"] = s.Name
				break
			}
		}

		// Check feature patterns
		for _, pattern := range featurePatterns {
			if strings.EqualFold(s.Name, pattern) {
				suggestions["feature"] = s.Name
				break
			}
		}

		// Generic checks if no specific match
		if _, ok := suggestions["production"]; !ok {
			if strings.Contains(nameLower, "prod") || strings.Contains(nameLower, "release") {
				suggestions["production"] = s.Name
			}
		}

		if _, ok := suggestions["development"]; !ok {
			if strings.Contains(nameLower, "dev") && !strings.Contains(nameLower, "feature") {
				suggestions["development"] = s.Name
			}
		}

		if _, ok := suggestions["feature"]; !ok {
			if strings.Contains(nameLower, "feature") || strings.Contains(nameLower, "debug") {
				suggestions["feature"] = s.Name
			}
		}
	}

	// If we have exactly one scheme, use it for all environments
	if len(schemes) == 1 {
		suggestions["production"] = schemes[0].Name
		suggestions["development"] = schemes[0].Name
		suggestions["feature"] = schemes[0].Name
	}

	return suggestions
}
