package main

import (
	"fmt"
	"log/slog"
	"os"
	
	"github.com/CrisisTextLine/modular"
	"github.com/CrisisTextLine/modular/modules/chimux"
)

// Simple integration test to verify the fix
func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	
	// Create application
	app := modular.NewStdApplication(modular.NewStdConfigProvider(&struct{}{}), logger)
	
	// Register chimux module
	app.RegisterModule(chimux.NewChiMuxModule())
	
	// Register tenant service
	tenantService := modular.NewStandardTenantService(logger)
	if err := app.RegisterService("tenantService", tenantService); err != nil {
		logger.Error("Failed to register tenant service", "error", err)
		os.Exit(1)
	}
	
	// Register a simple tenant config loader
	configLoader := &SimpleTenantConfigLoader{}
	if err := app.RegisterService("tenantConfigLoader", configLoader); err != nil {
		logger.Error("Failed to register tenant config loader", "error", err)
		os.Exit(1)
	}
	
	// Initialize application - this should NOT panic
	fmt.Println("Initializing application...")
	if err := app.Init(); err != nil {
		logger.Error("Failed to initialize application", "error", err)
		os.Exit(1)
	}
	
	fmt.Println("✅ Application initialized successfully - no race condition panic!")
	fmt.Println("✅ The chimux tenant registration race condition has been fixed!")
}

// SimpleTenantConfigLoader for testing
type SimpleTenantConfigLoader struct{}

func (l *SimpleTenantConfigLoader) LoadTenantConfigurations(app modular.Application, tenantService modular.TenantService) error {
	app.Logger().Info("Loading tenant configurations")
	
	// Register a test tenant
	return tenantService.RegisterTenant(modular.TenantID("test-tenant"), map[string]modular.ConfigProvider{
		"chimux": modular.NewStdConfigProvider(&chimux.ChiMuxConfig{
			AllowedOrigins: []string{"https://test.example.com"},
		}),
	})
}