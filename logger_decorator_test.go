package modular

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLogger is a test logger that captures log entries for verification
type TestLogger struct {
	entries []TestLogEntry
}

type TestLogEntry struct {
	Level   string
	Message string
	Args    []any
}

func NewTestLogger() *TestLogger {
	return &TestLogger{
		entries: make([]TestLogEntry, 0),
	}
}

func (t *TestLogger) Info(msg string, args ...any) {
	t.entries = append(t.entries, TestLogEntry{Level: "info", Message: msg, Args: args})
}

func (t *TestLogger) Error(msg string, args ...any) {
	t.entries = append(t.entries, TestLogEntry{Level: "error", Message: msg, Args: args})
}

func (t *TestLogger) Warn(msg string, args ...any) {
	t.entries = append(t.entries, TestLogEntry{Level: "warn", Message: msg, Args: args})
}

func (t *TestLogger) Debug(msg string, args ...any) {
	t.entries = append(t.entries, TestLogEntry{Level: "debug", Message: msg, Args: args})
}

func (t *TestLogger) GetEntries() []TestLogEntry {
	return t.entries
}

func (t *TestLogger) Clear() {
	t.entries = make([]TestLogEntry, 0)
}

func (t *TestLogger) FindEntry(level, message string) *TestLogEntry {
	for _, entry := range t.entries {
		if entry.Level == level && strings.Contains(entry.Message, message) {
			return &entry
		}
	}
	return nil
}

func (t *TestLogger) CountEntries(level string) int {
	count := 0
	for _, entry := range t.entries {
		if entry.Level == level {
			count++
		}
	}
	return count
}

// argsToMap converts a slice of alternating key-value arguments into a map.
// Keys must be strings; non-string keys are ignored.
// If args has odd length, the last unpaired argument is ignored.
func argsToMap(args []any) map[string]any {
	if len(args) == 0 {
		return make(map[string]any)
	}

	// Pre-allocate with maximum possible size (len(args)/2) to avoid map growth
	result := make(map[string]any, len(args)/2)
	for i := 0; i < len(args)-1; i += 2 {
		if key, ok := args[i].(string); ok {
			result[key] = args[i+1]
		}
	}
	return result
}

func TestBaseLoggerDecorator(t *testing.T) {
	t.Parallel()
	t.Run("Forwards all calls to inner logger", func(t *testing.T) {
		inner := NewTestLogger()
		decorator := NewBaseLoggerDecorator(inner)

		decorator.Info("test info", "key1", "value1")
		decorator.Error("test error", "key2", "value2")
		decorator.Warn("test warn", "key3", "value3")
		decorator.Debug("test debug", "key4", "value4")

		entries := inner.GetEntries()
		require.Len(t, entries, 4)

		assert.Equal(t, "info", entries[0].Level)
		assert.Equal(t, "test info", entries[0].Message)
		assert.Equal(t, []any{"key1", "value1"}, entries[0].Args)

		assert.Equal(t, "error", entries[1].Level)
		assert.Equal(t, "test error", entries[1].Message)

		assert.Equal(t, "warn", entries[2].Level)
		assert.Equal(t, "debug", entries[3].Level)
	})

	t.Run("GetInnerLogger returns wrapped logger", func(t *testing.T) {
		inner := NewTestLogger()
		decorator := NewBaseLoggerDecorator(inner)

		assert.Equal(t, inner, decorator.GetInnerLogger())
	})
}

