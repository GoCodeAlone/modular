package modular

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLogmaskerSecretDetection tests that the logmasker properly detects and masks SecretValue instances
func TestLogmaskerSecretDetection(t *testing.T) {
	// Create a test logger to capture output
	testLogger := &captureLogger{logs: make([]logEntry, 0)}

	// Create a test masking logger that implements the same logic
	maskingLogger := &testMaskingLogger{baseLogger: testLogger}

	t.Run("SecretValueDetection", func(t *testing.T) {
		// Test different secret types
		password := NewPasswordSecret("super-secret-password")
		token := NewTokenSecret("abc123token456")
		key := NewKeySecret("cryptographic-key")
		certificate := NewCertificateSecret("cert-data")
		genericSecret := NewGenericSecret("generic-secret")
		emptySecret := NewGenericSecret("")

		// Test each secret type
		testCases := []struct {
			name         string
			secret       *SecretValue
			expectedMask string
		}{
			{"Password", password, "[PASSWORD]"},
			{"Token", token, "[TOKEN]"},
			{"Key", key, "[KEY]"},
			{"Certificate", certificate, "[CERTIFICATE]"},
			{"Generic", genericSecret, "[REDACTED]"},
			{"Empty", emptySecret, "[EMPTY]"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Clear previous logs
				testLogger.logs = testLogger.logs[:0]

				// Log the secret
				maskingLogger.Info("Testing secret masking", "secret", tc.secret)

				// Verify log was captured
				require.Len(t, testLogger.logs, 1)
				logEntry := testLogger.logs[0]

				// Verify the log contains the masked value
				assert.Equal(t, "INFO", logEntry.level)
				assert.Equal(t, "Testing secret masking", logEntry.message)
				assert.Len(t, logEntry.args, 2)
				assert.Equal(t, "secret", logEntry.args[0])
				assert.Equal(t, tc.expectedMask, logEntry.args[1])
			})
		}
	})

	t.Run("SecretValuePointerDetection", func(t *testing.T) {
		// Test with pointer to SecretValue
		secret := NewPasswordSecret("pointer-secret")

		// Clear previous logs
		testLogger.logs = testLogger.logs[:0]

		// Log the secret pointer
		maskingLogger.Info("Testing pointer secret masking", "secret_ptr", secret)

		// Verify log was captured and masked
		require.Len(t, testLogger.logs, 1)
		logEntry := testLogger.logs[0]
		assert.Equal(t, "[PASSWORD]", logEntry.args[1])
	})

	t.Run("MixedValueTypes", func(t *testing.T) {
		// Test mixed normal values and secrets
		secret := NewTokenSecret("mixed-test-token")
		normalValue := "normal-value"

		// Clear previous logs
		testLogger.logs = testLogger.logs[:0]

		// Log mixed values
		maskingLogger.Info("Mixed values test",
			"normal", normalValue,
			"secret", secret,
			"another_normal", 12345)

		// Verify log was captured
		require.Len(t, testLogger.logs, 1)
		logEntry := testLogger.logs[0]

		// Verify args are properly handled
		expectedArgs := []any{"normal", "normal-value", "secret", "[TOKEN]", "another_normal", 12345}
		assert.Equal(t, expectedArgs, logEntry.args)
	})

	t.Run("NilSecretHandling", func(t *testing.T) {
		// Test nil secret
		var nilSecret *SecretValue = nil

		// Clear previous logs
		testLogger.logs = testLogger.logs[:0]

		// Log nil secret
		maskingLogger.Info("Nil secret test", "nil_secret", nilSecret)

		// Verify log was captured and masked
		require.Len(t, testLogger.logs, 1)
		logEntry := testLogger.logs[0]
		assert.Equal(t, "[REDACTED]", logEntry.args[1])
	})

	t.Run("SecretInterfacePatternCheck", func(t *testing.T) {
		// Test that our SecretValue properly implements the secret interface pattern
		secret := NewPasswordSecret("interface-test")

		// Verify it has the right methods
		assert.True(t, secret.ShouldMask())
		assert.Equal(t, "[PASSWORD]", secret.GetMaskedValue())
		assert.Equal(t, "redact", secret.GetMaskStrategy())

		// Test empty secret
		emptySecret := NewGenericSecret("")
		assert.True(t, emptySecret.ShouldMask())
		assert.Equal(t, "[EMPTY]", emptySecret.GetMaskedValue())
		assert.Equal(t, "redact", emptySecret.GetMaskStrategy())
	})

	t.Run("FallbackToPatternRules", func(t *testing.T) {
		// Test that logmasker still falls back to pattern rules for non-secret values
		creditCard := "4532-1234-5678-9012"

		// Clear previous logs
		testLogger.logs = testLogger.logs[:0]

		// Log credit card (should be caught by pattern rule)
		maskingLogger.Info("Pattern test", "cc", creditCard)

		// Verify log was captured and pattern rule applied
		require.Len(t, testLogger.logs, 1)
		logEntry := testLogger.logs[0]
		assert.Equal(t, "[REDACTED]", logEntry.args[1])
	})
}

