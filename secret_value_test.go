package modular

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSecretValueBasic tests basic SecretValue functionality without build tags
func TestSecretValueBasic(t *testing.T) {
	t.Run("should_create_secret_values", func(t *testing.T) {
		secret := NewSecretValue("my-secret-password", SecretTypePassword)
		assert.NotNil(t, secret)
		assert.False(t, secret.IsEmpty())
		assert.Equal(t, SecretTypePassword, secret.Type())
		
		// Should reveal the original value
		assert.Equal(t, "my-secret-password", secret.Reveal())
	})
	
	t.Run("should_create_empty_secret", func(t *testing.T) {
		secret := NewSecretValue("", SecretTypeGeneric)
		assert.NotNil(t, secret)
		assert.True(t, secret.IsEmpty())
		assert.Equal(t, "", secret.Reveal())
	})
	
	t.Run("should_redact_in_string_output", func(t *testing.T) {
		secret := NewGenericSecret("super-secret-value")
		
		// String() should redact
		assert.Equal(t, "[REDACTED]", secret.String())
		
		// fmt.Sprintf should redact
		formatted := fmt.Sprintf("Secret: %s", secret)
		assert.Equal(t, "Secret: [REDACTED]", formatted)
		
		// fmt.Sprintf with %v should redact
		formatted = fmt.Sprintf("Secret: %v", secret)
		assert.Equal(t, "Secret: [REDACTED]", formatted)
		
		// fmt.Sprintf with %#v should show type but redact value
		formatted = fmt.Sprintf("Secret: %#v", secret)
		assert.Contains(t, formatted, "SecretValue")
		assert.Contains(t, formatted, "[REDACTED]")
		assert.NotContains(t, formatted, "super-secret-value")
	})
	
	t.Run("should_redact_empty_secrets", func(t *testing.T) {
		secret := NewGenericSecret("")
		assert.Equal(t, "[EMPTY]", secret.String())
		
		// Nil secrets should also redact
		var nilSecret *SecretValue
		assert.Equal(t, "[REDACTED]", nilSecret.String())
	})
	
	t.Run("should_redact_in_json_marshaling", func(t *testing.T) {
		secret := NewTokenSecret("sk-123456789")
		
		data, err := json.Marshal(secret)
		assert.NoError(t, err)
		assert.Equal(t, `"[REDACTED]"`, string(data))
		
		// Should not contain the actual secret
		assert.NotContains(t, string(data), "sk-123456789")
	})
	
	t.Run("should_handle_json_unmarshaling", func(t *testing.T) {
		// Unmarshal regular value
		var secret SecretValue
		err := json.Unmarshal([]byte(`"test-secret"`), &secret)
		assert.NoError(t, err)
		assert.Equal(t, "test-secret", secret.Reveal())
		assert.Equal(t, SecretTypeGeneric, secret.Type())
		
		// Unmarshal redacted value should create empty secret
		var redactedSecret SecretValue
		err = json.Unmarshal([]byte(`"[REDACTED]"`), &redactedSecret)
		assert.NoError(t, err)
		assert.True(t, redactedSecret.IsEmpty())
	})
	
	t.Run("should_support_different_secret_types", func(t *testing.T) {
		password := NewPasswordSecret("pass123")
		token := NewTokenSecret("tok456")
		key := NewKeySecret("key789")
		cert := NewCertificateSecret("cert000")
		
		assert.Equal(t, SecretTypePassword, password.Type())
		assert.Equal(t, SecretTypeToken, token.Type())
		assert.Equal(t, SecretTypeKey, key.Type())
		assert.Equal(t, SecretTypeCertificate, cert.Type())
		
		// All should redact the same way
		assert.Equal(t, "[REDACTED]", password.String())
		assert.Equal(t, "[REDACTED]", token.String())
		assert.Equal(t, "[REDACTED]", key.String())
		assert.Equal(t, "[REDACTED]", cert.String())
	})
	
	t.Run("should_support_equality_comparison", func(t *testing.T) {
		secret1 := NewGenericSecret("same-value")
		secret2 := NewGenericSecret("same-value")
		secret3 := NewGenericSecret("different-value")
		
		// Same values should be equal
		assert.True(t, secret1.Equals(secret2))
		assert.True(t, secret1.EqualsString("same-value"))
		
		// Different values should not be equal
		assert.False(t, secret1.Equals(secret3))
		assert.False(t, secret1.EqualsString("different-value"))
		
		// Empty secrets should be equal
		empty1 := NewGenericSecret("")
		empty2 := NewGenericSecret("")
		assert.True(t, empty1.Equals(empty2))
		assert.True(t, empty1.EqualsString(""))
		
		// Nil secrets
		var nil1, nil2 *SecretValue
		assert.True(t, nil1.Equals(nil2))
		assert.False(t, nil1.Equals(secret1))
		assert.True(t, nil1.EqualsString(""))
	})
	
	t.Run("should_support_cloning", func(t *testing.T) {
		original := NewPasswordSecret("original-password")
		cloned := original.Clone()
		
		assert.NotNil(t, cloned)
		assert.Equal(t, original.Type(), cloned.Type())
		assert.True(t, original.Equals(cloned))
		assert.Equal(t, original.Reveal(), cloned.Reveal())
		
		// Should be different instances
		assert.NotSame(t, original, cloned)
		
		// Clone of empty secret
		empty := NewGenericSecret("")
		emptyClone := empty.Clone()
		assert.True(t, emptyClone.IsEmpty())
		
		// Clone of nil should be nil
		var nilSecret *SecretValue
		nilClone := nilSecret.Clone()
		assert.Nil(t, nilClone)
	})
	
	t.Run("should_support_destroy", func(t *testing.T) {
		secret := NewGenericSecret("destroy-me")
		assert.Equal(t, "destroy-me", secret.Reveal())
		
		secret.Destroy()
		assert.True(t, secret.IsEmpty())
		assert.Equal(t, "", secret.Reveal())
	})
}

