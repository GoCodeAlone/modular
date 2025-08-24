package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/GoCodeAlone/modular"
	"github.com/GoCodeAlone/modular/modules/eventbus"
)

// testLogger is a simple logger for the example
type testLogger struct{}

func (l *testLogger) Debug(msg string, args ...interface{}) {
	// Skip debug messages for cleaner output
}

func (l *testLogger) Info(msg string, args ...interface{}) {
	// Skip info messages for cleaner output
}

func (l *testLogger) Warn(msg string, args ...interface{}) {
	fmt.Printf("WARN: %s %v\n", msg, args)
}

func (l *testLogger) Error(msg string, args ...interface{}) {
	fmt.Printf("ERROR: %s %v\n", msg, args)
}

// AppConfig defines the main application configuration
type AppConfig struct {
	Name        string `yaml:"name" desc:"Application name"`
	Environment string `yaml:"environment" desc:"Environment (dev, staging, prod)"`
}

// UserEvent represents a user-related event
type UserEvent struct {
	UserID    string    `json:"userId"`
	Action    string    `json:"action"`
	Timestamp time.Time `json:"timestamp"`
}

// AnalyticsEvent represents an analytics event
type AnalyticsEvent struct {
	SessionID string    `json:"sessionId"`
	EventType string    `json:"eventType"`
	Page      string    `json:"page"`
	Timestamp time.Time `json:"timestamp"`
}

