package modular

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"time"
)

// SecretType represents different classifications of secrets
type SecretType int

const (
	// SecretTypeGeneric represents a generic secret
	SecretTypeGeneric SecretType = iota
	
	// SecretTypePassword represents a password secret
	SecretTypePassword
	
	// SecretTypeToken represents a token or API key secret
	SecretTypeToken
	
	// SecretTypeKey represents a cryptographic key secret
	SecretTypeKey
	
	// SecretTypeCertificate represents a certificate secret
	SecretTypeCertificate
)

// String returns the string representation of the secret type
func (s SecretType) String() string {
	switch s {
	case SecretTypePassword:
		return "password"
	case SecretTypeToken:
		return "token"
	case SecretTypeKey:
		return "key"
	case SecretTypeCertificate:
		return "certificate"
	default:
		return "generic"
	}
}

// SecretValue is a secure wrapper for sensitive configuration values.
// It ensures secrets are properly redacted in string output, JSON marshaling,
// and logging, while providing controlled access through the Reveal() method.
//
// Key features:
//   - Automatic redaction in String(), fmt output, and JSON marshaling
//   - Controlled access via Reveal() method
//   - Classification system for different secret types
//   - Memory safety with value zeroing on finalization
//   - Safe comparison methods that don't leak timing information
//   - Integration with structured logging to prevent accidental exposure
type SecretValue struct {
	// encryptedValue stores the secret in encrypted form
	encryptedValue []byte
	
	// key stores the encryption key
	key []byte
	
	// secretType classifies the type of secret
	secretType SecretType
	
	// isEmpty tracks if the secret is empty
	isEmpty bool
	
	// created tracks when the secret was created
	created time.Time
}

// NewSecretValue creates a new SecretValue with the given value and type
func NewSecretValue(value string, secretType SecretType) *SecretValue {
	if value == "" {
		return &SecretValue{
			secretType: secretType,
			isEmpty:    true,
			created:    time.Now(),
		}
	}
	
	// Generate a random key for encryption
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		// Fallback to a simple XOR if crypto/rand fails
		for i := range key {
			key[i] = byte(i * 7) // Simple but deterministic fallback
		}
	}
	
	// Simple XOR encryption (not cryptographically secure, but adds a layer)
	valueBytes := []byte(value)
	encrypted := make([]byte, len(valueBytes))
	for i, b := range valueBytes {
		encrypted[i] = b ^ key[i%len(key)]
	}
	
	secret := &SecretValue{
		encryptedValue: encrypted,
		key:            key,
		secretType:     secretType,
		isEmpty:        false,
		created:        time.Now(),
	}
	
	// Set finalizer to zero out memory when garbage collected
	runtime.SetFinalizer(secret, (*SecretValue).zeroMemory)
	
	return secret
}

// NewGenericSecret creates a new generic SecretValue
func NewGenericSecret(value string) *SecretValue {
	return NewSecretValue(value, SecretTypeGeneric)
}

// NewPasswordSecret creates a new password SecretValue
func NewPasswordSecret(value string) *SecretValue {
	return NewSecretValue(value, SecretTypePassword)
}

// NewTokenSecret creates a new token SecretValue
func NewTokenSecret(value string) *SecretValue {
	return NewSecretValue(value, SecretTypeToken)
}

// NewKeySecret creates a new key SecretValue
func NewKeySecret(value string) *SecretValue {
	return NewSecretValue(value, SecretTypeKey)
}

// NewCertificateSecret creates a new certificate SecretValue
func NewCertificateSecret(value string) *SecretValue {
	return NewSecretValue(value, SecretTypeCertificate)
}

// String returns a redacted representation of the secret
func (s *SecretValue) String() string {
	if s == nil {
		return "[REDACTED]"
	}
	
	if s.isEmpty {
		return "[EMPTY]"
	}
	
	return "[REDACTED]"
}

// GoString returns a redacted representation for fmt %#v
func (s *SecretValue) GoString() string {
	if s == nil {
		return "SecretValue{[REDACTED]}"
	}
	
	return fmt.Sprintf("SecretValue{type:%s, [REDACTED]}", s.secretType.String())
}

