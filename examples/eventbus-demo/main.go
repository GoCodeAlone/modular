package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"time"

	"github.com/GoCodeAlone/modular"
	"github.com/GoCodeAlone/modular/feeders"
	"github.com/GoCodeAlone/modular/modules/chimux"
	"github.com/GoCodeAlone/modular/modules/eventbus"
	"github.com/GoCodeAlone/modular/modules/httpserver"
	"github.com/go-chi/chi/v5"
)

type AppConfig struct {
	Name string `yaml:"name" default:"EventBus Demo"`
}

type Message struct {
	ID        string            `json:"id"`
	Topic     string            `json:"topic"`
	Content   string            `json:"content"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

type PublishRequest struct {
	Topic    string            `json:"topic"`
	Content  string            `json:"content"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type SubscriptionRequest struct {
	Topic string `json:"topic"`
}

type EventBusModule struct {
	eventBus eventbus.EventBus
	router   chi.Router
	messages []Message // Store received messages for demonstration
}

func (m *EventBusModule) Name() string {
	return "eventbus-demo"
}

func (m *EventBusModule) RequiresServices() []modular.ServiceDependency {
	return []modular.ServiceDependency{
		{
			Name:               "eventbus.provider",
			Required:           true,
			MatchByInterface:   true,
			SatisfiesInterface: reflect.TypeOf((*eventbus.EventBus)(nil)).Elem(),
		},
		{
			Name:               "router",
			Required:           true,
			MatchByInterface:   true,
			SatisfiesInterface: reflect.TypeOf((*chi.Router)(nil)).Elem(),
		},
	}
}

func (m *EventBusModule) Constructor() modular.ModuleConstructor {
	return func(app modular.Application, services map[string]any) (modular.Module, error) {
		eventBusService, ok := services["eventbus.provider"].(eventbus.EventBus)
		if !ok {
			return nil, fmt.Errorf("eventbus service not found or wrong type")
		}

		router, ok := services["router"].(chi.Router)
		if !ok {
			return nil, fmt.Errorf("router service not found or wrong type")
		}

		return &EventBusModule{
			eventBus: eventBusService,
			router:   router,
			messages: make([]Message, 0),
		}, nil
	}
}

func (m *EventBusModule) Init(app modular.Application) error {
	// Set up demonstration event subscribers
	ctx := context.Background()

	// Subscribe to user events
	_, err := m.eventBus.Subscribe(ctx, "user.*", func(ctx context.Context, event eventbus.Event) error {
		message := Message{
			ID:        fmt.Sprintf("msg-%d", time.Now().UnixNano()),
			Topic:     event.Topic,
			Content:   fmt.Sprintf("User event: %v", event.Payload),
			Timestamp: time.Now(),
		}
		if event.Metadata != nil {
			message.Metadata = make(map[string]string)
			for k, v := range event.Metadata {
				message.Metadata[k] = fmt.Sprintf("%v", v)
			}
		}
		m.messages = append(m.messages, message)
		slog.Info("Received user event", "topic", event.Topic, "payload", event.Payload)
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to user events: %w", err)
	}

	// Subscribe to order events asynchronously
	_, err = m.eventBus.SubscribeAsync(ctx, "order.*", func(ctx context.Context, event eventbus.Event) error {
		message := Message{
			ID:        fmt.Sprintf("msg-%d", time.Now().UnixNano()),
			Topic:     event.Topic,
			Content:   fmt.Sprintf("Order event (async): %v", event.Payload),
			Timestamp: time.Now(),
		}
		if event.Metadata != nil {
			message.Metadata = make(map[string]string)
			for k, v := range event.Metadata {
				message.Metadata[k] = fmt.Sprintf("%v", v)
			}
		}
		m.messages = append(m.messages, message)
		slog.Info("Received order event (async)", "topic", event.Topic, "payload", event.Payload)
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to order events: %w", err)
	}

	// Set up HTTP routes
	m.router.Route("/api/eventbus", func(r chi.Router) {
		r.Post("/publish", m.publishEvent)
		r.Get("/messages", m.getMessages)
		r.Get("/topics", m.getTopics)
		r.Get("/stats", m.getStats)
		r.Delete("/messages", m.clearMessages)
	})

	m.router.Get("/health", m.healthCheck)

	slog.Info("EventBus demo module initialized")
	return nil
}

func (m *EventBusModule) publishEvent(w http.ResponseWriter, r *http.Request) {
	var req PublishRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Topic == "" || req.Content == "" {
		http.Error(w, "Topic and content are required", http.StatusBadRequest)
		return
	}

	// Create event
	event := eventbus.Event{
		Topic:    req.Topic,
		Payload:  req.Content,
		Metadata: make(map[string]interface{}),
	}

	// Add metadata
	for k, v := range req.Metadata {
		event.Metadata[k] = v
	}
	event.Metadata["source"] = "http-api"
	event.Metadata["timestamp"] = time.Now().Format(time.RFC3339)

	// Publish event
	if err := m.eventBus.Publish(r.Context(), event); err != nil {
		http.Error(w, fmt.Sprintf("Failed to publish event: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Event published successfully",
		"topic":   req.Topic,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (m *EventBusModule) getMessages(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	// Get the most recent messages
	start := 0
	if len(m.messages) > limit {
		start = len(m.messages) - limit
	}

	messages := make([]Message, 0, limit)
	for i := start; i < len(m.messages); i++ {
		messages = append(messages, m.messages[i])
	}

	response := map[string]interface{}{
		"messages": messages,
		"total":    len(m.messages),
		"showing":  len(messages),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (m *EventBusModule) getTopics(w http.ResponseWriter, r *http.Request) {
	topics := m.eventBus.Topics()

	topicStats := make(map[string]map[string]interface{})
	for _, topic := range topics {
		topicStats[topic] = map[string]interface{}{
			"subscribers": m.eventBus.SubscriberCount(topic),
		}
	}

	response := map[string]interface{}{
		"topics": topics,
		"stats":  topicStats,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (m *EventBusModule) getStats(w http.ResponseWriter, r *http.Request) {
	topics := m.eventBus.Topics()
	totalSubscribers := 0
	for _, topic := range topics {
		totalSubscribers += m.eventBus.SubscriberCount(topic)
	}

	response := map[string]interface{}{
		"topics":            len(topics),
		"total_subscribers": totalSubscribers,
		"messages_received": len(m.messages),
		"uptime":           time.Since(time.Now().Add(-5 * time.Minute)).String(), // Approximate
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (m *EventBusModule) clearMessages(w http.ResponseWriter, r *http.Request) {
	m.messages = make([]Message, 0)
	response := map[string]interface{}{
		"success": true,
		"message": "Messages cleared",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (m *EventBusModule) healthCheck(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":           "healthy",
		"service":          "eventbus-demo",
		"topics":           len(m.eventBus.Topics()),
		"messages_handled": len(m.messages),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Create config provider
	appConfig := &AppConfig{}
	configProvider := modular.NewStdConfigProvider(appConfig)

	// Create application
	app := modular.NewStdApplication(configProvider, logger)

	// Set up configuration feeders
	modular.ConfigFeeders = []modular.Feeder{
		feeders.NewYamlFeeder("config.yaml"),
		feeders.NewEnvFeeder(),
	}

	// Register modules
	app.RegisterModule(eventbus.NewModule())
	app.RegisterModule(chimux.NewChiMuxModule())
	app.RegisterModule(httpserver.NewHTTPServerModule())
	app.RegisterModule(&EventBusModule{})

	logger.Info("Starting EventBus Demo Application")

	// Run application
	if err := app.Run(); err != nil {
		logger.Error("Application error", "error", err)
		os.Exit(1)
	}
}