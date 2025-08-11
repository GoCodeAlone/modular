package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/CrisisTextLine/modular"
)

// Static error variables for err113 compliance
var (
	errInvalidWebserverConfigType     = errors.New("invalid webserver config type")
	errAppNotTenantApplication        = errors.New("app does not implement TenantApplication interface")
	errInvalidContentConfigType       = errors.New("invalid content config type")
	errInvalidNotificationsConfigType = errors.New("invalid notifications config type")
)

// WebServer module - standard non-tenant aware module
type WebServer struct {
	config *WebConfig
	logger modular.Logger
}

func NewWebServer(logger modular.Logger) *WebServer {
	return &WebServer{
		logger: logger,
	}
}

func (w *WebServer) Name() string {
	return "webserver"
}

func (w *WebServer) RegisterConfig(app modular.Application) error {
	app.RegisterConfigSection("webserver", modular.NewStdConfigProvider(&WebConfig{}))
	return nil
}

func (w *WebServer) Init(app modular.Application) error {
	// Get config from app
	cp, err := app.GetConfigSection("webserver")
	if err != nil {
		return fmt.Errorf("webserver config not found: %w", err)
	}

	webConfig, ok := cp.GetConfig().(*WebConfig)
	if !ok {
		return errInvalidWebserverConfigType
	}
	w.config = webConfig

	w.logger.Info("WebServer initialized", "port", w.config.Port)
	return nil
}

func (w *WebServer) Start(context.Context) error {
	w.logger.Info("WebServer started", "port", w.config.Port)
	return nil
}

func (w *WebServer) Stop(context.Context) error {
	w.logger.Info("WebServer stopped")
	return nil
}

// Router module
type Router struct {
	logger modular.Logger
}

func NewRouter(logger modular.Logger) *Router {
	return &Router{
		logger: logger,
	}
}

func (r *Router) Name() string {
	return "router"
}

func (r *Router) Init(app modular.Application) error {
	r.logger.Info("Router initialized")
	return nil
}

func (r *Router) Dependencies() []string {
	return []string{"webserver"}
}

// APIModule module
type APIModule struct {
	logger modular.Logger
}

func NewAPIModule(logger modular.Logger) *APIModule {
	return &APIModule{
		logger: logger,
	}
}

func (a *APIModule) Name() string {
	return "api"
}

func (a *APIModule) Init(app modular.Application) error {
	a.logger.Info("API module initialized")
	return nil
}

func (a *APIModule) Dependencies() []string {
	return []string{"router"}
}

// ContentManager - tenant-aware module
type ContentManager struct {
	logger        modular.Logger
	app           modular.TenantApplication
	defaultConfig *ContentConfig
}

func NewContentManager(logger modular.Logger) *ContentManager {
	return &ContentManager{
		logger: logger,
	}
}

func (cm *ContentManager) Name() string {
	return "content"
}

func (cm *ContentManager) RegisterConfig(app modular.Application) error {
	app.RegisterConfigSection("content", modular.NewStdConfigProvider(&ContentConfig{}))
	return nil
}

func (cm *ContentManager) Init(app modular.Application) error {
	var ok bool
	cm.app, ok = app.(modular.TenantApplication)
	if !ok {
		return errAppNotTenantApplication
	}

	// Get default config
	cp, err := app.GetConfigSection("content")
	if err != nil {
		return fmt.Errorf("content config not found: %w", err)
	}

	contentConfig, ok := cp.GetConfig().(*ContentConfig)
	if !ok {
		return errInvalidContentConfigType
	}
	cm.defaultConfig = contentConfig

	cm.logger.Info("Content manager initialized with default template",
		"template", cm.defaultConfig.DefaultTemplate,
		"cacheTTL", cm.defaultConfig.CacheTTL)
	return nil
}

func (cm *ContentManager) OnTenantRegistered(tenantID modular.TenantID) {
	cm.logger.Info("Tenant registered in Content Manager", "tenant", tenantID)
}

func (cm *ContentManager) OnTenantRemoved(tenantID modular.TenantID) {
	cm.logger.Info("Tenant removed from Content Manager", "tenant", tenantID)
}

// NotificationManager - tenant-aware module
type NotificationManager struct {
	logger        modular.Logger
	app           modular.TenantApplication
	tenantService modular.TenantService
	defaultConfig *NotificationConfig
}

func NewNotificationManager(logger modular.Logger) *NotificationManager {
	return &NotificationManager{
		logger: logger,
	}
}

func (nm *NotificationManager) Name() string {
	return "notifications"
}

func (nm *NotificationManager) RegisterConfig(app modular.Application) error {
	app.RegisterConfigSection("notifications", modular.NewStdConfigProvider(&NotificationConfig{}))
	return nil
}

func (nm *NotificationManager) Init(app modular.Application) error {
	nm.app = app.(modular.TenantApplication)

	// Get tenant service
	ts, err := nm.app.GetTenantService()
	if err != nil {
		return fmt.Errorf("failed to get tenant service: %w", err)
	}
	nm.tenantService = ts

	// Get default config
	config, err := app.GetConfigSection("notifications")
	if err != nil {
		return fmt.Errorf("notifications config not found: %w", err)
	}

	notificationConfig, ok := config.GetConfig().(*NotificationConfig)
	if !ok {
		return errInvalidNotificationsConfigType
	}
	nm.defaultConfig = notificationConfig

	nm.logger.Info("Notification manager initialized",
		"provider", nm.defaultConfig.Provider,
		"fromAddress", nm.defaultConfig.FromAddress)

	return nil
}

func (nm *NotificationManager) ProvidesServices() []modular.ServiceProvider {
	return nil
}

func (nm *NotificationManager) RequiresServices() []modular.ServiceDependency {
	return nil
}

func (nm *NotificationManager) OnTenantRegistered(tenantID modular.TenantID) {
	nm.logger.Info("Tenant registered in Notification Manager", "tenant", tenantID)
}

func (nm *NotificationManager) OnTenantRemoved(tenantID modular.TenantID) {
	nm.logger.Info("Tenant removed from Notification Manager", "tenant", tenantID)
}
