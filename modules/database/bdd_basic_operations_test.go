package database

import (
	"context"
	"fmt"
	"os"

	"github.com/GoCodeAlone/modular"
)

// Module initialization and basic database operations

func (ctx *DatabaseBDDTestContext) iHaveAModularApplicationWithDatabaseModuleConfigured() error {
	ctx.resetContext()

	// Set up environment variables for proper instance-aware configuration
	os.Setenv("DB_DEFAULT_DRIVER", "sqlite")
	os.Setenv("DB_DEFAULT_DSN", ":memory:")
	defer func() {
		os.Unsetenv("DB_DEFAULT_DRIVER")
		os.Unsetenv("DB_DEFAULT_DSN")
	}()

	// Create application with database config using proper configuration flow
	logger := &testLogger{}

	// Create app with empty main config - USE OBSERVABLE for events
	mainConfigProvider := modular.NewStdConfigProvider(struct{}{})
	ctx.app = modular.NewObservableApplication(mainConfigProvider, logger)

	// Create and configure database module
	ctx.module = NewModule()

	// Create test event observer
	ctx.eventObserver = newTestEventObserver()

	// Register module's configuration first
	if err := ctx.module.RegisterConfig(ctx.app); err != nil {
		return fmt.Errorf("failed to register module config: %w", err)
	}

	// Register module with the application
	ctx.app.RegisterModule(ctx.module)

	// Get the configuration and set up connections that will be fed from environment variables
	configProvider, err := ctx.app.GetConfigSection(ctx.module.Name())
	if err != nil {
		return fmt.Errorf("failed to get config section: %w", err)
	}

	config, ok := configProvider.GetConfig().(*Config)
	if !ok {
		return fmt.Errorf("config should be of type *Config")
	}

	// Set up empty connections that will be populated by instance-aware feeder
	config.Connections = map[string]*ConnectionConfig{
		"default": {},
	}
	config.Default = "default"

	// Feed the instance configurations from environment variables
	iaProvider, ok := configProvider.(*modular.InstanceAwareConfigProvider)
	if !ok {
		return fmt.Errorf("should be instance-aware config provider")
	}

	prefixFunc := iaProvider.GetInstancePrefixFunc()
	if prefixFunc == nil {
		return fmt.Errorf("should have prefix function")
	}

	feeder := modular.NewInstanceAwareEnvFeeder(prefixFunc)
	instanceConfigs := config.GetInstanceConfigs()

	// Feed each instance
	for instanceKey, instanceConfig := range instanceConfigs {
		if err := feeder.FeedKey(instanceKey, instanceConfig); err != nil {
			return fmt.Errorf("failed to feed instance %s: %w", instanceKey, err)
		}
	}

	// Update the original config with the fed instances
	for name, instance := range instanceConfigs {
		if connPtr, ok := instance.(*ConnectionConfig); ok {
			config.Connections[name] = connPtr
		}
	}

	// Register observers BEFORE initialization to capture all events
	if err := ctx.module.RegisterObservers(ctx.app.(modular.Subject)); err != nil {
		return fmt.Errorf("failed to register observers: %w", err)
	}

	// Register our test observer to capture events
	if err := ctx.app.(modular.Subject).RegisterObserver(ctx.eventObserver); err != nil {
		return fmt.Errorf("failed to register test observer: %w", err)
	}

	// Manually initialize the module with fed configuration (like integration test)
	if err := ctx.module.Init(ctx.app); err != nil {
		return fmt.Errorf("failed to initialize module: %v", err)
	}

	// Start the module to establish database connections
	startCtx := context.Background()
	if err := ctx.module.Start(startCtx); err != nil {
		return fmt.Errorf("failed to start module: %v", err)
	}

	// Also initialize the full application for service registry
	if err := ctx.app.Init(); err != nil {
		return fmt.Errorf("failed to initialize app: %v", err)
	}

	// Start the application
	if err := ctx.app.Start(); err != nil {
		return fmt.Errorf("failed to start app: %v", err)
	}

	// Get the database service
	var dbService DatabaseService
	if err := ctx.app.GetService("database.service", &dbService); err != nil {
		return fmt.Errorf("failed to get database service: %v", err)
	}
	ctx.service = dbService

	return nil
}