func TestDualWriterLoggerDecorator(t *testing.T) {
	t.Parallel()
	t.Run("Logs to both primary and secondary loggers", func(t *testing.T) {
		primary := NewTestLogger()
		secondary := NewTestLogger()
		decorator := NewDualWriterLoggerDecorator(primary, secondary)

		decorator.Info("test message", "key", "value")

		// Both loggers should have received the log entry
		primaryEntries := primary.GetEntries()
		secondaryEntries := secondary.GetEntries()

		require.Len(t, primaryEntries, 1)
		require.Len(t, secondaryEntries, 1)

		assert.Equal(t, "info", primaryEntries[0].Level)
		assert.Equal(t, "test message", primaryEntries[0].Message)
		assert.Equal(t, []any{"key", "value"}, primaryEntries[0].Args)

		assert.Equal(t, "info", secondaryEntries[0].Level)
		assert.Equal(t, "test message", secondaryEntries[0].Message)
		assert.Equal(t, []any{"key", "value"}, secondaryEntries[0].Args)
	})

	t.Run("All log levels work correctly", func(t *testing.T) {
		primary := NewTestLogger()
		secondary := NewTestLogger()
		decorator := NewDualWriterLoggerDecorator(primary, secondary)

		decorator.Info("info", "k1", "v1")
		decorator.Error("error", "k2", "v2")
		decorator.Warn("warn", "k3", "v3")
		decorator.Debug("debug", "k4", "v4")

		assert.Equal(t, 4, len(primary.GetEntries()))
		assert.Equal(t, 4, len(secondary.GetEntries()))

		// Verify levels
		assert.Equal(t, 1, primary.CountEntries("info"))
		assert.Equal(t, 1, primary.CountEntries("error"))
		assert.Equal(t, 1, primary.CountEntries("warn"))
		assert.Equal(t, 1, primary.CountEntries("debug"))

		assert.Equal(t, 1, secondary.CountEntries("info"))
		assert.Equal(t, 1, secondary.CountEntries("error"))
		assert.Equal(t, 1, secondary.CountEntries("warn"))
		assert.Equal(t, 1, secondary.CountEntries("debug"))
	})
}

func TestValueInjectionLoggerDecorator(t *testing.T) {
	t.Parallel()
	t.Run("Injects values into all log events", func(t *testing.T) {
		inner := NewTestLogger()
		decorator := NewValueInjectionLoggerDecorator(inner, "service", "test-service", "version", "1.0.0")

		decorator.Info("test message", "key", "value")

		entries := inner.GetEntries()
		require.Len(t, entries, 1)

		args := entries[0].Args
		argsMap := argsToMap(args)

		assert.Equal(t, "test-service", argsMap["service"])
		assert.Equal(t, "1.0.0", argsMap["version"])
		assert.Equal(t, "value", argsMap["key"])
	})

	t.Run("Preserves original args and combines correctly", func(t *testing.T) {
		inner := NewTestLogger()
		decorator := NewValueInjectionLoggerDecorator(inner, "injected", "value")

		decorator.Error("error message", "original", "arg", "another", "pair")

		entries := inner.GetEntries()
		require.Len(t, entries, 1)

		args := entries[0].Args
		require.Len(t, args, 6) // 2 injected + 4 original

		// Injected args should come first
		assert.Equal(t, "injected", args[0])
		assert.Equal(t, "value", args[1])
		assert.Equal(t, "original", args[2])
		assert.Equal(t, "arg", args[3])
	})

	t.Run("Works with empty injected args", func(t *testing.T) {
		inner := NewTestLogger()
		decorator := NewValueInjectionLoggerDecorator(inner)

		decorator.Debug("debug message", "key", "value")

		entries := inner.GetEntries()
		require.Len(t, entries, 1)
		assert.Equal(t, []any{"key", "value"}, entries[0].Args)
	})
}

func TestFilterLoggerDecorator(t *testing.T) {
	t.Parallel()
	t.Run("Filters by message content", func(t *testing.T) {
		inner := NewTestLogger()
		decorator := NewFilterLoggerDecorator(inner, []string{"secret", "password"}, nil, nil)

		decorator.Info("normal message", "key", "value")
		decorator.Info("contains secret data", "key", "value")
		decorator.Error("password failed", "key", "value")
		decorator.Warn("normal warning", "key", "value")

		entries := inner.GetEntries()
		require.Len(t, entries, 2) // Should filter out 2 messages

		assert.Equal(t, "normal message", entries[0].Message)
		assert.Equal(t, "normal warning", entries[1].Message)
	})

	t.Run("Filters by key-value pairs", func(t *testing.T) {
		inner := NewTestLogger()
		keyFilters := map[string]string{"env": "test", "debug": "true"}
		decorator := NewFilterLoggerDecorator(inner, nil, keyFilters, nil)

		decorator.Info("message 1", "env", "production") // Should pass
		decorator.Info("message 2", "env", "test")       // Should be filtered
		decorator.Info("message 3", "debug", "false")    // Should pass
		decorator.Info("message 4", "debug", "true")     // Should be filtered

		entries := inner.GetEntries()
		require.Len(t, entries, 2)

		assert.Equal(t, "message 1", entries[0].Message)
		assert.Equal(t, "message 3", entries[1].Message)
	})

	t.Run("Filters by log level", func(t *testing.T) {
		inner := NewTestLogger()
		levelFilters := map[string]bool{"debug": false, "info": true, "warn": true, "error": true}
		decorator := NewFilterLoggerDecorator(inner, nil, nil, levelFilters)

		decorator.Info("info message")
		decorator.Debug("debug message") // Should be filtered
		decorator.Warn("warn message")
		decorator.Error("error message")

		entries := inner.GetEntries()
		require.Len(t, entries, 3)

		assert.Equal(t, "info", entries[0].Level)
		assert.Equal(t, "warn", entries[1].Level)
		assert.Equal(t, "error", entries[2].Level)
	})

	t.Run("Combines multiple filter types", func(t *testing.T) {
		inner := NewTestLogger()
		messageFilters := []string{"secret"}
		keyFilters := map[string]string{"env": "test"}
		levelFilters := map[string]bool{"debug": false}

		decorator := NewFilterLoggerDecorator(inner, messageFilters, keyFilters, levelFilters)

		decorator.Info("normal message", "env", "prod")  // Should pass
		decorator.Info("secret message", "env", "prod")  // Filtered by message
		decorator.Info("normal message", "env", "test")  // Filtered by key-value
		decorator.Debug("normal message", "env", "prod") // Filtered by level
		decorator.Error("normal message", "env", "prod") // Should pass

		entries := inner.GetEntries()
		require.Len(t, entries, 2)

		assert.Equal(t, "normal message", entries[0].Message)
		assert.Equal(t, "info", entries[0].Level)
		assert.Equal(t, "normal message", entries[1].Message)
		assert.Equal(t, "error", entries[1].Level)
	})
}

