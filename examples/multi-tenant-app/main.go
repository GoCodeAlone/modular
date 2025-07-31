package main

import (
	"fmt"
	"log/slog"
	"os"
	"regexp"

	"github.com/CrisisTextLine/modular"
	"github.com/CrisisTextLine/modular/feeders"
)

func main() {
	// Define config feeders
	modular.ConfigFeeders = []modular.Feeder{
		feeders.NewYamlFeeder("config.yaml"),
		feeders.NewEnvFeeder(),
	}

	// Create application with debug logging
	logger := slog.New(slog.NewTextHandler(
		os.Stdout,
		&slog.HandlerOptions{Level: slog.LevelDebug},
	))

	// Create application using new builder API
	app, err := modular.NewApplication(
		modular.WithLogger(logger),
		modular.WithConfigProvider(modular.NewStdConfigProvider(&AppConfig{})),
		modular.WithModules(
			NewWebServer(logger),
			NewRouter(logger),
			NewAPIModule(logger),
			NewContentManager(logger),
			NewNotificationManager(logger),
		),
	)

	if err != nil {
		logger.Error("Failed to create application", "error", err)
		os.Exit(1)
	}

	// Initialize TenantService (advanced setup still manual for now)
	tenantService := modular.NewStandardTenantService(app.Logger())
	if err := app.RegisterService("tenantService", tenantService); err != nil {
		logger.Error("Failed to register tenant service", "error", err)
		os.Exit(1)
	}

	// Register tenant config loader
	tenantConfigLoader := modular.NewFileBasedTenantConfigLoader(modular.TenantConfigParams{
		ConfigNameRegex: regexp.MustCompile(`^\w+\.yaml$`),
		ConfigDir:       "tenants",
		ConfigFeeders: []modular.Feeder{
			feeders.NewTenantAffixedEnvFeeder(func(tenantId string) string {
				return fmt.Sprintf("%s_", tenantId)
			}, func(s string) string { return "" }),
		},
	})
	if err := app.RegisterService("tenantConfigLoader", tenantConfigLoader); err != nil {
		logger.Error("Failed to register tenant config loader", "error", err)
		os.Exit(1)
	}

	// Run application with lifecycle management
	if err := app.Run(); err != nil {
		app.Logger().Error("Application error", "error", err)
		os.Exit(1)
	}
}

// AppConfig defines base application configuration
type AppConfig struct {
	AppName       string             `yaml:"appName"`
	Environment   string             `yaml:"environment"`
	WebServer     WebConfig          `yaml:"webserver"`
	Content       ContentConfig      `yaml:"content"`
	Notifications NotificationConfig `yaml:"notifications"`
}

type WebConfig struct {
	Port int `yaml:"port"`
}

type ContentConfig struct {
	DefaultTemplate string `yaml:"defaultTemplate"`
	CacheTTL        int    `yaml:"cacheTTL"`
}

type NotificationConfig struct {
	Provider    string `yaml:"provider"`
	FromAddress string `yaml:"fromAddress"`
	MaxRetries  int    `yaml:"maxRetries"`
}
