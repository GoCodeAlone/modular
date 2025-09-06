package modular

import (
	"time"
)

// ApplicationCore represents the core application state and metadata
// This skeleton provides fields as specified in the data model
type ApplicationCore struct {
	// RegisteredModules contains all modules registered with the application
	RegisteredModules []Module

	// ServiceRegistry provides access to the application's service registry
	ServiceRegistry ServiceRegistry

	// TenantContexts maps tenant IDs to their context data
	TenantContexts map[TenantID]*TenantContextData

	// InstanceContexts maps instance IDs to their contexts
	InstanceContexts map[string]*InstanceContext

	// Observers contains all registered observers for lifecycle events
	Observers []Observer

	// StartedAt tracks when the application was started
	StartedAt *time.Time

	// Status tracks the current application status
	Status ApplicationStatus
}

// ApplicationStatus represents the current status of the application
type ApplicationStatus string

const (
	// ApplicationStatusStopped indicates the application is stopped
	ApplicationStatusStopped ApplicationStatus = "stopped"

	// ApplicationStatusStarting indicates the application is starting up
	ApplicationStatusStarting ApplicationStatus = "starting"

	// ApplicationStatusRunning indicates the application is running
	ApplicationStatusRunning ApplicationStatus = "running"

	// ApplicationStatusStopping indicates the application is shutting down
	ApplicationStatusStopping ApplicationStatus = "stopping"
)
