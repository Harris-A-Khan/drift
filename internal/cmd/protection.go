package cmd

import (
	"fmt"
	"strings"

	"github.com/undrift/drift/internal/supabase"
	"github.com/undrift/drift/internal/ui"
)

// ProtectionLevel defines how strict the production protection should be.
type ProtectionLevel int

const (
	// ProtectionStandard requires a simple y/n confirmation.
	ProtectionStandard ProtectionLevel = iota
	// ProtectionStrict requires typing "yes" to confirm.
	ProtectionStrict
)

// ConfirmProductionOperation prompts for confirmation when performing operations
// on production environments. Returns true if the operation should proceed.
// If --yes flag is set, returns true without prompting (for automation).
func ConfirmProductionOperation(env supabase.Environment, operation string) (bool, error) {
	return confirmOperation(env, operation, ProtectionStandard)
}

// RequireProductionConfirmation is a stricter version that requires typing "yes"
// for particularly dangerous operations. Returns true if operation should proceed.
func RequireProductionConfirmation(env supabase.Environment, operation string) (bool, error) {
	return confirmOperation(env, operation, ProtectionStrict)
}

// confirmOperation handles the actual confirmation logic.
func confirmOperation(env supabase.Environment, operation string, level ProtectionLevel) (bool, error) {
	// Skip confirmation if --yes flag is set
	if IsYes() {
		return true, nil
	}

	// Only prompt for production environments
	if env != supabase.EnvProduction {
		return true, nil
	}

	ui.NewLine()
	ui.Warning(fmt.Sprintf("You are about to %s on PRODUCTION!", operation))

	if level == ProtectionStrict {
		// Require typing "yes"
		input, err := ui.PromptString("Type 'yes' to confirm", "")
		if err != nil {
			return false, err
		}

		if strings.ToLower(strings.TrimSpace(input)) != "yes" {
			ui.Info("Cancelled")
			return false, nil
		}
		return true, nil
	}

	// Standard y/n confirmation
	confirmed, err := ui.PromptYesNo("Continue?", false)
	if err != nil {
		return false, err
	}

	if !confirmed {
		ui.Info("Cancelled")
		return false, nil
	}

	return true, nil
}

// IsProductionEnvironment checks if the given environment is production.
func IsProductionEnvironment(env supabase.Environment) bool {
	return env == supabase.EnvProduction
}

// WarnIfProduction shows a warning banner if operating on production.
func WarnIfProduction(env supabase.Environment) {
	if env == supabase.EnvProduction {
		ui.Warning("Operating on PRODUCTION environment")
	}
}

// ConfirmDestructiveOperation prompts for confirmation on destructive operations.
// This is used for operations that could cause data loss, regardless of environment.
func ConfirmDestructiveOperation(description string) (bool, error) {
	if IsYes() {
		return true, nil
	}

	ui.NewLine()
	ui.Warning(fmt.Sprintf("This will %s", description))
	return ui.PromptYesNo("Continue?", false)
}

// RequireDestructiveConfirmation requires typing "yes" for destructive operations.
func RequireDestructiveConfirmation(description string) (bool, error) {
	if IsYes() {
		return true, nil
	}

	ui.NewLine()
	ui.Warning(fmt.Sprintf("This will %s", description))
	ui.Warning("This action cannot be undone!")

	input, err := ui.PromptString("Type 'yes' to confirm", "")
	if err != nil {
		return false, err
	}

	if strings.ToLower(strings.TrimSpace(input)) != "yes" {
		ui.Info("Cancelled")
		return false, nil
	}

	return true, nil
}
