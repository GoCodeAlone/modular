package modular

import (
	"context"
	"errors"
	"time"
)

// Reloadable defines the interface for modules that support dynamic configuration reloading.
// Modules implementing this interface can have their configuration updated at runtime
// without requiring a full application restart.
//
// This interface follows the design brief specification for FR-045 Dynamic Reload,
// using the ConfigChange structure to provide detailed information about what
// configuration fields have changed, including their previous and new values.
//
// Reload operations must be:
//   - Idempotent: calling Reload multiple times with the same changes should be safe
//   - Fast: operations should typically complete in <50ms to avoid blocking
//   - Atomic: either fully apply all changes or leave existing config unchanged on failure
type Reloadable interface {
	// Reload applies configuration changes to the module.
	// The changes parameter contains a slice of ConfigChange objects that
	// describe exactly what configuration fields have changed, along with
	// their old and new values.
	//
	// Implementations should:
	//   - Check context cancellation/timeout regularly
	//   - Validate all configuration changes before applying any
	//   - Apply changes atomically (all or nothing)
	//   - Preserve existing configuration on failure
	//   - Return meaningful errors for debugging
	//
	// Only fields tagged with `dynamic:"true"` will be included in the changes.
	// The context may have a timeout set based on ReloadTimeout().
	Reload(ctx context.Context, changes []ConfigChange) error

	// CanReload returns true if this module supports dynamic reloading.
	// This allows for compile-time or runtime determination of reload capability.
	//
	// Modules may return false if:
	//   - They require restart for configuration changes
	//   - They are in a state where reloading is temporarily unsafe
	//   - The current configuration doesn't support dynamic changes
	CanReload() bool

	// ReloadTimeout returns the maximum time the module needs to complete a reload.
	// This is used by the application to set appropriate context timeouts.
	//
	// Typical values:
	//   - Simple config changes: 1-5 seconds
	//   - Database reconnections: 10-30 seconds
	//   - Complex reconfigurations: 30-60 seconds
	//
	// A zero duration indicates the module will use a reasonable default.
	ReloadTimeout() time.Duration
}

// ReloadableLegacy defines the legacy interface for backward compatibility.
// New modules should implement Reloadable instead.
//
// Deprecated: Use Reloadable interface instead. This interface is maintained
// for backward compatibility but will be removed in a future version.
type ReloadableLegacy interface {
	// Reload applies configuration changes to the module using the legacy interface.
	Reload(ctx context.Context, newConfig interface{}) error

	// CanReload returns true if this module supports dynamic reloading.
	CanReload() bool

	// ReloadTimeout returns the maximum time the module needs to complete a reload.
	ReloadTimeout() time.Duration
}

// Additional errors for reload operations
var (
	// ErrReloadInProgress indicates that a reload operation is already in progress
	ErrReloadInProgress = errors.New("reload operation already in progress")

	// ErrReloadTimeout indicates that the reload operation exceeded its timeout
	ErrReloadTimeout = errors.New("reload operation timed out")
)
