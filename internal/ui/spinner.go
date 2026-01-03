package ui

import (
	"time"

	"github.com/briandowns/spinner"
)

// Spinner wraps the briandowns/spinner package for consistent styling.
type Spinner struct {
	s *spinner.Spinner
}

// NewSpinner creates a new spinner with the given message.
func NewSpinner(msg string) *Spinner {
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond) // Dots pattern
	s.Suffix = " " + msg
	s.Color("cyan")
	return &Spinner{s: s}
}

// Start starts the spinner.
func (sp *Spinner) Start() {
	sp.s.Start()
}

// Stop stops the spinner.
func (sp *Spinner) Stop() {
	sp.s.Stop()
}

// Success stops the spinner and prints a success message.
func (sp *Spinner) Success(msg string) {
	sp.s.Stop()
	Success(msg)
}

// Fail stops the spinner and prints an error message.
func (sp *Spinner) Fail(msg string) {
	sp.s.Stop()
	Error(msg)
}

// UpdateMessage updates the spinner's message.
func (sp *Spinner) UpdateMessage(msg string) {
	sp.s.Suffix = " " + msg
}

// WithSpinner runs a function while showing a spinner.
func WithSpinner(msg string, fn func() error) error {
	sp := NewSpinner(msg)
	sp.Start()
	err := fn()
	sp.Stop()
	return err
}

// WithSpinnerResult runs a function while showing a spinner and prints success/fail.
func WithSpinnerResult(msg string, fn func() error) error {
	sp := NewSpinner(msg)
	sp.Start()
	err := fn()
	if err != nil {
		sp.Fail(msg + " - failed")
		return err
	}
	sp.Success(msg)
	return nil
}

