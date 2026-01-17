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
func PromptMultiSelect(label string, items []string, preselected []string) ([]string, error) {
	// Track selected state
	selected := make(map[string]bool)
	for _, item := range preselected {
		selected[item] = true
	}

	for {
		// Build display items with checkboxes
		displayItems := make([]string, len(items)+1)
		for i, item := range items {
			checkbox := "[ ]"
			if selected[item] {
				checkbox = "[✓]"
			}
			displayItems[i] = fmt.Sprintf("%s %s", checkbox, item)
		}

		// Count selected
		count := 0
		for _, v := range selected {
			if v {
				count++
			}
		}
		displayItems[len(items)] = fmt.Sprintf("── Done (%d selected) ──", count)

		prompt := promptui.Select{
			Label: label,
			Items: displayItems,
			Size:  15,
		}

		idx, _, err := prompt.Run()
		if err != nil {
			return nil, err
		}

		// Check if "Done" was selected
		if idx == len(items) {
			break
		}

		// Toggle the selected item
		item := items[idx]
		selected[item] = !selected[item]
	}

	// Build result
	var result []string
	for _, item := range items {
		if selected[item] {
			result = append(result, item)
		}
	}

	return result, nil
}

