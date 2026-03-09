package modular

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/GoCodeAlone/modular/feeders"
)

// TestTenantConfigAffixedEnvBug tests the specific bug where tenant config loading
// fails with "env: prefix or suffix cannot be empty" when using TenantAffixedEnvFeeder
func TestTenantConfigAffixedEnvBug(t *testing.T) {
	// Create temp directory for tenant configs
	tempDir, err := os.MkdirTemp("", "tenant-affixed-env-bug-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to remove temp directory: %v", err)
		}
	}()

	// Create tenant config files
	createTenantConfigFiles(t, tempDir)

	// Create application with required services
	app, tenantService := setupAppWithTenantServices(t)

	// Create loader with TenantAffixedEnvFeeder (reproduces the bug)
	loader := NewFileBasedTenantConfigLoader(TenantConfigParams{
		ConfigNameRegex: regexp.MustCompile(`^\w+\.yaml$`),
		ConfigDir:       tempDir,
		ConfigFeeders: []Feeder{
			feeders.NewTenantAffixedEnvFeeder(func(tenantId string) string {
				return fmt.Sprintf("%s_", tenantId)
			}, func(s string) string { return "" }),
		},
	})

	// This should NOT fail with "env: prefix or suffix cannot be empty"
	err = loader.LoadTenantConfigurations(app, tenantService)
	if err != nil {
		t.Fatalf("Expected tenant config loading to succeed, but got error: %v", err)
	}

	// Verify tenants were loaded
	tenants := tenantService.GetTenants()
	if len(tenants) == 0 {
		t.Error("Expected at least one tenant to be loaded")
	}
}

// TestTenantConfigAffixedEnvBugReproduction confirms the bug has been fixed
func TestTenantConfigAffixedEnvBugReproduction(t *testing.T) {
	// Create temp directory for tenant configs
	tempDir, err := os.MkdirTemp("", "tenant-affixed-env-reproduction-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to remove temp directory: %v", err)
		}
	}()

	// Create tenant config files
	createTenantConfigFiles(t, tempDir)

	// Create application with required services
	app, tenantService := setupAppWithTenantServices(t)

	// Create loader with TenantAffixedEnvFeeder
	loader := NewFileBasedTenantConfigLoader(TenantConfigParams{
		ConfigNameRegex: regexp.MustCompile(`^\w+\.yaml$`),
		ConfigDir:       tempDir,
		ConfigFeeders: []Feeder{
			feeders.NewTenantAffixedEnvFeeder(func(tenantId string) string {
				return fmt.Sprintf("%s_", tenantId)
			}, func(s string) string { return "" }),
		},
	})

	// Try to load tenant configurations - this should now succeed
	err = loader.LoadTenantConfigurations(app, tenantService)

	if err != nil {
		t.Errorf("Expected tenant config loading to succeed after fix, but got error: %v", err)
	} else {
		t.Log("Bug has been fixed - tenant config loading succeeded")

		// Verify tenants were loaded
		tenants := tenantService.GetTenants()
		if len(tenants) == 0 {
			t.Error("Expected at least one tenant to be loaded")
		} else {
			t.Logf("Successfully loaded %d tenants", len(tenants))
		}
	}
}

// createTenantConfigFiles creates sample tenant config files
func createTenantConfigFiles(t *testing.T, tempDir string) {
	ctlConfig := `
content:
  defaultTemplate: ctl-branded
  cacheTTL: 600

notifications:
  provider: email
  fromAddress: support@ctl.example.com
  maxRetries: 5
`

	sampleAff1Config := `
content:
  defaultTemplate: sampleaff1-branded
  cacheTTL: 300

notifications:
  provider: sms
  fromAddress: support@sampleaff1.example.com
  maxRetries: 3
`

	// Write config files
	err := os.WriteFile(filepath.Join(tempDir, "ctl.yaml"), []byte(ctlConfig), 0600)
	if err != nil {
		t.Fatalf("Failed to write ctl.yaml: %v", err)
	}

	err = os.WriteFile(filepath.Join(tempDir, "sampleaff1.yaml"), []byte(sampleAff1Config), 0600)
	if err != nil {
		t.Fatalf("Failed to write sampleaff1.yaml: %v", err)
	}
}

// setupAppWithTenantServices creates app with required config sections and services
func setupAppWithTenantServices(t *testing.T) (Application, TenantService) {
	// Content config
	type ContentConfig struct {
		DefaultTemplate string `yaml:"defaultTemplate"`
		CacheTTL        int    `yaml:"cacheTTL"`
	}

	// Notifications config
	type NotificationsConfig struct {
		Provider    string `yaml:"provider"`
		FromAddress string `yaml:"fromAddress"`
		MaxRetries  int    `yaml:"maxRetries"`
	}

	log := &logger{t}
	app := NewStdApplication(NewStdConfigProvider(nil), log)

	// Register config sections that match the tenant YAML files
	app.RegisterConfigSection("content", NewStdConfigProvider(&ContentConfig{}))
	app.RegisterConfigSection("notifications", NewStdConfigProvider(&NotificationsConfig{}))

	// Create tenant service
	tenantService := NewStandardTenantService(log)

	return app, tenantService
}

// TestTenantAffixedEnvFeederDirectUsage tests the TenantAffixedEnvFeeder directly
func TestTenantAffixedEnvFeederDirectUsage(t *testing.T) {
	type TestConfig struct {
		Name string `env:"NAME"`
		Port int    `env:"PORT"`
	}

	config := &TestConfig{}

	// Create a TenantAffixedEnvFeeder
	feeder := feeders.NewTenantAffixedEnvFeeder(func(tenantId string) string {
		return fmt.Sprintf("%s_", tenantId)
	}, func(s string) string { return "" })

	// With the fix, Feed should now work (it will look for unprefixed env vars when no tenant is set)
	err := feeder.Feed(config)
	if err != nil {
		t.Errorf("Expected no error when calling Feed directly, but got: %v", err)
	} else {
		t.Log("Direct Feed call succeeded - fix is working")
	}

	// Test FeedKey with a tenant ID
	config2 := &TestConfig{}
	err = feeder.FeedKey("ctl", config2)
	if err != nil {
		t.Errorf("Expected no error when calling FeedKey with tenant ID, but got: %v", err)
	} else {
		t.Log("FeedKey call with tenant ID succeeded")
	}
}