// Reveal returns the actual secret value for controlled access
// This should only be used in internal paths where the secret is needed
func (s *SecretValue) Reveal() string {
	if s == nil || s.isEmpty {
		return ""
	}
	
	// Decrypt the value
	decrypted := make([]byte, len(s.encryptedValue))
	for i, b := range s.encryptedValue {
		decrypted[i] = b ^ s.key[i%len(s.key)]
	}
	
	result := string(decrypted)
	
	// Zero out the decrypted bytes immediately
	for i := range decrypted {
		decrypted[i] = 0
	}
	
	return result
}

// IsEmpty returns true if the secret value is empty
func (s *SecretValue) IsEmpty() bool {
	if s == nil {
		return true
	}
	return s.isEmpty
}

// Equals performs a constant-time comparison with another SecretValue
// This prevents timing attacks that could leak information about the secret
func (s *SecretValue) Equals(other *SecretValue) bool {
	if s == nil && other == nil {
		return true
	}
	
	if s == nil || other == nil {
		return false
	}
	
	// Compare empty status
	if s.isEmpty != other.isEmpty {
		return false
	}
	
	if s.isEmpty {
		return true
	}
	
	// For non-empty secrets, compare the revealed values
	// Note: This could be optimized to compare encrypted values directly
	// but that would require matching encryption keys
	val1 := s.Reveal()
	val2 := other.Reveal()
	
	// Constant-time comparison
	result := constantTimeEquals(val1, val2)
	
	// Zero out revealed values
	zeroString(&val1)
	zeroString(&val2)
	
	return result
}

// EqualsString performs a constant-time comparison with a string value
func (s *SecretValue) EqualsString(value string) bool {
	if s == nil {
		return value == ""
	}
	
	if s.isEmpty {
		return value == ""
	}
	
	revealed := s.Reveal()
	result := constantTimeEquals(revealed, value)
	
	// Zero out revealed value
	zeroString(&revealed)
	
	return result
}

// Type returns the secret type classification
func (s *SecretValue) Type() SecretType {
	if s == nil {
		return SecretTypeGeneric
	}
	return s.secretType
}

// Created returns when the secret was created
func (s *SecretValue) Created() time.Time {
	if s == nil {
		return time.Time{}
	}
	return s.created
}

// MarshalJSON implements json.Marshaler to always redact secrets in JSON
func (s *SecretValue) MarshalJSON() ([]byte, error) {
	return json.Marshal("[REDACTED]")
}

// UnmarshalJSON implements json.Unmarshaler to handle JSON input
// Note: This creates a generic secret from the input
func (s *SecretValue) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	
	// Don't allow unmarshaling of redacted values
	if value == "[REDACTED]" || value == "[EMPTY]" {
		*s = SecretValue{
			secretType: SecretTypeGeneric,
			isEmpty:    true,
			created:    time.Now(),
		}
		return nil
	}
	
	// Create a new secret
	newSecret := NewSecretValue(value, SecretTypeGeneric)
	*s = *newSecret
	
	return nil
}

// MarshalText implements encoding.TextMarshaler to redact in text formats
func (s *SecretValue) MarshalText() ([]byte, error) {
	return []byte("[REDACTED]"), nil
}

// UnmarshalText implements encoding.TextUnmarshaler 
func (s *SecretValue) UnmarshalText(text []byte) error {
	value := string(text)
	
	// Don't allow unmarshaling of redacted values
	if value == "[REDACTED]" || value == "[EMPTY]" {
		*s = SecretValue{
			secretType: SecretTypeGeneric,
			isEmpty:    true,
			created:    time.Now(),
		}
		return nil
	}
	
	// Create a new secret
	newSecret := NewSecretValue(value, SecretTypeGeneric)
	*s = *newSecret
	
	return nil
}

// Clone creates a copy of the SecretValue
func (s *SecretValue) Clone() *SecretValue {
	if s == nil {
		return nil
	}
	
	if s.isEmpty {
		return &SecretValue{
			secretType: s.secretType,
			isEmpty:    true,
			created:    time.Now(),
		}
	}
	
	// Clone by revealing and re-encrypting
	value := s.Reveal()
	result := NewSecretValue(value, s.secretType)
	
	// Zero out the revealed value
	zeroString(&value)
	
	return result
}