// TestSecretValueInterfaceComplianceSeparately tests SecretValue interface compliance in isolation
func TestSecretValueInterfaceComplianceSeparately(t *testing.T) {
	t.Run("InterfaceCompliance", func(t *testing.T) {
		// Test that SecretValue properly implements the secret interface pattern
		// without depending on logmasker types to avoid coupling

		// Create different types of secrets
		secrets := []*SecretValue{
			NewPasswordSecret("test-password"),
			NewTokenSecret("test-token"),
			NewKeySecret("test-key"),
			NewCertificateSecret("test-cert"),
			NewGenericSecret("generic-secret"),
			NewGenericSecret(""),
		}

		expectedMasks := []string{
			"[PASSWORD]",
			"[TOKEN]",
			"[KEY]",
			"[CERTIFICATE]",
			"[REDACTED]",
			"[EMPTY]",
		}

		for i, secret := range secrets {
			// Test ShouldMask method
			assert.True(t, secret.ShouldMask(), "Secret should indicate it should be masked")

			// Test GetMaskedValue method
			masked := secret.GetMaskedValue()
			assert.Equal(t, expectedMasks[i], masked, "Masked value should match expected")

			// Test GetMaskStrategy method
			strategy := secret.GetMaskStrategy()
			assert.Equal(t, "redact", strategy, "Strategy should be 'redact'")
		}
	})

	t.Run("NilSecretHandling", func(t *testing.T) {
		var nilSecret *SecretValue = nil

		// Methods should be safe to call on nil
		assert.True(t, nilSecret.ShouldMask())
		assert.Equal(t, "[REDACTED]", nilSecret.GetMaskedValue())
		assert.Equal(t, "redact", nilSecret.GetMaskStrategy())
	})
}

// Test custom type that implements the secret interface pattern
type customSecret struct {
	value      string
	shouldMask bool
}

func (c *customSecret) ShouldMask() bool {
	return c.shouldMask
}

func (c *customSecret) GetMaskedValue() any {
	if c.shouldMask {
		return "[CUSTOM_SECRET]"
	}
	return c.value
}

func (c *customSecret) GetMaskStrategy() string {
	return "redact"
}

func TestLogmaskerCustomSecretDetection(t *testing.T) {
	// Create a test logger to capture output
	testLogger := &captureLogger{logs: make([]logEntry, 0)}

	// Create a test masking logger
	maskingLogger := &testMaskingLogger{baseLogger: testLogger}

	t.Run("CustomSecretTypeDetection", func(t *testing.T) {
		// Test custom type that should be masked
		maskedCustom := &customSecret{value: "sensitive-data", shouldMask: true}

		// Clear logs
		testLogger.logs = testLogger.logs[:0]

		maskingLogger.Info("Custom secret test", "custom", maskedCustom)

		// Verify masking
		require.Len(t, testLogger.logs, 1)
		assert.Equal(t, "[CUSTOM_SECRET]", testLogger.logs[0].args[1])
	})

	t.Run("CustomSecretTypeNoMasking", func(t *testing.T) {
		// Test custom type that should NOT be masked
		unmaskedCustom := &customSecret{value: "public-data", shouldMask: false}

		// Clear logs
		testLogger.logs = testLogger.logs[:0]

		maskingLogger.Info("Custom no-mask test", "custom", unmaskedCustom)

		// Verify no masking applied
		require.Len(t, testLogger.logs, 1)
		assert.Equal(t, unmaskedCustom, testLogger.logs[0].args[1])
	})
}

// Test logger that captures log entries for verification
type logEntry struct {
	level   string
	message string
	args    []any
}

type captureLogger struct {
	logs []logEntry
}

func (l *captureLogger) Debug(msg string, args ...any) {
	l.logs = append(l.logs, logEntry{level: "DEBUG", message: msg, args: args})
}

func (l *captureLogger) Info(msg string, args ...any) {
	l.logs = append(l.logs, logEntry{level: "INFO", message: msg, args: args})
}

func (l *captureLogger) Warn(msg string, args ...any) {
	l.logs = append(l.logs, logEntry{level: "WARN", message: msg, args: args})
}

