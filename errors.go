package modular

import (
	"errors"
)

// Common error definitions for the modular framework.
// These errors are organized into categories for different subsystems:
//
// Configuration errors: Issues with loading, parsing, or validating configuration
// Service registry errors: Problems with service registration and lookup
// Module lifecycle errors: Errors during module initialization, startup, or shutdown
// Dependency resolution errors: Issues with resolving module or service dependencies
// Tenant management errors: Problems with multi-tenant functionality
//
// All errors follow Go 1.13+ error wrapping conventions and can be used
// with errors.Is() and errors.As() for error handling and testing.

// Application errors
var (
	// Configuration errors - issues with loading and managing configuration
	ErrConfigSectionNotFound = errors.New("config section not found")
	ErrApplicationNil        = errors.New("application is nil")
	ErrConfigProviderNil     = errors.New("failed to load app config: config provider is nil")
	ErrConfigSectionError    = errors.New("failed to load app config: error triggered by section")

	// Config validation errors - problems with configuration structure and values
	ErrConfigNil                  = errors.New("config is nil")
	ErrConfigNotPointer           = errors.New("config must be a pointer")
	ErrConfigNotStruct            = errors.New("config must be a struct")
	ErrConfigRequiredFieldMissing = errors.New("required field is missing")
	ErrConfigValidationFailed     = errors.New("config validation failed")
	ErrUnsupportedTypeForDefault  = errors.New("unsupported type for default value")
	ErrDefaultValueParseError     = errors.New("failed to parse default value")
	ErrDefaultValueOverflowsInt   = errors.New("default value overflows int")
	ErrDefaultValueOverflowsUint  = errors.New("default value overflows uint")
	ErrDefaultValueOverflowsFloat = errors.New("default value overflows float")
	ErrInvalidFieldKind           = errors.New("invalid field kind")
	ErrIncompatibleFieldKind      = errors.New("incompatible field kind")
	ErrUnexpectedFieldKind        = errors.New("unexpected field kind")
	ErrUnsupportedFormatType      = errors.New("unsupported format type")
	ErrConfigFeederError          = errors.New("config feeder error")
	ErrConfigSetupError           = errors.New("config setup error")
	ErrConfigNilPointer           = errors.New("config is nil pointer")
	ErrFieldCannotBeSet           = errors.New("field cannot be set")

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

	// Constructor errors
	ErrConstructorNotFunction              = errors.New("constructor must be a function")
	ErrConstructorInvalidReturnCount       = errors.New("constructor must return exactly two values (Module, error)")
	ErrConstructorInvalidReturnType        = errors.New("constructor must return a Module as first value")
	ErrConstructorParameterServiceNotFound = errors.New("no service found for constructor parameter")
	ErrInvalidInterfaceConfiguration       = errors.New("invalid interface configuration for required service")
	ErrInterfaceConfigurationNil           = errors.New("SatisfiesInterface is nil")
	ErrInterfaceConfigurationNotInterface  = errors.New("SatisfiesInterface is not an interface type")
	ErrServiceInterfaceIncompatible        = errors.New("service does not implement required interface")

	// Tenant errors
	ErrAppContextNotInitialized        = errors.New("application context not initialized")
	ErrTenantNotFound                  = errors.New("tenant not found")
	ErrTenantConfigNotFound            = errors.New("tenant config section not found")
	ErrTenantConfigProviderNil         = errors.New("tenant config provider is nil")
	ErrTenantConfigValueNil            = errors.New("tenant config value is nil")
	ErrTenantRegisterNilConfig         = errors.New("cannot register nil config for tenant")
	ErrMockTenantConfigsNotInitialized = errors.New("mock tenant configs not initialized")
	ErrConfigSectionNotFoundForTenant  = errors.New("config section not found for tenant")

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

	// Additional tenant config errors
	ErrTenantSectionConfigNil     = errors.New("tenant section config is nil after feeding")
	ErrCreatedNilProvider         = errors.New("created nil provider for tenant section")
	ErrIncompatibleFieldTypes     = errors.New("incompatible types for field assignment")
	ErrIncompatibleInterfaceValue = errors.New("incompatible interface value for field")
)