// zeroMemory zeros out the secret's memory (called by finalizer)
func (s *SecretValue) zeroMemory() {
	if s == nil {
		return
	}
	
	// Zero out encrypted value
	for i := range s.encryptedValue {
		s.encryptedValue[i] = 0
	}
	
	// Zero out key
	for i := range s.key {
		s.key[i] = 0
	}
	
	// Clear slices
	s.encryptedValue = nil
	s.key = nil
}

// Destroy explicitly zeros out the secret's memory
func (s *SecretValue) Destroy() {
	if s == nil {
		return
	}
	
	s.zeroMemory()
	s.isEmpty = true
}

// Helper functions

// constantTimeEquals performs constant-time string comparison to prevent timing attacks
func constantTimeEquals(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	
	result := 0
	for i := 0; i < len(a); i++ {
		result |= int(a[i]) ^ int(b[i])
	}
	
	return result == 0
}

// zeroString attempts to zero out a string's underlying memory
// Note: This is a best-effort approach that may not work in all Go implementations
// due to string immutability. In production, consider using dedicated secret management libraries.
func zeroString(s *string) {
	if s == nil || len(*s) == 0 {
		return
	}
	
	// Due to Go's string immutability and safety checks, we cannot safely
	// zero out string memory without potentially causing crashes.
	// Instead, we'll just set the string to empty.
	// For true secure memory handling, use specialized libraries.
	*s = ""
}


// SecretRedactor provides utility functions for secret redaction in logs and output
type SecretRedactor struct {
	patterns []string
	secrets  []*SecretValue
}

// NewSecretRedactor creates a new secret redactor
func NewSecretRedactor() *SecretRedactor {
	return &SecretRedactor{
		patterns: make([]string, 0),
		secrets:  make([]*SecretValue, 0),
	}
}

// AddSecret adds a secret to be redacted
func (r *SecretRedactor) AddSecret(secret *SecretValue) {
	if secret == nil || secret.IsEmpty() {
		return
	}
	
	r.secrets = append(r.secrets, secret)
}

// AddPattern adds a pattern to be redacted
func (r *SecretRedactor) AddPattern(pattern string) {
	if pattern == "" {
		return
	}
	
	r.patterns = append(r.patterns, pattern)
}

// Redact redacts secrets and patterns from the input text
func (r *SecretRedactor) Redact(text string) string {
	result := text
	
	// Redact secret values
	for _, secret := range r.secrets {
		if !secret.IsEmpty() {
			value := secret.Reveal()
			if value != "" {
				result = strings.ReplaceAll(result, value, "[REDACTED]")
			}
			zeroString(&value)
		}
	}
	
	// Redact patterns
	for _, pattern := range r.patterns {
		result = strings.ReplaceAll(result, pattern, "[REDACTED]")
	}
	
	return result
}

// RedactStructuredLog redacts secrets from structured log fields
func (r *SecretRedactor) RedactStructuredLog(fields map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	
	for key, value := range fields {
		switch v := value.(type) {
		case *SecretValue:
			result[key] = "[REDACTED]"
		case SecretValue:
			result[key] = "[REDACTED]"
		case string:
			result[key] = r.Redact(v)
		default:
			result[key] = value
		}
	}
	
	return result
}

// Global secret redactor instance for application-wide use
var globalSecretRedactor = NewSecretRedactor()

// RegisterGlobalSecret registers a secret with the global redactor
func RegisterGlobalSecret(secret *SecretValue) {
	globalSecretRedactor.AddSecret(secret)
}

// RegisterGlobalPattern registers a pattern with the global redactor
func RegisterGlobalPattern(pattern string) {
	globalSecretRedactor.AddPattern(pattern)
}

// RedactGlobally redacts secrets using the global redactor
func RedactGlobally(text string) string {
	return globalSecretRedactor.Redact(text)
}

// RedactGloballyStructured redacts secrets from structured log fields using the global redactor
func RedactGloballyStructured(fields map[string]interface{}) map[string]interface{} {
	return globalSecretRedactor.RedactStructuredLog(fields)
}