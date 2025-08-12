package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/CrisisTextLine/modular"
	"github.com/CrisisTextLine/modular/modules/eventbus"
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

func main() {
	ctx := context.Background()

	// Create application configuration
	appConfig := &AppConfig{
		Name:        "Multi-Engine EventBus Demo",
		Environment: "development",
	}

	// Create eventbus configuration with multiple engines and routing
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
				Name: "redis-durable",
				Type: "redis",
				Config: map[string]interface{}{
					"url":      "redis://localhost:6379",
					"db":       0,
					"poolSize": 10,
				},
			},
			{
				Name: "kafka-analytics",
				Type: "kafka",
				Config: map[string]interface{}{
					"brokers": []string{"localhost:9092"},
					"groupId": "multi-engine-demo",
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
				Topics: []string{"analytics.*", "metrics.*"},
				Engine: "kafka-analytics",
			},
			{
				Topics: []string{"system.*", "health.*"},
				Engine: "redis-durable",
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
	fmt.Println("  - kafka-analytics: Handles analytics.* and metrics.* topics (distributed, persistent)")
	fmt.Println("  - redis-durable: Handles system.* and health.* topics (Redis pub/sub, persistent)")
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

	// Graceful shutdown
	fmt.Println("\n🛑 Shutting down...")
	err = app.Stop()
	if err != nil {
		log.Printf("Error during shutdown: %v", err)
		os.Exit(1)
	}

	fmt.Println("✅ Application shutdown complete")
}

func setupEventHandlers(ctx context.Context, eventBus *eventbus.EventBusModule) {
	// User event handlers (routed to memory-fast engine)
	eventBus.Subscribe(ctx, "user.registered", func(ctx context.Context, event eventbus.Event) error {
		userEvent := event.Payload.(UserEvent)
		fmt.Printf("🔵 [MEMORY-FAST] User registered: %s (action: %s)\n", 
			userEvent.UserID, userEvent.Action)
		return nil
	})

	eventBus.Subscribe(ctx, "user.login", func(ctx context.Context, event eventbus.Event) error {
		userEvent := event.Payload.(UserEvent)
		fmt.Printf("🔵 [MEMORY-FAST] User login: %s at %s\n", 
			userEvent.UserID, userEvent.Timestamp.Format("15:04:05"))
		return nil
	})

	eventBus.Subscribe(ctx, "auth.failed", func(ctx context.Context, event eventbus.Event) error {
		userEvent := event.Payload.(UserEvent)
		fmt.Printf("🔴 [MEMORY-FAST] Auth failed for user: %s\n", userEvent.UserID)
		return nil
	})

	// Analytics event handlers (routed to kafka-analytics engine)
	eventBus.SubscribeAsync(ctx, "analytics.pageview", func(ctx context.Context, event eventbus.Event) error {
		analyticsEvent := event.Payload.(AnalyticsEvent)
		fmt.Printf("📈 [KAFKA-ANALYTICS] Page view: %s (session: %s)\n", 
			analyticsEvent.Page, analyticsEvent.SessionID)
		return nil
	})

	eventBus.SubscribeAsync(ctx, "analytics.click", func(ctx context.Context, event eventbus.Event) error {
		analyticsEvent := event.Payload.(AnalyticsEvent)
		fmt.Printf("📈 [KAFKA-ANALYTICS] Click event: %s on %s\n", 
			analyticsEvent.EventType, analyticsEvent.Page)
		return nil
	})
	
	eventBus.SubscribeAsync(ctx, "metrics.cpu_usage", func(ctx context.Context, event eventbus.Event) error {
		fmt.Printf("📊 [KAFKA-ANALYTICS] CPU usage metric received\n")
		return nil
	})

	// System event handlers (routed to redis-durable engine)
	eventBus.Subscribe(ctx, "system.health", func(ctx context.Context, event eventbus.Event) error {
		systemEvent := event.Payload.(SystemEvent)
		fmt.Printf("⚙️  [REDIS-DURABLE] System %s: %s - %s\n", 
			systemEvent.Level, systemEvent.Component, systemEvent.Message)
		return nil
	})
	
	eventBus.Subscribe(ctx, "health.check", func(ctx context.Context, event eventbus.Event) error {
		systemEvent := event.Payload.(SystemEvent)
		fmt.Printf("🏥 [REDIS-DURABLE] Health check: %s - %s\n", 
			systemEvent.Component, systemEvent.Message)
		return nil
	})

	// Fallback event handlers (routed to memory-reliable engine)
	eventBus.Subscribe(ctx, "fallback.test", func(ctx context.Context, event eventbus.Event) error {
		fmt.Printf("🔄 [MEMORY-RELIABLE] Fallback event processed\n")
		return nil
	})
}

func demonstrateMultiEngineEvents(ctx context.Context, eventBus *eventbus.EventBusModule) {
	fmt.Println("🎯 Publishing events to different engines based on topic routing:")
	fmt.Println()

	now := time.Now()

	// User events (routed to memory-fast engine)
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

		err := eventBus.Publish(ctx, topic, event)
		if err != nil {
			fmt.Printf("Error publishing user event: %v\n", err)
		}

		if i < len(userEvents)-1 {
			time.Sleep(200 * time.Millisecond)
		}
	}

	time.Sleep(500 * time.Millisecond)

	// Analytics events (routed to kafka-analytics engine)
	analyticsEvents := []AnalyticsEvent{
		{SessionID: "sess123", EventType: "pageview", Page: "/dashboard", Timestamp: now},
		{SessionID: "sess123", EventType: "click", Page: "/dashboard", Timestamp: now.Add(1 * time.Second)},
		{SessionID: "sess456", EventType: "pageview", Page: "/profile", Timestamp: now.Add(2 * time.Second)},
	}

	for i, event := range analyticsEvents {
		var topic string
		switch event.EventType {
		case "pageview":
			topic = "analytics.pageview"
		case "click":
			topic = "analytics.click"
		}

		err := eventBus.Publish(ctx, topic, event)
		if err != nil {
			fmt.Printf("Error publishing analytics event: %v\n", err)
		}

		if i < len(analyticsEvents)-1 {
			time.Sleep(200 * time.Millisecond)
		}
	}

	// Publish a metrics event to Kafka
	err := eventBus.Publish(ctx, "metrics.cpu_usage", map[string]interface{}{
		"cpu":       85.5,
		"timestamp": now,
	})
	if err != nil {
		fmt.Printf("Error publishing metrics event: %v\n", err)
	}

	time.Sleep(500 * time.Millisecond)

	// System events (routed to redis-durable engine)
	systemEvents := []SystemEvent{
		{Component: "database", Level: "info", Message: "Connection established", Timestamp: now},
		{Component: "cache", Level: "warning", Message: "High memory usage", Timestamp: now.Add(1 * time.Second)},
	}

	for i, event := range systemEvents {
		err := eventBus.Publish(ctx, "system.health", event)
		if err != nil {
			fmt.Printf("Error publishing system event: %v\n", err)
		}

		if i < len(systemEvents)-1 {
			time.Sleep(200 * time.Millisecond)
		}
	}

	// Health check events (also routed to redis-durable engine)
	healthEvent := SystemEvent{
		Component: "loadbalancer", 
		Level: "info", 
		Message: "All endpoints healthy", 
		Timestamp: now,
	}
	err = eventBus.Publish(ctx, "health.check", healthEvent)
	if err != nil {
		fmt.Printf("Error publishing health event: %v\n", err)
	}

	time.Sleep(500 * time.Millisecond)

	// Fallback event (routed to memory-reliable engine)
	err = eventBus.Publish(ctx, "fallback.test", map[string]interface{}{
		"message":   "This goes to fallback engine",
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
		"analytics.pageview", "analytics.click", "metrics.cpu_usage",
		"system.health", "health.check", "random.topic",
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
	
	if eventBus != nil && eventBus.GetRouter() != nil {
		// Test Redis connectivity by trying to get the engine
		redisEngine := eventBus.GetRouter().GetEngineForTopic("system.test")
		if redisEngine == "redis-durable" {
			fmt.Println("  ✅ Redis engine configured and ready")
		} else {
			fmt.Println("  ❌ Redis engine not available, events will route to fallback")
		}
		
		// Test Kafka connectivity by trying to get the engine
		kafkaEngine := eventBus.GetRouter().GetEngineForTopic("analytics.test")
		if kafkaEngine == "kafka-analytics" {
			fmt.Println("  ✅ Kafka engine configured and ready")
		} else {
			fmt.Println("  ❌ Kafka engine not available, events will route to fallback")
		}
	}
	
	fmt.Println("  💡 If external services are not available, run: ./run-demo.sh start")
	fmt.Println()
}