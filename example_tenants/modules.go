package main

import (
	"context"
	"fmt"

	"github.com/GoCodeAlone/modular"
)

// WebServer module - standard non-tenant aware module
type WebServer struct {
	config *WebConfig
	logger modular.Logger
}

func NewWebServer() *WebServer {
	return &WebServer{}
}

func (w *WebServer) Name() string {
	return "webserver"
}

func (w *WebServer) RegisterConfig(app modular.Application) {
	app.RegisterConfigSection("webserver", modular.NewStdConfigProvider(&WebConfig{}))
}

func (w *WebServer) ProvidesServices() []modular.ServiceProvider {
	return nil
}

func (w *WebServer) RequiresServices() []modular.ServiceDependency {
	return nil
}

func (w *WebServer) Init(app modular.Application) error {
	w.logger = app.Logger()

	// Get config from app
	cp, err := app.GetConfigSection("webserver")
	if err != nil {
		return fmt.Errorf("webserver config not found: %w", err)
	}

	webConfig, ok := cp.GetConfig().(*WebConfig)
	if !ok {
		return fmt.Errorf("invalid webserver config type")
	}
	w.config = webConfig

	w.logger.Info("WebServer initialized", "port", w.config.Port)
	return nil
}

func (w *WebServer) Start(ctx context.Context) error {
	w.logger.Info("WebServer started", "port", w.config.Port)
	return nil
}

func (w *WebServer) Stop(ctx context.Context) error {
	w.logger.Info("WebServer stopped")
	return nil
}

func (w *WebServer) Dependencies() []string {
	return nil
}

// Router module
type Router struct {
	logger modular.Logger
}

func NewRouter() *Router {
	return &Router{}
}

func (r *Router) Name() string {
	return "router"
}

func (r *Router) RegisterConfig(app modular.Application) {
}

func (r *Router) ProvidesServices() []modular.ServiceProvider {
	return nil
}

func (r *Router) RequiresServices() []modular.ServiceDependency {
	return nil
}

func (r *Router) Init(app modular.Application) error {
	r.logger = app.Logger()
	r.logger.Info("Router initialized")
	return nil
}

func (r *Router) Start(ctx context.Context) error {
	r.logger.Info("Router started")
	return nil
}

func (r *Router) Stop(ctx context.Context) error {
	r.logger.Info("Router stopped")
	return nil
}

func (r *Router) Dependencies() []string {
	return nil
}

// APIModule module
type APIModule struct {
	logger modular.Logger
}

func NewAPIModule() *APIModule {
	return &APIModule{}
}

func (a *APIModule) Name() string {
	return "api"
}

func (a *APIModule) RegisterConfig(app modular.Application) {
}

func (a *APIModule) ProvidesServices() []modular.ServiceProvider {
	return nil
}

func (a *APIModule) RequiresServices() []modular.ServiceDependency {
	return nil
}

func (a *APIModule) Init(app modular.Application) error {
	a.logger = app.Logger()
	a.logger.Info("API module initialized")
	return nil
}

func (a *APIModule) Start(ctx context.Context) error {
	a.logger.Info("API module started")
	return nil
}

func (a *APIModule) Stop(ctx context.Context) error {
	a.logger.Info("API module stopped")
	return nil
}

func (a *APIModule) Dependencies() []string {
	return nil
}

// ContentManager - tenant-aware module
type ContentManager struct {
	logger        modular.Logger
	app           modular.TenantApplication
	tenantService modular.TenantService
	defaultConfig *ContentConfig
}

func NewContentManager() *ContentManager {
	return &ContentManager{}
}

func (cm *ContentManager) Name() string {
	return "content"
}

func (cm *ContentManager) RegisterConfig(app modular.Application) {
	app.RegisterConfigSection("content", modular.NewStdConfigProvider(&ContentConfig{}))
}

func (cm *ContentManager) Init(app modular.Application) error {
	cm.logger = app.Logger()
	cm.app = app.(modular.TenantApplication)

	// Get tenant service
	ts, err := cm.app.GetTenantService()
	if err != nil {
		return err
	}
	cm.tenantService = ts

	// Get default config
	cp, err := app.GetConfigSection("content")
	if err != nil {
		return fmt.Errorf("content config not found: %w", err)
	}

	contentConfig, ok := cp.GetConfig().(*ContentConfig)
	if !ok {
		return fmt.Errorf("invalid content config type")
	}
	cm.defaultConfig = contentConfig

	cm.logger.Info("Content manager initialized with default template",
		"template", cm.defaultConfig.DefaultTemplate,
		"cacheTTL", cm.defaultConfig.CacheTTL)

	// Log tenant-specific configurations
	for _, tenantID := range ts.GetTenants() {
		cm.logTenantConfig(tenantID)
	}

	return nil
}

