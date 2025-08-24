package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/GoCodeAlone/modular"
)

// AppConfig represents our application configuration
type AppConfig struct {
	AppName    string            `yaml:"app_name"`
	Environment string           `yaml:"environment"`
	Database   DatabaseConfig    `yaml:"database"`
	Features   map[string]bool   `yaml:"features"`
	Server     ServerConfig      `yaml:"server"`
	ExternalServices ExternalServicesConfig `yaml:"external_services"`
}

type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Name     string `yaml:"name"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type ServerConfig struct {
	Host           string `yaml:"host"`
	Port           int    `yaml:"port"`
	Timeout        int    `yaml:"timeout"`
	MaxConnections int    `yaml:"max_connections"`
}

type ExternalServicesConfig struct {
	Redis    RedisConfig    `yaml:"redis"`
	RabbitMQ RabbitMQConfig `yaml:"rabbitmq"`
}

type RedisConfig struct {
	Enabled bool   `yaml:"enabled"`
	Host    string `yaml:"host"`
	Port    int    `yaml:"port"`
}

type RabbitMQConfig struct {
	Enabled bool   `yaml:"enabled"`
	Host    string `yaml:"host"`
	Port    int    `yaml:"port"`
}

func main() {
	fmt.Println("=== Base Configuration Example ===")
	fmt.Println()

	// Get environment from command line argument or environment variable
	environment := getEnvironment()
	fmt.Printf("Running in environment: %s\n", environment)
	fmt.Println()

	// Set up base configuration support  
	modular.SetBaseConfig("config", environment)

	// Create application configuration
	config := &AppConfig{}
	configProvider := modular.NewStdConfigProvider(config)
	
	// Create logger (simple console logger for this example)
	logger := &ConsoleLogger{}

	// Create and initialize the application
	app := modular.NewStdApplication(configProvider, logger)
	
	if err := app.Init(); err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}

	// Display the final merged configuration
	displayConfiguration(config, environment)
}

// getEnvironment gets the environment from command line args or env vars
func getEnvironment() string {
	// Check command line arguments first
	if len(os.Args) > 1 {
		return os.Args[1]
	}
	
	// Check environment variables
	if env := os.Getenv("APP_ENVIRONMENT"); env != "" {
		return env
	}
	if env := os.Getenv("ENVIRONMENT"); env != "" {
		return env
	}
	if env := os.Getenv("ENV"); env != "" {
		return env
	}
	
	// Default to development
	return "dev"
}

// displayConfiguration shows the final merged configuration
func displayConfiguration(config *AppConfig, environment string) {
	fmt.Println("=== Final Configuration ===")
	fmt.Printf("App Name: %s\n", config.AppName)
	fmt.Printf("Environment: %s\n", config.Environment)
	fmt.Println()
	
	fmt.Println("Database:")
	fmt.Printf("  Host: %s\n", config.Database.Host)
	fmt.Printf("  Port: %d\n", config.Database.Port)
	fmt.Printf("  Name: %s\n", config.Database.Name)
	fmt.Printf("  Username: %s\n", config.Database.Username)
	fmt.Printf("  Password: %s\n", maskPassword(config.Database.Password))
	fmt.Println()
	
	fmt.Println("Server:")
	fmt.Printf("  Host: %s\n", config.Server.Host)
	fmt.Printf("  Port: %d\n", config.Server.Port)
	fmt.Printf("  Timeout: %d seconds\n", config.Server.Timeout)
	fmt.Printf("  Max Connections: %d\n", config.Server.MaxConnections)
	fmt.Println()
	
	fmt.Println("Features:")
	for feature, enabled := range config.Features {
		status := "disabled"
		if enabled {
			status = "enabled"
		}
		fmt.Printf("  %s: %s\n", feature, status)
	}
	fmt.Println()
	
	fmt.Println("External Services:")
	fmt.Printf("  Redis: %s (Host: %s:%d)\n", 
		enabledStatus(config.ExternalServices.Redis.Enabled),
		config.ExternalServices.Redis.Host,
		config.ExternalServices.Redis.Port)
	fmt.Printf("  RabbitMQ: %s (Host: %s:%d)\n",
		enabledStatus(config.ExternalServices.RabbitMQ.Enabled),
		config.ExternalServices.RabbitMQ.Host,
		config.ExternalServices.RabbitMQ.Port)
	fmt.Println()
	
	// Show configuration summary
	showConfigurationSummary(environment, config)
}

func maskPassword(password string) string {
	if len(password) == 0 {
		return ""
	}
	// Always return at least 8 asterisks to avoid leaking length information
	minLength := 8
	if len(password) > minLength {
		return strings.Repeat("*", len(password))
	}
	return strings.Repeat("*", minLength)
}

func enabledStatus(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}

func showConfigurationSummary(environment string, config *AppConfig) {
	fmt.Println("=== Configuration Summary ===")
	fmt.Printf("Environment: %s\n", environment)
	
	// Count enabled features
	enabledFeatures := 0
	for _, enabled := range config.Features {
		if enabled {
			enabledFeatures++
		}
	}
	fmt.Printf("Enabled Features: %d/%d\n", enabledFeatures, len(config.Features))
	
	// Count enabled external services
	enabledServices := 0
	totalServices := 2
	if config.ExternalServices.Redis.Enabled {
		enabledServices++
	}
	if config.ExternalServices.RabbitMQ.Enabled {
		enabledServices++
	}
	fmt.Printf("Enabled External Services: %d/%d\n", enabledServices, totalServices)
	
	fmt.Printf("Database Host: %s\n", config.Database.Host)
	fmt.Printf("Server Port: %d\n", config.Server.Port)
}

// ConsoleLogger implements a simple console logger
type ConsoleLogger struct{}

func (l *ConsoleLogger) Debug(msg string, args ...any) {
	fmt.Printf("[DEBUG] %s %v\n", msg, args)
}

func (l *ConsoleLogger) Info(msg string, args ...any) {
	fmt.Printf("[INFO] %s %v\n", msg, args)
}

func (l *ConsoleLogger) Warn(msg string, args ...any) {
	fmt.Printf("[WARN] %s %v\n", msg, args)
}

func (l *ConsoleLogger) Error(msg string, args ...any) {
	fmt.Printf("[ERROR] %s %v\n", msg, args)
}