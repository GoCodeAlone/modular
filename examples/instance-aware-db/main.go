package main

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/CrisisTextLine/modular"
	"github.com/CrisisTextLine/modular/feeders"
	"github.com/CrisisTextLine/modular/modules/database"

	// Import SQLite driver
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// This example demonstrates how to use instance-aware environment variable configuration
	// for multiple database connections

	fmt.Println("=== Instance-Aware Database Configuration Example ===")

	// Set up environment variables for multiple database connections
	// In a real application, these would be set externally
	envVars := map[string]string{
		"DB_PRIMARY_DRIVER":   "sqlite3",
		"DB_PRIMARY_DSN":      "./primary.db",
		"DB_SECONDARY_DRIVER": "sqlite3",
		"DB_SECONDARY_DSN":    "./secondary.db",
		"DB_CACHE_DRIVER":     "sqlite3",
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

	// Create application
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	app := modular.NewStdApplication(
		modular.NewStdConfigProvider(&AppConfig{}),
		logger,
	)
	if stdApp, ok := app.(*modular.StdApplication); ok {
		stdApp.SetConfigFeeders([]modular.Feeder{
			feeders.NewYamlFeeder("config.yaml"), // Load YAML config first
			feeders.NewEnvFeeder(),               // Then apply environment variables
		})
	}

	// Enable verbose configuration debugging
	if stdApp, ok := app.(*modular.StdApplication); ok {
		stdApp.SetVerboseConfig(true)
	}

	// Register the database module
	dbModule := database.NewModule()
	app.RegisterModule(dbModule)

	// Initialize the application
	fmt.Println("\nInitializing application...")
	if err := app.Init(); err != nil {
		fmt.Printf("Failed to initialize application: %v\n", err)
		os.Exit(1)
	}

	// Get the database module to demonstrate multiple connections
	var dbManager *database.Module
	if err := app.GetService("database.manager", &dbManager); err != nil {
		fmt.Printf("Failed to get database manager: %v\n", err)
		os.Exit(1)
	}

	// Debug: Check what connections are available before using them
	fmt.Printf("\nDEBUG: Database configuration after initialization:\n")
	if configProvider, err := app.GetConfigSection("database"); err == nil {
		if iaProvider, ok := configProvider.(*modular.InstanceAwareConfigProvider); ok {
			if cfg, ok := iaProvider.GetConfig().(*database.Config); ok {
				fmt.Printf("  Default: %s\n", cfg.Default)
				fmt.Printf("  Connections count: %d\n", len(cfg.Connections))
				for name, conn := range cfg.Connections {
					fmt.Printf("    %s: driver=%s, dsn=%s\n", name, conn.Driver, conn.DSN)
				}
			}
		}
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
	fmt.Println("6. YAML configuration defines structure, ENV vars provide values")

	// If running in CI, keep the process alive a bit longer for CI validation
	if os.Getenv("CI") == "true" || os.Getenv("GITHUB_ACTIONS") == "true" {
		fmt.Println("\n🤖 Detected CI environment - keeping process alive for validation...")
		time.Sleep(4 * time.Second)
		fmt.Println("✅ CI validation complete")
	}
}

// AppConfig demonstrates basic application configuration
type AppConfig struct {
	App AppSettings `yaml:"app"`
}

// AppSettings contains basic app configuration
type AppSettings struct {
	Name        string `yaml:"name" env:"APP_NAME" default:"Instance-Aware DB Example"`
	Environment string `yaml:"environment" env:"ENVIRONMENT" default:"development"`

	Database database.Config `yaml:"database" desc:"Database configuration"`
}

// Validate implements basic validation
func (c *AppConfig) Validate() error {
	// Add any validation logic here
	return nil
}
