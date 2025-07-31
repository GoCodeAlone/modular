package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/CrisisTextLine/modular"
	"github.com/CrisisTextLine/modular/modules/eventlogger"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

func main() {
	// Create logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Create a simple tenant loader
	tenantLoader := &SimpleTenantLoader{}

	// Create application using the new builder API
	app, err := modular.NewApplication(
		modular.WithLogger(logger),
		modular.WithConfigProvider(modular.NewStdConfigProvider(&AppConfig{})),
		modular.WithConfigDecorators(
			modular.InstanceAwareConfig(),
			modular.TenantAwareConfigDecorator(tenantLoader),
		),
		modular.WithTenantAware(tenantLoader),
		modular.WithObserver(customEventObserver),
		modular.WithModules(
			eventlogger.NewModule(),
			&DemoModule{},
		),
	)

	if err != nil {
		logger.Error("Failed to create application", "error", err)
		os.Exit(1)
	}

	// Initialize and start the application
	if err := app.Init(); err != nil {
		logger.Error("Failed to initialize application", "error", err)
		os.Exit(1)
	}

	if err := app.Start(); err != nil {
		logger.Error("Failed to start application", "error", err)
		os.Exit(1)
	}

	// Simulate some work and event emission
	time.Sleep(2 * time.Second)

	// Stop the application
	if err := app.Stop(); err != nil {
		logger.Error("Failed to stop application", "error", err)
		os.Exit(1)
	}

	logger.Info("Observer demo completed successfully")
}

// AppConfig demonstrates configuration structure
type AppConfig struct {
	AppName string `yaml:"appName" default:"Observer Demo App" desc:"Application name"`
	Debug   bool   `yaml:"debug" default:"true" desc:"Enable debug mode"`
}

// SimpleTenantLoader implements TenantLoader for demo purposes
type SimpleTenantLoader struct{}

func (l *SimpleTenantLoader) LoadTenants() ([]modular.Tenant, error) {
	return []modular.Tenant{
		{ID: "demo-tenant-1", Name: "Demo Tenant 1"},
		{ID: "demo-tenant-2", Name: "Demo Tenant 2"},
	}, nil
}

// customEventObserver is a functional observer that logs events
func customEventObserver(ctx context.Context, event cloudevents.Event) error {
	fmt.Printf("🔔 Custom Observer: Received event [%s] from [%s] at [%s]\n",
		event.Type(), event.Source(), event.Time().Format(time.RFC3339))
	return nil
}

// DemoModule demonstrates a module that emits events
type DemoModule struct{}

func (m *DemoModule) Name() string {
	return "demo-module"
}

func (m *DemoModule) Init(app modular.Application) error {
	// Register as an observer if the app supports it
	if subject, ok := app.(modular.Subject); ok {
		observer := modular.NewFunctionalObserver("demo-module-observer", m.handleEvent)
		return subject.RegisterObserver(observer, "com.modular.application.after.start")
	}
	return nil
}

func (m *DemoModule) handleEvent(ctx context.Context, event cloudevents.Event) error {
	if event.Type() == "com.modular.application.after.start" {
		fmt.Printf("🚀 DemoModule: Application started! Emitting custom event...\n")
		
		// Create a custom event
		customEvent := modular.NewCloudEvent(
			"com.demo.module.message",
			"demo-module",
			map[string]string{"message": "Hello from DemoModule!"},
			map[string]interface{}{"timestamp": time.Now().Format(time.RFC3339)},
		)

		// Emit the event if the app supports it
		if subject, ok := ctx.Value("app").(modular.Subject); ok {
			return subject.NotifyObservers(ctx, customEvent)
		}
	}
	return nil
}