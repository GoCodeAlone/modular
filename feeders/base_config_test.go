package feeders

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BaseTestConfig represents a simple test configuration structure for base config tests
type BaseTestConfig struct {
	AppName     string             `yaml:"app_name"`
	Environment string             `yaml:"environment"`
	Database    BaseDatabaseConfig `yaml:"database"`
	Features    map[string]bool    `yaml:"features"`
	Servers     []BaseServerConfig `yaml:"servers"`
}

type BaseDatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Name     string `yaml:"name"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type BaseServerConfig struct {
	Name string `yaml:"name"`
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

func TestBaseConfigFeeder_BasicMerging(t *testing.T) {
	// Create temporary directory structure
	tempDir := setupTestConfigStructure(t)
	defer os.RemoveAll(tempDir)

	// Create base config
	baseConfig := `
app_name: "MyApp"
environment: "base"
database:
  host: "localhost"
  port: 5432
  name: "myapp"
  username: "user"
  password: "password"
features:
  logging: true
  metrics: false
  caching: true
servers:
  - name: "web1"
    host: "localhost"
    port: 8080
  - name: "web2"
    host: "localhost"
    port: 8081
`

	// Create production overrides
	prodConfig := `
environment: "production"
database:
  host: "prod-db.example.com"
  password: "prod-secret"
features:
  metrics: true
servers:
  - name: "web1"
    host: "prod-web1.example.com"
    port: 8080
  - name: "web2"
    host: "prod-web2.example.com"
    port: 8080
  - name: "web3"
    host: "prod-web3.example.com"
    port: 8080
`

	// Write config files
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "base", "default.yaml"), []byte(baseConfig), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "environments", "prod", "overrides.yaml"), []byte(prodConfig), 0644))

	// Create feeder and test
	feeder := NewBaseConfigFeeder(tempDir, "prod")

	var config BaseTestConfig
	err := feeder.Feed(&config)
	require.NoError(t, err)

	// Verify merged configuration
	assert.Equal(t, "MyApp", config.AppName, "App name should come from base config")
	assert.Equal(t, "production", config.Environment, "Environment should be overridden")

	// Database config should be merged
	assert.Equal(t, "prod-db.example.com", config.Database.Host, "Database host should be overridden")
	assert.Equal(t, 5432, config.Database.Port, "Database port should come from base")
	assert.Equal(t, "myapp", config.Database.Name, "Database name should come from base")
	assert.Equal(t, "user", config.Database.Username, "Database username should come from base")
	assert.Equal(t, "prod-secret", config.Database.Password, "Database password should be overridden")

	// Features should be merged
	assert.True(t, config.Features["logging"], "Logging should come from base")
	assert.True(t, config.Features["metrics"], "Metrics should be overridden to true")
	assert.True(t, config.Features["caching"], "Caching should come from base")

	// Servers should be completely replaced (not merged)
	require.Len(t, config.Servers, 3, "Should have 3 servers from prod override")
	assert.Equal(t, "web1", config.Servers[0].Name)
	assert.Equal(t, "prod-web1.example.com", config.Servers[0].Host)
}

func TestBaseConfigFeeder_BaseOnly(t *testing.T) {
	// Create temporary directory structure
	tempDir := setupTestConfigStructure(t)
	defer os.RemoveAll(tempDir)

	baseConfig := `
app_name: "BaseApp"
environment: "development"
database:
  host: "localhost"
  port: 5432
`

	// Write only base config (no environment overrides)
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "base", "default.yaml"), []byte(baseConfig), 0644))

	// Create feeder for non-existent environment
	feeder := NewBaseConfigFeeder(tempDir, "nonexistent")

	var config BaseTestConfig
	err := feeder.Feed(&config)
	require.NoError(t, err)

	// Should use only base config
	assert.Equal(t, "BaseApp", config.AppName)
	assert.Equal(t, "development", config.Environment)
	assert.Equal(t, "localhost", config.Database.Host)
	assert.Equal(t, 5432, config.Database.Port)
}

func TestBaseConfigFeeder_OverrideOnly(t *testing.T) {
	// Create temporary directory structure
	tempDir := setupTestConfigStructure(t)
	defer os.RemoveAll(tempDir)

	prodConfig := `
app_name: "ProdApp"
environment: "production"
database:
  host: "prod-db.example.com"
  port: 3306
