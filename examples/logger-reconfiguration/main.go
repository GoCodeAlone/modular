package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/GoCodeAlone/modular"
	"github.com/GoCodeAlone/modular/feeders"
)

func main() {
	// Create initial logger with text format
	// This will be reconfigured based on loaded configuration
	initialLogger := slog.New(slog.NewTextHandler(
		os.Stdout,
		&slog.HandlerOptions{Level: slog.LevelInfo},
	))

	initialLogger.Info("Starting application with initial logger")

	// Create application using new builder API
	app, err := modular.NewApplication(
		modular.WithLogger(initialLogger),
		modular.WithConfigProvider(modular.NewStdConfigProvider(&AppConfig{})),
		// Register hook to reconfigure logger based on loaded configuration
		modular.WithOnConfigLoaded(func(app modular.Application) error {
			// Access the loaded configuration
			cfg := app.ConfigProvider().GetConfig().(*AppConfig)

			// Create a new logger based on configuration settings
			var handler slog.Handler
			
			// Configure handler based on log format
			switch cfg.LogFormat {
			case "json":
				handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
					Level: parseLogLevel(cfg.LogLevel),
				})
			case "text":
				handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
					Level: parseLogLevel(cfg.LogLevel),
				})
			default:
				handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
					Level: parseLogLevel(cfg.LogLevel),
				})
			}

			// Create new logger with configuration-based settings
			newLogger := slog.New(handler)
			
			// Replace the logger before modules initialize
			app.SetLogger(newLogger)
			
			newLogger.Info("Logger reconfigured from configuration",
				"format", cfg.LogFormat,
				"level", cfg.LogLevel)

			return nil
		}),
		// Register modules that will receive the reconfigured logger
		modular.WithModules(
			NewLoggingModule(),
			NewServiceModule(),
		),
	)

	if err != nil {
		initialLogger.Error("Failed to create application", "error", err)
		os.Exit(1)
	}

	// Set up configuration feeders
	if stdApp, ok := app.(*modular.StdApplication); ok {
		stdApp.SetConfigFeeders([]modular.Feeder{
			feeders.NewYamlFeeder("config.yaml"),
			feeders.NewEnvFeeder(), // Environment variables override YAML
		})
	}

	// Initialize application (this will trigger config loading and logger reconfiguration)
	if err := app.Init(); err != nil {
		app.Logger().Error("Application initialization failed", "error", err)
		os.Exit(1)
	}

	// Test logging after reconfiguration
	app.Logger().Info("Application initialized successfully")
	app.Logger().Debug("Debug logging is enabled", "config_loaded", true)

	fmt.Println("\n=== Logger Reconfiguration Example Complete ===")
	fmt.Println("The logger was successfully reconfigured based on configuration before modules initialized")
	fmt.Println("All modules received the reconfigured logger instance")
}

// AppConfig demonstrates logger configuration
type AppConfig struct {
	LogFormat string `yaml:"logFormat" default:"text" desc:"Log format (text or json)"`
	LogLevel  string `yaml:"logLevel" default:"info" desc:"Log level (debug, info, warn, error)"`
}

// parseLogLevel converts string log level to slog.Level
func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// LoggingModule demonstrates a module that caches the logger
type LoggingModule struct {
	logger modular.Logger
}

func NewLoggingModule() *LoggingModule {
	return &LoggingModule{}
}

func (m *LoggingModule) Name() string {
	return "logging"
}

func (m *LoggingModule) Init(app modular.Application) error {
	// Module caches the logger reference during initialization
	m.logger = app.Logger()
	m.logger.Info("LoggingModule initialized", "module", m.Name())
	m.logger.Debug("This debug message will only appear if log level is debug", "module", m.Name())
	return nil
}

// ServiceModule demonstrates another module that uses the logger
type ServiceModule struct {
	logger modular.Logger
}

func NewServiceModule() *ServiceModule {
	return &ServiceModule{}
}

func (m *ServiceModule) Name() string {
	return "service"
}

func (m *ServiceModule) Init(app modular.Application) error {
	// This module also caches the logger
	m.logger = app.Logger()
	m.logger.Info("ServiceModule initialized", "module", m.Name(), "status", "ready")
	
	// Demonstrate that the logger has the correct configuration
	m.logger.Debug("Service module debug information", 
		"feature", "logger_reconfiguration",
		"working", true)
	
	return nil
}
