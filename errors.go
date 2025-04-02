package modular

import (
	"errors"
)

// Application errors
var (
	// Configuration errors
	ErrConfigSectionNotFound = errors.New("config section not found")
	ErrConfigFeederError     = errors.New("config: feeder error")
	ErrConfigSetupError      = errors.New("config: setup error")
	ErrConfigNilPointer      = errors.New("cannot create temp config: config pointer is nil")
	ErrConfigNil             = errors.New("cannot create temp config: config is nil")
	ErrApplicationNil        = errors.New("application is nil")
	ErrConfigProviderNil     = errors.New("failed to load app config: config provider is nil")
	ErrConfigSectionError    = errors.New("failed to load app config: error triggered by section")

	// Service registry errors
	ErrServiceAlreadyRegistered = errors.New("service already registered")
	ErrServiceNotFound          = errors.New("service not found")

	// Service injection errors
	ErrTargetNotPointer      = errors.New("target must be a non-nil pointer")
	ErrTargetValueInvalid    = errors.New("target value is invalid")
	ErrServiceIncompatible   = errors.New("service cannot be assigned to target")
	ErrServiceNil            = errors.New("service is nil")
	ErrServiceWrongType      = errors.New("service doesn't satisfy required type")
	ErrServiceWrongInterface = errors.New("service doesn't satisfy required interface")

	// Dependency resolution errors
	ErrCircularDependency      = errors.New("circular dependency detected")
	ErrModuleDependencyMissing = errors.New("module depends on non-existent module")
	ErrRequiredServiceNotFound = errors.New("required service not found for module")

	// Tenant errors
	ErrAppContextNotInitialized = errors.New("application context not initialized")
	ErrTenantNotFound           = errors.New("tenant not found")
	ErrTenantConfigNotFound     = errors.New("tenant config section not found")
	ErrTenantConfigProviderNil  = errors.New("tenant config provider is nil")
	ErrTenantConfigValueNil     = errors.New("tenant config value is nil")
	ErrTenantRegisterNilConfig  = errors.New("cannot register nil config for tenant")

	// Test-specific errors
	ErrSetupFailed   = errors.New("setup error")
	ErrFeedFailed    = errors.New("feed error")
	ErrFeedKeyFailed = errors.New("feedKey error")

	// Tenant config errors
	ErrConfigCastFailed      = errors.New("failed to cast config to expected type")
	ErrOriginalOrLoadedNil   = errors.New("original or loaded config is nil")
	ErrDestinationNotPointer = errors.New("destination must be a pointer")
	ErrCannotCopyMapToStruct = errors.New("cannot copy from map to non-struct")
	ErrUnsupportedSourceType = errors.New("unsupported source type")
)
