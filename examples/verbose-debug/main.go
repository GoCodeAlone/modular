package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/GoCodeAlone/modular"
	"github.com/GoCodeAlone/modular/feeders"
	"github.com/GoCodeAlone/modular/modules/database"

	// Import SQLite driver for database connections
	_ "modernc.org/sqlite"
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
	// We need a composite feeder that can handle both regular and instance-aware feeding
	verboseFeeder := feeders.NewVerboseEnvFeeder()
	instanceFeeder := newVerboseInstanceFeeder()

	modular.ConfigFeeders = []modular.Feeder{
		verboseFeeder,  // Use verbose environment feeder for app config
		instanceFeeder, // Custom feeder for instance-aware configs with verbose support
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

	// Enable verbose debugging on our custom instance feeder
	if verboseAware, ok := instanceFeeder.(*VerboseInstanceFeeder); ok {
		verboseAware.SetVerboseDebug(true, logger)
	}

	// Create a custom database module that has predefined connections
	dbModule := createPreConfiguredDatabaseModule()
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

// newVerboseInstanceFeeder creates a verbose-aware instance feeder
// This feeder can handle instance-aware configurations with verbose debugging
func newVerboseInstanceFeeder() modular.ComplexFeeder {
	return &VerboseInstanceFeeder{}
}

// VerboseInstanceFeeder is a custom feeder that handles instance-aware configs with verbose debugging
type VerboseInstanceFeeder struct {
	verboseEnabled bool
	logger         interface{ Debug(msg string, args ...any) }
}

// Feed implements the basic Feeder interface (no-op for complex feeders)
func (f *VerboseInstanceFeeder) Feed(structure interface{}) error {
	// Basic feeding is handled by other feeders, this is for complex feeding only
	return nil
}

// FeedKey implements the ComplexFeeder interface for instance-aware feeding
func (f *VerboseInstanceFeeder) FeedKey(key string, target interface{}) error {
	if f.verboseEnabled && f.logger != nil {
		f.logger.Debug("VerboseInstanceFeeder: Processing configuration key", "key", key)
	}

	// For database configuration, we need to handle it specially because
	// the database module's GetInstanceConfigs creates copies instead of returning
	// pointers to the original configurations
	if key == "database" {
		if dbConfig, ok := target.(*database.Config); ok {
			if f.verboseEnabled && f.logger != nil {
				f.logger.Debug("VerboseInstanceFeeder: Found database configuration", "key", key)
			}

			// Ensure connections map exists
			if dbConfig.Connections == nil {
				dbConfig.Connections = make(map[string]database.ConnectionConfig)
			}

			// If no instances, create the expected instances
			if len(dbConfig.Connections) == 0 {
				dbConfig.Connections["primary"] = database.ConnectionConfig{}
				dbConfig.Connections["secondary"] = database.ConnectionConfig{}
				dbConfig.Connections["cache"] = database.ConnectionConfig{}
				dbConfig.Default = "primary"
			}

			if f.verboseEnabled && f.logger != nil {
				f.logger.Debug("VerboseInstanceFeeder: Processing database connections", "count", len(dbConfig.Connections))
			}

			// Create an instance-aware feeder with the database prefix function
			prefixFunc := func(instanceKey string) string {
				return "DB_" + instanceKey + "_"
			}
			instanceFeeder := modular.NewInstanceAwareEnvFeeder(prefixFunc)

			// Enable verbose debugging on the instance feeder
			if f.verboseEnabled && f.logger != nil {
				if verboseAware, ok := instanceFeeder.(modular.VerboseAwareFeeder); ok {
					verboseAware.SetVerboseDebug(true, f.logger)
				}
			}

			// Process each connection directly
			updatedConnections := make(map[string]database.ConnectionConfig)
			for connName, conn := range dbConfig.Connections {
				if f.verboseEnabled && f.logger != nil {
					f.logger.Debug("VerboseInstanceFeeder: Processing database connection", "key", key, "connectionName", connName)
				}

				// Create a copy of the connection config that we can modify
				connCopy := conn
				
				// Feed the instance configuration
				if err := instanceFeeder.FeedKey(connName, &connCopy); err != nil {
					if f.verboseEnabled && f.logger != nil {
						f.logger.Debug("VerboseInstanceFeeder: Failed to feed connection", "key", key, "connectionName", connName, "error", err)
					}
					return fmt.Errorf("failed to feed connection %s for key %s: %w", connName, key, err)
				}

				// Store the updated connection
				updatedConnections[connName] = connCopy

				if f.verboseEnabled && f.logger != nil {
					f.logger.Debug("VerboseInstanceFeeder: Successfully fed connection",
						"key", key, "connectionName", connName,
						"driver", connCopy.Driver, "dsn", connCopy.DSN)
				}
			}

			// Update the original configuration with the fed values
			dbConfig.Connections = updatedConnections

			if f.verboseEnabled && f.logger != nil {
				f.logger.Debug("VerboseInstanceFeeder: Completed database configuration feeding", "key", key)
			}

			return nil
		}
	}

	// For non-database configurations, use the standard instance-aware approach
	if instanceConfig, ok := target.(modular.InstanceAwareConfigSupport); ok {
		if f.verboseEnabled && f.logger != nil {
			f.logger.Debug("VerboseInstanceFeeder: Found instance-aware configuration", "key", key)
		}

		// Get instance configurations
		instances := instanceConfig.GetInstanceConfigs()
		if f.verboseEnabled && f.logger != nil {
			f.logger.Debug("VerboseInstanceFeeder: Retrieved instance configurations", "key", key, "instanceCount", len(instances))
		}

		// Find the associated prefix function
		var prefixFunc func(string) string
		prefixFunc = func(instanceKey string) string {
			return strings.ToUpper(key) + "_" + instanceKey + "_"
		}

		// Create an instance-aware feeder
		instanceFeeder := modular.NewInstanceAwareEnvFeeder(prefixFunc)

		// Enable verbose debugging if this feeder has it enabled
		if f.verboseEnabled && f.logger != nil {
			if verboseAware, ok := instanceFeeder.(modular.VerboseAwareFeeder); ok {
				verboseAware.SetVerboseDebug(true, f.logger)
			}
		}

		for instanceKey, instanceTarget := range instances {
			if f.verboseEnabled && f.logger != nil {
				f.logger.Debug("VerboseInstanceFeeder: Feeding instance configuration", "key", key, "instanceKey", instanceKey)
			}

			if err := instanceFeeder.FeedKey(instanceKey, instanceTarget); err != nil {
				if f.verboseEnabled && f.logger != nil {
					f.logger.Debug("VerboseInstanceFeeder: Failed to feed instance", "key", key, "instanceKey", instanceKey, "error", err)
				}
				return fmt.Errorf("failed to feed instance %s for key %s: %w", instanceKey, key, err)
			}

			if f.verboseEnabled && f.logger != nil {
				f.logger.Debug("VerboseInstanceFeeder: Successfully fed instance configuration", "key", key, "instanceKey", instanceKey)
			}
		}

		if f.verboseEnabled && f.logger != nil {
			f.logger.Debug("VerboseInstanceFeeder: Completed instance-aware feeding", "key", key)
		}
	} else {
		if f.verboseEnabled && f.logger != nil {
			f.logger.Debug("VerboseInstanceFeeder: Configuration is not instance-aware, skipping", "key", key)
		}
	}

	return nil
}

// SetVerboseDebug enables verbose debugging for this feeder
func (f *VerboseInstanceFeeder) SetVerboseDebug(enabled bool, logger interface{ Debug(msg string, args ...any) }) {
	f.verboseEnabled = enabled
	f.logger = logger
}

// This allows the instance-aware configuration to work properly during the automatic config loading
func createPreConfiguredDatabaseModule() modular.Module {
	// Create a custom database module that overrides the RegisterConfig method
	return &PreConfiguredDatabaseModule{
		Module: database.NewModule(),
	}
}

// PreConfiguredDatabaseModule wraps the standard database module to provide predefined connections
type PreConfiguredDatabaseModule struct {
	*database.Module
}

// Name returns the module name
func (m *PreConfiguredDatabaseModule) Name() string {
	return "database"
}

// Init initializes the PreConfiguredDatabaseModule with debug logging
func (m *PreConfiguredDatabaseModule) Init(app modular.Application) error {
	if app.IsVerboseConfig() {
		app.Logger().Debug("PreConfiguredDatabaseModule: Starting initialization")
	}

	// Get the configuration to debug what we have
	provider, err := app.GetConfigSection(m.Name())
	if err != nil {
		if app.IsVerboseConfig() {
			app.Logger().Debug("PreConfiguredDatabaseModule: Failed to get config section", "error", err)
		}
		return fmt.Errorf("failed to get config section: %w", err)
	}

	config := provider.GetConfig()
	if app.IsVerboseConfig() {
		if dbConfig, ok := config.(*database.Config); ok {
			app.Logger().Debug("PreConfiguredDatabaseModule: Retrieved database config",
				"default", dbConfig.Default,
				"connectionCount", len(dbConfig.Connections))
			for name, conn := range dbConfig.Connections {
				app.Logger().Debug("PreConfiguredDatabaseModule: Connection details",
					"name", name,
					"driver", conn.Driver,
					"dsn", conn.DSN)
			}
		} else {
			app.Logger().Debug("PreConfiguredDatabaseModule: Config is not database.Config", "configType", fmt.Sprintf("%T", config))
		}
	}

	// Call the parent Init method
	return m.Module.Init(app)
}
func (m *PreConfiguredDatabaseModule) RegisterConfig(app modular.Application) error {
	// Create configuration with predefined connection names that will be populated from environment variables
	defaultConfig := &database.Config{
		Default: "primary",
		Connections: map[string]database.ConnectionConfig{
			"primary":   {}, // Will be populated from DB_PRIMARY_* env vars
			"secondary": {}, // Will be populated from DB_SECONDARY_* env vars
			"cache":     {}, // Will be populated from DB_CACHE_* env vars
		},
	}

	if app.IsVerboseConfig() {
		app.Logger().Debug("PreConfiguredDatabaseModule: Creating database config with predefined connections",
			"connectionCount", len(defaultConfig.Connections),
			"connections", []string{"primary", "secondary", "cache"})
	}

	// Create instance-aware config provider with database-specific prefix
	instancePrefixFunc := func(instanceKey string) string {
		prefix := "DB_" + instanceKey + "_"
		if app.IsVerboseConfig() {
			app.Logger().Debug("PreConfiguredDatabaseModule: Generated prefix for instance",
				"instanceKey", instanceKey, "prefix", prefix)
		}
		return prefix
	}

	configProvider := modular.NewInstanceAwareConfigProvider(defaultConfig, instancePrefixFunc)
	app.RegisterConfigSection(m.Name(), configProvider)

	if app.IsVerboseConfig() {
		app.Logger().Debug("PreConfiguredDatabaseModule: Registered database configuration section")
	}

	return nil
}
