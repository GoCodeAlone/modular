package main

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/CrisisTextLine/modular"
	"github.com/CrisisTextLine/modular/feeders"
	"github.com/CrisisTextLine/modular/modules/database"

	// Import SQLite driver for database connections
	_ "modernc.org/sqlite"
)

type AppConfig struct {
	AppName  string `yaml:"appName" env:"APP_NAME" desc:"Application name"`
	Debug    bool   `yaml:"debug" env:"APP_DEBUG" desc:"Enable debug mode"`
	LogLevel string `yaml:"logLevel" env:"APP_LOG_LEVEL" desc:"Log level"`
}

func main() {
	fmt.Println("=== Verbose Configuration Debug Example ===")
	fmt.Println("This example demonstrates the built-in verbose configuration debugging")
	fmt.Println("functionality to troubleshoot InstanceAware environment variable mapping.")

	// Set up environment variables for both app and database configuration
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

	fmt.Println("🔧 Setting up environment variables:")
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

	// Configure feeders with verbose environment feeder
	envFeeder := feeders.NewEnvFeeder()

	modular.ConfigFeeders = []modular.Feeder{
		envFeeder, // Use environment feeder with verbose support when enabled
		// Instance-aware feeding is handled automatically by the database module
	}

	// Create logger with DEBUG level to see verbose output
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create application
	app := modular.NewStdApplication(
		modular.NewStdConfigProvider(&AppConfig{}),
		logger,
	)

	// *** ENABLE VERBOSE CONFIGURATION DEBUGGING ***
	// This is the key feature - it enables detailed DEBUG logging throughout
	// the configuration loading process
	fmt.Println("\n🔧 Enabling verbose configuration debugging...")
	app.SetVerboseConfig(true)

	// Register the database module - it will automatically handle instance-aware configuration
	dbModule := database.NewModule()
	app.RegisterModule(dbModule)

	// Initialize the application - this will trigger verbose config logging
	fmt.Println("\n🚀 Initializing application with verbose debugging...")
	if err := app.Init(); err != nil {
		fmt.Printf("❌ Failed to initialize application: %v\n", err)
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

	fmt.Println("\n🗄️  Database connections loaded:")

	// Get database module to show connections
	var dbManager *database.Module
	if err := app.GetService("database.manager", &dbManager); err != nil {
		fmt.Printf("❌ Failed to get database manager: %v\n", err)
	} else {
		connections := dbManager.GetConnections()
		for _, connName := range connections {
			fmt.Printf("  ✅ %s connection\n", connName)
		}
	}

	fmt.Println("\n▶️  Starting application...")
	if err := app.Start(); err != nil {
		fmt.Printf("❌ Failed to start application: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n🧪 Testing database connections:")
	if dbManager != nil {
		connections := dbManager.GetConnections()
		for _, connName := range connections {
			if db, exists := dbManager.GetConnection(connName); exists {
				if err := db.Ping(); err != nil {
					fmt.Printf("  ❌ %s: Failed to ping - %v\n", connName, err)
				} else {
					fmt.Printf("  ✅ %s: Connection healthy\n", connName)
				}
			}
		}
	}

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

	// If running in CI, keep the process alive a bit longer for CI validation
	if os.Getenv("CI") == "true" || os.Getenv("GITHUB_ACTIONS") == "true" {
		fmt.Println("\n🤖 Detected CI environment - keeping process alive for validation...")
		time.Sleep(4 * time.Second)
		fmt.Println("✅ CI validation complete")
	}
}
