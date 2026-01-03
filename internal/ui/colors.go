// Package ui provides terminal UI utilities including colors, spinners, and prompts.
package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
)

// Color functions for styled output
var (
	Green   = color.New(color.FgGreen).SprintFunc()
	Yellow  = color.New(color.FgYellow).SprintFunc()
	Red     = color.New(color.FgRed).SprintFunc()
	Blue    = color.New(color.FgBlue).SprintFunc()
	Cyan    = color.New(color.FgCyan).SprintFunc()
	Magenta = color.New(color.FgMagenta).SprintFunc()
	White   = color.New(color.FgWhite).SprintFunc()
	Bold    = color.New(color.Bold).SprintFunc()
	Dim     = color.New(color.Faint).SprintFunc()
)

// Success prints a success message with a green checkmark.
func Success(msg string) {
	fmt.Printf("%s %s\n", Green("✓"), msg)
}

// Successf prints a formatted success message.
func Successf(format string, args ...interface{}) {
	Success(fmt.Sprintf(format, args...))
}

// Warning prints a warning message with a yellow warning symbol.
func Warning(msg string) {
	fmt.Printf("%s %s\n", Yellow("⚠"), msg)
}

// Warningf prints a formatted warning message.
func Warningf(format string, args ...interface{}) {
	Warning(fmt.Sprintf(format, args...))
}

// Error prints an error message with a red X.
func Error(msg string) {
	fmt.Fprintf(os.Stderr, "%s %s\n", Red("✗"), msg)
}

// Errorf prints a formatted error message.
func Errorf(format string, args ...interface{}) {
	Error(fmt.Sprintf(format, args...))
}

// Info prints an info message with a blue arrow.
func Info(msg string) {
	fmt.Printf("%s %s\n", Blue("→"), msg)
}

// Infof prints a formatted info message.
func Infof(format string, args ...interface{}) {
	Info(fmt.Sprintf(format, args...))
}

// Debug prints a debug message with a dim bullet (only if DRIFT_DEBUG is set).
func Debug(msg string) {
	if os.Getenv("DRIFT_DEBUG") != "" {
		fmt.Printf("%s %s\n", Dim("•"), Dim(msg))
	}
}

// Debugf prints a formatted debug message.
func Debugf(format string, args ...interface{}) {
	Debug(fmt.Sprintf(format, args...))
}

// Header prints a styled header box.
func Header(title string) {
	width := 62
	titleLen := len(title)
	padding := width - titleLen - 4 // -4 for "║  " and "║"

	if padding < 0 {
		padding = 0
	}

	fmt.Println()
	fmt.Printf("%s\n", Cyan("╔"+strings.Repeat("═", width)+"╗"))
	fmt.Printf("%s  %s%s%s\n", Cyan("║"), Bold(title), strings.Repeat(" ", padding), Cyan("║"))
	fmt.Printf("%s\n", Cyan("╚"+strings.Repeat("═", width)+"╝"))
	fmt.Println()
}

// SubHeader prints a styled sub-header.
func SubHeader(title string) {
	fmt.Printf("\n%s %s\n", Cyan("─────"), Bold(title))
}

// KeyValue prints a formatted key-value pair.
func KeyValue(key, value string) {
	fmt.Printf("  %-18s %s\n", Dim(key+":"), value)
}

// KeyValueColored prints a formatted key-value pair with colored value.
func KeyValueColored(key, value string, colorFn func(a ...interface{}) string) {
	fmt.Printf("  %-18s %s\n", Dim(key+":"), colorFn(value))
}

// List prints a bulleted list item.
func List(item string) {
	fmt.Printf("  %s %s\n", Dim("•"), item)
}

// NumberedList prints a numbered list item.
func NumberedList(num int, item string) {
	fmt.Printf("  %s %s\n", Dim(fmt.Sprintf("%d.", num)), item)
}

// Divider prints a horizontal divider.
func Divider() {
	fmt.Printf("\n%s\n\n", Dim(strings.Repeat("─", 60)))
}

// NewLine prints a blank line.
func NewLine() {
	fmt.Println()
}

// PrintEnv prints environment information in a formatted way.
func PrintEnv(gitBranch, env, supabaseBranch, projectRef string) {
	KeyValue("Git Branch", Cyan(gitBranch))
	KeyValue("Environment", envColor(env))
	KeyValue("Supabase Branch", Cyan(supabaseBranch))
	KeyValue("Project Ref", Cyan(projectRef))
}

// envColor returns the colored environment string.
func envColor(env string) string {
	switch strings.ToLower(env) {
	case "production":
		return Red(env)
	case "development":
		return Yellow(env)
	default:
		return Green(env) // Feature branches
	}
}

// Badge returns a colored badge string.
func Badge(label, color string) string {
	switch color {
	case "green":
		return Green("[" + label + "]")
	case "yellow":
		return Yellow("[" + label + "]")
	case "red":
		return Red("[" + label + "]")
	case "blue":
		return Blue("[" + label + "]")
	case "cyan":
		return Cyan("[" + label + "]")
	default:
		return "[" + label + "]"
	}
}

// ProgressStart prints a progress message (typically used with a spinner).
func ProgressStart(msg string) {
	fmt.Printf("%s %s...", Blue("⋯"), msg)
}

// ProgressDone completes a progress message.
func ProgressDone() {
	fmt.Printf(" %s\n", Green("done"))
}

// ProgressFail marks a progress message as failed.
func ProgressFail() {
	fmt.Printf(" %s\n", Red("failed"))
}

// Confirm prints a confirmation prompt message (actual prompting is handled by promptui).
func Confirm(msg string) {
	fmt.Printf("%s %s ", Yellow("?"), msg)
}

