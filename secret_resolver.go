package modular

import (
	"context"
	"fmt"
	"regexp"
)

// SecretResolver resolves secret references in configuration values.
type SecretResolver interface {
	ResolveSecret(ctx context.Context, ref string) (string, error)
	CanResolve(ref string) bool
}

var secretRefPattern = regexp.MustCompile(`^\$\{([^:}]+:[^}]+)\}$`)

// ExpandSecrets walks a config map and replaces string values matching
// ${prefix:path} with the resolved secret value. Recurses into nested maps.
func ExpandSecrets(ctx context.Context, config map[string]any, resolvers ...SecretResolver) error {
	for key, val := range config {
		switch v := val.(type) {
		case string:
			resolved, err := resolveSecretString(ctx, v, resolvers)
			if err != nil {
				return fmt.Errorf("resolving %q: %w", key, err)
			}
			config[key] = resolved
		case map[string]any:
			if err := ExpandSecrets(ctx, v, resolvers...); err != nil {
				return err
			}
		case []any:
			if err := expandSecretsSlice(ctx, v, resolvers); err != nil {
				return fmt.Errorf("resolving %q: %w", key, err)
			}
		}
	}
	return nil
}

func expandSecretsSlice(ctx context.Context, slice []any, resolvers []SecretResolver) error {
	for i, elem := range slice {
		switch v := elem.(type) {
		case string:
			resolved, err := resolveSecretString(ctx, v, resolvers)
			if err != nil {
				return err
			}
			slice[i] = resolved
		case map[string]any:
			if err := ExpandSecrets(ctx, v, resolvers...); err != nil {
				return err
			}
		case []any:
			if err := expandSecretsSlice(ctx, v, resolvers); err != nil {
				return err
			}
		}
	}
	return nil
}

func resolveSecretString(ctx context.Context, val string, resolvers []SecretResolver) (string, error) {
	match := secretRefPattern.FindStringSubmatch(val)
	if match == nil {
		return val, nil
	}
	ref := match[1]
	for _, r := range resolvers {
		if r.CanResolve(ref) {
			return r.ResolveSecret(ctx, ref)
		}
	}
	return val, nil
}
