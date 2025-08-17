package main

import (
	"log"

	"github.com/CrisisTextLine/modular"
	"github.com/CrisisTextLine/modular/modules/logmasker"
)

// SimpleLogger implements modular.Logger for demonstration
type SimpleLogger struct{}

func (s *SimpleLogger) Info(msg string, args ...any) {
	log.Printf("[INFO] %s %v", msg, args)
}

func (s *SimpleLogger) Error(msg string, args ...any) {
	log.Printf("[ERROR] %s %v", msg, args)
}

func (s *SimpleLogger) Warn(msg string, args ...any) {
	log.Printf("[WARN] %s %v", msg, args)
}

func (s *SimpleLogger) Debug(msg string, args ...any) {
	log.Printf("[DEBUG] %s %v", msg, args)
}

// SensitiveToken demonstrates the MaskableValue interface
type SensitiveToken struct {
	Value    string
	IsPublic bool
}

func (t *SensitiveToken) ShouldMask() bool {
	return !t.IsPublic
}

func (t *SensitiveToken) GetMaskedValue() any {
	return "[SENSITIVE-TOKEN]"
}

func (t *SensitiveToken) GetMaskStrategy() logmasker.MaskStrategy {
	return logmasker.MaskStrategyRedact
}

func main() {
	// Create a simple logger
	logger := &SimpleLogger{}

	// Create application
	app := modular.NewStdApplication(nil, logger)

	// Register the logmasker module
	app.RegisterModule(logmasker.NewModule())

	// Initialize the application
	if err := app.Init(); err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}

	// Get the masking logger service
	var maskingLogger modular.Logger
	if err := app.GetService("logmasker.logger", &maskingLogger); err != nil {
		log.Fatalf("Failed to get masking logger: %v", err)
	}

	// Demonstrate field-based masking
	log.Println("\n=== Field-based Masking ===")
	maskingLogger.Info("User authentication",
		"username", "johndoe",
		"email", "john.doe@example.com", // Will be partially masked
		"password", "supersecret123",    // Will be redacted
		"sessionId", "abc-123-def")      // Will remain unchanged

	// Demonstrate pattern-based masking
	log.Println("\n=== Pattern-based Masking ===")
	maskingLogger.Info("Payment processing",
		"orderId", "ORD-12345",
		"card", "4111-1111-1111-1111", // Will be redacted (credit card pattern)
		"amount", "$29.99",
		"ssn", "123-45-6789") // Will be redacted (SSN pattern)

	// Demonstrate MaskableValue interface
	log.Println("\n=== MaskableValue Interface ===")
	publicToken := &SensitiveToken{Value: "public-token", IsPublic: true}
	privateToken := &SensitiveToken{Value: "private-token", IsPublic: false}

	maskingLogger.Info("API tokens",
		"public", publicToken,  // Will not be masked
		"private", privateToken) // Will be masked

	// Demonstrate different log levels
	log.Println("\n=== Different Log Levels ===")
	maskingLogger.Error("Authentication failed", "password", "failed123")
	maskingLogger.Warn("Suspicious activity", "token", "suspicious-token")
	maskingLogger.Debug("Debug info", "secret", "debug-secret")

	log.Println("\nExample completed!")
}