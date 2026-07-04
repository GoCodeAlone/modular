// Package modular provides tenant-aware functionality for multi-tenant applications.
// This file contains the core tenant service implementation.
package modular

import (
	"fmt"
	"slices"
	"sync"
)

// StandardTenantService provides a basic implementation of the TenantService interface
type StandardTenantService struct {
	tenantConfigs      map[TenantID]*TenantConfigProvider
	mutex              sync.RWMutex
	logger             Logger
	tenantAwareModules []TenantAwareModule
	// Track which modules have been notified about which tenants
	moduleNotifications map[TenantAwareModule]map[TenantID]bool
}

type tenantModuleNotification struct {
	module   TenantAwareModule
	tenantID TenantID
}

// NewStandardTenantService creates a new tenant service
func NewStandardTenantService(logger Logger) *StandardTenantService {
	return &StandardTenantService{
		tenantConfigs:       make(map[TenantID]*TenantConfigProvider),
		logger:              logger,
		tenantAwareModules:  make([]TenantAwareModule, 0),
		moduleNotifications: make(map[TenantAwareModule]map[TenantID]bool),
	}
}

// GetTenantConfig retrieves tenant-specific configuration
func (ts *StandardTenantService) GetTenantConfig(tenantID TenantID, section string) (ConfigProvider, error) {
	ts.mutex.RLock()
	defer ts.mutex.RUnlock()

	tenantCfg, exists := ts.tenantConfigs[tenantID]
	if !exists {
		ts.logger.Debug("Tenant not found", "tenantID", tenantID)
		return nil, fmt.Errorf("%w: %s", ErrTenantNotFound, tenantID)
	}

	provider, err := tenantCfg.GetTenantConfig(tenantID, section)
	if err != nil {
		ts.logger.Debug("Tenant config section not found", "tenantID", tenantID, "section", section)
		return nil, err
	}

	return provider, nil
}

// GetTenants returns all registered tenant IDs
func (ts *StandardTenantService) GetTenants() []TenantID {
	ts.mutex.RLock()
	defer ts.mutex.RUnlock()

	tenants := make([]TenantID, 0, len(ts.tenantConfigs))
	for tenantID := range ts.tenantConfigs {
		tenants = append(tenants, tenantID)
	}
	return tenants
}

// RegisterTenant registers a new tenant with optional initial configs
func (ts *StandardTenantService) RegisterTenant(tenantID TenantID, configs map[string]ConfigProvider) error {
	ts.mutex.Lock()

	// Check if tenant already exists and update existing configs instead of returning an error
	if existingConfig, exists := ts.tenantConfigs[tenantID]; exists {
		ts.logger.Info("Tenant already registered, merging configurations", "tenantID", tenantID)

		// Add or update configs for existing tenant
		if len(configs) > 0 {
			for section, provider := range configs {
				if provider == nil || provider.GetConfig() == nil {
					ts.logger.Warn("Skipping nil config provider or config", "tenantID", tenantID, "section", section)
					continue
				}
				ts.logger.Debug("Updating config for tenant", "tenantID", tenantID, "section", section)
				existingConfig.SetTenantConfig(tenantID, section, provider)
			}
		}
		ts.mutex.Unlock()
		return nil
	}

	// Create new tenant configuration
	tenantCfg := NewTenantConfigProvider(nil)
	ts.tenantConfigs[tenantID] = tenantCfg

	// Add initial configs if provided
	if len(configs) > 0 {
		for section, provider := range configs {
			if provider == nil || provider.GetConfig() == nil {
				ts.logger.Warn("Skipping nil config provider or config", "tenantID", tenantID, "section", section)
				continue
			}
			ts.logger.Debug("Registering config for tenant", "tenantID", tenantID, "section", section)
			tenantCfg.SetTenantConfig(tenantID, section, provider)
		}
	} else {
		// Make sure the tenant has an empty configs map initialized
		tenantCfg.initializeConfigsForTenant(tenantID)
	}

	ts.logger.Info("Registered tenant", "tenantID", tenantID)

	notifications := ts.prepareTenantNotificationsLocked(tenantID, ts.tenantAwareModules)
	ts.mutex.Unlock()
	ts.notifyModulesAboutTenants(notifications, "Notified module about tenant")

	return nil
}

// prepareTenantNotificationsLocked records pending tenant notifications while ts.mutex is held.
func (ts *StandardTenantService) prepareTenantNotificationsLocked(
	tenantID TenantID,
	modules []TenantAwareModule,
) []tenantModuleNotification {
	notifications := make([]tenantModuleNotification, 0, len(modules))
	for _, module := range modules {
		if _, exists := ts.moduleNotifications[module]; !exists {
			ts.moduleNotifications[module] = make(map[TenantID]bool)
		}
		if ts.moduleNotifications[module][tenantID] {
			ts.logger.Debug("Module already notified about tenant",
				"module", fmt.Sprintf("%T", module), "tenantID", tenantID)
			continue
		}
		ts.moduleNotifications[module][tenantID] = true
		notifications = append(notifications, tenantModuleNotification{module: module, tenantID: tenantID})
	}
	return notifications
}

