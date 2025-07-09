package feeders

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
)

// parseDotEnvFile parses a .env file and returns the key-value pairs
func parseDotEnvFile(filename string) (map[string]string, error) {
	result := make(map[string]string)

	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open .env file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse key=value pairs
		idx := strings.Index(line, "=")
		if idx == -1 {
			return nil, fmt.Errorf("%w at line %d: %s", ErrDotEnvInvalidLineFormat, lineNum, line)
		}

		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])

		// Remove quotes if present
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		result[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanner error: %w", err)
	}

	return result, nil
}

// EnvCatalog manages a unified view of environment variables from multiple sources
type EnvCatalog struct {
	variables map[string]string
	mutex     sync.RWMutex
	sources   map[string]string // tracks which source provided each variable
}

// NewEnvCatalog creates a new environment variable catalog
func NewEnvCatalog() *EnvCatalog {
	catalog := &EnvCatalog{
		variables: make(map[string]string),
		sources:   make(map[string]string),
	}
	// Load OS environment variables
	catalog.loadOSEnvironment()
	return catalog
}

// loadOSEnvironment loads all OS environment variables into the catalog
func (c *EnvCatalog) loadOSEnvironment() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for _, env := range os.Environ() {
		if parts := strings.SplitN(env, "=", 2); len(parts) == 2 {
			key, value := parts[0], parts[1]
			c.variables[key] = value
			c.sources[key] = "os_env"
		}
	}
}

// LoadFromDotEnv loads variables from a .env file into the catalog
func (c *EnvCatalog) LoadFromDotEnv(filename string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	dotEnvVars, err := parseDotEnvFile(filename)
	if err != nil {
		return err
	}

	for key, value := range dotEnvVars {
		// Only set if not already present (OS env and existing values take precedence)
		if _, exists := c.variables[key]; !exists {
			// Also check OS environment before setting
			if osValue := os.Getenv(key); osValue != "" {
				// OS environment takes precedence, set that instead
				c.variables[key] = osValue
				c.sources[key] = "os_env"
			} else {
				// No OS env, use .env value
				c.variables[key] = value
				c.sources[key] = "dotenv:" + filename
			}
		}
	}

	return nil
}

// Set manually sets a variable in the catalog
func (c *EnvCatalog) Set(key, value, source string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.variables[key] = value
	c.sources[key] = source
}

// Get retrieves a variable from the catalog, always checking current OS environment
func (c *EnvCatalog) Get(key string) (string, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	// Always check OS environment first for the most current value
	if osValue := os.Getenv(key); osValue != "" {
		// If OS env value differs from cached, update cache
		if cachedValue, exists := c.variables[key]; !exists || cachedValue != osValue {
			c.mutex.RUnlock()
			c.mutex.Lock()
			c.variables[key] = osValue
			if c.sources[key] == "" {
				c.sources[key] = "os_env"
			}
			c.mutex.Unlock()
			c.mutex.RLock()
		}
		return osValue, true
	}

	// If not in OS environment, check our internal catalog (for dotenv values)
	if value, exists := c.variables[key]; exists {
		return value, true
	}

	return "", false
}

// GetSource returns the source that provided a variable
func (c *EnvCatalog) GetSource(key string) string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.sources[key]
}

// GetAll returns all variables in the catalog
func (c *EnvCatalog) GetAll() map[string]string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	result := make(map[string]string, len(c.variables))
	for k, v := range c.variables {
		result[k] = v
	}
	return result
}

// Clear removes all variables from the catalog
func (c *EnvCatalog) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.variables = make(map[string]string)
	c.sources = make(map[string]string)
}

// ClearDynamicEnvCache clears dynamically loaded environment variables from cache
// This is useful for testing when environment variables change between tests
func (c *EnvCatalog) ClearDynamicEnvCache() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Remove dynamically loaded env vars but keep initial OS env and dotenv
	for key, source := range c.sources {
		if source == "os_env_dynamic" {
			delete(c.variables, key)
			delete(c.sources, key)
		}
	}
}

// Global catalog instance for all env-based feeders to use
var globalEnvCatalog = NewEnvCatalog()

// GetGlobalEnvCatalog returns the global environment catalog
func GetGlobalEnvCatalog() *EnvCatalog {
	return globalEnvCatalog
}

// ResetGlobalEnvCatalog resets the global environment catalog (useful for testing)
func ResetGlobalEnvCatalog() {
	globalEnvCatalog = NewEnvCatalog()
}
