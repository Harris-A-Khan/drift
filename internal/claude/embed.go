package claude

import (
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

//go:embed commands/*.md
var CommandFiles embed.FS

// CommandInfo holds metadata about an embedded skill command.
type CommandInfo struct {
	Name     string // e.g. "db-schema-change"
	Filename string // e.g. "db-schema-change.md"
}

// ListCommands returns all embedded skill commands.
func ListCommands() ([]CommandInfo, error) {
	entries, err := fs.ReadDir(CommandFiles, "commands")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded commands: %w", err)
	}

	var commands []CommandInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		commands = append(commands, CommandInfo{
			Name:     name,
			Filename: entry.Name(),
		})
	}
	return commands, nil
}

// ReadCommand returns the content of an embedded skill command by name.
func ReadCommand(name string) ([]byte, error) {
	filename := name + ".md"
	data, err := CommandFiles.ReadFile("commands/" + filename)
	if err != nil {
		return nil, fmt.Errorf("command %q not found: %w", name, err)
	}
	return data, nil
}

// IsDriftSkill checks if file content starts with the drift skill version marker.
func IsDriftSkill(content []byte) bool {
	return strings.HasPrefix(strings.TrimSpace(string(content)), "<!-- drift-skill v1 -->")
}
