// Package modular provides tenant functionality for multi-tenant applications.
// This file contains tenant-related types and interfaces.
package modular

import (
	"context"
)

// TenantID represents a unique tenant identifier
type TenantID string

// TenantContext is a context for tenant-aware operations
type TenantContext struct {
	context.Context
	tenantID TenantID
}

// NewTenantContext creates a new context with tenant information
func NewTenantContext(ctx context.Context, tenantID TenantID) *TenantContext {
	return &TenantContext{
		Context:  ctx,
		tenantID: tenantID,
	}
}

// GetTenantID returns the tenant ID from the context
func (tc *TenantContext) GetTenantID() TenantID {
	return tc.tenantID
}

// GetTenantIDFromContext attempts to extract tenant ID from a context
func GetTenantIDFromContext(ctx context.Context) (TenantID, bool) {
	if tc, ok := ctx.(*TenantContext); ok {
		return tc.GetTenantID(), true
	}
	return "", false
}

// TenantService provides tenant management functionality
type TenantService interface {
	// GetTenantConfig returns tenant-specific config for the given tenant and section
	GetTenantConfig(tenantID TenantID, section string) (ConfigProvider, error)

	// GetTenants returns all tenant IDs
	GetTenants() []TenantID

	// RegisterTenant registers a new tenant with optional initial configs
	RegisterTenant(tenantID TenantID, configs map[string]ConfigProvider) error

	// RegisterTenantAwareModule registers a module that wants to be notified about tenant lifecycle events
	RegisterTenantAwareModule(module TenantAwareModule) error
}

// TenantAwareModule is an optional interface that modules can implement
// to receive notifications about tenant lifecycle events
type TenantAwareModule interface {
	Module

	// OnTenantRegistered is called when a new tenant is registered
	OnTenantRegistered(tenantID TenantID)

	// OnTenantRemoved is called when a tenant is removed
	OnTenantRemoved(tenantID TenantID)
}
