package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/GoCodeAlone/modular"
	"github.com/GoCodeAlone/modular/modules/database"

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
	fmt.Println("functionality to troubleshoot InstanceAware environment variable mapping.\n")

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

	// Create logger with DEBUG level to see verbose output
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create configuration provider that includes instance-aware environment feeder
	configProvider := modular.NewStdConfigProvider(&AppConfig{})
	
	// Add the instance-aware environment feeder for database configuration
	instanceFeeder := modular.NewInstanceAwareEnvFeeder(func(instanceKey string) string {
		return "DB_" + instanceKey + "_"
	})
	configProvider.AddFeeder(instanceFeeder)

	// Create application
	app := modular.NewStdApplication(configProvider, logger)

	// *** ENABLE VERBOSE CONFIGURATION DEBUGGING ***
	// This is the key feature - it enables detailed DEBUG logging throughout 
	// the configuration loading process
	fmt.Println("\n🔧 Enabling verbose configuration debugging...")
	app.SetVerboseConfig(true)

	// Register the database module 
	dbModule := database.NewModule()
	app.RegisterModule(dbModule)

	// Register database configuration
	// This will show verbose debugging of instance-aware env var mapping
	dbConfig := &database.Config{
		Default: "primary",
		Connections: map[string]database.ConnectionConfig{
			"primary":   {}, // Will be populated from DB_PRIMARY_* env vars
			"secondary": {}, // Will be populated from DB_SECONDARY_* env vars  
			"cache":     {}, // Will be populated from DB_CACHE_* env vars
		},
	}
	app.RegisterConfig("database", dbConfig)

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
	
	// Get database service and show connections
	if dbService, found := app.GetService("database.service"); found {
		if db, ok := dbService.(*database.DatabaseService); ok {
			for name := range db.GetConnectionNames() {
				fmt.Printf("  ✅ %s connection\n", name)
			}
		}
	}

	fmt.Println("\n▶️  Starting application...")
	if err := app.Start(); err != nil {
		fmt.Printf("❌ Failed to start application: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n🧪 Testing database connections:")
	if dbService, found := app.GetService("database.service"); found {
		if db, ok := dbService.(*database.DatabaseService); ok {
			for _, name := range db.GetConnectionNames() {
				if conn, err := db.GetConnection(name); err == nil {
					if err := conn.Ping(); err == nil {
						fmt.Printf("  ✅ %s connection: ping successful\n", name)
					} else {
						fmt.Printf("  ❌ %s connection: ping failed: %v\n", name, err)
					}
				} else {
					fmt.Printf("  ❌ %s connection: failed to get: %v\n", name, err)
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
package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/GoCodeAlone/modular"
	"github.com/GoCodeAlone/modular/modules/database"

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
	fmt.Println("functionality to troubleshoot InstanceAware environment variable mapping.\n")

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

	// Create logger with DEBUG level to see verbose output
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create configuration provider that includes instance-aware environment feeder
	configProvider := modular.NewStdConfigProvider(&AppConfig{})
	
	// Add the instance-aware environment feeder for database configuration
	instanceFeeder := modular.NewInstanceAwareEnvFeeder(func(instanceKey string) string {
		return "DB_" + instanceKey + "_"
	})
	configProvider.AddFeeder(instanceFeeder)

	// Create application
	app := modular.NewStdApplication(configProvider, logger)

	// *** ENABLE VERBOSE CONFIGURATION DEBUGGING ***
	// This is the key feature - it enables detailed DEBUG logging throughout 
	// the configuration loading process
	fmt.Println("\n🔧 Enabling verbose configuration debugging...")
	app.SetVerboseConfig(true)

	// Register the database module 
	dbModule := database.NewModule()
	app.RegisterModule(dbModule)

	// Register database configuration
	// This will show verbose debugging of instance-aware env var mapping
	dbConfig := &database.Config{
		Default: "primary",
		Connections: map[string]database.ConnectionConfig{
			"primary":   {}, // Will be populated from DB_PRIMARY_* env vars
			"secondary": {}, // Will be populated from DB_SECONDARY_* env vars  
			"cache":     {}, // Will be populated from DB_CACHE_* env vars
		},
	}
	app.RegisterConfig("database", dbConfig)

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
	
	// Get database service and show connections
	if dbService, found := app.GetService("database.service"); found {
		if db, ok := dbService.(*database.DatabaseService); ok {
			for name := range db.GetConnectionNames() {
				fmt.Printf("  ✅ %s connection\n", name)
			}
		}
	}

	fmt.Println("\n▶️  Starting application...")
	if err := app.Start(); err != nil {
		fmt.Printf("❌ Failed to start application: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n🧪 Testing database connections:")
	if dbService, found := app.GetService("database.service"); found {
		if db, ok := dbService.(*database.DatabaseService); ok {
			for _, name := range db.GetConnectionNames() {
				if conn, err := db.GetConnection(name); err == nil {
					if err := conn.Ping(); err == nil {
						fmt.Printf("  ✅ %s connection: ping successful\n", name)
					} else {
						fmt.Printf("  ❌ %s connection: ping failed: %v\n", name, err)
					}
				} else {
					fmt.Printf("  ❌ %s connection: failed to get: %v\n", name, err)
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
}
