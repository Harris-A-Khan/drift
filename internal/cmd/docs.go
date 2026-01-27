package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/undrift/drift/internal/ui"
	"github.com/undrift/drift/pkg/shell"
)

var docsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Serve documentation locally",
	Long: `Start a local HTTP server to browse Drift documentation.

The documentation is served using Docsify and will be opened
in your default browser automatically.

Press Ctrl+C to stop the server.`,
	Example: `  drift docs              # Serve on localhost:3000
  drift docs --port 8080  # Serve on custom port
  drift docs --no-open    # Don't open browser`,
	RunE: runDocs,
}

var (
	docsPort   int
	docsNoOpen bool
	docsHost   string
)

func init() {
	docsCmd.Flags().IntVar(&docsPort, "port", 3000, "Port to serve documentation on")
	docsCmd.Flags().BoolVar(&docsNoOpen, "no-open", false, "Don't automatically open browser")
	docsCmd.Flags().StringVar(&docsHost, "host", "localhost", "Host to bind to")
	rootCmd.AddCommand(docsCmd)
}

func runDocs(cmd *cobra.Command, args []string) error {
	// Find docs directory
	docsDir, err := findDocsDir()
	if err != nil {
		return fmt.Errorf("documentation not found: %w\n\nMake sure you're running from the drift project directory", err)
	}

	// Create file server
	fs := http.FileServer(http.Dir(docsDir))

	// Create server
	addr := fmt.Sprintf("%s:%d", docsHost, docsPort)
	server := &http.Server{
		Addr:              addr,
		Handler:           fs,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Set up graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Channel to capture server errors
	serverErr := make(chan error, 1)

	// Start server in goroutine
	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			serverErr <- err
		}
		close(serverErr)
	}()

	// Give the server a moment to start (or fail)
	time.Sleep(100 * time.Millisecond)

	// Check if server failed to start
	select {
	case err := <-serverErr:
		if err != nil {
			return fmt.Errorf("failed to start server: %w", err)
		}
	default:
		// Server started successfully
	}

	// Display server info
	url := fmt.Sprintf("http://%s", addr)
	ui.Header("Documentation Server")
	ui.KeyValue("URL", ui.Cyan(url))
	ui.KeyValue("Docs", docsDir)
	ui.NewLine()
	ui.Info("Press Ctrl+C to stop the server")
	ui.NewLine()

	// Open browser (macOS uses "open", Linux uses "xdg-open")
	if !docsNoOpen {
		openBrowser(url)
	}

	// Wait for shutdown signal
	<-ctx.Done()

	ui.NewLine()
	ui.Info("Shutting down server...")

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown error: %w", err)
	}

	ui.Success("Server stopped")
	return nil
}

// findDocsDir locates the documentation directory.
func findDocsDir() (string, error) {
	// Try multiple locations
	candidates := []string{
		"docs",
		filepath.Join(filepath.Dir(os.Args[0]), "docs"),
		filepath.Join(filepath.Dir(os.Args[0]), "..", "docs"),
	}

	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			// Verify index.html exists (Docsify entry point)
			if _, err := os.Stat(filepath.Join(path, "index.html")); err == nil {
				return filepath.Abs(path)
			}
		}
	}

	return "", fmt.Errorf("docs directory with index.html not found")
}

// openBrowser opens the URL in the default browser.
func openBrowser(url string) {
	// Use "open" on macOS, which is the most common platform for drift
	_, _ = shell.Run("open", url)
}