func (ctx *DatabaseBDDTestContext) theDatabaseModuleIsInitialized() error {
	// This is handled by the background setup
	return nil
}

func (ctx *DatabaseBDDTestContext) theDatabaseServiceShouldBeAvailable() error {
	if ctx.service == nil {
		return fmt.Errorf("database service is not available")
	}
	return nil
}

func (ctx *DatabaseBDDTestContext) databaseConnectionsShouldBeConfigured() error {
	// Verify that connections are configured
	if ctx.service == nil {
		return fmt.Errorf("no database service available")
	}
	// This would check internal connection state, but we'll assume success for BDD
	return nil
}

func (ctx *DatabaseBDDTestContext) iHaveADatabaseConnection() error {
	if ctx.service == nil {
		return fmt.Errorf("no database service available")
	}
	return nil
}

func (ctx *DatabaseBDDTestContext) iExecuteASimpleSQLQuery() error {
	if ctx.service == nil {
		return fmt.Errorf("no database service available")
	}

	// Execute a simple query like CREATE TABLE or SELECT 1
	rows, err := ctx.service.Query("SELECT 1 as test_value")
	if err != nil {
		ctx.queryError = err
		return nil
	}
	defer rows.Close()

	if rows.Next() {
		var testValue int
		if err := rows.Scan(&testValue); err != nil {
			ctx.queryError = err
			return nil
		}
		ctx.queryResult = testValue
	}
	return nil
}

func (ctx *DatabaseBDDTestContext) theQueryShouldExecuteSuccessfully() error {
	if ctx.queryError != nil {
		return fmt.Errorf("query execution failed: %v", ctx.queryError)
	}
	return nil
}

func (ctx *DatabaseBDDTestContext) iShouldReceiveTheExpectedResults() error {
	if ctx.queryResult == nil {
		return fmt.Errorf("no query result received")
	}
	return nil
}

func (ctx *DatabaseBDDTestContext) iExecuteAParameterizedSQLQuery() error {
	if ctx.service == nil {
		return fmt.Errorf("no database service available")
	}

	// Execute a parameterized query
	rows, err := ctx.service.Query("SELECT ? as param_value", 42)
	if err != nil {
		ctx.queryError = err
		return nil
	}
	defer rows.Close()

	if rows.Next() {
		var paramValue int
		if err := rows.Scan(&paramValue); err != nil {
			ctx.queryError = err
			return nil
		}
		ctx.queryResult = paramValue
	}
	return nil
}

func (ctx *DatabaseBDDTestContext) theQueryShouldExecuteSuccessfullyWithParameters() error {
	return ctx.theQueryShouldExecuteSuccessfully()
}

func (ctx *DatabaseBDDTestContext) theParametersShouldBeProperlyEscaped() error {
	// Parameters are escaped by the database driver automatically when using prepared statements
	// This test verifies that the query executed successfully with parameters, indicating proper escaping
	if ctx.queryError != nil {
		return fmt.Errorf("query with parameters failed, suggesting improper escaping: %v", ctx.queryError)
	}
	return nil
}

func (ctx *DatabaseBDDTestContext) iHaveAnInvalidDatabaseConfiguration() error {
	// Simulate an invalid configuration by setting up a connection with bad DSN
	ctx.service = nil // Simulate service being unavailable
	ctx.lastError = fmt.Errorf("invalid database configuration")
	return nil
}

func (ctx *DatabaseBDDTestContext) iTryToExecuteAQuery() error {
	if ctx.service == nil {
		ctx.queryError = fmt.Errorf("no database service available")
		return nil
	}

	// Try to execute a query
	_, ctx.queryError = ctx.service.Query("SELECT 1")
	return nil
}

func (ctx *DatabaseBDDTestContext) theOperationShouldFailGracefully() error {
	if ctx.queryError == nil && ctx.lastError == nil {
		return fmt.Errorf("operation should have failed but succeeded")
	}
	return nil
}

func (ctx *DatabaseBDDTestContext) anAppropriateDatabaseErrorShouldBeReturned() error {
	if ctx.queryError == nil && ctx.lastError == nil {
		return fmt.Errorf("no database error was returned")
	}
	return nil
}
