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

	app := modular.NewStdApplication(
		modular.NewStdConfigProvider(&AppConfig{}),
		logger,
	)

	// Initialize TenantService
	tenantService := modular.NewStandardTenantService(app.Logger())
	app.RegisterService("tenantService", tenantService)

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
	app.RegisterService("tenantConfigLoader", tenantConfigLoader)

	// Register standard modules
	app.RegisterModule(NewWebServer(app.Logger()))
	app.RegisterModule(NewRouter(app.Logger()))
	app.RegisterModule(NewAPIModule(app.Logger()))

	// Register tenant-aware module
	app.RegisterModule(NewContentManager(app.Logger()))
	app.RegisterModule(NewNotificationManager(app.Logger()))

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
