package modular

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// testHandle is a simple SecretHandle implementation for tests
type testHandle struct{ id string }

func (h *testHandle) ID() string       { return h.id }
func (h *testHandle) Provider() string { return "test" }
func (h *testHandle) IsValid() bool    { return true }

// failingStoreProvider forces Store errors to trigger legacy fallback
type failingStoreProvider struct{}

func (p *failingStoreProvider) Name() string   { return "failing-store" }
func (p *failingStoreProvider) IsSecure() bool { return false }
func (p *failingStoreProvider) Store(value string, secretType SecretType) (SecretHandle, error) {
	return nil, assert.AnError
}
func (p *failingStoreProvider) Retrieve(handle SecretHandle) (string, error) {
	return "", assert.AnError
}
func (p *failingStoreProvider) Destroy(handle SecretHandle) error { return nil }
func (p *failingStoreProvider) Compare(handle SecretHandle, value string) (bool, error) {
	return false, assert.AnError
}
func (p *failingStoreProvider) IsEmpty(handle SecretHandle) bool { return false }
func (p *failingStoreProvider) Clone(handle SecretHandle) (SecretHandle, error) {
	return nil, assert.AnError
}
func (p *failingStoreProvider) GetMetadata(handle SecretHandle) (SecretMetadata, error) {
	return SecretMetadata{}, nil
}
func (p *failingStoreProvider) Cleanup() error { return nil }

// errorRetrieveProvider returns a valid handle but retrieval fails (forces Equals fallback)
type errorRetrieveProvider struct{ stored string }

func (p *errorRetrieveProvider) Name() string   { return "error-retrieve" }
func (p *errorRetrieveProvider) IsSecure() bool { return false }
func (p *errorRetrieveProvider) Store(value string, secretType SecretType) (SecretHandle, error) {
	p.stored = value
	return &testHandle{id: "1"}, nil
}
func (p *errorRetrieveProvider) Retrieve(handle SecretHandle) (string, error) {
	return "", assert.AnError
}
func (p *errorRetrieveProvider) Destroy(handle SecretHandle) error { return nil }
func (p *errorRetrieveProvider) Compare(handle SecretHandle, value string) (bool, error) {
	return false, assert.AnError
}
func (p *errorRetrieveProvider) IsEmpty(handle SecretHandle) bool { return p.stored == "" }
func (p *errorRetrieveProvider) Clone(handle SecretHandle) (SecretHandle, error) {
	return &testHandle{id: "2"}, nil
}
func (p *errorRetrieveProvider) GetMetadata(handle SecretHandle) (SecretMetadata, error) {
	return SecretMetadata{Type: SecretTypeGeneric, Created: time.Now()}, nil
}
func (p *errorRetrieveProvider) Cleanup() error { return nil }

// compareErrorProvider causes Compare to error so EqualsString fallback executes
type compareErrorProvider struct{ stored string }

func (p *compareErrorProvider) Name() string   { return "compare-error" }
func (p *compareErrorProvider) IsSecure() bool { return false }
func (p *compareErrorProvider) Store(value string, secretType SecretType) (SecretHandle, error) {
	p.stored = value
	return &testHandle{id: "c"}, nil
}
func (p *compareErrorProvider) Retrieve(handle SecretHandle) (string, error) { return p.stored, nil }
func (p *compareErrorProvider) Destroy(handle SecretHandle) error            { return nil }
func (p *compareErrorProvider) Compare(handle SecretHandle, value string) (bool, error) {
	return false, assert.AnError
}
func (p *compareErrorProvider) IsEmpty(handle SecretHandle) bool { return p.stored == "" }
func (p *compareErrorProvider) Clone(handle SecretHandle) (SecretHandle, error) {
	return &testHandle{id: "c2"}, nil
}
func (p *compareErrorProvider) GetMetadata(handle SecretHandle) (SecretMetadata, error) {
	return SecretMetadata{Type: SecretTypeGeneric, Created: time.Now()}, nil
}
func (p *compareErrorProvider) Cleanup() error { return nil }

// destroyErrorProvider triggers error in Destroy path
type destroyErrorProvider struct{ stored string }

func (p *destroyErrorProvider) Name() string   { return "destroy-error" }
func (p *destroyErrorProvider) IsSecure() bool { return false }
func (p *destroyErrorProvider) Store(value string, secretType SecretType) (SecretHandle, error) {
	p.stored = value
	return &testHandle{id: "d"}, nil
}
func (p *destroyErrorProvider) Retrieve(handle SecretHandle) (string, error) { return p.stored, nil }
func (p *destroyErrorProvider) Destroy(handle SecretHandle) error            { return assert.AnError }
func (p *destroyErrorProvider) Compare(handle SecretHandle, value string) (bool, error) {
	return value == p.stored, nil
}
func (p *destroyErrorProvider) IsEmpty(handle SecretHandle) bool { return p.stored == "" }
func (p *destroyErrorProvider) Clone(handle SecretHandle) (SecretHandle, error) {
	return &testHandle{id: "d2"}, nil
}
func (p *destroyErrorProvider) GetMetadata(handle SecretHandle) (SecretMetadata, error) {
	return SecretMetadata{Type: SecretTypeGeneric, Created: time.Now()}, nil
}
func (p *destroyErrorProvider) Cleanup() error { return nil }

