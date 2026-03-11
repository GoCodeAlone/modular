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
	if cfgValue.Kind() == reflect.Pointer && cfgValue.IsNil() {
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
		return nil, fmt.Errorf("%w: %s", ErrTenantNotFound, tenantID)
	}

	cfg, exists := tenantCfgs[section]
	if !exists {
		return nil, fmt.Errorf("%w: section '%s' for tenant %s", ErrTenantConfigNotFound, section, tenantID)
	}

	if cfg == nil {
		return nil, fmt.Errorf("%w: section '%s' for tenant %s", ErrTenantConfigProviderNil, section, tenantID)
	}

	if cfg.GetConfig() == nil {
		return nil, fmt.Errorf("%w: section '%s' for tenant %s", ErrTenantConfigValueNil, section, tenantID)
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

// NewTenantConfigProviderWithIsolation creates a tenant configuration provider
// using IsolatedConfigProvider for the default config. This ensures complete
// isolation between tenants and prevents configuration pollution.
//
// This is the RECOMMENDED approach for multi-tenant applications to ensure
// each tenant gets an isolated copy of the configuration.
//
// Example:
//
//	defaultCfg := &MyConfig{Host: "localhost", Port: 8080}
//	tcp := modular.NewTenantConfigProviderWithIsolation(defaultCfg)
//	// Each tenant will get an isolated copy
func NewTenantConfigProviderWithIsolation(defaultConfig any) *TenantConfigProvider {
	return &TenantConfigProvider{
		defaultConfig: NewIsolatedConfigProvider(defaultConfig),
		tenantConfigs: make(map[TenantID]map[string]ConfigProvider),
	}
}

// NewTenantConfigProviderImmutable creates a tenant configuration provider
// using ImmutableConfigProvider for the default config. This provides thread-safe
// shared access to the default configuration across all tenants.
//
// This is suitable when:
//   - Multiple tenants share the same configuration
//   - You need thread-safe concurrent access
//   - Configuration updates should be atomic
//
// Example:
//
//	defaultCfg := &MyConfig{Host: "localhost", Port: 8080}
//	tcp := modular.NewTenantConfigProviderImmutable(defaultCfg)
//	// All tenants share the same immutable config (thread-safe)
func NewTenantConfigProviderImmutable(defaultConfig any) *TenantConfigProvider {
	return &TenantConfigProvider{
		defaultConfig: NewImmutableConfigProvider(defaultConfig),
		tenantConfigs: make(map[TenantID]map[string]ConfigProvider),
	}
}

// SetTenantConfigIsolated sets an isolated configuration for a specific tenant and section.
// The provided config will be wrapped in an IsolatedConfigProvider, ensuring that
// each access returns a deep copy.
//
// This is useful when you want to ensure tenant configurations are completely isolated
// from each other and from modifications.
//
// Example:
//
//	tcp.SetTenantConfigIsolated(tenantID, "database", &DatabaseConfig{
//	    Host: "tenant-specific-db.example.com",
//	})
func (tcp *TenantConfigProvider) SetTenantConfigIsolated(tenantID TenantID, section string, config any) {
	tcp.SetTenantConfig(tenantID, section, NewIsolatedConfigProvider(config))
}

// SetTenantConfigImmutable sets an immutable configuration for a specific tenant and section.
// The provided config will be wrapped in an ImmutableConfigProvider, providing thread-safe
// access with atomic updates.
//
// This is useful when you want thread-safe shared access to tenant configurations
// with the ability to atomically update them.
//
// Example:
//
//	tcp.SetTenantConfigImmutable(tenantID, "cache", &CacheConfig{
//	    TTL: 60 * time.Second,
//	})
func (tcp *TenantConfigProvider) SetTenantConfigImmutable(tenantID TenantID, section string, config any) {
	tcp.SetTenantConfig(tenantID, section, NewImmutableConfigProvider(config))
}
