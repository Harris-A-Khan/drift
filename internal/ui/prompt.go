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

