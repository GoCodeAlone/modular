package modular

import (
	"time"
)

// ModuleCore represents the core module metadata and state
// This skeleton provides fields as specified in the data model
type ModuleCore struct {
	// Name is the unique identifier for this module
	Name string

	// Version is the module version
	Version string

	// DeclaredDependencies lists the dependencies this module requires
	DeclaredDependencies []DependencyDeclaration

	// ProvidesServices lists the services this module provides
	ProvidesServices []ServiceDeclaration

	// ConfigSpec contains schema metadata for this module's configuration
	ConfigSpec *ConfigurationSchema

	// DynamicFields lists configuration keys that support hot-reload
	DynamicFields []string

	// RegisteredAt tracks when this module was registered
	RegisteredAt time.Time

	// InitializedAt tracks when this module was initialized
	InitializedAt *time.Time

	// StartedAt tracks when this module was started (if Startable)
	StartedAt *time.Time

	// Status tracks the current module status
	Status ModuleStatus
}

// DependencyDeclaration represents a declared dependency
type DependencyDeclaration struct {
	// Name is the service name or interface name
	Name string

	// Optional indicates if this dependency is optional
	Optional bool

	// InterfaceType is the Go interface type if dependency is interface-based
	InterfaceType string
}

// ServiceDeclaration represents a service provided by a module
type ServiceDeclaration struct {
	// Name is the service name
	Name string

	// InterfaceType is the Go interface type this service implements
	InterfaceType string

	// Scope indicates the service scope (global, tenant, instance)
	Scope ServiceScope
}

// ServiceScope represents the scope of a service
type ServiceScope string

const (
	// ServiceScopeGlobal indicates a globally available service
	ServiceScopeGlobal ServiceScope = "global"

	// ServiceScopeTenant indicates a tenant-scoped service
	ServiceScopeTenant ServiceScope = "tenant"

	// ServiceScopeInstance indicates an instance-scoped service
	ServiceScopeInstance ServiceScope = "instance"
)

// ModuleStatus represents the current status of a module
type ModuleStatus string

const (
	// ModuleStatusRegistered indicates the module is registered
	ModuleStatusRegistered ModuleStatus = "registered"

	// ModuleStatusInitialized indicates the module is initialized
	ModuleStatusInitialized ModuleStatus = "initialized"

	// ModuleStatusStarted indicates the module is started
	ModuleStatusStarted ModuleStatus = "started"

	// ModuleStatusStopped indicates the module is stopped
	ModuleStatusStopped ModuleStatus = "stopped"

	// ModuleStatusError indicates the module encountered an error
	ModuleStatusError ModuleStatus = "error"
)
