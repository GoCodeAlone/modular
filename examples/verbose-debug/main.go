package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/GoCodeAlone/modular"
	"github.com/GoCodeAlone/modular/feeders"
	"github.com/GoCodeAlone/modular/modules/database"
)

func main() {
	// This example demonstrates verbose debug logging for configuration processing
	// to help troubleshoot InstanceAware env mapping issues

	fmt.Println("=== Verbose Configuration Debug Example ===")

	// Set up environment variables for database configuration
	envVars := map[string]string{
		"APP_NAME":               "Verbose Debug Example",
		"APP_DEBUG":              "true",
		"APP_LOG_LEVEL":          "debug",
		"DB_PRIMARY_DRIVER":      "sqlite",
		"DB_PRIMARY_DSN":         "./primary.db",
		"DB_PRIMARY_MAX_CONNS":   "10",
		"DB_SECONDARY_DRIVER":    "sqlite",
		"DB_SECONDARY_DSN":       "./secondary.db",
		"DB_SECONDARY_MAX_CONNS": "5",
		"DB_CACHE_DRIVER":        "sqlite",
		"DB_CACHE_DSN":           ":memory:",
		"DB_CACHE_MAX_CONNS":     "3",
	}

	fmt.Println("Setting up environment variables:")
	for key, value := range envVars {
		os.Setenv(key, value)
		fmt.Printf("  %s=%s\n", key, value)
	}

	// Clean up environment variables at the end
	defer func() {
		for key := range envVars {
			os.Unsetenv(key)
		}
	}()

	// Configure feeders with verbose-aware environment feeders
	modular.ConfigFeeders = []modular.Feeder{
		feeders.NewVerboseEnvFeeder(), // Use verbose environment feeder
	}

	// Create logger with DEBUG level to see verbose output
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create application with app config
	app := modular.NewStdApplication(
		modular.NewStdConfigProvider(&AppConfig{}),
		logger,
	)

	// ENABLE VERBOSE CONFIGURATION DEBUGGING
	fmt.Println("\n🔧 Enabling verbose configuration debugging...")
	app.SetVerboseConfig(true)

	// Register the database module to demonstrate instance-aware configuration
	dbModule := database.NewModule()
	app.RegisterModule(dbModule)

	// Initialize the application - this will trigger verbose config logging
	fmt.Println("\n🚀 Initializing application with verbose debugging...")
	if err := app.Init(); err != nil {
		fmt.Printf("❌ Failed to initialize application: %v\n", err)
		os.Exit(1)
	}

	// After init, configure the database connections
	if err := setupDatabaseConnections(app, dbModule); err != nil {
		fmt.Printf("❌ Failed to setup database connections: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n📊 Configuration Results:")

	// Show the loaded app configuration
	appConfigProvider := app.ConfigProvider()
	if appConfig, ok := appConfigProvider.GetConfig().(*AppConfig); ok {
		fmt.Printf("  App Name: %s\n", appConfig.AppName)
		fmt.Printf("  Debug: %t\n", appConfig.Debug)
		fmt.Printf("  Log Level: %s\n", appConfig.LogLevel)
	}

	// Get the database manager to show loaded connections
	var dbManager *database.Module
	if err := app.GetService("database.manager", &dbManager); err != nil {
		fmt.Printf("❌ Failed to get database manager: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n🗄️  Database connections loaded:")
	connections := dbManager.GetConnections()
	for _, connName := range connections {
		fmt.Printf("  - %s\n", connName)
	}

	// Start the application
	fmt.Println("\n▶️  Starting application...")
	if err := app.Start(); err != nil {
		fmt.Printf("❌ Failed to start application: %v\n", err)
		os.Exit(1)
	}

	// Test the database connections
	fmt.Println("\n🧪 Testing database connections:")
	for _, connName := range connections {
		if db, exists := dbManager.GetConnection(connName); exists {
			if err := db.Ping(); err != nil {
				fmt.Printf("  ❌ %s: Failed to ping - %v\n", connName, err)
			} else {
				fmt.Printf("  ✅ %s: Connection healthy\n", connName)
			}
		}
	}

	// Stop the application
	fmt.Println("\n⏹️  Stopping application...")
	if err := app.Stop(); err != nil {
		fmt.Printf("❌ Failed to stop application: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n✅ Application stopped successfully")
	fmt.Println("\n=== Verbose Debug Benefits ===")
	fmt.Println("1. See exactly which configuration sections are being processed")
	fmt.Println("2. Track which environment variables are being looked up")
	fmt.Println("3. Monitor which configuration keys are being evaluated")
	fmt.Println("4. Debug instance-aware environment variable mapping")
	fmt.Println("5. Troubleshoot configuration loading issues step by step")
	fmt.Println("\nUse app.SetVerboseConfig(true) to enable this debugging in your application!")
}

// AppConfig demonstrates application-level configuration with verbose debugging
type AppConfig struct {
	AppName  string `yaml:"appName" env:"APP_NAME" default:"Verbose Debug App"`
	Debug    bool   `yaml:"debug" env:"APP_DEBUG" default:"false"`
	LogLevel string `yaml:"logLevel" env:"APP_LOG_LEVEL" default:"info"`
}

// Validate implements basic validation
func (c *AppConfig) Validate() error {
	return nil
}

// setupDatabaseConnections configures the database connections for instance-aware loading
func setupDatabaseConnections(app modular.Application, dbModule *database.Module) error {
	// Get the database configuration section
	configProvider, err := app.GetConfigSection(dbModule.Name())
	if err != nil {
		return fmt.Errorf("failed to get database config section: %w", err)
	}

	config, ok := configProvider.GetConfig().(*database.Config)
	if !ok {
		return fmt.Errorf("invalid database config type")
	}

	// Set up the connections that should be configured from environment variables
	config.Connections = map[string]database.ConnectionConfig{
		"primary":   {}, // Will be populated from DB_PRIMARY_* env vars
		"secondary": {}, // Will be populated from DB_SECONDARY_* env vars
		"cache":     {}, // Will be populated from DB_CACHE_* env vars
	}
	config.Default = "primary"

	// Apply instance-aware configuration with verbose debugging
	if iaProvider, ok := configProvider.(*modular.InstanceAwareConfigProvider); ok {
		prefixFunc := iaProvider.GetInstancePrefixFunc()
		if prefixFunc != nil {
			// Create instance-aware feeder with verbose debugging
			feeder := modular.NewInstanceAwareEnvFeeder(prefixFunc)

			// Enable verbose debugging on the feeder if app has it enabled
			if app.IsVerboseConfig() {
				if verboseFeeder, ok := feeder.(modular.VerboseAwareFeeder); ok {
					verboseFeeder.SetVerboseDebug(true, app.Logger())
				}
			}

			instanceConfigs := config.GetInstanceConfigs()

			// Feed each instance with environment variables
			for instanceKey, instanceConfig := range instanceConfigs {
				if err := feeder.FeedKey(instanceKey, instanceConfig); err != nil {
					return fmt.Errorf("failed to feed instance config for %s: %w", instanceKey, err)
				}
			}

			// Update the original config with the fed instances
			for name, instance := range instanceConfigs {
				if connPtr, ok := instance.(*database.ConnectionConfig); ok {
					config.Connections[name] = *connPtr
				}
			}
		}
	}

	return nil
}
