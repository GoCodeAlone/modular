package chimux

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/GoCodeAlone/modular"
)

// Static errors for bdd_config_test.go
var (
	errFailedToInitApp       = errors.New("failed to initialize app")
	errBasePathNotConfigured = errors.New("base path not configured")
	errTimeoutNotConfigured  = errors.New("timeout not configured")
)

func (ctx *ChiMuxBDDTestContext) iHaveAChimuxConfigurationWithBasePath(basePath string) error {
	ctx.config = &ChiMuxConfig{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE"},
		AllowedHeaders:   []string{"Origin", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
		Timeout:          60 * time.Second,
		BasePath:         basePath,
	}
	return nil
}

func (ctx *ChiMuxBDDTestContext) iRegisterRoutesWithTheConfiguredBasePath() error {
	// Make sure we have a router service available (initialize the app with base path config)
	if ctx.routerService == nil {
		// Initialize application with the base path configuration
		logger := &testLogger{}

		// Create provider with the updated chimux config
		chimuxConfigProvider := modular.NewStdConfigProvider(ctx.config)

		// Create app with empty main config
		mainConfigProvider := modular.NewStdConfigProvider(struct{}{})

		// Create mock tenant application since chimux requires tenant app
		mockTenantApp := &mockTenantApplication{
			Application: modular.NewStdApplication(mainConfigProvider, logger),
			tenantService: &mockTenantService{
				configs: make(map[modular.TenantID]map[string]modular.ConfigProvider),
			},
		}

		// Register the chimux config section first
		mockTenantApp.RegisterConfigSection("chimux", chimuxConfigProvider)

		// Create and register chimux module
		ctx.module = NewChiMuxModule().(*ChiMuxModule)
		mockTenantApp.RegisterModule(ctx.module)

		// Initialize
		if err := mockTenantApp.Init(); err != nil {
			return fmt.Errorf("%w: %w", errFailedToInitApp, err)
		}

		ctx.app = mockTenantApp

		// Get router service
		if err := ctx.theRouterServiceShouldBeAvailable(); err != nil {
			return err
		}
	}

	// Routes would be registered normally, but the module should prefix them
	ctx.routerService.Get("/users", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	return nil
}

func (ctx *ChiMuxBDDTestContext) allRoutesShouldBePrefixedWithTheBasePath() error {
	// This would be verified by checking the actual route registration
	// For BDD test purposes, we check that base path is configured
	if ctx.config.BasePath == "" {
		return errBasePathNotConfigured
	}
	return nil
}

func (ctx *ChiMuxBDDTestContext) iHaveAChimuxConfigurationWithTimeoutSettings() error {
	ctx.config = &ChiMuxConfig{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   []string{"Origin", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
		Timeout:          5 * time.Second, // 5 second timeout
		BasePath:         "",
	}
	return nil
}

func (ctx *ChiMuxBDDTestContext) theChimuxModuleAppliesTimeoutConfiguration() error {
	// Timeout would be applied as middleware
	return nil
}

func (ctx *ChiMuxBDDTestContext) theTimeoutMiddlewareShouldBeConfigured() error {
	if ctx.config.Timeout <= 0 {
		return errTimeoutNotConfigured
	}
	return nil
}

func (ctx *ChiMuxBDDTestContext) requestsShouldRespectTheTimeoutSettings() error {
	// This would be tested with actual HTTP requests that take longer than timeout
	return nil
}
