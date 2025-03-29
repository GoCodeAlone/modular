package modular

import (
	"fmt"
	"reflect"
	"sync"
)

// TenantConfigProvider manages configurations for multiple tenants
type TenantConfigProvider struct {
	defaultConfig ConfigProvider
	tenantConfigs map[TenantID]map[string]ConfigProvider
	mutex         sync.RWMutex
}

// NewTenantConfigProvider creates a new tenant configuration provider
func NewTenantConfigProvider(defaultConfig ConfigProvider) *TenantConfigProvider {
	return &TenantConfigProvider{
		defaultConfig: defaultConfig,
		tenantConfigs: make(map[TenantID]map[string]ConfigProvider),
	}
}

// GetDefaultConfig returns the default configuration (non-tenant specific)
func (tcp *TenantConfigProvider) GetDefaultConfig() ConfigProvider {
	return tcp.defaultConfig
}

// GetConfig returns the default configuration to satisfy ConfigProvider interface
func (tcp *TenantConfigProvider) GetConfig() any {
	if tcp.defaultConfig == nil {
		return nil
	}
	return tcp.defaultConfig.GetConfig()
}

// Initialize the configs map for a tenant
func (tcp *TenantConfigProvider) initializeConfigsForTenant(tenantID TenantID) {
	tcp.mutex.Lock()
	defer tcp.mutex.Unlock()

	if _, exists := tcp.tenantConfigs[tenantID]; !exists {
		tcp.tenantConfigs[tenantID] = make(map[string]ConfigProvider)
	}
}

// SetTenantConfig sets a configuration for a specific tenant and section
func (tcp *TenantConfigProvider) SetTenantConfig(tenantID TenantID, section string, provider ConfigProvider) {
	tcp.mutex.Lock()
	defer tcp.mutex.Unlock()

	if _, exists := tcp.tenantConfigs[tenantID]; !exists {
		tcp.tenantConfigs[tenantID] = make(map[string]ConfigProvider)
	}

	// Validate the provider before setting
	if provider == nil {
		return
	}

	cfg := provider.GetConfig()
	if cfg == nil {
		return
	}

	// Ensure the config is a valid, non-zero value
	cfgValue := reflect.ValueOf(cfg)
	if cfgValue.Kind() == reflect.Ptr && cfgValue.IsNil() {
		return
	}

	tcp.tenantConfigs[tenantID][section] = provider
}

// GetTenantConfig retrieves a configuration for a specific tenant and section
func (tcp *TenantConfigProvider) GetTenantConfig(tenantID TenantID, section string) (ConfigProvider, error) {
	tcp.mutex.RLock()
	defer tcp.mutex.RUnlock()

	tenantCfgs, exists := tcp.tenantConfigs[tenantID]
	if !exists {
		return nil, fmt.Errorf("no configs found for tenant %s", tenantID)
	}

	cfg, exists := tenantCfgs[section]
	if !exists {
		return nil, fmt.Errorf("config section '%s' not found for tenant %s", section, tenantID)
	}

	if cfg == nil {
		return nil, fmt.Errorf("config provider for tenant %s section '%s' is nil", tenantID, section)
	}

	if cfg.GetConfig() == nil {
		return nil, fmt.Errorf("config for tenant %s section '%s' is nil", tenantID, section)
	}

	return cfg, nil
}

// HasTenantConfig checks if a configuration exists for a specific tenant and section
func (tcp *TenantConfigProvider) HasTenantConfig(tenantID TenantID, section string) bool {
	tcp.mutex.RLock()
	defer tcp.mutex.RUnlock()

	if tenantCfgs, exists := tcp.tenantConfigs[tenantID]; exists {
		_, hasSection := tenantCfgs[section]
		return hasSection
	}
	return false
}
