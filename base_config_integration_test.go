package modular

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBaseConfigTenantSupport(t *testing.T) {
	// Create temporary directory structure
	tempDir, err := os.MkdirTemp("", "base-config-tenant-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create directory structure
	baseDir := filepath.Join(tempDir, "base")
	envDir := filepath.Join(tempDir, "environments", "prod")
	baseTenantDir := filepath.Join(baseDir, "tenants")
	envTenantDir := filepath.Join(envDir, "tenants")

	require.NoError(t, os.MkdirAll(baseDir, 0755))
	require.NoError(t, os.MkdirAll(envDir, 0755))
	require.NoError(t, os.MkdirAll(baseTenantDir, 0755))
	require.NoError(t, os.MkdirAll(envTenantDir, 0755))

	// Create base tenant config
	baseTenantConfig := `
# Base tenant config
content:
  name: "Base Content"
  enabled: true
  
notifications:
  email: true
  sms: false
  webhook_url: "http://base.example.com"
`

	// Create production tenant overrides
	prodTenantConfig := `
# Production tenant overrides
content:
  name: "Production Content"
  
notifications:
  sms: true
  webhook_url: "http://prod.example.com"
`

	// Write tenant config files
	require.NoError(t, os.WriteFile(filepath.Join(baseTenantDir, "tenant1.yaml"), []byte(baseTenantConfig), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(envTenantDir, "tenant1.yaml"), []byte(prodTenantConfig), 0644))

	// Set up base config
	SetBaseConfig(tempDir, "prod")

	// Create application and tenant service
	logger := &baseConfigTestLogger{t}
	app := NewStdApplication(nil, logger)
	tenantService := NewStandardTenantService(logger)

	// Register test config sections
	app.RegisterConfigSection("content", NewStdConfigProvider(&ContentConfig{}))
	app.RegisterConfigSection("notifications", NewStdConfigProvider(&NotificationsConfig{}))

	// Load tenant configurations
	tenantConfigParams := TenantConfigParams{
		ConfigNameRegex: regexp.MustCompile(`^\w+\.yaml$`),
		ConfigDir:       tempDir, // This will be detected as base config structure
		ConfigFeeders:   []Feeder{},
	}

	err = LoadTenantConfigs(app, tenantService, tenantConfigParams)
	require.NoError(t, err)

	// Verify tenant configuration was loaded and merged correctly
	tenantID := TenantID("tenant1")

	// Check content config
	contentProvider, err := tenantService.GetTenantConfig(tenantID, "content")
	require.NoError(t, err)
	contentConfig := contentProvider.GetConfig().(*ContentConfig)
	assert.Equal(t, "Production Content", contentConfig.Name, "Content name should be overridden")
	assert.True(t, contentConfig.Enabled, "Content enabled should come from base")

	// Check notifications config
	notificationsProvider, err := tenantService.GetTenantConfig(tenantID, "notifications")
	require.NoError(t, err)
	notificationsConfig := notificationsProvider.GetConfig().(*NotificationsConfig)
	assert.True(t, notificationsConfig.Email, "Email should come from base")
	assert.True(t, notificationsConfig.SMS, "SMS should be overridden to true")
	assert.Equal(t, "http://prod.example.com", notificationsConfig.WebhookURL, "Webhook URL should be overridden")
}

// Test config structures for tenant tests
type ContentConfig struct {
	Name    string `yaml:"name"`
	Enabled bool   `yaml:"enabled"`
}

type NotificationsConfig struct {
	Email      bool   `yaml:"email"`
	SMS        bool   `yaml:"sms"`
	WebhookURL string `yaml:"webhook_url"`
}

// baseConfigTestLogger implements Logger for testing
type baseConfigTestLogger struct {
	t *testing.T
}

func (l *baseConfigTestLogger) Debug(msg string, args ...any) {
	l.t.Logf("DEBUG: %s %v", msg, args)
}

func (l *baseConfigTestLogger) Info(msg string, args ...any) {
	l.t.Logf("INFO: %s %v", msg, args)
}

func (l *baseConfigTestLogger) Warn(msg string, args ...any) {
	l.t.Logf("WARN: %s %v", msg, args)
}

func (l *baseConfigTestLogger) Error(msg string, args ...any) {
	l.t.Logf("ERROR: %s %v", msg, args)
}
