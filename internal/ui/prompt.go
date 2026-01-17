package ui

import (
	"fmt"
	"strings"

	"github.com/manifoldco/promptui"
)

// PromptYesNo asks a yes/no question and returns true for yes.
func PromptYesNo(question string, defaultYes bool) (bool, error) {
	defaultLabel := "y/N"
	if defaultYes {
		defaultLabel = "Y/n"
	}

	prompt := promptui.Prompt{
		Label:     fmt.Sprintf("%s [%s]", question, defaultLabel),
		IsConfirm: true,
		Default:   "",
	}

	result, err := prompt.Run()
	if err != nil {
		if err == promptui.ErrAbort {
			return false, nil
		}
		// If user just pressed enter, use default
		if result == "" {
			return defaultYes, nil
		}
		return false, err
	}

	result = strings.ToLower(strings.TrimSpace(result))
	return result == "y" || result == "yes", nil
}

// PromptString asks for a string input.
func PromptString(label string, defaultValue string) (string, error) {
	prompt := promptui.Prompt{
		Label:   label,
		Default: defaultValue,
	}

	return prompt.Run()
}

// PromptPassword asks for a password input (hidden).
func PromptPassword(label string) (string, error) {
	prompt := promptui.Prompt{
		Label: label,
		Mask:  '*',
	}

	return prompt.Run()
}

// PromptSelect shows a selection list and returns the selected item.
func PromptSelect(label string, items []string) (string, error) {
	prompt := promptui.Select{
		Label: label,
		Items: items,
		Size:  10,
	}

	_, result, err := prompt.Run()
	return result, err
}

// PromptSelectWithIndex shows a selection list and returns the index and item.
func PromptSelectWithIndex(label string, items []string) (int, string, error) {
	prompt := promptui.Select{
		Label: label,
		Items: items,
		Size:  10,
	}

	return prompt.Run()
}

// SelectItem represents an item in a detailed select list.
type SelectItem struct {
	Name        string
	Description string
}

// PromptSelectDetailed shows a selection list with descriptions.
func PromptSelectDetailed(label string, items []SelectItem) (int, error) {
	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   "▸ {{ .Name | cyan }} {{ .Description | faint }}",
		Inactive: "  {{ .Name }} {{ .Description | faint }}",
		Selected: "{{ .Name | green }}",
	}

	prompt := promptui.Select{
		Label:     label,
		Items:     items,
		Templates: templates,
		Size:      10,
	}

	idx, _, err := prompt.Run()
	return idx, err
}

// PromptMultiSelect allows selecting multiple items from a list.
// Returns the selected items. Uses a toggle-based approach where
// users can select/deselect items and press "Done" when finished.
// Features: search filtering, select all, deselect all, cursor position preserved.
func PromptMultiSelect(label string, items []string, preselected []string) ([]string, error) {
	// Track selected state
	selected := make(map[string]bool)
	for _, item := range preselected {
		selected[item] = true
	}

	cursorPos := 3 // Start at first item (after actions)

	for {
		// Count selected
		count := 0
		for _, v := range selected {
			if v {
				count++
			}
		}

		// Build display items: actions first, then items with checkboxes
		displayItems := make([]string, len(items)+3)
		displayItems[0] = fmt.Sprintf("   ✓ Select All (%d tables)", len(items))
		displayItems[1] = "   ✗ Deselect All"
		displayItems[2] = fmt.Sprintf("   ✔ Done (%d selected)", count)

		for i, item := range items {
			checkbox := "[ ]"
			if selected[item] {
				checkbox = "[✓]"
			}
			displayItems[i+3] = fmt.Sprintf("%s %s", checkbox, item)
		}

		// Search function - searches the item name (strips checkbox)
		searcher := func(input string, index int) bool {
			// Always show action items when not searching
			if index < 3 {
				return strings.Contains(strings.ToLower(displayItems[index]), strings.ToLower(input))
			}
			item := items[index-3]
			return strings.Contains(strings.ToLower(item), strings.ToLower(input))
		}

		prompt := promptui.Select{
			Label:             label + " (↑↓ navigate, type to search)",
			Items:             displayItems,
			Size:              18,
			CursorPos:         cursorPos,
			Searcher:          searcher,
			StartInSearchMode: false,
		}

		idx, _, err := prompt.Run()
		if err != nil {
			return nil, err
		}

		// Handle actions
		switch idx {
		case 0: // Select All
			for _, item := range items {
				selected[item] = true
			}
			cursorPos = 2 // Move to Done
		case 1: // Deselect All
			for _, item := range items {
				selected[item] = false
			}
			cursorPos = 0 // Move to Select All
		case 2: // Done
			// Build result preserving original order
			var result []string
			for _, item := range items {
				if selected[item] {
					result = append(result, item)
				}
			}
			return result, nil
		default:
			// Toggle the selected item
			itemIdx := idx - 3
			if itemIdx >= 0 && itemIdx < len(items) {
				item := items[itemIdx]
				selected[item] = !selected[item]
			}
			cursorPos = idx // Stay at same position
		}
	}
}