// TestSecretRedactor tests the secret redaction functionality
func TestSecretRedactor(t *testing.T) {
	t.Run("should_create_redactor", func(t *testing.T) {
		redactor := NewSecretRedactor()
		assert.NotNil(t, redactor)
		
		// Should not redact anything initially
		text := "no secrets here"
		assert.Equal(t, text, redactor.Redact(text))
	})
	
	t.Run("should_redact_secrets", func(t *testing.T) {
		redactor := NewSecretRedactor()
		secret := NewGenericSecret("my-secret-123")
		redactor.AddSecret(secret)
		
		text := "The secret is my-secret-123 in this text"
		redacted := redactor.Redact(text)
		
		assert.Equal(t, "The secret is [REDACTED] in this text", redacted)
		assert.NotContains(t, redacted, "my-secret-123")
	})
	
	t.Run("should_redact_patterns", func(t *testing.T) {
		redactor := NewSecretRedactor()
		redactor.AddPattern("password=secret123")
		
		text := "Connection string: user:pass@host?password=secret123"
		redacted := redactor.Redact(text)
		
		assert.Equal(t, "Connection string: user:pass@host?[REDACTED]", redacted)
	})
	
	t.Run("should_redact_structured_logs", func(t *testing.T) {
		redactor := NewSecretRedactor()
		secret := NewTokenSecret("token-abc123")
		redactor.AddSecret(secret)
		
		fields := map[string]interface{}{
			"level":   "info",
			"message": "Authentication successful with token-abc123",
			"token":   secret,
			"user":    "john",
		}
		
		redacted := redactor.RedactStructuredLog(fields)
		
		assert.Equal(t, "info", redacted["level"])
		assert.Equal(t, "Authentication successful with [REDACTED]", redacted["message"])
		assert.Equal(t, "[REDACTED]", redacted["token"])
		assert.Equal(t, "john", redacted["user"])
	})
	
	t.Run("should_handle_empty_secrets", func(t *testing.T) {
		redactor := NewSecretRedactor()
		
		// Adding nil or empty secrets should not cause issues
		redactor.AddSecret(nil)
		redactor.AddSecret(NewGenericSecret(""))
		
		text := "no secrets to redact"
		assert.Equal(t, text, redactor.Redact(text))
	})
}