func TestLevelModifierLoggerDecorator(t *testing.T) {
	t.Parallel()
	t.Run("Modifies log levels according to mapping", func(t *testing.T) {
		inner := NewTestLogger()
		levelMappings := map[string]string{
			"info":  "debug",
			"error": "warn",
		}
		decorator := NewLevelModifierLoggerDecorator(inner, levelMappings)

		decorator.Info("info message")   // Should become debug
		decorator.Error("error message") // Should become warn
		decorator.Warn("warn message")   // Should stay warn
		decorator.Debug("debug message") // Should stay debug

		entries := inner.GetEntries()
		require.Len(t, entries, 4)

		assert.Equal(t, "debug", entries[0].Level)
		assert.Equal(t, "info message", entries[0].Message)

		assert.Equal(t, "warn", entries[1].Level)
		assert.Equal(t, "error message", entries[1].Message)

		assert.Equal(t, "warn", entries[2].Level)
		assert.Equal(t, "warn message", entries[2].Message)

		assert.Equal(t, "debug", entries[3].Level)
		assert.Equal(t, "debug message", entries[3].Message)
	})

	t.Run("Handles unknown target levels gracefully", func(t *testing.T) {
		inner := NewTestLogger()
		levelMappings := map[string]string{
			"info": "unknown-level",
		}
		decorator := NewLevelModifierLoggerDecorator(inner, levelMappings)

		decorator.Info("test message")

		entries := inner.GetEntries()
		require.Len(t, entries, 1)
		// Should fall back to original level
		assert.Equal(t, "info", entries[0].Level)
	})
}

func TestPrefixLoggerDecorator(t *testing.T) {
	t.Parallel()
	t.Run("Adds prefix to all messages", func(t *testing.T) {
		inner := NewTestLogger()
		decorator := NewPrefixLoggerDecorator(inner, "[MODULE]")

		decorator.Info("test message", "key", "value")
		decorator.Error("error occurred", "error", "details")

		entries := inner.GetEntries()
		require.Len(t, entries, 2)

		assert.Equal(t, "[MODULE] test message", entries[0].Message)
		assert.Equal(t, "[MODULE] error occurred", entries[1].Message)
	})

	t.Run("Handles empty prefix", func(t *testing.T) {
		inner := NewTestLogger()
		decorator := NewPrefixLoggerDecorator(inner, "")

		decorator.Info("test message")

		entries := inner.GetEntries()
		require.Len(t, entries, 1)
		assert.Equal(t, "test message", entries[0].Message)
	})
}

