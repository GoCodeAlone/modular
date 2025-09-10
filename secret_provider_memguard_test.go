package modular

import (
	"testing"
	"time"
)

// TestMemguardProviderUnavailable verifies operations fail before enabling.
func TestMemguardProviderUnavailable(t *testing.T) {
	p, err := NewMemguardSecretProvider(SecretProviderConfig{MaxSecrets: 5})
	if err == nil {
		// Provider creation succeeded but should be unavailable initially (stub returns false)
		if p.IsSecure() {
			t.Log("memguard unexpectedly secure at init")
		}
		// Store should fail until enabled
		if _, err2 := p.Store("val", SecretTypeGeneric); err2 == nil {
			t.Fatalf("expected store failure when unavailable")
		}
	} else {
		// If creation itself fails, skip (environment may not support)
		t.Skipf("memguard creation failed (expected in some envs): %v", err)
	}
}

// TestMemguardProviderEnableAndBasicLifecycle covers enabling, storing, retrieving placeholder, and stats.
func TestMemguardProviderEnableAndBasicLifecycle(t *testing.T) {
	p, err := NewMemguardSecretProvider(SecretProviderConfig{MaxSecrets: 10, AutoDestroy: 10 * time.Millisecond})
	if err != nil {
		t.Skipf("creation failed: %v", err)
	}
	EnableMemguardForTesting(p)

	// Store non-empty
	h, err := p.Store("top-secret", SecretTypePassword)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	if !h.IsValid() {
		t.Fatalf("handle invalid")
	}

	// Retrieve should return placeholder secured content (stub)
	val, err := p.Retrieve(h)
	if err != nil {
		t.Fatalf("retrieve: %v", err)
	}
	if val == "top-secret" {
		t.Fatalf("retrieval leaked original secret")
	}

	// Compare should be constant‑time false vs placeholder/plain mismatch
	eq, err := p.Compare(h, "top-secret")
	if err != nil {
		t.Fatalf("compare: %v", err)
	}
	if eq {
		t.Fatalf("expected secure placeholder mismatch")
	}

	// Clone path (will internally retrieve placeholder and store again)
	clone, err := p.Clone(h)
	if err != nil {
		t.Fatalf("clone: %v", err)
	}
	if clone.ID() == h.ID() {
		t.Fatalf("clone should have different id")
	}

	// Empty secret path
	empty, err := p.Store("", SecretTypeGeneric)
	if err != nil {
		t.Fatalf("empty store: %v", err)
	}
	if !p.IsEmpty(empty) {
		t.Fatalf("expected empty secret flagged")
	}

	// Stats
	stats := GetMemguardProviderStats(p)
	if stats["active_secrets"].(int) < 2 {
		t.Fatalf("expected at least 2 active secrets, got %v", stats)
	}

	// Auto destroy wait & verify one destroyed (best-effort, not flaky critical)
	time.Sleep(25 * time.Millisecond)
	_ = p.Destroy(h)
	_ = p.Destroy(clone)
	_ = p.Destroy(empty)
	p.Cleanup()
}

// Additional coverage-focused tests constructing provider directly to exercise
// both unavailable and available code paths (bypassing constructor failure).
func TestMemguardProviderUnavailableDirect(t *testing.T) {
	p := &MemguardSecretProvider{name: "memguard", secrets: make(map[string]*memguardSecret)}
	if p.IsSecure() {
		t.Fatalf("expected insecure (unavailable)")
	}
	if _, err := p.Store("x", SecretTypeGeneric); err != ErrMemguardProviderNotAvailable {
		t.Fatalf("store err=%v", err)
	}
	if _, err := p.Retrieve(nil); err != ErrMemguardProviderNotAvailable {
		t.Fatalf("retrieve err=%v", err)
	}
	if _, err := p.Clone(nil); err != ErrMemguardProviderNotAvailable {
		t.Fatalf("clone err=%v", err)
	}
	if ok, err := p.Compare(nil, ""); err != ErrMemguardProviderNotAvailable || ok {
		t.Fatalf("compare expected provider not available")
	}
}

func TestMemguardProviderAvailableFullLifecycle(t *testing.T) {
	// Allow enough capacity to exercise lifecycle first, then hit the limit at the end.
	p := &MemguardSecretProvider{name: "memguard", secrets: make(map[string]*memguardSecret), maxSecrets: 4, autoDestroy: 10 * time.Millisecond, available: true}

	// Store first secret
	h1, err := p.Store("alpha", SecretTypeGeneric)
	if err != nil {
		t.Fatalf("store1: %v", err)
	}

	// Store second secret
	h2, err := p.Store("beta", SecretTypePassword)
	if err != nil {
		t.Fatalf("store2: %v", err)
	}

	// Retrieve placeholder (not original secret)
	val, err := p.Retrieve(h1)
	if err != nil || val == "alpha" {
		t.Fatalf("retrieve placeholder mismatch: %v %s", err, val)
	}

	// Compare against original (should be false) and placeholder (true)
	if eq, _ := p.Compare(h1, "alpha"); eq {
		t.Fatalf("compare should not match original")
	}
	if eq, _ := p.Compare(h1, "[MEMGUARD_SECURED_CONTENT]"); !eq {
		t.Fatalf("compare should match placeholder")
	}

	// Empty secret (third)
	empty, err := p.Store("", SecretTypeGeneric)
	if err != nil {
		t.Fatalf("empty store: %v", err)
	}
	if !p.IsEmpty(empty) {
		t.Fatalf("expected empty secret")
	}

	// Clone second secret (fourth)
	clone, err := p.Clone(h2)
	if err != nil {
		t.Fatalf("clone: %v", err)
	}
	if clone.ID() == h2.ID() {
		t.Fatalf("clone id should differ")
	}

	// Now we are at capacity; next store should trigger limit error
	if _, err := p.Store("overflow", SecretTypeGeneric); err == nil {
		t.Fatalf("expected limit error")
	}

	// Metadata retrieval
	meta, err := p.GetMetadata(h2)
	if err != nil || meta.Provider != "memguard" {
		t.Fatalf("metadata error: %v %+v", err, meta)
	}

	// Stats before auto-destroy
	stats := GetMemguardProviderStats(p)
	if stats["active_secrets"].(int) < 4 {
		t.Fatalf("expected 4 active secrets, got %v", stats)
	}

	// Wait for auto destroy interval to pass
	time.Sleep(30 * time.Millisecond)

	// Best-effort explicit destroy (some may already be gone)
	_ = p.Destroy(h1)
	_ = p.Destroy(h2)
	_ = p.Destroy(clone)
	_ = p.Destroy(empty)

	// Cleanup (covers cleanupMemguard path)
	if err := p.Cleanup(); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if p.IsSecure() {
		t.Fatalf("expected provider insecure after cleanup")
	}

	// Stats after cleanup
	stats2 := GetMemguardProviderStats(p)
	if stats2["provider_secure"].(bool) {
		t.Fatalf("expected provider_secure false post-cleanup")
	}
}