// SystemEvent represents a system event
type SystemEvent struct {
	Component string    `json:"component"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// NotificationEvent represents a notification event
type NotificationEvent struct {
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Priority  string    `json:"priority"`
	Timestamp time.Time `json:"timestamp"`
}

func main() {
	ctx := context.Background()

	// Create application configuration
	appConfig := &AppConfig{
		Name:        "Multi-Engine EventBus Demo",
		Environment: "development",
	}

	// Create eventbus configuration with Redis as primary external service
	// and simplified multi-engine setup
	eventbusConfig := &eventbus.EventBusConfig{
		Engines: []eventbus.EngineConfig{
			{
				Name: "memory-fast",
				Type: "memory",
				Config: map[string]interface{}{
					"maxEventQueueSize":      500,
					"defaultEventBufferSize": 10,
					"workerCount":            3,
					"retentionDays":          1,
				},
			},
			{
				Name: "redis-primary",
				Type: "redis",
				Config: map[string]interface{}{
					"url":      "redis://localhost:6379",
					"db":       0,
					"poolSize": 10,
				},
			},
			{
				Name: "memory-reliable",
				Type: "custom",
				Config: map[string]interface{}{
					"enableMetrics":          true,
					"maxEventQueueSize":      2000,
					"defaultEventBufferSize": 50,
					"metricsInterval":        "30s",
				},
			},
		},
		Routing: []eventbus.RoutingRule{
			{
				Topics: []string{"user.*", "auth.*"},
				Engine: "memory-fast",
			},
			{
				Topics: []string{"system.*", "health.*", "notifications.*"},
				Engine: "redis-primary",
			},
			{
				Topics: []string{"*"}, // Fallback for all other topics
				Engine: "memory-reliable",
			},
		},
	}

	// Initialize application
	mainConfigProvider := modular.NewStdConfigProvider(appConfig)
	app := modular.NewStdApplication(mainConfigProvider, &testLogger{})

	// Register configurations
	app.RegisterConfigSection("eventbus", modular.NewStdConfigProvider(eventbusConfig))

	// Register modules
	app.RegisterModule(eventbus.NewModule())

	// Initialize application
	err := app.Init()
	if err != nil {
		log.Fatal("Failed to initialize application:", err)
	}

	// Get services
	var eventBusService *eventbus.EventBusModule
	err = app.GetService("eventbus.provider", &eventBusService)
	if err != nil {
		log.Fatal("Failed to get eventbus service:", err)
	}

	// Start application
	err = app.Start()
	if err != nil {
		log.Fatal("Failed to start application:", err)
	}

	fmt.Printf("🚀 Started %s in %s environment\n", appConfig.Name, appConfig.Environment)
	fmt.Println("📊 Multi-Engine EventBus Configuration:")
	fmt.Println("  - memory-fast: Handles user.* and auth.* topics (in-memory, low latency)")
	fmt.Println("  - redis-primary: Handles system.*, health.*, and notifications.* topics (Redis pub/sub, distributed)")
	fmt.Println("  - memory-reliable: Handles fallback topics (in-memory with metrics)")
	fmt.Println()

	// Check if external services are available
	checkServiceAvailability(eventBusService)

	// Set up event handlers
	setupEventHandlers(ctx, eventBusService)

	// Demonstrate multi-engine event publishing
	demonstrateMultiEngineEvents(ctx, eventBusService)

	// Wait a bit for event processing
	fmt.Println("⏳ Processing events...")
	time.Sleep(2 * time.Second)

	// Show routing information
	showRoutingInfo(eventBusService)

	// Graceful shutdown with proper error handling
	fmt.Println("\n🛑 Shutting down...")

	// Create a timeout context for shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	err = app.Stop()
	if err != nil {
		// Log the error but don't exit with error code
		// External services being unavailable during shutdown is expected
		log.Printf("Warning during shutdown (this is normal if external services are unavailable): %v", err)
	}

	fmt.Println("✅ Application shutdown complete")

	// Check if shutdown context was cancelled (timeout)
	select {
	case <-shutdownCtx.Done():
		if shutdownCtx.Err() == context.DeadlineExceeded {
			log.Println("Shutdown completed within timeout")
		}
	default:
		// Shutdown completed normally
	}
}

func setupEventHandlers(ctx context.Context, eventBus *eventbus.EventBusModule) {
	fmt.Println("📡 Setting up event handlers (showing consumption patterns)...")

	// User event handlers (routed to memory-fast engine)
	eventBus.Subscribe(ctx, "user.registered", func(ctx context.Context, event eventbus.Event) error {
		userEvent := event.Payload.(UserEvent)
		fmt.Printf("📨 [CONSUMED] User registered: %s (action: %s) → memory-fast engine\n",
			userEvent.UserID, userEvent.Action)
		return nil
	})

	eventBus.Subscribe(ctx, "user.login", func(ctx context.Context, event eventbus.Event) error {
		userEvent := event.Payload.(UserEvent)
		fmt.Printf("📨 [CONSUMED] User login: %s at %s → memory-fast engine\n",
			userEvent.UserID, userEvent.Timestamp.Format("15:04:05"))
		return nil
	})

	eventBus.Subscribe(ctx, "auth.failed", func(ctx context.Context, event eventbus.Event) error {
		userEvent := event.Payload.(UserEvent)
		fmt.Printf("📨 [CONSUMED] Auth failed for user: %s → memory-fast engine\n", userEvent.UserID)
		return nil
	})

	// System event handlers (routed to redis-primary engine)
	eventBus.Subscribe(ctx, "system.health", func(ctx context.Context, event eventbus.Event) error {
		systemEvent := event.Payload.(SystemEvent)
		fmt.Printf("📨 [CONSUMED] System %s: %s - %s → redis-primary engine\n",
			systemEvent.Level, systemEvent.Component, systemEvent.Message)
		return nil
	})

	eventBus.Subscribe(ctx, "health.check", func(ctx context.Context, event eventbus.Event) error {
		systemEvent := event.Payload.(SystemEvent)
		fmt.Printf("📨 [CONSUMED] Health check: %s - %s → redis-primary engine\n",
			systemEvent.Component, systemEvent.Message)
		return nil
	})

	eventBus.Subscribe(ctx, "notifications.alert", func(ctx context.Context, event eventbus.Event) error {
		notificationEvent := event.Payload.(NotificationEvent)
		fmt.Printf("📨 [CONSUMED] Notification alert: %s - %s → redis-primary engine\n",
			notificationEvent.Type, notificationEvent.Message)
		return nil
	})

	// Fallback event handlers (routed to memory-reliable engine)
	eventBus.Subscribe(ctx, "fallback.test", func(ctx context.Context, event eventbus.Event) error {
		fmt.Printf("📨 [CONSUMED] Fallback event processed → memory-reliable engine\n")
		return nil
	})

	fmt.Println("✅ All event handlers configured and ready to consume events")
	fmt.Println()
}

func demonstrateMultiEngineEvents(ctx context.Context, eventBus *eventbus.EventBusModule) {
	fmt.Println("🎯 Publishing events to different engines based on topic routing:")
	fmt.Println("   📤 [PUBLISHED] = Event sent    📨 [CONSUMED] = Event received by handler")
	fmt.Println()

	now := time.Now()

	// User events (routed to memory-fast engine)
	fmt.Println("🔵 Memory-Fast Engine Events:")
	userEvents := []UserEvent{
		{UserID: "user123", Action: "register", Timestamp: now},
		{UserID: "user456", Action: "login", Timestamp: now.Add(1 * time.Second)},
		{UserID: "user789", Action: "failed_login", Timestamp: now.Add(2 * time.Second)},
	}

	for i, event := range userEvents {
		var topic string
		switch event.Action {
		case "register":
			topic = "user.registered"
		case "login":
			topic = "user.login"
		case "failed_login":
			topic = "auth.failed"
		}

		fmt.Printf("📤 [PUBLISHED] %s: %s\n", topic, event.UserID)
		err := eventBus.Publish(ctx, topic, event)
		if err != nil {
			fmt.Printf("Error publishing user event: %v\n", err)
		}

		if i < len(userEvents)-1 {
			time.Sleep(300 * time.Millisecond)
		}
	}

	time.Sleep(500 * time.Millisecond)
	fmt.Println()

	// System events (routed to redis-primary engine)
	fmt.Println("🔴 Redis-Primary Engine Events:")
	systemEvents := []SystemEvent{
		{Component: "database", Level: "info", Message: "Connection established", Timestamp: now},
		{Component: "cache", Level: "warning", Message: "High memory usage", Timestamp: now.Add(1 * time.Second)},
	}

	for i, event := range systemEvents {
		fmt.Printf("📤 [PUBLISHED] system.health: %s - %s\n", event.Component, event.Message)
		err := eventBus.Publish(ctx, "system.health", event)
		if err != nil {
			fmt.Printf("Error publishing system event: %v\n", err)
		}

		if i < len(systemEvents)-1 {
			time.Sleep(300 * time.Millisecond)
		}
	}

	// Health check events (also routed to redis-primary engine)
	healthEvent := SystemEvent{
		Component: "loadbalancer",
		Level:     "info",
		Message:   "All endpoints healthy",
		Timestamp: now,
	}
	fmt.Printf("📤 [PUBLISHED] health.check: %s - %s\n", healthEvent.Component, healthEvent.Message)
	err := eventBus.Publish(ctx, "health.check", healthEvent)
	if err != nil {
		fmt.Printf("Error publishing health event: %v\n", err)
	}

	time.Sleep(500 * time.Millisecond)

	// Notification events (also routed to redis-primary engine)
	notificationEvent := NotificationEvent{
		Type:      "alert",
		Message:   "System resource usage high",
		Priority:  "medium",
		Timestamp: now,
	}
	fmt.Printf("📤 [PUBLISHED] notifications.alert: %s - %s\n", notificationEvent.Type, notificationEvent.Message)
	err = eventBus.Publish(ctx, "notifications.alert", notificationEvent)
	if err != nil {
		fmt.Printf("Error publishing notification event: %v\n", err)
	}

	time.Sleep(500 * time.Millisecond)
	fmt.Println()

	// Fallback events (routed to memory-reliable engine)
	fmt.Println("🟡 Memory-Reliable Engine (Fallback):")
	fmt.Printf("📤 [PUBLISHED] fallback.test: sample fallback event\n")
	err = eventBus.Publish(ctx, "fallback.test", map[string]interface{}{
		"message":   "This event uses the fallback engine",
		"timestamp": now,
	})
	if err != nil {
		fmt.Printf("Error publishing fallback event: %v\n", err)
	}
}

func showRoutingInfo(eventBus *eventbus.EventBusModule) {
	fmt.Println()
	fmt.Println("📋 Event Bus Routing Information:")

	// Show how different topics are routed
	topics := []string{
		"user.registered", "user.login", "auth.failed",
		"system.health", "health.check", "notifications.alert",
		"fallback.test", "random.topic",
	}

	if eventBus != nil && eventBus.GetRouter() != nil {
		for _, topic := range topics {
			engine := eventBus.GetRouter().GetEngineForTopic(topic)
			fmt.Printf("  %s -> %s\n", topic, engine)
		}
	}

	// Show active topics and subscriber counts
	activeTopics := eventBus.Topics()
	if len(activeTopics) > 0 {
		fmt.Println()
		fmt.Println("📊 Active Topics and Subscriber Counts:")
		for _, topic := range activeTopics {
			count := eventBus.SubscriberCount(topic)
			fmt.Printf("  %s: %d subscribers\n", topic, count)
		}
	}
}

func checkServiceAvailability(eventBus *eventbus.EventBusModule) {
	fmt.Println("🔍 Checking external service availability:")

	// Check Redis connectivity directly
	redisAvailable := false
	if conn, err := net.DialTimeout("tcp", "localhost:6379", 2*time.Second); err == nil {
		conn.Close()
		redisAvailable = true
	}

	if redisAvailable {
		fmt.Println("  ✅ Redis service is reachable on localhost:6379")

		// Now check if the EventBus router is using Redis
		if eventBus != nil && eventBus.GetRouter() != nil {
			redisTopics := []string{"system.test", "health.test", "notifications.test"}
			routedToRedis := false

			for _, topic := range redisTopics {
				engineName := eventBus.GetRouter().GetEngineForTopic(topic)
				if engineName == "redis-primary" {
					routedToRedis = true
					break
				}
			}

			if routedToRedis {
				fmt.Println("  ✅ EventBus router is correctly routing to redis-primary engine")
			} else {
				fmt.Println("  ⚠️ EventBus router is not routing to redis-primary (engine may have failed to start)")
			}
		}
	} else {
		fmt.Println("  ❌ Redis service not reachable, system/health/notifications events will route to fallback")
		fmt.Println("  💡 To enable Redis: docker run -d -p 6379:6379 redis:alpine")
	}
	fmt.Println()
}
