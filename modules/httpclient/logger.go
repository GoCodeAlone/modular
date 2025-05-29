// Package httpclient provides a configurable HTTP client module for the modular framework.
package httpclient

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/GoCodeAlone/modular"
)

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
	requestFile := filepath.Join(f.requestDir, fmt.Sprintf("request_%s_%d.log", id, time.Now().UnixNano()))
	return os.WriteFile(requestFile, data, 0644)
}

// LogResponse writes response data to a file.
func (f *FileLogger) LogResponse(id string, data []byte) error {
	responseFile := filepath.Join(f.responseDir, fmt.Sprintf("response_%s_%d.log", id, time.Now().UnixNano()))
	return os.WriteFile(responseFile, data, 0644)
}

// LogTransactionToFile logs both request and response data to a single file for easier analysis.
func (f *FileLogger) LogTransactionToFile(id string, reqData, respData []byte, duration time.Duration, url string) error {
	// Create a filename that's safe for the filesystem
	safeURL := strings.ReplaceAll(url, "/", "_")
	safeURL = strings.ReplaceAll(safeURL, ":", "_")
	if len(safeURL) > 100 {
		safeURL = safeURL[:100] // Limit length to avoid issues with too-long filenames
	}

	txnFile := filepath.Join(f.txnDir, fmt.Sprintf("txn_%s_%s_%d.log", id, safeURL, time.Now().UnixNano()))

	file, err := os.Create(txnFile)
	if err != nil {
		return fmt.Errorf("failed to create transaction log file: %w", err)
	}
	defer file.Close()

	// Write transaction metadata
	if _, err := fmt.Fprintf(file, "Transaction ID: %s\n", id); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(file, "URL: %s\n", url); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(file, "Time: %s\n", time.Now().Format(time.RFC3339)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(file, "Duration: %d ms\n", duration.Milliseconds()); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(file, "\n----- REQUEST -----\n\n"); err != nil {
		return err
	}

	// Write request data
	if _, err := file.Write(reqData); err != nil {
		return fmt.Errorf("failed to write request data: %w", err)
	}

	// Write response data with a separator
	if _, err := fmt.Fprintf(file, "\n\n----- RESPONSE -----\n\n"); err != nil {
		return err
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
