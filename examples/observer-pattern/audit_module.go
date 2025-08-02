package main

import (
	"context"
	"fmt"
	"time"

	"github.com/GoCodeAlone/modular"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// AuditModule demonstrates an observer that logs all events for compliance
type AuditModule struct {
	name   string
	logger modular.Logger
	events []AuditEntry
}

// AuditEntry represents an audit log entry
type AuditEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	EventType string                 `json:"eventType"`
	Source    string                 `json:"source"`
	Data      interface{}            `json:"data"`
	Metadata  map[string]interface{} `json:"metadata"`
}

func NewAuditModule() modular.Module {
	return &AuditModule{
		name:   "auditModule",
		events: make([]AuditEntry, 0),
	}
}

func (m *AuditModule) Name() string {
	return m.name
}

func (m *AuditModule) Init(app modular.Application) error {
	m.logger = app.Logger()
	m.logger.Info("Audit module initialized")
	return nil
}

func (m *AuditModule) Dependencies() []string {
	return nil
}

func (m *AuditModule) ProvidesServices() []modular.ServiceProvider {
	return []modular.ServiceProvider{
		{
			Name:        "auditModule",
			Description: "Audit logging module",
			Instance:    m,
		},
	}
}

func (m *AuditModule) RequiresServices() []modular.ServiceDependency {
	return nil
}

func (m *AuditModule) Constructor() modular.ModuleConstructor {
	return func(app modular.Application, services map[string]any) (modular.Module, error) {
		return m, nil
	}
}

// RegisterObservers implements ObservableModule to register for all events
func (m *AuditModule) RegisterObservers(subject modular.Subject) error {
	// Register to observe ALL events (no filter)
	err := subject.RegisterObserver(m)
	if err != nil {
		return fmt.Errorf("failed to register audit module as observer: %w", err)
	}
	
	m.logger.Info("Audit module registered as observer for ALL events")
	return nil
}

// EmitEvent allows the module to emit events (not used in this example)
func (m *AuditModule) EmitEvent(ctx context.Context, event cloudevents.Event) error {
	return fmt.Errorf("audit module does not emit events")
}

// OnEvent implements Observer interface to audit all events
func (m *AuditModule) OnEvent(ctx context.Context, event cloudevents.Event) error {
	// Extract data from CloudEvent
	var data interface{}
	if event.Data() != nil {
		if err := event.DataAs(&data); err != nil {
			data = event.Data()
		}
	}

	// Extract metadata from CloudEvent extensions
	metadata := make(map[string]interface{})
	for key, value := range event.Extensions() {
		metadata[key] = value
	}

	// Create audit entry
	entry := AuditEntry{
		Timestamp: event.Time(),
		EventType: event.Type(),
		Source:    event.Source(),
		Data:      data,
		Metadata:  metadata,
	}
	
	// Store in memory (in real app, would persist to database/file)
	m.events = append(m.events, entry)
	
	// Log the audit entry
	m.logger.Info("📋 AUDIT", 
		"eventType", event.Type(),
		"source", event.Source(),
		"timestamp", event.Time().Format(time.RFC3339),
		"totalEvents", len(m.events),
	)
	
	// Special handling for certain event types
	switch event.Type() {
	case "user.created", "user.login":
		fmt.Printf("🛡️  SECURITY AUDIT: %s event from %s\n", event.Type(), event.Source())
	case modular.EventTypeApplicationFailed, modular.EventTypeModuleFailed:
		fmt.Printf("⚠️  ERROR AUDIT: %s event - investigation required\n", event.Type())
	}
	
	return nil
}

// ObserverID implements Observer interface
func (m *AuditModule) ObserverID() string {
	return m.name
}

// GetAuditSummary provides a summary of audited events
func (m *AuditModule) GetAuditSummary() map[string]int {
	summary := make(map[string]int)
	for _, entry := range m.events {
		summary[entry.EventType]++
	}
	return summary
}

// Start implements Startable interface to show audit summary
func (m *AuditModule) Start(ctx context.Context) error {
	m.logger.Info("Audit module started - beginning event auditing")
	return nil
}

// Stop implements Stoppable interface to show final audit summary
func (m *AuditModule) Stop(ctx context.Context) error {
	summary := m.GetAuditSummary()
	m.logger.Info("📊 FINAL AUDIT SUMMARY", "totalEvents", len(m.events))
	
	fmt.Println("\n📊 Audit Summary:")
	fmt.Println("=================")
	for eventType, count := range summary {
		fmt.Printf("  %s: %d events\n", eventType, count)
	}
	fmt.Printf("  Total Events Audited: %d\n", len(m.events))
	
	return nil
}