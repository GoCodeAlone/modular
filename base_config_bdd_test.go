package modular

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/cucumber/godog"
)

// BaseConfigBDDTestContext holds state for base configuration BDD tests
type BaseConfigBDDTestContext struct {
	app               Application
	logger            Logger
	configDir         string
	environment       string
	baseConfigContent string
	envConfigContent  string
	tenantConfigs     map[string]string
	actualConfig      *TestBDDConfig
	configError       error
	tempDirs          []string
}

// TestBDDConfig represents a test configuration structure for BDD tests
type TestBDDConfig struct {
	AppName     string                `yaml:"app_name"`
	Environment string                `yaml:"environment"`
	Database    TestBDDDatabaseConfig `yaml:"database"`
	Features    map[string]bool       `yaml:"features"`
	Servers     []TestBDDServerConfig `yaml:"servers"`
}

type TestBDDDatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Name     string `yaml:"name"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type TestBDDServerConfig struct {
	Name string `yaml:"name"`
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// BDD Step implementations for base configuration

func (ctx *BaseConfigBDDTestContext) iHaveABaseConfigStructureWithEnvironment(environment string) error {
	ctx.environment = environment

	// Create temporary directory structure
	tempDir, err := os.MkdirTemp("", "base-config-bdd-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	ctx.tempDirs = append(ctx.tempDirs, tempDir)
	ctx.configDir = tempDir

	// Create base config directory structure
	baseDir := filepath.Join(tempDir, "base")
	envDir := filepath.Join(tempDir, "environments", environment)
	tenantBaseDir := filepath.Join(baseDir, "tenants")
	tenantEnvDir := filepath.Join(envDir, "tenants")

	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return fmt.Errorf("failed to create base directory: %w", err)
	}
	if err := os.MkdirAll(envDir, 0755); err != nil {
		return fmt.Errorf("failed to create environment directory: %w", err)
	}
	if err := os.MkdirAll(tenantBaseDir, 0755); err != nil {
		return fmt.Errorf("failed to create tenant base directory: %w", err)
	}
	if err := os.MkdirAll(tenantEnvDir, 0755); err != nil {
		return fmt.Errorf("failed to create tenant env directory: %w", err)
	}

	return nil
}

func (ctx *BaseConfigBDDTestContext) theBaseConfigContains(configContent string) error {
	ctx.baseConfigContent = configContent

	baseConfigPath := filepath.Join(ctx.configDir, "base", "default.yaml")
	if err := os.WriteFile(baseConfigPath, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("failed to write base config: %w", err)
	}

	return nil
}

func (ctx *BaseConfigBDDTestContext) theEnvironmentConfigContains(configContent string) error {
	ctx.envConfigContent = configContent

	envConfigPath := filepath.Join(ctx.configDir, "environments", ctx.environment, "overrides.yaml")
	if err := os.WriteFile(envConfigPath, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("failed to write environment config: %w", err)
	}

	return nil
}

func (ctx *BaseConfigBDDTestContext) iSetTheEnvironmentToAndLoadTheConfiguration(environment string) error {
	// Set base config settings
	SetBaseConfig(ctx.configDir, environment)

	// Create application with test config
	ctx.actualConfig = &TestBDDConfig{}
	configProvider := NewStdConfigProvider(ctx.actualConfig)
	ctx.logger = &testBDDLogger{}
	ctx.app = NewStdApplication(configProvider, ctx.logger)

	// Initialize the application to trigger config loading
	if err := ctx.app.Init(); err != nil {
		ctx.configError = err
	}

	return nil
}

func (ctx *BaseConfigBDDTestContext) theConfigurationShouldHaveAppName(expectedAppName string) error {
	if ctx.actualConfig.AppName != expectedAppName {
		return fmt.Errorf("expected app name '%s', got '%s'", expectedAppName, ctx.actualConfig.AppName)
	}
	return nil
}

func (ctx *BaseConfigBDDTestContext) theConfigurationShouldHaveEnvironment(expectedEnvironment string) error {
	if ctx.actualConfig.Environment != expectedEnvironment {
		return fmt.Errorf("expected environment '%s', got '%s'", expectedEnvironment, ctx.actualConfig.Environment)
	}
	return nil
}

func (ctx *BaseConfigBDDTestContext) theConfigurationShouldHaveDatabaseHost(expectedHost string) error {
	if ctx.actualConfig.Database.Host != expectedHost {
		return fmt.Errorf("expected database host '%s', got '%s'", expectedHost, ctx.actualConfig.Database.Host)
	}
	return nil
}

func (ctx *BaseConfigBDDTestContext) theConfigurationShouldHaveDatabasePassword(expectedPassword string) error {
	if ctx.actualConfig.Database.Password != expectedPassword {
		return fmt.Errorf("expected database password '%s', got '%s'", expectedPassword, ctx.actualConfig.Database.Password)
	}
	return nil
}

func (ctx *BaseConfigBDDTestContext) theFeatureShouldBeEnabled(featureName string) error {
	if enabled, exists := ctx.actualConfig.Features[featureName]; !exists || !enabled {
		return fmt.Errorf("expected feature '%s' to be enabled, but it was %v", featureName, enabled)
	}
	return nil
}

func (ctx *BaseConfigBDDTestContext) theFeatureShouldBeDisabled(featureName string) error {
	if enabled, exists := ctx.actualConfig.Features[featureName]; !exists || enabled {
		return fmt.Errorf("expected feature '%s' to be disabled, but it was %v", featureName, enabled)
	}
	return nil
}

func (ctx *BaseConfigBDDTestContext) iHaveBaseTenantConfigForTenant(tenantID string, configContent string) error {
	if ctx.tenantConfigs == nil {
		ctx.tenantConfigs = make(map[string]string)
	}
	ctx.tenantConfigs[tenantID] = configContent

	baseTenantPath := filepath.Join(ctx.configDir, "base", "tenants", tenantID+".yaml")
	if err := os.WriteFile(baseTenantPath, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("failed to write base tenant config: %w", err)
	}

	return nil
}

func (ctx *BaseConfigBDDTestContext) iHaveEnvironmentTenantConfigForTenant(tenantID string, configContent string) error {
	envTenantPath := filepath.Join(ctx.configDir, "environments", ctx.environment, "tenants", tenantID+".yaml")
	if err := os.WriteFile(envTenantPath, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("failed to write environment tenant config: %w", err)
	}

	return nil
}

func (ctx *BaseConfigBDDTestContext) theConfigurationLoadingShouldSucceed() error {
	if ctx.configError != nil {
		return fmt.Errorf("expected configuration loading to succeed, but got error: %v", ctx.configError)
	}
	return nil
}

// Cleanup function
func (ctx *BaseConfigBDDTestContext) cleanup() {
	// Reset base config settings
	BaseConfigSettings = BaseConfigOptions{}

	// Clean up temporary directories
	for _, dir := range ctx.tempDirs {
		os.RemoveAll(dir)
	}
}

// testBDDLogger implements a simple logger for BDD tests
type testBDDLogger struct{}

func (l *testBDDLogger) Debug(msg string, args ...any) {}
func (l *testBDDLogger) Info(msg string, args ...any)  {}
func (l *testBDDLogger) Warn(msg string, args ...any)  {}
func (l *testBDDLogger) Error(msg string, args ...any) {}

// Test scenarios initialization
func InitializeBaseConfigScenario(ctx *godog.ScenarioContext) {
	bddCtx := &BaseConfigBDDTestContext{}

	// Hook to clean up after each scenario
	ctx.After(func(ctx context.Context, sc *godog.Scenario, err error) (context.Context, error) {
		bddCtx.cleanup()
		return ctx, nil
	})

	ctx.Step(`^I have a base config structure with environment "([^"]*)"$`, bddCtx.iHaveABaseConfigStructureWithEnvironment)
	ctx.Step(`^the base config contains:$`, bddCtx.theBaseConfigContains)
	ctx.Step(`^the environment config contains:$`, bddCtx.theEnvironmentConfigContains)
	ctx.Step(`^I set the environment to "([^"]*)" and load the configuration$`, bddCtx.iSetTheEnvironmentToAndLoadTheConfiguration)
	ctx.Step(`^the configuration should have app name "([^"]*)"$`, bddCtx.theConfigurationShouldHaveAppName)
	ctx.Step(`^the configuration should have environment "([^"]*)"$`, bddCtx.theConfigurationShouldHaveEnvironment)
	ctx.Step(`^the configuration should have database host "([^"]*)"$`, bddCtx.theConfigurationShouldHaveDatabaseHost)
	ctx.Step(`^the configuration should have database password "([^"]*)"$`, bddCtx.theConfigurationShouldHaveDatabasePassword)
	ctx.Step(`^the feature "([^"]*)" should be enabled$`, bddCtx.theFeatureShouldBeEnabled)
	ctx.Step(`^the feature "([^"]*)" should be disabled$`, bddCtx.theFeatureShouldBeDisabled)
	ctx.Step(`^I have base tenant config for tenant "([^"]*)" containing:$`, bddCtx.iHaveBaseTenantConfigForTenant)
	ctx.Step(`^I have environment tenant config for tenant "([^"]*)" containing:$`, bddCtx.iHaveEnvironmentTenantConfigForTenant)
	ctx.Step(`^the configuration loading should succeed$`, bddCtx.theConfigurationLoadingShouldSucceed)
}

// Test runner
func TestBaseConfigBDDFeatures(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: InitializeBaseConfigScenario,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features/base_config.feature"},
			TestingT: t,
			Strict:   true,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run feature tests")
	}
}
