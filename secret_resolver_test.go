package modular

import (
	"context"
	"strings"
	"testing"
)

type mockSecretResolver struct {
	prefix string
	values map[string]string
}

func (r *mockSecretResolver) CanResolve(ref string) bool {
	return strings.HasPrefix(ref, r.prefix+":")
}

func (r *mockSecretResolver) ResolveSecret(ctx context.Context, ref string) (string, error) {
	key := strings.TrimPrefix(ref, r.prefix+":")
	if v, ok := r.values[key]; ok {
		return v, nil
	}
	return "", ErrServiceNotFound
}

func TestExpandSecrets_ResolvesRefs(t *testing.T) {
	resolver := &mockSecretResolver{
		prefix: "vault",
		values: map[string]string{"secret/db-pass": "s3cret"},
	}
	config := map[string]any{
		"host":     "localhost",
		"password": "${vault:secret/db-pass}",
		"nested":   map[string]any{"key": "${vault:secret/db-pass}"},
	}
	if err := ExpandSecrets(context.Background(), config, resolver); err != nil {
		t.Fatalf("ExpandSecrets: %v", err)
	}
	if config["password"] != "s3cret" {
		t.Errorf("expected s3cret, got %v", config["password"])
	}
	nested := config["nested"].(map[string]any)
	if nested["key"] != "s3cret" {
		t.Errorf("expected nested s3cret, got %v", nested["key"])
	}
}

func TestExpandSecrets_SkipsNonRefs(t *testing.T) {
	config := map[string]any{"host": "localhost", "port": 5432}
	if err := ExpandSecrets(context.Background(), config); err != nil {
		t.Fatalf("ExpandSecrets: %v", err)
	}
	if config["host"] != "localhost" {
		t.Errorf("expected localhost, got %v", config["host"])
	}
}

func TestExpandSecrets_NoMatchingResolver(t *testing.T) {
	config := map[string]any{"password": "${aws:secret/key}"}
	resolver := &mockSecretResolver{prefix: "vault", values: map[string]string{}}
	if err := ExpandSecrets(context.Background(), config, resolver); err != nil {
		t.Fatalf("ExpandSecrets: %v", err)
	}
	if config["password"] != "${aws:secret/key}" {
		t.Errorf("expected unchanged ref, got %v", config["password"])
	}
}