`

	// Write only environment config (no base)
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "environments", "prod", "overrides.yaml"), []byte(prodConfig), 0644))

	feeder := NewBaseConfigFeeder(tempDir, "prod")

	var config BaseTestConfig
	err := feeder.Feed(&config)
	require.NoError(t, err)

	// Should use only override config
	assert.Equal(t, "ProdApp", config.AppName)
	assert.Equal(t, "production", config.Environment)
	assert.Equal(t, "prod-db.example.com", config.Database.Host)
	assert.Equal(t, 3306, config.Database.Port)
}

func TestBaseConfigFeeder_FeedKey_TenantConfigs(t *testing.T) {
	// Create temporary directory structure
	tempDir := setupTestConfigStructure(t)
	defer os.RemoveAll(tempDir)

	// Create base tenant config
	baseTenantConfig := `
database:
  host: "base-tenant-db.example.com"
  port: 5432
  name: "tenant_base"
features:
  logging: true
  metrics: false
`

	// Create production tenant overrides
	prodTenantConfig := `
database:
  host: "prod-tenant-db.example.com"
  password: "tenant-prod-secret"
features:
  metrics: true
`

	// Write tenant config files
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "base", "tenants"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "environments", "prod", "tenants"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "base", "tenants", "tenant1.yaml"), []byte(baseTenantConfig), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "environments", "prod", "tenants", "tenant1.yaml"), []byte(prodTenantConfig), 0644))

	feeder := NewBaseConfigFeeder(tempDir, "prod")

	var config BaseTestConfig
	err := feeder.FeedKey("tenant1", &config)
	require.NoError(t, err)

	// Verify merged tenant configuration
	assert.Equal(t, "prod-tenant-db.example.com", config.Database.Host, "Database host should be overridden")
	assert.Equal(t, 5432, config.Database.Port, "Database port should come from base")
	assert.Equal(t, "tenant_base", config.Database.Name, "Database name should come from base")
	assert.Equal(t, "tenant-prod-secret", config.Database.Password, "Password should be overridden")
	assert.True(t, config.Features["logging"], "Logging should come from base")
	assert.True(t, config.Features["metrics"], "Metrics should be overridden")
}

func TestBaseConfigFeeder_VerboseDebug(t *testing.T) {
	// Create temporary directory structure
	tempDir := setupTestConfigStructure(t)
	defer os.RemoveAll(tempDir)

	baseConfig := `app_name: "TestApp"`
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "base", "default.yaml"), []byte(baseConfig), 0644))

	// Create a mock logger to capture debug messages
	var logMessages []string
	mockLogger := &baseMockLogger{messages: &logMessages}

	feeder := NewBaseConfigFeeder(tempDir, "prod")
	feeder.SetVerboseDebug(true, mockLogger)

	var config BaseTestConfig
	err := feeder.Feed(&config)
	require.NoError(t, err)

	// Verify debug logging was enabled
	assert.Contains(t, logMessages, "Verbose BaseConfig feeder debugging enabled")
	assert.Greater(t, len(logMessages), 1, "Should have multiple debug messages")
}

func TestIsBaseConfigStructure(t *testing.T) {
	// Create temporary directory with base config structure
	tempDir := setupTestConfigStructure(t)
	defer os.RemoveAll(tempDir)

	assert.True(t, IsBaseConfigStructure(tempDir), "Should detect base config structure")

	// Test with directory that doesn't have base config structure
	tempDir2, err := os.MkdirTemp("", "non-base-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir2)

	assert.False(t, IsBaseConfigStructure(tempDir2), "Should not detect base config structure")
}

func TestGetAvailableEnvironments(t *testing.T) {
	// Create temporary directory structure with multiple environments
	tempDir := setupTestConfigStructure(t)
	defer os.RemoveAll(tempDir)

	// Create additional environment directories
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "environments", "staging"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "environments", "dev"), 0755))

	environments := GetAvailableEnvironments(tempDir)
	require.Len(t, environments, 3)
	assert.Contains(t, environments, "prod")
	assert.Contains(t, environments, "staging")
	assert.Contains(t, environments, "dev")
}

// setupTestConfigStructure creates the required directory structure for base config tests
func setupTestConfigStructure(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "base-config-test-*")
	require.NoError(t, err)

	// Create base config directory structure
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "base"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "environments", "prod"), 0755))

	return tempDir
}

// baseMockLogger implements a simple logger for testing base config
type baseMockLogger struct {
	messages *[]string
}

func (m *baseMockLogger) Debug(msg string, args ...interface{}) {
	*m.messages = append(*m.messages, msg)
}
