package feeders

import (
	"bufio"
	"fmt"
	"os"
	"reflect"
	"strings"
)

// DotEnvFeeder is a feeder that reads .env files with optional verbose debug logging
type DotEnvFeeder struct {
	Path         string
	verboseDebug bool
	logger       interface {
		Debug(msg string, args ...any)
	}
}

// NewDotEnvFeeder creates a new DotEnvFeeder that reads from the specified .env file
func NewDotEnvFeeder(filePath string) DotEnvFeeder {
	return DotEnvFeeder{
		Path:         filePath,
		verboseDebug: false,
		logger:       nil,
	}
}

// SetVerboseDebug enables or disables verbose debug logging
func (f *DotEnvFeeder) SetVerboseDebug(enabled bool, logger interface{ Debug(msg string, args ...any) }) {
	f.verboseDebug = enabled
	f.logger = logger
	if enabled && logger != nil {
		f.logger.Debug("Verbose dot env feeder debugging enabled")
	}
}

// Feed reads the .env file and populates the provided structure
func (f DotEnvFeeder) Feed(structure interface{}) error {
	if f.verboseDebug && f.logger != nil {
		f.logger.Debug("DotEnvFeeder: Starting feed process", "filePath", f.Path, "structureType", reflect.TypeOf(structure))
	}

	// Load environment variables from .env file
	err := f.loadDotEnvFile()
	if err != nil {
		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("DotEnvFeeder: Failed to load .env file", "filePath", f.Path, "error", err)
		}
		return err
	}

	// Use the env feeder logic to populate the structure
	envFeeder := EnvFeeder{
		verboseDebug: f.verboseDebug,
		logger:       f.logger,
	}
	return envFeeder.Feed(structure)
}

// loadDotEnvFile loads environment variables from the .env file
func (f DotEnvFeeder) loadDotEnvFile() error {
	if f.verboseDebug && f.logger != nil {
		f.logger.Debug("DotEnvFeeder: Loading .env file", "filePath", f.Path)
	}

	file, err := os.Open(f.Path)
	if err != nil {
		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("DotEnvFeeder: Failed to open .env file", "filePath", f.Path, "error", err)
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			if f.verboseDebug && f.logger != nil {
				f.logger.Debug("DotEnvFeeder: Skipping line", "lineNum", lineNum, "reason", "empty or comment")
			}
			continue
		}

		// Parse key=value pairs
		if err := f.parseEnvLine(line, lineNum); err != nil {
			if f.verboseDebug && f.logger != nil {
				f.logger.Debug("DotEnvFeeder: Failed to parse line", "lineNum", lineNum, "line", line, "error", err)
			}
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("DotEnvFeeder: Scanner error", "error", err)
		}
		return err
	}

	if f.verboseDebug && f.logger != nil {
		f.logger.Debug("DotEnvFeeder: Successfully loaded .env file", "filePath", f.Path, "linesProcessed", lineNum)
	}
	return nil
}

// parseEnvLine parses a single line from the .env file
func (f DotEnvFeeder) parseEnvLine(line string, lineNum int) error {
	// Find the first = character
	idx := strings.Index(line, "=")
	if idx == -1 {
		return fmt.Errorf("invalid line format at line %d: %s", lineNum, line)
	}

	key := strings.TrimSpace(line[:idx])
	value := strings.TrimSpace(line[idx+1:])

	// Remove quotes if present
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
			value = value[1 : len(value)-1]
		}
	}

	if f.verboseDebug && f.logger != nil {
		f.logger.Debug("DotEnvFeeder: Setting environment variable", "key", key, "value", value, "lineNum", lineNum)
	}

	// Set the environment variable
	os.Setenv(key, value)
	return nil
}