func TestDecoratorComposition(t *testing.T) {
	t.Parallel()
	t.Run("Can compose multiple decorators", func(t *testing.T) {
		primary := NewTestLogger()
		secondary := NewTestLogger()

		// Create a complex decorator chain:
		// PrefixDecorator -> ValueInjectionDecorator -> DualWriterDecorator
		dualWriter := NewDualWriterLoggerDecorator(primary, secondary)
		valueInjection := NewValueInjectionLoggerDecorator(dualWriter, "service", "composed")
		prefix := NewPrefixLoggerDecorator(valueInjection, "[COMPOSED]")

		prefix.Info("test message", "key", "value")

		// Both loggers should receive the fully decorated log
		primaryEntries := primary.GetEntries()
		secondaryEntries := secondary.GetEntries()

		require.Len(t, primaryEntries, 1)
		require.Len(t, secondaryEntries, 1)

		// Check message has prefix
		assert.Equal(t, "[COMPOSED] test message", primaryEntries[0].Message)
		assert.Equal(t, "[COMPOSED] test message", secondaryEntries[0].Message)

		// Check injected values are present
		primaryArgs := argsToMap(primaryEntries[0].Args)
		secondaryArgs := argsToMap(secondaryEntries[0].Args)

		assert.Equal(t, "composed", primaryArgs["service"])
		assert.Equal(t, "value", primaryArgs["key"])

		assert.Equal(t, "composed", secondaryArgs["service"])
		assert.Equal(t, "value", secondaryArgs["key"])
	})
}

// TestSanitizeLogArgs_SensitiveKeys verifies that sanitizeLogArgs masks values for the
// precise set of genuinely-sensitive keys, including case-insensitive and compound forms
// (go/clear-text-logging).
func TestSanitizeLogArgs_SensitiveKeys(t *testing.T) {
	t.Parallel()

	sensitiveKeys := []string{
		// substring matches (precise — no collisions with innocent words)
		"password", "Password", "PASSWORD",
		"passwd", "Passwd",
		"secret", "Secret", "SECRET",
		"db_secret",
		"credential", "Credential", "credentials",
		"apikey", "ApiKey", "APIKEY",
		"api_key", "Api_Key", "API_KEY",
		"x-api-key",
		"accesskey", "AccessKey",
		"access_key", "Access_Key",
		"privatekey", "PrivateKey",
		"private_key", "Private_Key",
		"authorization", "Authorization", "AUTHORIZATION",
		"cookie", "Cookie", "COOKIE",
		"set-cookie", "Set-Cookie", // contains "cookie"
		"bearer", "Bearer", "BEARER",
		"access_token", "Access_Token",
		"refresh_token", "Refresh_Token",
		"id_token",
		"session_token",
		"auth_token", "Auth_Token",
		// exact matches
		"tenant", "Tenant", "TENANT",
		"requestId", "requestid", "REQUESTID",
	}

	for _, key := range sensitiveKeys {
		key := key
		t.Run("masks_"+key, func(t *testing.T) {
			t.Parallel()
			args := []any{key, "super-secret-value", "safe_key", "safe-value"}
			result := sanitizeLogArgs(args)
			assert.Equal(t, "***", result[1], "value for key %q should be masked", key)
			assert.Equal(t, "safe-value", result[3], "non-sensitive key should pass through")
		})
	}
}

// TestSanitizeLogArgs_NotMasked guards against over-masking: precise patterns must NOT
// collide with innocent observability keys. This is a regression guard for the
// adversarial-review findings (tenantID, token_count, author, etc.).
func TestSanitizeLogArgs_NotMasked(t *testing.T) {
	t.Parallel()

	innocentKeys := []string{
		// "tenant" is exact-only, so these compound forms must pass through.
		"tenantID", "tenantId", "tenantName", "tenantCount",
		// bare "token" / "auth" / "key" are intentionally NOT substrings.
		"token_count", "tokenCount", "numTokens",
		"author", "authority", "authenticated", "authn", "authz",
		"primary_key", "primaryKey", "key", "keyspace",
		// general observability fields.
		"service", "version", "content_length", "request", "status",
		"method", "url", "duration_ms", "id",
	}

	for _, key := range innocentKeys {
		key := key
		t.Run("passes_"+key, func(t *testing.T) {
			t.Parallel()
			args := []any{key, "observable-value"}
			result := sanitizeLogArgs(args)
			assert.Equal(t, "observable-value", result[1], "innocent key %q must NOT be masked", key)
		})
	}
}

// TestSanitizeLogArgs_NonSensitivePassThrough verifies benign keys are not masked while
// the exact-match keys (requestId) still are.
func TestSanitizeLogArgs_NonSensitivePassThrough(t *testing.T) {
	t.Parallel()
	args := []any{"service", "my-service", "version", "1.2.3", "requestId", "abc123"}
	result := sanitizeLogArgs(args)
	assert.Equal(t, "my-service", result[1])
	assert.Equal(t, "1.2.3", result[3])
	// requestId is masked (exact-match key).
	assert.Equal(t, "***", result[5])
}

