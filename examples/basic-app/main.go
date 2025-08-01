package main

import (
	"basic-app/api"
	"basic-app/router"
	"basic-app/webserver"
	"fmt"
	"log/slog"
	"os"

	"github.com/GoCodeAlone/modular"
	"github.com/GoCodeAlone/modular/feeders"
)

func main() {
	// Generate sample config file if requested
	if len(os.Args) > 1 && os.Args[1] == "--generate-config" {
		format := "yaml"
		if len(os.Args) > 2 {
			format = os.Args[2]
		}
		outputFile := "config-sample." + format
		if len(os.Args) > 3 {
			outputFile = os.Args[3]
		}

		cfg := &AppConfig{}
		if err := modular.SaveSampleConfig(cfg, format, outputFile); err != nil {
			fmt.Printf("Error generating sample config: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Sample config generated at %s\n", outputFile)
		os.Exit(0)
	}

	modular.ConfigFeeders = []modular.Feeder{
		feeders.NewYamlFeeder("config.yaml"),
		feeders.NewEnvFeeder(),
	}

	// Create logger
	logger := slog.New(slog.NewTextHandler(
		os.Stdout,
		&slog.HandlerOptions{},
	))

	// Create application using new builder API
	app, err := modular.NewApplication(
		modular.WithLogger(logger),
		modular.WithConfigProvider(modular.NewStdConfigProvider(&AppConfig{})),
		modular.WithModules(
			webserver.NewWebServer(),
			router.NewRouter(),
			api.NewAPIModule(),
		),
	)

	if err != nil {
		logger.Error("Failed to create application", "error", err)
		os.Exit(1)
	}

	// Run application with lifecycle management
	if err := app.Run(); err != nil {
		app.Logger().Error("Application error", "error", err)
		os.Exit(1)
	}
}

// AppConfig demonstrates the new validation, default values, and required fields
type AppConfig struct {
	AppName     string          `yaml:"appName" default:"My Modular App" desc:"Application name"`
	Environment string          `yaml:"environment" required:"true" desc:"Environment (dev, test, prod)"`
	Server      ServerConfig    `yaml:"server" desc:"Server configuration"`
	Database    DatabaseConfig  `yaml:"database" desc:"Database configuration"`
	Features    map[string]bool `yaml:"features" default:"{\"logging\":true,\"metrics\":true}" desc:"Feature toggles"`
	Cors        []string        `yaml:"cors" default:"[\"localhost:3000\"]" desc:"Allowed CORS origins"`
	Admins      []string        `yaml:"admins" desc:"Admin user emails"`
}

// ServerConfig contains server-specific settings
type ServerConfig struct {
	Host        string `yaml:"host" default:"localhost" desc:"Server host"`
	Port        int    `yaml:"port" default:"8080" required:"true" desc:"Server port"`
	ReadTimeout int    `yaml:"readTimeout" default:"30" desc:"Read timeout in seconds"`
	Debug       bool   `yaml:"debug" default:"false" desc:"Enable debug mode"`
}

// DatabaseConfig contains database settings
type DatabaseConfig struct {
	Enabled  bool   `yaml:"enabled" default:"true" desc:"Enable database connectivity"`
	Host     string `yaml:"host" default:"localhost" desc:"Database host"`
	Port     int    `yaml:"port" default:"5432" desc:"Database port"`
	Name     string `yaml:"name" required:"true" desc:"Database name"`
	User     string `yaml:"user" default:"postgres" desc:"Database user"`
	Password string `yaml:"password" desc:"Database password"`
	SSLMode  string `yaml:"sslMode" default:"disable" desc:"SSL mode for database connection"`
}

// Validate implements the ConfigValidator interface
func (c *AppConfig) Validate() error {
	// Validate environment is one of the expected values
	validEnvs := map[string]bool{"dev": true, "test": true, "prod": true}
	if !validEnvs[c.Environment] {
		return fmt.Errorf("%w: environment must be one of [dev, test, prod]", modular.ErrConfigValidationFailed)
	}

	// Validate port range
	if c.Server.Port < 1024 || c.Server.Port > 65535 {
		return fmt.Errorf("%w: server port must be between 1024 and 65535", modular.ErrConfigValidationFailed)
	}

	return nil
}
