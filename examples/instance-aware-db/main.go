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
	// This example demonstrates how to use instance-aware environment variable configuration
	// for multiple database connections

	fmt.Println("=== Instance-Aware Database Configuration Example ===")

	// Set up environment variables for multiple database connections
	// In a real application, these would be set externally
	envVars := map[string]string{
		"DB_PRIMARY_DRIVER":   "sqlite",
		"DB_PRIMARY_DSN":      "./primary.db",
		"DB_SECONDARY_DRIVER": "sqlite",
		"DB_SECONDARY_DSN":    "./secondary.db",
		"DB_CACHE_DRIVER":     "sqlite",
		"DB_CACHE_DSN":        ":memory:",
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

	// Configure feeders - just basic env feeding since we don't need a YAML file
	modular.ConfigFeeders = []modular.Feeder{
		feeders.NewEnvFeeder(), // Regular env feeding for app config
		// Instance-aware feeding is handled automatically by the database module
	}

	// Create application
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	app := modular.NewStdApplication(
		modular.NewStdConfigProvider(&AppConfig{}),
		logger,
	)

	// Register the database module
	dbModule := database.NewModule()
	app.RegisterModule(dbModule)

	// Initialize the application
	fmt.Println("\nInitializing application...")
	if err := app.Init(); err != nil {
		fmt.Printf("Failed to initialize application: %v\n", err)
		os.Exit(1)
	}

	// After init, configure the database connections that should be loaded from env vars
	if err := setupDatabaseConnections(app, dbModule); err != nil {
		fmt.Printf("Failed to setup database connections: %v\n", err)
		os.Exit(1)
	}

	// Get the database module to demonstrate multiple connections
	var dbManager *database.Module
	if err := app.GetService("database.manager", &dbManager); err != nil {
		fmt.Printf("Failed to get database manager: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\nAvailable database connections:")
	connections := dbManager.GetConnections()
	for _, connName := range connections {
		fmt.Printf("  - %s\n", connName)

		if db, exists := dbManager.GetConnection(connName); exists {
			if err := db.Ping(); err != nil {
				fmt.Printf("    ❌ Failed to ping %s: %v\n", connName, err)
			} else {
				fmt.Printf("    ✅ %s connection is healthy\n", connName)
			}
		}
	}

	// Start the application
	fmt.Println("\nStarting application...")
	if err := app.Start(); err != nil {
		fmt.Printf("Failed to start application: %v\n", err)
		os.Exit(1)
	}

	// Demonstrate using different connections
	fmt.Println("\nDemonstrating multiple database connections:")

	// Use primary connection
	if primaryDB, exists := dbManager.GetConnection("primary"); exists {
		fmt.Println("Using primary database...")
		if _, err := primaryDB.Exec("CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
			fmt.Printf("  ❌ Failed to create table in primary: %v\n", err)
		} else {
			fmt.Println("  ✅ Created table in primary database")
		}
	}

	// Use secondary connection
	if secondaryDB, exists := dbManager.GetConnection("secondary"); exists {
		fmt.Println("Using secondary database...")
		if _, err := secondaryDB.Exec("CREATE TABLE IF NOT EXISTS logs (id INTEGER PRIMARY KEY, message TEXT)"); err != nil {
			fmt.Printf("  ❌ Failed to create table in secondary: %v\n", err)
		} else {
			fmt.Println("  ✅ Created table in secondary database")
		}
	}

	// Use cache connection
	if cacheDB, exists := dbManager.GetConnection("cache"); exists {
		fmt.Println("Using cache database...")
		if _, err := cacheDB.Exec("CREATE TABLE IF NOT EXISTS cache (key TEXT PRIMARY KEY, value TEXT)"); err != nil {
			fmt.Printf("  ❌ Failed to create table in cache: %v\n", err)
		} else {
			fmt.Println("  ✅ Created table in cache database")
		}
	}

	// Stop the application
	fmt.Println("\nStopping application...")
	if err := app.Stop(); err != nil {
		fmt.Printf("Failed to stop application: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Application stopped successfully")
	fmt.Println("\n=== Key Benefits of Instance-Aware Configuration ===")
	fmt.Println("1. Multiple database connections with separate environment variables")
	fmt.Println("2. Consistent naming convention (DB_<instance>_<field>)")
	fmt.Println("3. Automatic configuration from environment variables")
	fmt.Println("4. No conflicts between different database instances")
	fmt.Println("5. Easy to configure in different environments (dev, test, prod)")
}

// AppConfig demonstrates basic application configuration
type AppConfig struct {
	AppName     string `yaml:"appName" env:"APP_NAME" default:"Instance-Aware DB Example"`
	Environment string `yaml:"environment" env:"ENVIRONMENT" default:"development"`
}

// Validate implements basic validation
func (c *AppConfig) Validate() error {
	// Add any validation logic here
	return nil
}

// setupDatabaseConnections configures the database connections that should be loaded from environment variables
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

	// Apply instance-aware configuration
	if iaProvider, ok := configProvider.(*modular.InstanceAwareConfigProvider); ok {
		prefixFunc := iaProvider.GetInstancePrefixFunc()
		if prefixFunc != nil {
			feeder := modular.NewInstanceAwareEnvFeeder(prefixFunc)
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