// TestGlobalRedactor tests the global secret redaction functionality
func TestGlobalRedactor(t *testing.T) {
	t.Run("should_register_and_redact_globally", func(t *testing.T) {
		// Register a secret globally
		secret := NewGenericSecret("global-secret-456")
		RegisterGlobalSecret(secret)
		
		text := "This contains global-secret-456 somewhere"
		redacted := RedactGlobally(text)
		
		assert.Equal(t, "This contains [REDACTED] somewhere", redacted)
		
		// Also test structured redaction
		fields := map[string]interface{}{
			"data": "global-secret-456",
			"safe": "public-data",
		}
		
		redactedFields := RedactGloballyStructured(fields)
		assert.Equal(t, "[REDACTED]", redactedFields["data"])
		assert.Equal(t, "public-data", redactedFields["safe"])
	})
}

// TestSecretTypes tests secret type functionality
func TestSecretTypes(t *testing.T) {
	t.Run("should_convert_types_to_string", func(t *testing.T) {
		assert.Equal(t, "generic", SecretTypeGeneric.String())
		assert.Equal(t, "password", SecretTypePassword.String())
		assert.Equal(t, "token", SecretTypeToken.String())
		assert.Equal(t, "key", SecretTypeKey.String())
		assert.Equal(t, "certificate", SecretTypeCertificate.String())
	})
}

// TestSecretValueMemorySafety tests memory safety features
func TestSecretValueMemorySafety(t *testing.T) {
	t.Run("should_not_leak_secrets_in_debug_output", func(t *testing.T) {
		secret := NewPasswordSecret("super-secret-password")
		
		// Various ways someone might try to inspect the secret
		debugOutput := fmt.Sprintf("%+v", secret)
		assert.NotContains(t, debugOutput, "super-secret-password")
		
		// GoString output should be safe
		goString := secret.GoString()
		assert.NotContains(t, goString, "super-secret-password")
		assert.Contains(t, goString, "[REDACTED]")
	})
	
	t.Run("should_zero_revealed_values", func(t *testing.T) {
		secret := NewGenericSecret("temporary-reveal")
		
		// Reveal the value
		revealed := secret.Reveal()
		assert.Equal(t, "temporary-reveal", revealed)
		
		// The revealed string should still work normally
		assert.True(t, strings.Contains(revealed, "temporary"))
	})
}

// TestSecretValueEdgeCases tests edge cases and error conditions
func TestSecretValueEdgeCases(t *testing.T) {
	t.Run("should_handle_nil_secret_operations", func(t *testing.T) {
		var secret *SecretValue
		
		assert.Equal(t, "[REDACTED]", secret.String())
		assert.Equal(t, "", secret.Reveal())
		assert.True(t, secret.IsEmpty())
		assert.Equal(t, SecretTypeGeneric, secret.Type())
		assert.Nil(t, secret.Clone())
		
		// Should not panic on destroy
		secret.Destroy()
	})
	
	t.Run("should_handle_very_long_secrets", func(t *testing.T) {
		longSecret := strings.Repeat("a", 10000)
		secret := NewGenericSecret(longSecret)
		
		assert.Equal(t, longSecret, secret.Reveal())
		assert.Equal(t, "[REDACTED]", secret.String())
		
		// Should handle JSON marshaling of long secrets
		data, err := json.Marshal(secret)
		assert.NoError(t, err)
		assert.Equal(t, `"[REDACTED]"`, string(data))
	})
	
	t.Run("should_handle_special_characters", func(t *testing.T) {
		specialSecret := "secret with spaces & symbols!@#$%^&*()"
		secret := NewGenericSecret(specialSecret)
		
		assert.Equal(t, specialSecret, secret.Reveal())
		assert.Equal(t, "[REDACTED]", secret.String())
		
		// Should handle in redaction
		redactor := NewSecretRedactor()
		redactor.AddSecret(secret)
		
		text := fmt.Sprintf("The secret is: %s", specialSecret)
		redacted := redactor.Redact(text)
		assert.Equal(t, "The secret is: [REDACTED]", redacted)
	})
}