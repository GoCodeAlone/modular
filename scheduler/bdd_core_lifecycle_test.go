package scheduler

import (
	"fmt"
	"time"

	"github.com/GoCodeAlone/modular"
)

// Core module lifecycle step implementations

func (ctx *SchedulerBDDTestContext) iHaveAModularApplicationWithSchedulerModuleConfigured() error {
	ctx.resetContext()

	// Create basic scheduler configuration for testing
	ctx.config = &SchedulerConfig{
		WorkerCount:        3,
		QueueSize:          100,
		CheckInterval:      10 * time.Millisecond,
		ShutdownTimeout:    30 * time.Second,
		StorageType:        "memory",
		RetentionDays:      1,
		PersistenceBackend: PersistenceBackendNone,
	}

	// Create application
	logger := &testLogger{}

	mainConfigProvider := modular.NewStdConfigProvider(struct{}{})
	ctx.app = modular.NewStdApplication(mainConfigProvider, logger)
	// Ensure per-app feeder isolation without mutating global feeders.
	if cfSetter, ok := ctx.app.(interface{ SetConfigFeeders([]modular.Feeder) }); ok {
		cfSetter.SetConfigFeeders([]modular.Feeder{})
	}

	// Create and register scheduler module
	module := NewModule()
	ctx.module = module.(*SchedulerModule)

	// Register the scheduler config section
	schedulerConfigProvider := modular.NewStdConfigProvider(ctx.config)
	ctx.app.RegisterConfigSection("scheduler", schedulerConfigProvider)

	// Register the module
	ctx.app.RegisterModule(ctx.module)

	return nil
}

func (ctx *SchedulerBDDTestContext) theSchedulerModuleIsInitialized() error {
	err := ctx.app.Init()
	if err != nil {
		ctx.lastError = err
		return err
	}
	return nil
}

func (ctx *SchedulerBDDTestContext) theSchedulerServiceShouldBeAvailable() error {
	err := ctx.app.GetService("scheduler.provider", &ctx.service)
	if err != nil {
		return err
	}
	if ctx.service == nil {
		return fmt.Errorf("scheduler service not available")
	}

	// For testing purposes, ensure we use the same instance as the module
	// This works around potential service resolution issues
	if ctx.module != nil {
		ctx.service = ctx.module
	}

	return nil
}

func (ctx *SchedulerBDDTestContext) theModuleShouldBeReadyToScheduleJobs() error {
	// Verify the module is properly configured
	if ctx.service == nil || ctx.service.config == nil {
		return fmt.Errorf("module not properly initialized")
	}
	return nil
}