// TestDecorators_SensitiveArgsAreMasked verifies each decorator sanitizes sensitive args
// before forwarding to the inner logger (go/clear-text-logging fix).
func TestDecorators_SensitiveArgsAreMasked(t *testing.T) {
	t.Parallel()

	sensitiveArgs := []any{"password", "hunter2", "safe", "value"}

	t.Run("BaseLoggerDecorator is a pure passthrough (subclasses own masking)", func(t *testing.T) {
		// BaseLoggerDecorator intentionally does NOT sanitize — it is a foundation
		// type used by MaskingLogger and others that own their own redaction pipeline.
		// Sanitization is applied by the higher-level decorators (DualWriter, Filter, etc.).
		t.Parallel()
		inner := NewTestLogger()
		dec := NewBaseLoggerDecorator(inner)
		dec.Info("msg", sensitiveArgs...)
		require.Len(t, inner.entries, 1)
		// Passthrough: value is NOT masked by base (masking comes from the caller layer)
		m := argsToMap(inner.entries[0].Args)
		assert.Equal(t, "hunter2", m["password"], "BaseLoggerDecorator passes args through unchanged")
	})

	t.Run("DualWriterLoggerDecorator masks sensitive args in both inner and secondary", func(t *testing.T) {
		t.Parallel()
		primary := NewTestLogger()
		secondary := NewTestLogger()
		dec := NewDualWriterLoggerDecorator(primary, secondary)
		dec.Error("msg", sensitiveArgs...)
		require.Len(t, primary.entries, 1)
		require.Len(t, secondary.entries, 1)
		pm := argsToMap(primary.entries[0].Args)
		sm := argsToMap(secondary.entries[0].Args)
		assert.Equal(t, "***", pm["password"])
		assert.Equal(t, "***", sm["password"])
		assert.Equal(t, "value", pm["safe"])
		assert.Equal(t, "value", sm["safe"])
	})

	t.Run("ValueInjectionLoggerDecorator masks sensitive args in combined args", func(t *testing.T) {
		t.Parallel()
		inner := NewTestLogger()
		dec := NewValueInjectionLoggerDecorator(inner, "service", "svc")
		dec.Warn("msg", sensitiveArgs...)
		require.Len(t, inner.entries, 1)
		m := argsToMap(inner.entries[0].Args)
		assert.Equal(t, "***", m["password"])
		assert.Equal(t, "svc", m["service"])
		assert.Equal(t, "value", m["safe"])
	})

	t.Run("FilterLoggerDecorator masks sensitive args that pass filter", func(t *testing.T) {
		t.Parallel()
		inner := NewTestLogger()
		dec := NewFilterLoggerDecorator(inner, nil, nil, nil)
		dec.Info("msg", sensitiveArgs...)
		require.Len(t, inner.entries, 1)
		m := argsToMap(inner.entries[0].Args)
		assert.Equal(t, "***", m["password"])
		assert.Equal(t, "value", m["safe"])
	})

	t.Run("LevelModifierLoggerDecorator masks sensitive args", func(t *testing.T) {
		t.Parallel()
		inner := NewTestLogger()
		dec := NewLevelModifierLoggerDecorator(inner, map[string]string{"info": "debug"})
		dec.Info("msg", sensitiveArgs...)
		require.Len(t, inner.entries, 1)
		m := argsToMap(inner.entries[0].Args)
		assert.Equal(t, "***", m["password"])
		assert.Equal(t, "value", m["safe"])
	})

	t.Run("Authorization key is masked across decorators", func(t *testing.T) {
		t.Parallel()
		inner := NewTestLogger()
		dec := NewDualWriterLoggerDecorator(inner, NewTestLogger())
		dec.Info("request", "authorization", "Bearer abc123", "method", "GET")
		require.Len(t, inner.entries, 1)
		m := argsToMap(inner.entries[0].Args)
		assert.Equal(t, "***", m["authorization"])
		assert.Equal(t, "GET", m["method"])
	})

	t.Run("access_token key is masked in ValueInjection decorator", func(t *testing.T) {
		t.Parallel()
		inner := NewTestLogger()
		dec := NewValueInjectionLoggerDecorator(inner, "env", "prod")
		dec.Debug("auth", "access_token", "secret-token-value", "user", "alice")
		require.Len(t, inner.entries, 1)
		m := argsToMap(inner.entries[0].Args)
		assert.Equal(t, "***", m["access_token"])
		assert.Equal(t, "alice", m["user"])
	})

	t.Run("innocent keys (token_count, tenantID, author) are NOT masked across decorators", func(t *testing.T) {
		t.Parallel()
		inner := NewTestLogger()
		dec := NewDualWriterLoggerDecorator(inner, NewTestLogger())
		dec.Info("metrics", "token_count", 42, "tenantID", "tenant-7", "author", "alice")
		require.Len(t, inner.entries, 1)
		m := argsToMap(inner.entries[0].Args)
		assert.Equal(t, 42, m["token_count"], "token_count must not be masked")
		assert.Equal(t, "tenant-7", m["tenantID"], "tenantID must not be masked")
		assert.Equal(t, "alice", m["author"], "author must not be masked")
	})
}