func TestSecretValue_LegacyFallbackAndEqualsFallback(t *testing.T) {
	// Force legacy path via provider Store error
	failingProv := &failingStoreProvider{}
	s := NewSecretValueWithProvider("legacy-secret", SecretTypeGeneric, failingProv)
	assert.NotNil(t, s)
	assert.Nil(t, s.handle) // legacy path
	assert.NotNil(t, s.encryptedValue)
	assert.Equal(t, "legacy-secret", s.Reveal()) // exercises revealLegacy

	// Equals fallback when other provider Retrieve errors (expected false because other cannot reveal value)
	good := NewGenericSecret("legacy-secret") // provider-backed default
	errProv := &errorRetrieveProvider{}
	other := NewSecretValueWithProvider("legacy-secret", SecretTypeGeneric, errProv)
	// Retrieval error forces legacy comparison; other cannot reveal -> inequality
	assert.False(t, good.Equals(other))

	// EqualsString fallback when Compare errors
	cmpErrProv := &compareErrorProvider{}
	cmpSecret := NewSecretValueWithProvider("abc", SecretTypeGeneric, cmpErrProv)
	assert.True(t, cmpSecret.EqualsString("abc"))
}

func TestSecretValue_TextMarshalAndUnmarshal(t *testing.T) {
	s := NewPasswordSecret("text-secret")
	data, err := s.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, []byte("[REDACTED]"), data)

	var u SecretValue
	// Unmarshal redacted becomes empty
	assert.NoError(t, u.UnmarshalText([]byte("[REDACTED]")))
	assert.True(t, u.IsEmpty())

	// Unmarshal real value
	assert.NoError(t, u.UnmarshalText([]byte("real-text")))
	assert.Equal(t, "real-text", u.Reveal())
}

func TestSecretValue_JSONCreatedAndMasking(t *testing.T) {
	// nil secret Created() returns zero
	var nilSecret *SecretValue
	assert.True(t, nilSecret.Created().IsZero())

	// MarshalJSON already covered; add Unmarshal redacted + empty
	var s SecretValue
	assert.NoError(t, json.Unmarshal([]byte("\"[EMPTY]\""), &s))
	assert.True(t, s.IsEmpty())

	// Masking helpers
	pw := NewPasswordSecret("pw")
	tk := NewTokenSecret("tk")
	key := NewKeySecret("k1")
	cert := NewCertificateSecret("c1")
	gen := NewGenericSecret("g1")
	empty := NewGenericSecret("")

	cases := []struct {
		sv     *SecretValue
		expect any
	}{
		{pw, "[PASSWORD]"},
		{tk, "[TOKEN]"},
		{key, "[KEY]"},
		{cert, "[CERTIFICATE]"},
		{gen, "[REDACTED]"},
		{empty, "[EMPTY]"},
		{nil, "[REDACTED]"},
	}
	for _, c := range cases {
		assert.True(t, c.sv.ShouldMask())
		assert.Equal(t, "redact", c.sv.GetMaskStrategy())
		assert.Equal(t, c.expect, c.sv.GetMaskedValue())
	}
}

func TestSecretRedactor_StructuredValueCopyAndEmptyPattern(t *testing.T) {
	r := NewSecretRedactor()
	// Add empty pattern should be ignored
	r.AddPattern("")

	secret := NewGenericSecret("val123")
	r.AddSecret(secret)

	// Put a value copy (non-pointer) in structured log
	fields := map[string]interface{}{
		"secretVal": *secret,
		"other":     "val123", // will be redacted via AddSecret
	}
	red := r.RedactStructuredLog(fields)
	assert.Equal(t, "[REDACTED]", red["secretVal"])
	assert.Equal(t, "[REDACTED]", red["other"]) // value inside string replaced
}

func TestSecretValue_DestroyWithProviderError(t *testing.T) {
	prov := &destroyErrorProvider{}
	s := NewSecretValueWithProvider("to-destroy", SecretTypeGeneric, prov)
	assert.False(t, s.IsEmpty())
	// Destroy should not panic even if provider Destroy errors
	s.Destroy()
	assert.True(t, s.IsEmpty())
	assert.Equal(t, "", s.Reveal())
}
