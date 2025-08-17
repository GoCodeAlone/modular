package modular

import (
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoggerDecoratorIntegrationScenarios provides comprehensive feature testing
// for various realistic logging scenarios with decorators
func TestLoggerDecoratorIntegrationScenarios(t *testing.T) {
	t.Run("Scenario 1: Audit Trail with Service Context", func(t *testing.T) {
		// Setup: Create a logger system that logs to both console and audit file
		// with automatic service context injection

		consoleLogger := NewTestLogger()
		auditLogger := NewTestLogger()

		// Create the decorator chain: DualWriter -> ServiceContext
		// This way, service context is applied to the output of both loggers
		dualWriteLogger := NewDualWriterLoggerDecorator(consoleLogger, auditLogger)

		serviceContextLogger := NewValueInjectionLoggerDecorator(dualWriteLogger,
			"service", "user-management",
			"version", "1.2.3",
			"environment", "production")

		// Setup application with this logger
		app := NewStdApplication(NewStdConfigProvider(&struct{}{}), serviceContextLogger)

		// Simulate module operations
		app.Logger().Info("User login attempt", "user_id", "12345", "ip", "192.168.1.1")
		app.Logger().Error("Authentication failed", "user_id", "12345", "reason", "invalid_password")
		app.Logger().Info("User logout", "user_id", "12345", "session_duration", "45m")

		// Verify both loggers received all events with service context
		consoleEntries := consoleLogger.GetEntries()
		auditEntries := auditLogger.GetEntries()

		require.Len(t, consoleEntries, 3)
		require.Len(t, auditEntries, 3)

		// Check service context is injected
		for _, entry := range consoleEntries {
			args := argsToMap(entry.Args)
			assert.Equal(t, "user-management", args["service"])
			assert.Equal(t, "1.2.3", args["version"])
			assert.Equal(t, "production", args["environment"])
		}

		// Verify audit logger has identical entries
		for i, entry := range auditEntries {
			assert.Equal(t, consoleEntries[i].Level, entry.Level)
			assert.Equal(t, consoleEntries[i].Message, entry.Message)
			assert.Equal(t, consoleEntries[i].Args, entry.Args)
		}

		// Verify specific security events are logged
		loginEntry := consoleLogger.FindEntry("info", "User login attempt")
		require.NotNil(t, loginEntry)
		args := argsToMap(loginEntry.Args)
		assert.Equal(t, "12345", args["user_id"])
		assert.Equal(t, "192.168.1.1", args["ip"])
	})

	t.Run("Scenario 2: Development vs Production Logging with Filters", func(t *testing.T) {
		baseLogger := NewTestLogger()

		// Production environment: Filter out debug logs and sensitive information
		messageFilters := []string{"password", "secret", "key"}
		levelFilters := map[string]bool{"debug": false, "info": true, "warn": true, "error": true}

		productionLogger := NewFilterLoggerDecorator(baseLogger, messageFilters, nil, levelFilters)

		app := NewStdApplication(NewStdConfigProvider(&struct{}{}), productionLogger)

		// Test various log types
		app.Logger().Debug("Database connection details", "password", "secret123") // Should be filtered (debug level)
		app.Logger().Info("User created successfully", "user_id", "456")           // Should pass
		app.Logger().Error("Database password incorrect", "error", "auth failed")  // Should be filtered (contains "password")
		app.Logger().Warn("High memory usage detected", "usage", "85%")            // Should pass
		app.Logger().Info("Authentication successful", "user_id", "456")           // Should pass

		entries := baseLogger.GetEntries()
		require.Len(t, entries, 3) // Only 3 should pass the filters

		assert.Equal(t, "info", entries[0].Level)
		assert.Contains(t, entries[0].Message, "User created")

		assert.Equal(t, "warn", entries[1].Level)
		assert.Contains(t, entries[1].Message, "High memory usage")

		assert.Equal(t, "info", entries[2].Level)
		assert.Contains(t, entries[2].Message, "Authentication successful")
	})

	t.Run("Scenario 3: Module-Specific Logging with Prefixes", func(t *testing.T) {
		baseLogger := NewTestLogger()

		// Create module-specific loggers with different prefixes
		dbModuleLogger := NewPrefixLoggerDecorator(baseLogger, "[DB-MODULE]")
		apiModuleLogger := NewPrefixLoggerDecorator(baseLogger, "[API-MODULE]")

		// Simulate different modules logging
		dbModuleLogger.Info("Connection established", "host", "localhost", "port", 5432)
		apiModuleLogger.Info("Request received", "method", "POST", "path", "/users")
		dbModuleLogger.Error("Query timeout", "query", "SELECT * FROM users", "timeout", "30s")
		apiModuleLogger.Warn("Rate limit approaching", "remaining", "10", "window", "1m")

		entries := baseLogger.GetEntries()
		require.Len(t, entries, 4)

		// Verify prefixes are correctly applied
		assert.Equal(t, "[DB-MODULE] Connection established", entries[0].Message)
		assert.Equal(t, "[API-MODULE] Request received", entries[1].Message)
		assert.Equal(t, "[DB-MODULE] Query timeout", entries[2].Message)
		assert.Equal(t, "[API-MODULE] Rate limit approaching", entries[3].Message)

		// Verify all other data is preserved
		args := argsToMap(entries[0].Args)
		assert.Equal(t, "localhost", args["host"])
		assert.Equal(t, 5432, args["port"])
	})

	t.Run("Scenario 4: Dynamic Log Level Promotion for Errors", func(t *testing.T) {
		baseLogger := NewTestLogger()

		// Create a level modifier that promotes warnings to errors in production
		levelMappings := map[string]string{
			"warn": "error", // Treat warnings as errors in production
			"info": "info",  // Keep info as info
		}

		levelModifierLogger := NewLevelModifierLoggerDecorator(baseLogger, levelMappings)

		app := NewStdApplication(NewStdConfigProvider(&struct{}{}), levelModifierLogger)

		app.Logger().Info("Service started", "port", 8080)
		app.Logger().Warn("Deprecated API usage", "endpoint", "/old-api", "client", "mobile-app")
		app.Logger().Error("Database connection failed", "host", "db.example.com")
		app.Logger().Debug("Processing request", "request_id", "123")

		entries := baseLogger.GetEntries()
		require.Len(t, entries, 4)

		// Verify level modifications
		assert.Equal(t, "info", entries[0].Level)  // info stays info
		assert.Equal(t, "error", entries[1].Level) // warn becomes error
		assert.Equal(t, "error", entries[2].Level) // error stays error
		assert.Equal(t, "debug", entries[3].Level) // debug stays debug (no mapping)

		// Verify message content is preserved
		assert.Contains(t, entries[1].Message, "Deprecated API usage")
		args := argsToMap(entries[1].Args)
		assert.Equal(t, "/old-api", args["endpoint"])
	})

	t.Run("Scenario 5: Complex Decorator Chain - Full Featured Logging", func(t *testing.T) {
		// Create a comprehensive logging system with multiple decorators
		primaryLogger := NewTestLogger()
		auditLogger := NewTestLogger()

		// Build the decorator chain:
		// 1. Dual write to primary and audit (at the base level)
		// 2. Add service context
		// 3. Add environment prefix
		// 4. Filter sensitive information (at the top level)

		step1 := NewDualWriterLoggerDecorator(primaryLogger, auditLogger)

		step2 := NewValueInjectionLoggerDecorator(step1,
			"service", "payment-processor",
			"instance_id", "instance-001",
			"region", "us-east-1")

		step3 := NewPrefixLoggerDecorator(step2, "[PAYMENT]")

		finalLogger := NewFilterLoggerDecorator(step3,
			[]string{"credit_card", "ssn", "password"}, // Filter sensitive terms
			nil,
			map[string]bool{"debug": false}) // No debug logs in production

		// Setup application with complex logger
		app := NewStdApplication(NewStdConfigProvider(&struct{}{}), finalLogger)

		// Test various scenarios
		app.Logger().Info("Payment processing started", "transaction_id", "tx-12345", "amount", "99.99")
		app.Logger().Debug("Credit card validation details", "last_four", "1234") // Should be filtered (debug + sensitive)
		app.Logger().Warn("Payment gateway timeout", "gateway", "stripe", "retry_count", 2)
		app.Logger().Error("Payment failed", "transaction_id", "tx-12345", "error", "insufficient_funds")
		app.Logger().Info("Refund processed", "transaction_id", "tx-67890", "amount", "50.00")

		// Verify primary logger received filtered and decorated logs
		primaryEntries := primaryLogger.GetEntries()
		require.Len(t, primaryEntries, 4) // Debug entry should be filtered out

		// Check all entries have the expected decorations
		for _, entry := range primaryEntries {
			// Check prefix
			assert.True(t, strings.HasPrefix(entry.Message, "[PAYMENT] "))

			// Check injected context
			args := argsToMap(entry.Args)
			assert.Equal(t, "payment-processor", args["service"])
			assert.Equal(t, "instance-001", args["instance_id"])
			assert.Equal(t, "us-east-1", args["region"])
		}

		// Verify audit logger received the same entries
		auditEntries := auditLogger.GetEntries()
		require.Len(t, auditEntries, 4)

		// Check specific entries
		paymentStartEntry := primaryLogger.FindEntry("info", "Payment processing started")
		require.NotNil(t, paymentStartEntry)
		assert.Equal(t, "[PAYMENT] Payment processing started", paymentStartEntry.Message)

		paymentFailedEntry := primaryLogger.FindEntry("error", "Payment failed")
		require.NotNil(t, paymentFailedEntry)
		args := argsToMap(paymentFailedEntry.Args)
		assert.Equal(t, "tx-12345", args["transaction_id"])
		assert.Equal(t, "insufficient_funds", args["error"])

		// Verify debug entry with sensitive info was filtered
		debugEntry := primaryLogger.FindEntry("debug", "Credit card validation")
		assert.Nil(t, debugEntry, "Sensitive debug entry should have been filtered")
	})

	t.Run("Scenario 6: SetLogger with Decorators in Module Context", func(t *testing.T) {
		// Test that modules continue to work correctly when SetLogger is used with decorators

		originalLogger := NewTestLogger()
		app := NewStdApplication(NewStdConfigProvider(&struct{}{}), originalLogger)

		// Create a mock module that uses logger service
		type MockModule struct {
			name   string
			logger Logger
		}

		mockModule := &MockModule{name: "test-module"}

		// Simulate module getting logger from service registry (like DI would do)
		var moduleLogger Logger
		err := app.GetService("logger", &moduleLogger)
		require.NoError(t, err)
		mockModule.logger = moduleLogger

		// Module uses its logger
		mockModule.logger.Info("Module initialized", "module", mockModule.name)

		// Verify original logger received the message
		require.Len(t, originalLogger.GetEntries(), 1)
		assert.Equal(t, "Module initialized", originalLogger.GetEntries()[0].Message)

		// Now create a decorated logger and set it
		newBaseLogger := NewTestLogger()
		decoratedLogger := NewPrefixLoggerDecorator(
			NewValueInjectionLoggerDecorator(newBaseLogger, "app_version", "2.0.0"),
			"[APP-V2]")

		app.SetLogger(decoratedLogger)

		// Module should get the updated logger when it asks for it again
		var updatedModuleLogger Logger
		err = app.GetService("logger", &updatedModuleLogger)
		require.NoError(t, err)
		mockModule.logger = updatedModuleLogger

		// Module uses the new decorated logger
		mockModule.logger.Info("Module operation completed", "module", mockModule.name, "operation", "startup")

		// Verify the new decorated logger received the message with all decorations
		newEntries := newBaseLogger.GetEntries()
		require.Len(t, newEntries, 1)

		entry := newEntries[0]
		assert.Equal(t, "[APP-V2] Module operation completed", entry.Message)

		args := argsToMap(entry.Args)
		assert.Equal(t, "2.0.0", args["app_version"])
		assert.Equal(t, "test-module", args["module"])
		assert.Equal(t, "startup", args["operation"])

		// Verify original logger didn't receive the new message
		assert.Len(t, originalLogger.GetEntries(), 1) // Still just the original message
	})
}

// Helper to create a realistic slog-based logger for testing
func createSlogTestLogger() Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}