func (ts *StandardTenantService) notifyModulesAboutTenants(notifications []tenantModuleNotification, message string) {
	for _, notification := range notifications {
		notification.module.OnTenantRegistered(notification.tenantID)
		ts.logger.Debug(message,
			"module", fmt.Sprintf("%T", notification.module), "tenantID", notification.tenantID)
	}
}

// RemoveTenant removes a tenant and its configurations
func (ts *StandardTenantService) RemoveTenant(tenantID TenantID) error {
	ts.mutex.Lock()

	if _, exists := ts.tenantConfigs[tenantID]; !exists {
		ts.mutex.Unlock()
		return fmt.Errorf("%w: %s", ErrTenantNotFound, tenantID)
	}

	delete(ts.tenantConfigs, tenantID)
	ts.logger.Info("Removed tenant", "tenantID", tenantID)

	notifications := make([]tenantModuleNotification, 0, len(ts.tenantAwareModules))
	for _, module := range ts.tenantAwareModules {
		if notifications, exists := ts.moduleNotifications[module]; exists {
			delete(notifications, tenantID)
		}
		notifications = append(notifications, tenantModuleNotification{module: module, tenantID: tenantID})
	}
	ts.mutex.Unlock()

	// Notify tenant-aware modules outside the service mutex.
	for _, notification := range notifications {
		notification.module.OnTenantRemoved(notification.tenantID)
		ts.logger.Debug("Notified module about tenant removal",
			"module", fmt.Sprintf("%T", notification.module), "tenantID", notification.tenantID)
	}

	return nil
}

// RegisterTenantAwareModule registers a module to receive tenant events
func (ts *StandardTenantService) RegisterTenantAwareModule(module TenantAwareModule) error {
	ts.mutex.Lock()

	// Check if the module is already registered to avoid duplicates
	if slices.Contains(ts.tenantAwareModules, module) {
		ts.logger.Debug("Module already registered as tenant-aware",
			"module", fmt.Sprintf("%T", module), "name", module.Name())
		ts.mutex.Unlock()
		return nil
	}

	ts.tenantAwareModules = append(ts.tenantAwareModules, module)
	ts.logger.Debug("Registered tenant-aware module",
		"module", fmt.Sprintf("%T", module), "name", module.Name())

	// Notify about existing tenants
	notifications := make([]tenantModuleNotification, 0, len(ts.tenantConfigs))
	for tenantID := range ts.tenantConfigs {
		notifications = append(notifications, ts.prepareTenantNotificationsLocked(tenantID, []TenantAwareModule{module})...)
	}
	ts.mutex.Unlock()
	ts.notifyModulesAboutTenants(notifications, "Notified module about tenant")
	return nil
}

// RegisterTenantConfigSection registers a configuration section for a specific tenant
func (ts *StandardTenantService) RegisterTenantConfigSection(
	tenantID TenantID,
	section string,
	provider ConfigProvider,
) error {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()

	tenantCfg, exists := ts.tenantConfigs[tenantID]
	if !exists {
		// Create tenant if it doesn't exist
		tenantCfg = NewTenantConfigProvider(nil)
		ts.tenantConfigs[tenantID] = tenantCfg
		ts.logger.Info("Created new tenant during config section registration", "tenantID", tenantID)
	}

	if provider == nil || provider.GetConfig() == nil {
		ts.mutex.Unlock()
		return fmt.Errorf("%w: section '%s' for tenant %s", ErrTenantRegisterNilConfig, section, tenantID)
	}

	tenantCfg.SetTenantConfig(tenantID, section, provider)
	ts.logger.Info("Registered tenant config section", "tenantID", tenantID, "section", section)
	notifications := []tenantModuleNotification(nil)
	if !exists {
		notifications = ts.prepareTenantNotificationsLocked(tenantID, ts.tenantAwareModules)
	}
	ts.mutex.Unlock()
	ts.notifyModulesAboutTenants(notifications, "Notified module about tenant")
	return nil
}

// logTenantConfigStatus logs information about the configuration status for a tenant
func (ts *StandardTenantService) logTenantConfigStatus(tenantID TenantID) {
	ts.mutex.RLock()
	defer ts.mutex.RUnlock()

	tenantCfg, exists := ts.tenantConfigs[tenantID]
	if !exists {
		ts.logger.Warn("Attempting to log status for unregistered tenant", "tenantID", tenantID)
		return
	}

	if tenantCfg == nil {
		ts.logger.Warn("Tenant has nil config provider", "tenantID", tenantID)
		return
	}

	// Count sections and log them
	var sections []string
	for section := range tenantCfg.tenantConfigs[tenantID] {
		sections = append(sections, section)
	}

	ts.logger.Info("Tenant configuration status",
		"tenantID", tenantID,
		"sectionCount", len(sections),
		"sections", sections)
}
