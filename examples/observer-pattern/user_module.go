package main

import (
	"context"
	"fmt"

	"github.com/CrisisTextLine/modular"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// UserModuleConfig configures the user module
type UserModuleConfig struct {
	MaxUsers int    `yaml:"maxUsers" default:"1000" desc:"Maximum number of users"`
	LogLevel string `yaml:"logLevel" default:"INFO" desc:"Log level for user events"`
}

// UserModule demonstrates a module that both observes and emits events
type UserModule struct {
	name       string
	config     *UserModuleConfig
	logger     modular.Logger
	userStore  *UserStore
	subject    modular.Subject // Reference to emit events
}

// User represents a user entity
type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// UserStore provides user storage functionality
type UserStore struct {
	users map[string]*User
}

func NewUserModule() modular.Module {
	return &UserModule{
		name: "userModule",
	}
}

func (m *UserModule) Name() string {
	return m.name
}

func (m *UserModule) RegisterConfig(app modular.Application) error {
	defaultConfig := &UserModuleConfig{
		MaxUsers: 1000,
		LogLevel: "INFO",
	}
	app.RegisterConfigSection(m.Name(), modular.NewStdConfigProvider(defaultConfig))
	return nil
}

func (m *UserModule) Init(app modular.Application) error {
	// Get configuration
	cfg, err := app.GetConfigSection(m.name)
	if err != nil {
		return fmt.Errorf("failed to get config section '%s': %w", m.name, err)
	}
	m.config = cfg.GetConfig().(*UserModuleConfig)
	m.logger = app.Logger()

	// Store reference to app for event emission if it supports observer pattern
	if observable, ok := app.(modular.Subject); ok {
		m.subject = observable
	}

	m.logger.Info("User module initialized", "maxUsers", m.config.MaxUsers)
	return nil
}

func (m *UserModule) Dependencies() []string {
	return nil // No module dependencies
}

func (m *UserModule) ProvidesServices() []modular.ServiceProvider {
	return []modular.ServiceProvider{
		{
			Name:        "userModule",
			Description: "User management module",
			Instance:    m,
		},
	}
}

func (m *UserModule) RequiresServices() []modular.ServiceDependency {
	return []modular.ServiceDependency{
		{
			Name:     "userStore",
			Required: true,
		},
		{
			Name:     "emailService",
			Required: true,
		},
	}
}

func (m *UserModule) Constructor() modular.ModuleConstructor {
	return func(app modular.Application, services map[string]any) (modular.Module, error) {
		m.userStore = services["userStore"].(*UserStore)
		// Store reference to app for event emission if it supports observer pattern
		if observable, ok := app.(modular.Subject); ok {
			m.subject = observable
		}
		return m, nil
	}
}

// RegisterObservers implements ObservableModule to register as an observer
func (m *UserModule) RegisterObservers(subject modular.Subject) error {
	// Register to observe application events
	err := subject.RegisterObserver(m, 
		modular.EventTypeApplicationStarted,
		modular.EventTypeApplicationStopped,
		modular.EventTypeServiceRegistered,
	)
	if err != nil {
		return fmt.Errorf("failed to register user module as observer: %w", err)
	}
	
	m.logger.Info("User module registered as observer for application events")
	return nil
}

// EmitEvent allows the module to emit events
func (m *UserModule) EmitEvent(ctx context.Context, event cloudevents.Event) error {
	if m.subject != nil {
		return m.subject.NotifyObservers(ctx, event)
	}
	return fmt.Errorf("no subject available for event emission")
}

// OnEvent implements Observer interface to receive events
func (m *UserModule) OnEvent(ctx context.Context, event cloudevents.Event) error {
	switch event.Type() {
	case modular.EventTypeApplicationStarted:
		m.logger.Info("🎉 User module received application started event")
		// Initialize user data or perform startup tasks
		
	case modular.EventTypeApplicationStopped:
		m.logger.Info("👋 User module received application stopped event")
		// Cleanup tasks
		
	case modular.EventTypeServiceRegistered:
		var data map[string]interface{}
		if err := event.DataAs(&data); err == nil {
			if serviceName, ok := data["serviceName"].(string); ok {
				m.logger.Info("🔧 User module notified of service registration", "service", serviceName)
			}
		}
	}
	return nil
}

// ObserverID implements Observer interface
func (m *UserModule) ObserverID() string {
	return m.name
}

// Business logic methods that emit custom events

func (m *UserModule) CreateUser(id, email string) error {
	if len(m.userStore.users) >= m.config.MaxUsers {
		return fmt.Errorf("maximum users reached: %d", m.config.MaxUsers)
	}

	user := &User{ID: id, Email: email}
	m.userStore.users[id] = user
	
	// Emit custom CloudEvent
	event := modular.NewCloudEvent(
		"com.example.user.created",
		m.name,
		map[string]interface{}{
			"userID": id,
			"email":  email,
		},
		map[string]interface{}{
			"module": m.name,
		},
	)
	
	if err := m.EmitEvent(context.Background(), event); err != nil {
		m.logger.Error("Failed to emit user.created event", "error", err)
	}
	
	m.logger.Info("👤 User created", "userID", id, "email", email)
	return nil
}

func (m *UserModule) LoginUser(id string) error {
	user, exists := m.userStore.users[id]
	if !exists {
		return fmt.Errorf("user not found: %s", id)
	}
	
	// Emit custom CloudEvent
	event := modular.NewCloudEvent(
		"com.example.user.login",
		m.name,
		map[string]interface{}{
			"userID": id,
			"email":  user.Email,
		},
		map[string]interface{}{
			"module": m.name,
		},
	)
	
	if err := m.EmitEvent(context.Background(), event); err != nil {
		m.logger.Error("Failed to emit user.login event", "error", err)
	}
	
	m.logger.Info("🔐 User logged in", "userID", id)
	return nil
}