// Test the SetLogger/Service integration fix
func TestSetLoggerServiceIntegration(t *testing.T) {
	t.Parallel()
	t.Run("SetLogger updates both app.Logger() and service registry", func(t *testing.T) {
		initialLogger := NewTestLogger()
		app := NewStdApplication(NewStdConfigProvider(&struct{}{}), initialLogger)

		// Verify initial state
		assert.Equal(t, initialLogger, app.Logger())

		var retrievedLogger Logger
		err := app.GetService("logger", &retrievedLogger)
		require.NoError(t, err)
		assert.Equal(t, initialLogger, retrievedLogger)

		// Create and set new logger
		newLogger := NewTestLogger()
		app.SetLogger(newLogger)

		// Both app.Logger() and service should return the new logger
		assert.Equal(t, newLogger, app.Logger())

		var updatedLogger Logger
		err = app.GetService("logger", &updatedLogger)
		require.NoError(t, err)
		assert.Equal(t, newLogger, updatedLogger)

		// Old logger should not be returned anymore
		assert.NotSame(t, initialLogger, app.Logger())
		assert.NotSame(t, initialLogger, updatedLogger)
	})

	t.Run("SetLogger with decorated logger works with service registry", func(t *testing.T) {
		initialLogger := NewTestLogger()
		app := NewStdApplication(NewStdConfigProvider(&struct{}{}), initialLogger)

		// Create a decorated logger
		secondaryLogger := NewTestLogger()
		decoratedLogger := NewDualWriterLoggerDecorator(initialLogger, secondaryLogger)

		// Set the decorated logger
		app.SetLogger(decoratedLogger)

		// Both app.Logger() and service should return the decorated logger
		assert.Equal(t, decoratedLogger, app.Logger())

		var retrievedLogger Logger
		err := app.GetService("logger", &retrievedLogger)
		require.NoError(t, err)
		assert.Equal(t, decoratedLogger, retrievedLogger)

		// Test that the decorated logger actually works
		app.Logger().Info("test message", "key", "value")

		// Both underlying loggers should have received the message
		assert.Equal(t, 1, len(initialLogger.GetEntries()))
		assert.Equal(t, 1, len(secondaryLogger.GetEntries()))
	})

	t.Run("Modules get updated logger after SetLogger", func(t *testing.T) {
		initialLogger := NewTestLogger()
		app := NewStdApplication(NewStdConfigProvider(&struct{}{}), initialLogger)

		// Simulate what a module would do - get logger from service registry
		var moduleLogger Logger
		err := app.GetService("logger", &moduleLogger)
		require.NoError(t, err)

		// Use the logger
		moduleLogger.Info("initial message")
		assert.Equal(t, 1, len(initialLogger.GetEntries()))

		// Now set a new logger
		newLogger := NewTestLogger()
		app.SetLogger(newLogger)

		// Module gets the logger again (as it would in real usage)
		var updatedModuleLogger Logger
		err = app.GetService("logger", &updatedModuleLogger)
		require.NoError(t, err)

		// Use the updated logger
		updatedModuleLogger.Info("updated message")

		// New logger should have the message, old one should not have the new message
		assert.Equal(t, 1, len(initialLogger.GetEntries())) // Still just the initial message
		assert.Equal(t, 1, len(newLogger.GetEntries()))     // Should have the updated message
	})

	t.Run("SetLogger nil works correctly for app.Logger()", func(t *testing.T) {
		initialLogger := NewTestLogger()
		app := NewStdApplication(NewStdConfigProvider(&struct{}{}), initialLogger)

		// Set logger to nil
		app.SetLogger(nil)

		// app.Logger() should return nil
		assert.Nil(t, app.Logger())

		// Note: GetService with nil services may not be supported by the current implementation
		// but SetLogger should at least update the direct logger reference
	})
}
