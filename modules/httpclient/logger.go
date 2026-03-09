// Package httpclient provides a configurable HTTP client module for the modular framework.
package httpclient

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/CrisisTextLine/modular"
)

// sanitizeForFilename replaces unsafe filename characters and ensures no directory traversal or special segments are allowed.
func sanitizeForFilename(s string) string {
	// Replace path separators and common unsafe characters with '_'
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)
	safe := replacer.Replace(s)

	// Remove any remaining possible ".." sequences (to prevent traversal)
	safe = strings.ReplaceAll(safe, "..", "_")

	// Optionally remove any non-alphanumeric, non-underscore, non-hyphen, non-dot characters.
	builder := strings.Builder{}
	for _, r := range safe {
		if ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z') || ('0' <= r && r <= '9') || r == '_' || r == '-' || r == '.' {
			builder.WriteRune(r)
		} else {
			builder.WriteRune('_')
		}
	}
	safe = builder.String()

	// Limit length to 100 characters to avoid filesystem issues.
	if len(safe) > 100 {
		safe = safe[:100]
	}

	// Don't allow empty filenames.
	if len(strings.Trim(safe, "_-.")) == 0 {
		return ""
	}

	return safe
}

// FileLogger handles logging HTTP request and response data to files.
type FileLogger struct {
	baseDir     string
	logger      modular.Logger
	mu          sync.Mutex
	openFiles   map[string]*os.File
	requestDir  string
	responseDir string
	txnDir      string
}

// NewFileLogger creates a new file logger that writes HTTP data to files.
func NewFileLogger(baseDir string, logger modular.Logger) (*FileLogger, error) {
	// Create the required directories
	requestDir := filepath.Join(baseDir, "requests")
	responseDir := filepath.Join(baseDir, "responses")
	txnDir := filepath.Join(baseDir, "transactions")

	dirs := []string{requestDir, responseDir, txnDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory '%s': %w", dir, err)
		}
	}

	return &FileLogger{
		baseDir:     baseDir,
		logger:      logger,
		openFiles:   make(map[string]*os.File),
		requestDir:  requestDir,
		responseDir: responseDir,
		txnDir:      txnDir,
	}, nil
}

// LogRequest writes request data to a file.
func (f *FileLogger) LogRequest(id string, data []byte) error {
	safeID := sanitizeForFilename(id)
	if safeID == "" {
		return fmt.Errorf("request ID %q: %w", id, ErrUnsafeFilename)
	}
	requestFile := filepath.Join(f.requestDir, fmt.Sprintf("request_%s_%d.log", safeID, time.Now().UnixNano()))
	if err := os.WriteFile(requestFile, data, 0600); err != nil { //nolint:gosec // path components are sanitized above
		return fmt.Errorf("failed to write request log file %s: %w", requestFile, err)
	}
	return nil
}

// LogResponse writes response data to a file.
func (f *FileLogger) LogResponse(id string, data []byte) error {
	safeID := sanitizeForFilename(id)
	if safeID == "" {
		return fmt.Errorf("response ID %q: %w", id, ErrUnsafeFilename)
	}
	responseFile := filepath.Join(f.responseDir, fmt.Sprintf("response_%s_%d.log", safeID, time.Now().UnixNano()))
	if err := os.WriteFile(responseFile, data, 0600); err != nil { //nolint:gosec // path components are sanitized above
		return fmt.Errorf("failed to write response log file %s: %w", responseFile, err)
	}
	return nil
}

// LogTransactionToFile logs both request and response data to a single file for easier analysis.
func (f *FileLogger) LogTransactionToFile(id string, reqData, respData []byte, duration time.Duration, url string) error {
	// Create a filename that's safe for the filesystem
	safeID := sanitizeForFilename(id)
	if safeID == "" {
		return fmt.Errorf("transaction ID %q: %w", id, ErrUnsafeFilename)
	}
	safeURL := sanitizeForFilename(url)
	if safeURL == "" {
		return fmt.Errorf("URL %q: %w", url, ErrUnsafeFilename)
	}

	txnFile := filepath.Join(f.txnDir, fmt.Sprintf("txn_%s_%s_%d.log", safeID, safeURL, time.Now().UnixNano()))

	file, err := os.Create(txnFile) //nolint:gosec // path components are sanitized above
	if err != nil {
		return fmt.Errorf("failed to create transaction log file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			// Log error but don't fail the operation
			fmt.Printf("Failed to close transaction log file: %v\n", closeErr)
		}
	}()

	// Write transaction metadata
	if _, err := fmt.Fprintf(file, "Transaction ID: %s\n", id); err != nil { //nolint:gosec // G705 false positive: writing to local file, not HTTP response
		return fmt.Errorf("failed to write transaction ID to log file: %w", err)
	}
	if _, err := fmt.Fprintf(file, "URL: %s\n", url); err != nil {
		return fmt.Errorf("failed to write URL to log file: %w", err)
	}
	if _, err := fmt.Fprintf(file, "Time: %s\n", time.Now().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("failed to write timestamp to log file: %w", err)
	}
	if _, err := fmt.Fprintf(file, "Duration: %d ms\n", duration.Milliseconds()); err != nil { //nolint:gosec // G705 false positive: writing to local file, not HTTP response
		return fmt.Errorf("failed to write duration to log file: %w", err)
	}
	if _, err := fmt.Fprintf(file, "\n----- REQUEST -----\n\n"); err != nil {
		return fmt.Errorf("failed to write request separator to log file: %w", err)
	}

	// Write request data
	if _, err := file.Write(reqData); err != nil {
		return fmt.Errorf("failed to write request data: %w", err)
	}

	// Write response data with a separator
	if _, err := fmt.Fprintf(file, "\n\n----- RESPONSE -----\n\n"); err != nil {
		return fmt.Errorf("failed to write response separator to log file: %w", err)
	}
	if _, err := file.Write(respData); err != nil {
		return fmt.Errorf("failed to write response data: %w", err)
	}

	return nil
}

// Close closes any open files and cleans up resources.
func (f *FileLogger) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	var lastErr error
	for name, file := range f.openFiles {
		if err := file.Close(); err != nil {
			f.logger.Error("Failed to close log file",
				"file", name,
				"error", err,
			)
			lastErr = err
		}
		delete(f.openFiles, name)
	}

	return lastErr
}
