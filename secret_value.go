// Package modular provides SecretValue for basic secret protection.
//
// SECURITY NOTICE: The SecretValue type in this package provides protection
// against accidental exposure but has significant security limitations due to
// Go's memory model. It cannot guarantee secure memory handling and should NOT
// be used for highly sensitive secrets like private keys or critical passwords.
//
// For maximum security, use dedicated secure memory libraries or OS-level
// secret storage mechanisms.
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

// SecretValue is a wrapper for sensitive configuration values that helps prevent
// accidental exposure in logs, JSON output, and debugging. It provides basic
// protection against accidental disclosure but has important security limitations.
//
// Security features:
//   - Automatic redaction in String(), fmt output, and JSON marshaling
//   - Controlled access via Reveal() method
//   - Classification system for different secret types
//   - Basic encryption of stored values (XOR-based, not cryptographically secure)
//   - Constant-time comparison methods to prevent timing attacks
//   - Integration with structured logging to prevent accidental exposure
//
// IMPORTANT SECURITY LIMITATIONS:
//   - Cannot zero string memory due to Go's immutable strings
//   - Garbage collector may leave copies of secrets in memory
//   - XOR encryption provides obfuscation, not cryptographic security
//   - Memory dumps may contain plaintext secrets
//   - Not suitable for highly sensitive secrets (e.g., private keys, passwords for critical systems)
//
// For maximum security, consider dedicated libraries like:
//   - github.com/awnumar/memguard (secure memory handling)
//   - Operating system secure storage (Keychain, Credential Manager, etc.)
//   - Hardware Security Modules (HSMs) for critical secrets
//
// Use this type for:
//   - Preventing accidental logging of API keys, tokens
//   - Basic protection against casual inspection
//   - Configuration values where convenience outweighs maximum security
type SecretValue struct {
	// Legacy fields (for backward compatibility)
	// encryptedValue stores the secret in encrypted form
	encryptedValue []byte

	// key stores the encryption key
	key []byte

	// Provider-based fields (new)
	// handle references the stored secret in the provider
	handle SecretHandle

	// provider is the secret provider managing this secret
	provider SecretProvider

	// Common fields
	// secretType classifies the type of secret
	secretType SecretType

	// isEmpty tracks if the secret is empty
	isEmpty bool

	// created tracks when the secret was created
	created time.Time
}

// NewSecretValue creates a new SecretValue with the given value and type
// This function now uses the global secret provider by default
func NewSecretValue(value string, secretType SecretType) *SecretValue {
	// Try to use the provider system first
	provider := GetGlobalSecretProvider()
	if provider != nil {
		return NewSecretValueWithProvider(value, secretType, provider)
	}

	// Fallback to legacy implementation if no provider is available
	return newLegacySecretValue(value, secretType)
}

// NewSecretValueWithProvider creates a new SecretValue using a specific provider
func NewSecretValueWithProvider(value string, secretType SecretType, provider SecretProvider) *SecretValue {
	if value == "" {
		return &SecretValue{
			secretType: secretType,
			isEmpty:    true,
			created:    time.Now(),
			provider:   provider,
		}
	}

	handle, err := provider.Store(value, secretType)
	if err != nil {
		// Fallback to legacy implementation if provider fails
		return newLegacySecretValue(value, secretType)
	}

	return &SecretValue{
		handle:     handle,
		provider:   provider,
		secretType: secretType,
		isEmpty:    false,
		created:    time.Now(),
	}
}

// newLegacySecretValue creates a SecretValue using the original XOR implementation
func newLegacySecretValue(value string, secretType SecretType) *SecretValue {
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

	// Check if using provider system
	if s.handle != nil && s.provider != nil {
		value, err := s.provider.Retrieve(s.handle)
		if err != nil {
			// If provider fails, fallback to legacy if available
			if s.encryptedValue != nil && s.key != nil {
				return s.revealLegacy()
			}
			return ""
		}
		return value
	}

	// Use legacy implementation
	return s.revealLegacy()
}

