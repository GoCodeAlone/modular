package main

import (
	"context"
	"fmt"

	"github.com/GoCodeAlone/modular"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// NotificationModule demonstrates an observer that reacts to user events
type NotificationModule struct {
	name         string
	logger       modular.Logger
	emailService *EmailService
}

// EmailService provides email functionality
type EmailService struct{}

func (e *EmailService) SendEmail(to, subject, body string) error {
	// Simulate sending email
	fmt.Printf("📧 EMAIL SENT: To=%s, Subject=%s, Body=%s\n", to, subject, body)
	return nil
}

func NewNotificationModule() modular.Module {
	return &NotificationModule{
		name: "notificationModule",
	}
}

func (m *NotificationModule) Name() string {
	return m.name
}

func (m *NotificationModule) Init(app modular.Application) error {
	m.logger = app.Logger()
	m.logger.Info("Notification module initialized")
	return nil
}

func (m *NotificationModule) Dependencies() []string {
	return nil // No module dependencies
}

func (m *NotificationModule) ProvidesServices() []modular.ServiceProvider {
	return []modular.ServiceProvider{
		{
			Name:        "notificationModule",
			Description: "Notification handling module",
			Instance:    m,
		},
	}
}

func (m *NotificationModule) RequiresServices() []modular.ServiceDependency {
	return []modular.ServiceDependency{
		{
			Name:     "emailService",
			Required: true,
		},
	}
}

func (m *NotificationModule) Constructor() modular.ModuleConstructor {
	return func(app modular.Application, services map[string]any) (modular.Module, error) {
		m.emailService = services["emailService"].(*EmailService)
		return m, nil
	}
}

// RegisterObservers implements ObservableModule to register for user events
func (m *NotificationModule) RegisterObservers(subject modular.Subject) error {
	// Register to observe user events
	err := subject.RegisterObserver(m, "user.created", "user.login")
	if err != nil {
		return fmt.Errorf("failed to register notification module as observer: %w", err)
	}

	m.logger.Info("Notification module registered as observer for user events")
	return nil
}

// EmitEvent allows the module to emit events (not used in this example)
func (m *NotificationModule) EmitEvent(ctx context.Context, event cloudevents.Event) error {
	return errNotificationModuleDoesNotEmitEvents
}

// OnEvent implements Observer interface to handle user events
func (m *NotificationModule) OnEvent(ctx context.Context, event cloudevents.Event) error {
	switch event.Type() {
	case "com.example.user.created":
		return m.handleUserCreated(ctx, event)
	case "com.example.user.login":
		return m.handleUserLogin(ctx, event)
	default:
		m.logger.Debug("Notification module received unhandled event", "type", event.Type())
	}
	return nil
}

// ObserverID implements Observer interface
func (m *NotificationModule) ObserverID() string {
	return m.name
}

func (m *NotificationModule) handleUserCreated(ctx context.Context, event cloudevents.Event) error {
	var data map[string]interface{}
	if err := event.DataAs(&data); err != nil {
		return fmt.Errorf("invalid event data for user.created: %w", err)
	}

	userID, _ := data["userID"].(string)
	email, _ := data["email"].(string)

	m.logger.Info("🔔 Notification: Handling user creation", "userID", userID)

	// Send welcome email
	subject := "Welcome to Observer Pattern Demo!"
	body := fmt.Sprintf("Hello %s! Welcome to our platform. Your account has been created successfully.", userID)

	if err := m.emailService.SendEmail(email, subject, body); err != nil {
		return fmt.Errorf("failed to send welcome email: %w", err)
	}

	return nil
}

func (m *NotificationModule) handleUserLogin(ctx context.Context, event cloudevents.Event) error {
	var data map[string]interface{}
	if err := event.DataAs(&data); err != nil {
		return fmt.Errorf("invalid event data for user.login: %w", err)
	}

	userID, _ := data["userID"].(string)

	m.logger.Info("🔔 Notification: Handling user login", "userID", userID)

	// Could send login notification email, update last seen, etc.
	fmt.Printf("🔐 LOGIN NOTIFICATION: User %s has logged in\n", userID)

	return nil
}
