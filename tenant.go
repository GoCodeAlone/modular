// Package modular provides tenant functionality for multi-tenant applications.
// This file contains tenant-related types and interfaces.
//
// The tenant functionality enables a single application instance to serve
// multiple isolated tenants, each with their own configuration, data, and
// potentially customized behavior.
//
// Key concepts:
//   - TenantID: unique identifier for each tenant
//   - TenantContext: context that carries tenant information through the call chain
//   - TenantService: manages tenant registration and configuration
//   - TenantAwareModule: modules that can adapt their behavior per tenant
//
// Example multi-tenant application setup:
//
//	// Create tenant service
//	tenantSvc := modular.NewStandardTenantService(logger)
//
//	// Register tenant service
//	app.RegisterService("tenantService", tenantSvc)
//
//	// Register tenant-aware modules
//	app.RegisterModule(&MyTenantAwareModule{})
//
//	// Register tenants with specific configurations
//	tenantSvc.RegisterTenant("tenant-1", map[string]ConfigProvider{
//	    "database": modular.NewStdConfigProvider(&DatabaseConfig{Host: "tenant1-db"}),
//	})
package modular

import (
	"context"
)

// TenantID represents a unique tenant identifier.
// Tenant IDs should be stable, unique strings that identify tenants
// throughout the application lifecycle. Common patterns include:
//   - Customer IDs: "customer-12345"
//   - Domain names: "example.com"
//   - UUIDs: "550e8400-e29b-41d4-a716-446655440000"
type TenantID string

// TenantContext is a context for tenant-aware operations.
// It extends the standard Go context.Context interface to carry tenant
// identification through the call chain, enabling tenant-specific behavior
// in modules and services.
//
// TenantContext should be used whenever performing operations that need
// to be tenant-specific, such as database queries, configuration lookups,
// or service calls.
type TenantContext struct {
	context.Context
	tenantID TenantID
}

// NewTenantContext creates a new context with tenant information.
// The returned context carries the tenant ID and can be used throughout
// the application to identify which tenant an operation belongs to.
//
// Example:
//
//	tenantCtx := modular.NewTenantContext(ctx, "customer-123")
//	result, err := tenantAwareService.DoSomething(tenantCtx, data)
func NewTenantContext(ctx context.Context, tenantID TenantID) *TenantContext {
	return &TenantContext{
		Context:  ctx,
		tenantID: tenantID,
	}
}

// GetTenantID returns the tenant ID from the context.
// This allows modules and services to determine which tenant
// the current operation is for.
func (tc *TenantContext) GetTenantID() TenantID {
	return tc.tenantID
}

// GetTenantIDFromContext attempts to extract tenant ID from a context.
// Returns the tenant ID and true if the context is a TenantContext,
// or empty string and false if it's not a tenant-aware context.
//
// This is useful for functions that may or may not receive a tenant context:
//
//	if tenantID, ok := modular.GetTenantIDFromContext(ctx); ok {
//	    // Handle tenant-specific logic
//	} else {
//	    // Handle default/non-tenant logic
//	}
func GetTenantIDFromContext(ctx context.Context) (TenantID, bool) {
	if tc, ok := ctx.(*TenantContext); ok {
		return tc.GetTenantID(), true
	}
	return "", false
}

