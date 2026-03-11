package modular

import (
	"context"
	"errors"
	"os"

	"github.com/cucumber/godog"
)

// Static errors for configuration BDD tests
var (
	errPortOutOfRange                = errors.New("port must be between 1 and 65535")
	errNameCannotBeEmpty             = errors.New("name cannot be empty")
	errDatabaseDriverRequired        = errors.New("database driver is required")
	errModuleNotConfigurable         = errors.New("module is not configurable")
	errNoEnvironmentVariablesSet     = errors.New("no environment variables set")
	errNoYAMLFileAvailable           = errors.New("no YAML file available")
	errNoYAMLFileCreated             = errors.New("no YAML file was created")
	errNoJSONFileAvailable           = errors.New("no JSON file available")
	errNoJSONFileCreated             = errors.New("no JSON file was created")
	errNoConfigurationData           = errors.New("no configuration data available")
	errExpectedNoValidationErrors    = errors.New("expected no validation errors")
	errValidationShouldHaveFailed    = errors.New("validation should have failed but passed")
	errNoValidationErrorReported     = errors.New("no validation error reported")
	errValidationErrorMessageEmpty   = errors.New("validation error message is empty")
	errRequiredFieldMissing          = errors.New("required configuration field 'database.driver' is missing")
	errConfigLoadingShouldHaveFailed = errors.New("configuration loading should have failed")
	errNoErrorToCheckConfig          = errors.New("no error to check")
	errErrorMessageEmpty             = errors.New("error message is empty")
	errNoFieldsTracked               = errors.New("no fields were tracked")
	errFieldNotTracked               = errors.New("field was not tracked")
	errFieldSourceMismatch           = errors.New("field expected source mismatch")
)

// Configuration BDD Test Context
type ConfigBDDTestContext struct {
	app              Application
	logger           Logger
	module           Module
	configError      error
	validationError  error
	yamlFile         string
	jsonFile         string
	environmentVars  map[string]string
	originalEnvVars  map[string]string
	configData       any
	isValid          bool
	validationErrors []string
	fieldTracker     *TestFieldTracker
}

// Test configuration structures
type TestModuleConfig struct {
	Name     string `yaml:"name" json:"name" default:"test-module" required:"true" desc:"Module name"`
	Port     int    `yaml:"port" json:"port" default:"8080" desc:"Port number"`
	Enabled  bool   `yaml:"enabled" json:"enabled" default:"true" desc:"Whether module is enabled"`
	Host     string `yaml:"host" json:"host" default:"localhost" desc:"Host address"`
	Database struct {
		Driver string `yaml:"driver" json:"driver" required:"true" desc:"Database driver"`
		DSN    string `yaml:"dsn" json:"dsn" required:"true" desc:"Database connection string"`
	} `yaml:"database" json:"database" desc:"Database configuration"`
	Optional string `yaml:"optional" json:"optional" desc:"Optional field"`
}

// ConfigValidator implementation for TestModuleConfig
func (c *TestModuleConfig) ValidateConfig() error {
	if c.Port < 1 || c.Port > 65535 {
		return errPortOutOfRange
	}
	if c.Name == "" {
		return errNameCannotBeEmpty
	}
	if c.Database.Driver == "" {
		return errDatabaseDriverRequired
	}
	return nil
}

type TestConfigurableModule struct {
	name   string
	config *TestModuleConfig
}

func (m *TestConfigurableModule) Name() string {
	return m.name
}

func (m *TestConfigurableModule) Init(app Application) error {
	return nil
}

func (m *TestConfigurableModule) RegisterConfig(app Application) error {
	m.config = &TestModuleConfig{}
	cp := NewStdConfigProvider(m.config)
	app.RegisterConfigSection(m.name, cp)
	return nil
}

// Test field tracker for configuration tracking
type TestFieldTracker struct {
	fields map[string]string
}

func (t *TestFieldTracker) TrackField(fieldPath, source string) {
	if t.fields == nil {
		t.fields = make(map[string]string)
	}
	t.fields[fieldPath] = source
}

func (t *TestFieldTracker) GetFieldSource(fieldPath string) string {
	return t.fields[fieldPath]
}

func (t *TestFieldTracker) GetTrackedFields() map[string]string {
	return t.fields
}

// Step definitions for configuration BDD tests

func (ctx *ConfigBDDTestContext) resetContext() {
	ctx.app = nil
	ctx.logger = nil
	ctx.module = nil
	ctx.configError = nil
	ctx.validationError = nil
	ctx.yamlFile = ""
	ctx.jsonFile = ""
	ctx.environmentVars = make(map[string]string)
	ctx.originalEnvVars = make(map[string]string)
	ctx.configData = nil
	ctx.isValid = false
	ctx.validationErrors = nil
	ctx.fieldTracker = &TestFieldTracker{}
}

func (ctx *ConfigBDDTestContext) iHaveANewModularApplication() error {
	ctx.resetContext()
	return nil
}

func (ctx *ConfigBDDTestContext) iHaveALoggerConfigured() error {
	ctx.logger = &BDDTestLogger{}
	cp := NewStdConfigProvider(struct{}{})
	ctx.app = NewStdApplication(cp, ctx.logger)
	return nil
}

func (ctx *ConfigBDDTestContext) iHaveAModuleWithConfigurationRequirements() error {
	ctx.module = &TestConfigurableModule{name: "test-config-module"}
	return nil
}

// Clean up temp files and environment variables
func (ctx *ConfigBDDTestContext) cleanup() {
	// Clean up temp files
	if ctx.yamlFile != "" {
		os.Remove(ctx.yamlFile)
	}
	if ctx.jsonFile != "" {
		os.Remove(ctx.jsonFile)
	}

	// Restore original environment variables
	for key, originalValue := range ctx.originalEnvVars {
		if originalValue == "" {
			os.Unsetenv(key)
		} else {
			os.Setenv(key, originalValue)
		}
	}
}

// InitializeConfigurationScenario initializes the configuration BDD test scenario
// InitializeConfigurationScenario initializes the configuration BDD test scenario
func InitializeConfigurationScenario(ctx *godog.ScenarioContext, testCtx *ConfigBDDTestContext) {

	// Reset context before each scenario
	ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		testCtx.resetContext()
		return ctx, nil
	})

	// Clean up after each scenario
	ctx.After(func(ctx context.Context, sc *godog.Scenario, err error) (context.Context, error) {
		testCtx.cleanup()
		return ctx, nil
	})

	// Background steps
	ctx.Step(`^I have a new modular application$`, testCtx.iHaveANewModularApplication)
	ctx.Step(`^I have a logger configured$`, testCtx.iHaveALoggerConfigured)

	// Configuration registration steps
	ctx.Step(`^I have a module with configuration requirements$`, testCtx.iHaveAModuleWithConfigurationRequirements)
}