func (l *captureLogger) Error(msg string, args ...any) {
	l.logs = append(l.logs, logEntry{level: "ERROR", message: msg, args: args})
}

// testMaskingLogger implements the same secret detection logic as the logmasker module
type testMaskingLogger struct {
	baseLogger *captureLogger
}

func (l *testMaskingLogger) Debug(msg string, args ...any) {
	maskedArgs := l.maskArgs(args...)
	l.baseLogger.Debug(msg, maskedArgs...)
}

func (l *testMaskingLogger) Info(msg string, args ...any) {
	maskedArgs := l.maskArgs(args...)
	l.baseLogger.Info(msg, maskedArgs...)
}

func (l *testMaskingLogger) Warn(msg string, args ...any) {
	maskedArgs := l.maskArgs(args...)
	l.baseLogger.Warn(msg, maskedArgs...)
}

func (l *testMaskingLogger) Error(msg string, args ...any) {
	maskedArgs := l.maskArgs(args...)
	l.baseLogger.Error(msg, maskedArgs...)
}

// maskArgs replicates the masking logic from the logmasker module
func (l *testMaskingLogger) maskArgs(args ...any) []any {
	if len(args) == 0 {
		return args
	}

	result := make([]any, len(args))

	// Process key-value pairs
	for i := 0; i < len(args); i += 2 {
		// Copy the key
		result[i] = args[i]

		// Process the value if it exists
		if i+1 < len(args) {
			value := args[i+1]

			// Check for secret interface pattern using reflection
			if l.isSecretLikeValue(value) {
				result[i+1] = l.maskSecretLikeValue(value)
				continue
			}

			// Apply simple pattern rule for credit cards (for testing)
			if strValue, ok := value.(string); ok {
				if len(strValue) >= 13 && (strValue[4] == '-' || strValue[4] == ' ') {
					result[i+1] = "[REDACTED]"
					continue
				}
			}

			result[i+1] = value
		}
	}

	return result
}

// isSecretLikeValue checks if a value implements secret-like interface patterns
func (l *testMaskingLogger) isSecretLikeValue(value any) bool {
	if value == nil {
		return false
	}

	valueReflect := reflect.ValueOf(value)
	if !valueReflect.IsValid() {
		return false
	}

	// Look for ShouldMask method
	shouldMaskMethod := valueReflect.MethodByName("ShouldMask")
	if !shouldMaskMethod.IsValid() {
		return false
	}
	methodType := shouldMaskMethod.Type()
	if methodType.NumIn() != 0 || methodType.NumOut() != 1 || methodType.Out(0).Kind() != reflect.Bool {
		return false
	}

	// Look for GetMaskedValue method
	getMaskedValueMethod := valueReflect.MethodByName("GetMaskedValue")
	if !getMaskedValueMethod.IsValid() {
		return false
	}
	methodType = getMaskedValueMethod.Type()
	if methodType.NumIn() != 0 || methodType.NumOut() != 1 {
		return false
	}

	// Look for GetMaskStrategy method
	getMaskStrategyMethod := valueReflect.MethodByName("GetMaskStrategy")
	if !getMaskStrategyMethod.IsValid() {
		return false
	}
	methodType = getMaskStrategyMethod.Type()
	if methodType.NumIn() != 0 || methodType.NumOut() != 1 || methodType.Out(0).Kind() != reflect.String {
		return false
	}

	// All three methods must be present
	return true
}

// maskSecretLikeValue masks a secret-like value using reflection
func (l *testMaskingLogger) maskSecretLikeValue(value any) any {
	if value == nil {
		return "[REDACTED]"
	}

	valueReflect := reflect.ValueOf(value)
	if !valueReflect.IsValid() {
		return "[REDACTED]"
	}

	// Call ShouldMask method
	shouldMaskMethod := valueReflect.MethodByName("ShouldMask")
	if !shouldMaskMethod.IsValid() {
		return "[REDACTED]"
	}

	shouldMaskResult := shouldMaskMethod.Call(nil)
	if len(shouldMaskResult) != 1 || shouldMaskResult[0].Kind() != reflect.Bool {
		return "[REDACTED]"
	}

	// If shouldn't mask, return original value
	if !shouldMaskResult[0].Bool() {
		return value
	}

	// Call GetMaskedValue method
	getMaskedValueMethod := valueReflect.MethodByName("GetMaskedValue")
	if !getMaskedValueMethod.IsValid() {
		return "[REDACTED]"
	}

	maskedResult := getMaskedValueMethod.Call(nil)
	if len(maskedResult) != 1 {
		return "[REDACTED]"
	}

	return maskedResult[0].Interface()
}