// TenantService provides tenant management functionality.
// The tenant service is responsible for:
//   - Managing tenant registration and lifecycle
//   - Providing tenant-specific configuration
//   - Notifying modules about tenant events
//   - Coordinating tenant-aware operations
//
// Applications that need multi-tenant functionality should register
// a TenantService implementation as a service named "tenantService".
type TenantService interface {
	// GetTenantConfig returns tenant-specific configuration for the given tenant and section.
	// This method looks up configuration that has been specifically registered for
	// the tenant, falling back to default configuration if tenant-specific config
	// is not available.
	//
	// The section parameter identifies which configuration section to retrieve
	// (e.g., "database", "cache", "api").
	//
	// Example:
	//   cfg, err := tenantSvc.GetTenantConfig("tenant-123", "database")
	//   if err != nil {
	//       return err
	//   }
	//   dbConfig := cfg.GetConfig().(*DatabaseConfig)
	GetTenantConfig(tenantID TenantID, section string) (ConfigProvider, error)

	// GetTenants returns all registered tenant IDs.
	// This is useful for operations that need to iterate over all tenants,
	// such as maintenance tasks, reporting, or health checks.
	//
	// Example:
	//   for _, tenantID := range tenantSvc.GetTenants() {
	//       // Perform operation for each tenant
	//       err := performMaintenanceForTenant(tenantID)
	//   }
	GetTenants() []TenantID

	// RegisterTenant registers a new tenant with optional initial configurations.
	// The configs map provides tenant-specific configuration for different sections.
	// If a section is not provided in the configs map, the tenant will use the
	// default application configuration for that section.
	//
	// Example:
	//   tenantConfigs := map[string]ConfigProvider{
	//       "database": modular.NewStdConfigProvider(&DatabaseConfig{
	//           Host: "tenant-specific-db.example.com",
	//       }),
	//       "cache": modular.NewStdConfigProvider(&CacheConfig{
	//           Prefix: "tenant-123:",
	//       }),
	//   }
	//   err := tenantSvc.RegisterTenant("tenant-123", tenantConfigs)
	RegisterTenant(tenantID TenantID, configs map[string]ConfigProvider) error

	// RegisterTenantAwareModule registers a module that wants to be notified about tenant lifecycle events.
	// Modules implementing the TenantAwareModule interface can register to receive
	// notifications when tenants are added or removed, allowing them to perform
	// tenant-specific initialization or cleanup.
	//
	// This is typically called automatically by the application framework during
	// module initialization, but can also be called directly if needed.
	//
	// Example:
	//   module := &MyTenantAwareModule{}
	//   err := tenantSvc.RegisterTenantAwareModule(module)
	RegisterTenantAwareModule(module TenantAwareModule) error
}

// TenantAwareModule is an optional interface that modules can implement
// to receive notifications about tenant lifecycle events.
//
// Modules implementing this interface will be automatically registered
// with the tenant service during application initialization, and will
// receive callbacks when tenants are added or removed.
//
// This enables modules to:
//   - Initialize tenant-specific resources when tenants are added
//   - Clean up tenant-specific resources when tenants are removed
//   - Maintain tenant-specific caches or connections
//   - Perform tenant-specific migrations or setup
//
// Example implementation:
//
//	type MyModule struct {
//	    tenantConnections map[TenantID]*Connection
//	}
//
//	func (m *MyModule) OnTenantRegistered(tenantID TenantID) {
//	    // Initialize tenant-specific resources
//	    conn := createConnectionForTenant(tenantID)
//	    m.tenantConnections[tenantID] = conn
//	}
//
//	func (m *MyModule) OnTenantRemoved(tenantID TenantID) {
//	    // Clean up tenant-specific resources
//	    if conn, ok := m.tenantConnections[tenantID]; ok {
//	        conn.Close()
//	        delete(m.tenantConnections, tenantID)
//	    }
//	}
type TenantAwareModule interface {
	Module

	// OnTenantRegistered is called when a new tenant is registered.
	// This method should be used to initialize any tenant-specific resources,
	// such as database connections, caches, or configuration.
	//
	// The method should be non-blocking and handle errors gracefully.
	// If initialization fails, the module should log the error but not
	// prevent the tenant registration from completing.
	OnTenantRegistered(tenantID TenantID)

	// OnTenantRemoved is called when a tenant is removed.
	// This method should be used to clean up any tenant-specific resources
	// to prevent memory leaks or resource exhaustion.
	//
	// The method should be non-blocking and handle cleanup failures gracefully.
	// Even if cleanup fails, the tenant removal should proceed.
	OnTenantRemoved(tenantID TenantID)
}