func TestDecoratorWithRealSlogLogger(t *testing.T) {
	t.Run("Decorators work with real slog logger", func(t *testing.T) {
		// This test shows that decorators work with actual slog implementation
		realLogger := createSlogTestLogger()

		// Create a prefix decorator around the real logger
		decoratedLogger := NewPrefixLoggerDecorator(realLogger, "[TEST]")

		// This will actually output to stdout - useful for manual verification
		decoratedLogger.Info("Testing decorator with real slog", "test", "integration")

		// If we get here without panicking, the decorator works with real loggers
		assert.True(t, true, "Decorator worked with real slog logger")
	})
}

func TestDecoratorErrorHandling(t *testing.T) {
	t.Run("Decorators handle nil inner logger gracefully", func(t *testing.T) {
		// Note: This tests what happens if someone creates a decorator with nil
		// In practice this shouldn't happen, but we should handle it gracefully

		defer func() {
			if r := recover(); r != nil {
				t.Logf("Expected panic when using nil inner logger: %v", r)
				// This is expected behavior - decorators should panic if inner is nil
				// because that indicates a programming error
			}
		}()

		decorator := NewBaseLoggerDecorator(nil)
		decorator.Info("This should panic")

		// If we get here, no panic occurred (which would be unexpected)
		t.Fatal("Expected panic when using nil inner logger, but none occurred")
	})

	t.Run("Nested decorators work correctly", func(t *testing.T) {
		baseLogger := NewTestLogger()

		// Create multiple levels of nesting
		level1 := NewPrefixLoggerDecorator(baseLogger, "[L1]")
		level2 := NewValueInjectionLoggerDecorator(level1, "level", "2")
		level3 := NewPrefixLoggerDecorator(level2, "[L3]")

		level3.Info("Deeply nested message", "test", "nesting")

		entries := baseLogger.GetEntries()
		require.Len(t, entries, 1)

		entry := entries[0]
		// The order should be: level1 first, then level3 applied on top
		assert.Equal(t, "[L1] [L3] Deeply nested message", entry.Message)

		args := argsToMap(entry.Args)
		assert.Equal(t, "2", args["level"])
		assert.Equal(t, "nesting", args["test"])
	})
}