func (cm *ContentManager) ProvidesServices() []modular.ServiceProvider {
	return nil
}

func (cm *ContentManager) RequiresServices() []modular.ServiceDependency {
	return nil
}

func (cm *ContentManager) logTenantConfig(tenantID modular.TenantID) {
	provider, err := cm.tenantService.GetTenantConfig(tenantID, "content")
	if err != nil {
		cm.logger.Error("Failed to get tenant content config", "tenant", tenantID, "error", err)
		return
	}

	config, ok := provider.GetConfig().(*ContentConfig)
	if !ok {
		cm.logger.Error("Invalid tenant content config type", "tenant", tenantID, "content", config)
		return
	}

	cm.logger.Info("Tenant content configuration",
		"tenant", tenantID,
		"template", config.DefaultTemplate,
		"cacheTTL", config.CacheTTL)
}

func (cm *ContentManager) Start(ctx context.Context) error {
	cm.logger.Info("Content manager started")
	return nil
}

func (cm *ContentManager) Stop(ctx context.Context) error {
	cm.logger.Info("Content manager stopped")
	return nil
}

func (cm *ContentManager) OnTenantRegistered(tenantID modular.TenantID) {
	cm.logger.Info("Tenant registered in Content Manager", "tenant", tenantID)
	cm.logTenantConfig(tenantID)
}

func (cm *ContentManager) OnTenantRemoved(tenantID modular.TenantID) {
	cm.logger.Info("Tenant removed from Content Manager", "tenant", tenantID)
}

func (cm *ContentManager) Dependencies() []string {
	return nil
}

// NotificationManager - tenant-aware module
type NotificationManager struct {
	logger        modular.Logger
	app           modular.TenantApplication
	tenantService modular.TenantService
	defaultConfig *NotificationConfig
}

func NewNotificationManager() *NotificationManager {
	return &NotificationManager{}
}

func (nm *NotificationManager) Name() string {
	return "notifications"
}

func (nm *NotificationManager) RegisterConfig(app modular.Application) {
	app.RegisterConfigSection("notifications", modular.NewStdConfigProvider(&NotificationConfig{}))
}

func (nm *NotificationManager) Init(app modular.Application) error {
	nm.logger = app.Logger()
	nm.app = app.(modular.TenantApplication)

	// Get tenant service
	ts, err := nm.app.GetTenantService()
	if err != nil {
		return err
	}
	nm.tenantService = ts

	// Get default config
	config, err := app.GetConfigSection("notifications")
	if err != nil {
		return fmt.Errorf("notifications config not found: %w", err)
	}

	notificationConfig, ok := config.GetConfig().(*NotificationConfig)
	if !ok {
		return fmt.Errorf("invalid notifications config type")
	}
	nm.defaultConfig = notificationConfig

	nm.logger.Info("Notification manager initialized",
		"provider", nm.defaultConfig.Provider,
		"fromAddress", nm.defaultConfig.FromAddress)

	// Log tenant-specific configurations
	for _, tenantID := range ts.GetTenants() {
		nm.logTenantConfig(tenantID)
	}

	return nil
}

func (nm *NotificationManager) ProvidesServices() []modular.ServiceProvider {
	return nil
}

func (nm *NotificationManager) RequiresServices() []modular.ServiceDependency {
	return nil
}

func (nm *NotificationManager) logTenantConfig(tenantID modular.TenantID) {
	provider, err := nm.tenantService.GetTenantConfig(tenantID, "notifications")
	if err != nil {
		nm.logger.Error("Failed to get tenant notification config", "tenant", tenantID, "error", err)
		return
	}

	config, ok := provider.GetConfig().(*NotificationConfig)
	if !ok {
		nm.logger.Error("Invalid tenant notification config type", "tenant", tenantID)
		return
	}

	nm.logger.Info("Tenant notification configuration",
		"tenant", tenantID,
		"provider", config.Provider,
		"fromAddress", config.FromAddress,
		"maxRetries", config.MaxRetries)
}

func (nm *NotificationManager) Start(ctx context.Context) error {
	nm.logger.Info("Notification manager started")
	return nil
}

func (nm *NotificationManager) Stop(ctx context.Context) error {
	nm.logger.Info("Notification manager stopped")
	return nil
}

func (nm *NotificationManager) OnTenantRegistered(tenantID modular.TenantID) {
	nm.logger.Info("Tenant registered in Notification Manager", "tenant", tenantID)
	nm.logTenantConfig(tenantID)
}

func (nm *NotificationManager) OnTenantRemoved(tenantID modular.TenantID) {
	nm.logger.Info("Tenant removed from Notification Manager", "tenant", tenantID)
}

func (nm *NotificationManager) Dependencies() []string {
	return nil
}
