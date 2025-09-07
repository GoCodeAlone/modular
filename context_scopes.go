package modular

import (
	"time"
)

// TenantContextData represents tenant-specific context and configuration data
// This extends the basic TenantContext with additional metadata
type TenantContextData struct {
	// TenantID is the unique identifier for this tenant
	TenantID TenantID

	// TenantConfig contains merged tenant-specific configuration
	TenantConfig map[string]interface{}

	// CreatedAt tracks when this tenant context was created
	CreatedAt time.Time

	// UpdatedAt tracks when this tenant context was last updated
	UpdatedAt time.Time

	// Active indicates if this tenant is currently active
	Active bool

	// Metadata contains additional tenant-specific metadata
	Metadata map[string]interface{}

	// ConfigProviders maps module names to tenant-specific config providers
	ConfigProviders map[string]ConfigProvider

	// Services maps service names to tenant-specific service instances
	Services map[string]interface{}
}

// InstanceContext represents instance-specific context and configuration
type InstanceContext struct {
	// InstanceID is the unique identifier for this instance
	InstanceID string

	// InstanceConfig contains merged instance-specific configuration
	InstanceConfig map[string]interface{}

	// CreatedAt tracks when this instance context was created
	CreatedAt time.Time

	// UpdatedAt tracks when this instance context was last updated
	UpdatedAt time.Time

	// Active indicates if this instance is currently active
	Active bool

	// Metadata contains additional instance-specific metadata
	Metadata map[string]interface{}

	// ConfigProviders maps module names to instance-specific config providers
	ConfigProviders map[string]ConfigProvider

	// Services maps service names to instance-specific service instances
	Services map[string]interface{}

	// ParentInstanceID references a parent instance if this is a child instance
	ParentInstanceID string
}

// ContextScope represents the scope level for configuration and services
type ContextScope string

const (
	// ContextScopeGlobal represents global scope (application-wide)
	ContextScopeGlobal ContextScope = "global"

	// ContextScopeInstance represents instance scope
	ContextScopeInstance ContextScope = "instance"

	// ContextScopeTenant represents tenant scope
	ContextScopeTenant ContextScope = "tenant"
)

// ScopedResource represents a resource that can exist at different scopes
type ScopedResource struct {
	// Name is the resource name
	Name string

	// Scope is the scope level of this resource
	Scope ContextScope

	// TenantID is set when scope is tenant
	TenantID TenantID

	// InstanceID is set when scope is instance
	InstanceID string

	// Resource is the actual resource instance
	Resource interface{}

	// CreatedAt tracks when this resource was created
	CreatedAt time.Time
}