// revealLegacy uses the original XOR decryption method
func (s *SecretValue) revealLegacy() string {
	if s == nil || s.isEmpty || s.encryptedValue == nil || s.key == nil {
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

	// If both use provider system, use provider comparison for better security
	if s.handle != nil && s.provider != nil && other.handle != nil && other.provider != nil {
		// Get other's value for comparison
		otherValue, err := other.provider.Retrieve(other.handle)
		if err != nil {
			// Fallback to revealing both values
			return s.equalsLegacy(other)
		}

		result, err := s.provider.Compare(s.handle, otherValue)
		if err != nil {
			// Fallback to revealing both values
			return s.equalsLegacy(other)
		}

		// Zero out the retrieved value
		zeroString(&otherValue)
		return result
	}

	// Use legacy comparison
	return s.equalsLegacy(other)
}

// equalsLegacy performs the original comparison method
func (s *SecretValue) equalsLegacy(other *SecretValue) bool {
	// For non-empty secrets, compare the revealed values
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

	// Use provider comparison if available
	if s.handle != nil && s.provider != nil {
		result, err := s.provider.Compare(s.handle, value)
		if err != nil {
			// Fallback to revealing and comparing
			revealed := s.Reveal()
			result := constantTimeEquals(revealed, value)
			zeroString(&revealed)
			return result
		}
		return result
	}

	// Use legacy comparison
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
		cloned := &SecretValue{
			secretType: s.secretType,
			isEmpty:    true,
			created:    time.Now(),
		}
		// If original has a provider, use the same provider for empty clone
		if s.provider != nil {
			cloned.provider = s.provider
		}
		return cloned
	}

	// Use provider clone if available
	if s.handle != nil && s.provider != nil {
		newHandle, err := s.provider.Clone(s.handle)
		if err != nil {
			// Fallback to revealing and re-creating
			return s.cloneLegacy()
		}

		return &SecretValue{
			handle:     newHandle,
			provider:   s.provider,
			secretType: s.secretType,
			isEmpty:    false,
			created:    time.Now(),
		}
	}

	// Use legacy clone
	return s.cloneLegacy()
}

// cloneLegacy creates a copy using the original method
func (s *SecretValue) cloneLegacy() *SecretValue {
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

	// Use provider destroy if available
	if s.handle != nil && s.provider != nil {
		s.provider.Destroy(s.handle)
		s.handle = nil
		s.provider = nil
	}

	// Also clean up legacy fields
	s.zeroMemory()
	s.isEmpty = true
}

// MaskableValue interface implementation for logmasker compatibility
// These methods allow SecretValue to be detected by logmasker without explicit coupling

// ShouldMask returns true indicating this value should be masked in logs
func (s *SecretValue) ShouldMask() bool {
	// Always mask secrets in logs
	return true
}

// GetMaskedValue returns a masked representation of this secret
func (s *SecretValue) GetMaskedValue() any {
	if s == nil {
		return "[REDACTED]"
	}

	if s.isEmpty {
		return "[EMPTY]"
	}

	// Return type-specific redaction
	switch s.secretType {
	case SecretTypePassword:
		return "[PASSWORD]"
	case SecretTypeToken:
		return "[TOKEN]"
	case SecretTypeKey:
		return "[KEY]"
	case SecretTypeCertificate:
		return "[CERTIFICATE]"
	default:
		return "[REDACTED]"
	}
}

// GetMaskStrategy returns the preferred masking strategy for this secret
func (s *SecretValue) GetMaskStrategy() string {
	// Always use redaction strategy for secrets
	return "redact"
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

// zeroString attempts to clear a string reference but CANNOT actually zero
// the underlying string memory due to Go's string immutability.
//
// SECURITY WARNING: This function provides NO memory security guarantees.
// The original string data remains in memory until garbage collected, and
// even then may persist in memory dumps or swap files.
//
// This function only:
//   - Sets the string reference to empty (for API cleanliness)
//   - Provides a consistent interface for memory clearing attempts
//
// For actual secure memory handling, use dedicated libraries like:
//   - github.com/awnumar/memguard
//   - github.com/secure-systems-lab/go-securesocketlayer
func zeroString(s *string) {
	if s == nil || len(*s) == 0 {
		return
	}

	// This only clears the reference, not the underlying memory
	// The original string data remains accessible until garbage collected
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
