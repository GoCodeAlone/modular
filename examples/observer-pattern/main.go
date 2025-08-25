package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/CrisisTextLine/modular"
	"github.com/CrisisTextLine/modular/feeders"
	"github.com/CrisisTextLine/modular/modules/eventlogger"
)

func main() {
	// Generate sample config file if requested
	if len(os.Args) > 1 && os.Args[1] == "--generate-config" {
		format := "yaml"
		if len(os.Args) > 2 {
			format = os.Args[2]
		}
		outputFile := "config-sample." + format
		if len(os.Args) > 3 {
			outputFile = os.Args[3]
		}

		cfg := &AppConfig{}
		if err := modular.SaveSampleConfig(cfg, format, outputFile); err != nil {
			fmt.Printf("Error generating sample config: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Sample config generated at %s\n", outputFile)
		os.Exit(0)
	}

	// Create observable application with observer pattern support
	app := modular.NewObservableApplication(
		modular.NewStdConfigProvider(&AppConfig{}),
		slog.New(slog.NewTextHandler(
			os.Stdout,
			&slog.HandlerOptions{Level: slog.LevelDebug},
		)),
	)
	// ObservableApplication embeds *StdApplication, so access directly
	app.StdApplication.SetConfigFeeders([]modular.Feeder{
		feeders.NewYamlFeeder("config.yaml"),
		feeders.NewEnvFeeder(),
	})

	fmt.Println("🔍 Observer Pattern Demo - Starting Application")
	fmt.Println("==================================================")

	// Register the event logger module first (it will auto-register as observer)
	fmt.Println("\n📝 Registering EventLogger module...")
	app.RegisterModule(eventlogger.NewModule())

	// Register demo modules to show observer pattern in action
	fmt.Println("\n🏗️  Registering demo modules...")
	app.RegisterModule(NewUserModule())
	app.RegisterModule(NewNotificationModule())
	app.RegisterModule(NewAuditModule())

	// Register CloudEvents demo module
	fmt.Println("\n☁️  Registering CloudEvents demo module...")
	app.RegisterModule(NewCloudEventsModule())

	// Register demo services
	fmt.Println("\n🔧 Registering demo services...")
	if err := app.RegisterService("userStore", &UserStore{users: make(map[string]*User)}); err != nil {
		panic(err)
	}
	if err := app.RegisterService("emailService", &EmailService{}); err != nil {
		panic(err)
	}

	// Initialize application - this will trigger many observable events
	fmt.Println("\n🚀 Initializing application (watch for logged events)...")
	if err := app.Init(); err != nil {
		fmt.Printf("❌ Application initialization failed: %v\n", err)
		os.Exit(1)
	}

	// Start application - more observable events
	fmt.Println("\n▶️  Starting application...")
	if err := app.Start(); err != nil {
		fmt.Printf("❌ Application start failed: %v\n", err)
		os.Exit(1)
	}

	// Demonstrate manual event emission by modules
	fmt.Println("\n👤 Triggering user-related events...")

	// Get the user module to trigger events - but it needs to be the same instance
	// The module that was registered should have the subject reference
	// Let's trigger events directly through the app instead

	// First, let's test that the module received the subject reference
	fmt.Println("📋 Testing CloudEvent emission capabilities...")

	// Create a test CloudEvent directly through the application
	testEvent := modular.NewCloudEvent(
		"com.example.user.created",
		"test-source",
		map[string]interface{}{
			"userID": "test-user",
			"email":  "test@example.com",
		},
		map[string]interface{}{
			"test": "true",
		},
	)

	if err := app.NotifyObservers(context.Background(), testEvent); err != nil {
		fmt.Printf("❌ Failed to emit test event: %v\n", err)
	} else {
		fmt.Println("✅ Test event emitted successfully!")
	}

	// Demonstrate more CloudEvents
	fmt.Println("\n☁️  Testing additional CloudEvents emission...")
	testCloudEvent := modular.NewCloudEvent(
		"com.example.user.login",
		"authentication-service",
		map[string]interface{}{
			"userID":    "cloud-user",
			"email":     "cloud@example.com",
			"loginTime": time.Now(),
		},
		map[string]interface{}{
			"sourceip":  "192.168.1.1",
			"useragent": "test-browser",
		},
	)

	if err := app.NotifyObservers(context.Background(), testCloudEvent); err != nil {
		fmt.Printf("❌ Failed to emit CloudEvent: %v\n", err)
	} else {
		fmt.Println("✅ CloudEvent emitted successfully!")
	}

	// Wait a moment for async processing
	time.Sleep(200 * time.Millisecond)

	// Show observer info
	fmt.Println("\n📊 Current Observer Information:")
	observers := app.GetObservers()
	for _, observer := range observers {
		fmt.Printf("  - %s (Event Types: %v)\n", observer.ID, observer.EventTypes)
	}

	// Graceful shutdown - more observable events
	fmt.Println("\n⏹️  Stopping application...")
	if err := app.Stop(); err != nil {
		fmt.Printf("❌ Application stop failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n✅ Observer Pattern Demo completed successfully!")
	fmt.Println("Check the event logs above to see all the Observer pattern events.")
}

// AppConfig demonstrates configuration with observer pattern settings
type AppConfig struct {
	AppName         string                        `yaml:"appName" default:"Observer Pattern Demo" desc:"Application name"`
	Environment     string                        `yaml:"environment" default:"demo" desc:"Environment (dev, test, prod, demo)"`
	EventLogger     eventlogger.EventLoggerConfig `yaml:"eventlogger" desc:"Event logger configuration"`
	UserModule      UserModuleConfig              `yaml:"userModule" desc:"User module configuration"`
	CloudEventsDemo CloudEventsConfig             `yaml:"cloudevents-demo" desc:"CloudEvents demo configuration"`
}

// Validate implements the ConfigValidator interface
func (c *AppConfig) Validate() error {
	validEnvs := map[string]bool{"dev": true, "test": true, "prod": true, "demo": true}
	if !validEnvs[c.Environment] {
		return errInvalidEnvironment
	}
	return nil
}
