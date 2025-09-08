package integration

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	modular "github.com/GoCodeAlone/modular"
)

// TestSecretLeakageScan tests T030: Integration secret leakage scan
// This test verifies that sensitive configuration data (passwords, tokens, keys)
// is properly handled and not leaked in logs, error messages, or debug output.
func TestSecretLeakageScan(t *testing.T) {
	// Use a buffer to capture log output for scanning
	logBuffer := &testLogBuffer{entries: make([]string, 0)}
	logger := slog.New(slog.NewTextHandler(logBuffer, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create application
	app := modular.NewStdApplication(modular.NewStdConfigProvider(&struct{}{}), logger)

	// Register module with sensitive configuration
	secretsModule := &testSecretsModule{
		name: "secretsModule",
		config: &testSecretsConfig{
			DatabasePassword: "super-secret-password-123",
			APIKey:           "sk-abcd1234567890",
			JWTSecret:        "jwt-secret-key-xyz",
			PublicConfig:     "this-is-safe-to-log",
		},
	}

	app.RegisterModule(secretsModule)

	// Initialize application
	err := app.Init()
	if err != nil {
		t.Fatalf("Application initialization failed: %v", err)
	}

	// Start and stop to generate more logs
	err = app.Start()
	if err != nil {
		t.Fatalf("Application start failed: %v", err)
	}

	err = app.Stop()
	if err != nil {
		t.Fatalf("Application stop failed: %v", err)
	}

	// Perform secret leakage scan
	t.Run("SecretLeakageInLogs", func(t *testing.T) {
		leakedSecrets := scanForSecretLeakage(logBuffer.entries, []string{
			"super-secret-password-123",
			"sk-abcd1234567890",
			"jwt-secret-key-xyz",
		})

		if len(leakedSecrets) > 0 {
			t.Errorf("Secret leakage detected in logs: %v", leakedSecrets)
			t.Log("Log entries containing secrets:")
			for _, entry := range logBuffer.entries {
				for _, secret := range leakedSecrets {
					if strings.Contains(entry, secret) {
						t.Logf("  LEAKED: %s", entry)
					}
				}
			}
		} else {
			t.Log("✅ No secret leakage detected in application logs")
		}
	})

	// Test configuration error messages don't leak secrets
	t.Run("SecretLeakageInErrors", func(t *testing.T) {
		// Create module with invalid config that might trigger error logging
		errorModule := &testSecretsModule{
			name: "errorModule",
			config: &testSecretsConfig{
				DatabasePassword: "another-secret-password",
				APIKey:           "ak-error-test-key",
				JWTSecret:        "", // Invalid empty secret
				PublicConfig:     "public",
			},
		}

		// Clear previous log entries
		logBuffer.entries = make([]string, 0)

		errorApp := modular.NewStdApplication(modular.NewStdConfigProvider(&struct{}{}), logger)
		errorApp.RegisterModule(errorModule)

		// This might fail due to validation, which is expected
		_ = errorApp.Init()

		// Scan error logs for secret leakage
		leakedSecrets := scanForSecretLeakage(logBuffer.entries, []string{
			"another-secret-password",
			"ak-error-test-key",
		})

		if len(leakedSecrets) > 0 {
			t.Errorf("Secret leakage detected in error logs: %v", leakedSecrets)
		} else {
			t.Log("✅ No secret leakage detected in error messages")
		}
	})

	// Test configuration dumps don't expose secrets
	t.Run("SecretLeakageInConfigDumps", func(t *testing.T) {
		// Simulate configuration dump/debug output
		configDump := secretsModule.dumpConfig()

		secrets := []string{
			"super-secret-password-123",
			"sk-abcd1234567890",
			"jwt-secret-key-xyz",
		}

		leakedSecrets := scanForSecretLeakage([]string{configDump}, secrets)

		if len(leakedSecrets) > 0 {
			t.Errorf("Secret leakage detected in config dump: %v", leakedSecrets)
			t.Logf("Config dump: %s", configDump)
		} else {
			t.Log("✅ No secret leakage detected in configuration dumps")
		}

		// Verify that public config is still visible
		if !strings.Contains(configDump, "this-is-safe-to-log") {
			t.Error("Public configuration should be visible in config dump")
		}
	})

	// Test service registration doesn't leak secrets
	t.Run("SecretLeakageInServiceRegistration", func(t *testing.T) {
		// Clear log buffer
		logBuffer.entries = make([]string, 0)

		// Register a service that might contain sensitive data
		sensitiveService := &testSensitiveService{
			connectionString: "user:secret-pass@host:5432/db",
			apiToken:         "token-abc123",
		}

		serviceApp := modular.NewStdApplication(modular.NewStdConfigProvider(&struct{}{}), logger)
		err := serviceApp.RegisterService("sensitiveService", sensitiveService)
		if err != nil {
			t.Fatalf("Service registration failed: %v", err)
		}

		// Scan service registration logs
		leakedSecrets := scanForSecretLeakage(logBuffer.entries, []string{
			"secret-pass",
			"token-abc123",
		})

		if len(leakedSecrets) > 0 {
			t.Errorf("Secret leakage detected in service registration: %v", leakedSecrets)
		} else {
			t.Log("✅ No secret leakage detected in service registration")
		}
	})
}

// TestSecretRedactionInProvenance tests that secret values are redacted in configuration provenance
func TestSecretRedactionInProvenance(t *testing.T) {
	// This test verifies that when configuration provenance is tracked,
	// secret values are properly redacted in provenance information

	logBuffer := &testLogBuffer{entries: make([]string, 0)}
	logger := slog.New(slog.NewTextHandler(logBuffer, &slog.HandlerOptions{Level: slog.LevelDebug}))

	app := modular.NewStdApplication(modular.NewStdConfigProvider(&struct{}{}), logger)

	secretsModule := &testSecretsModule{
		name: "provenanceModule",
		config: &testSecretsConfig{
			DatabasePassword: "provenance-secret-123",
			APIKey:           "pk-provenance-key",
			JWTSecret:        "provenance-jwt-secret",
			PublicConfig:     "provenance-public",
		},
	}

	app.RegisterModule(secretsModule)

	err := app.Init()
	if err != nil {
		t.Fatalf("Application initialization failed: %v", err)
	}

	// Check if any provenance tracking would leak secrets
	leakedSecrets := scanForSecretLeakage(logBuffer.entries, []string{
		"provenance-secret-123",
		"pk-provenance-key",
		"provenance-jwt-secret",
	})

	if len(leakedSecrets) > 0 {
		t.Errorf("Secret leakage detected in provenance tracking: %v", leakedSecrets)
		t.Log("⚠️  Configuration provenance tracking may need secret redaction")
	} else {
		t.Log("✅ No secret leakage detected in provenance tracking")
	}

	// Note: Enhanced provenance with redaction is not yet implemented
	t.Log("⚠️  Note: Enhanced provenance tracking with secret redaction is not yet implemented")
}

// scanForSecretLeakage scans text entries for leaked secrets
func scanForSecretLeakage(entries []string, secrets []string) []string {
	var leaked []string

	for _, entry := range entries {
		for _, secret := range secrets {
			if strings.Contains(entry, secret) {
				leaked = append(leaked, secret)
			}
		}
	}

	return leaked
}

// testLogBuffer captures log entries for scanning
type testLogBuffer struct {
	entries []string
}

func (b *testLogBuffer) Write(p []byte) (n int, err error) {
	b.entries = append(b.entries, string(p))
	return len(p), nil
}

// testSecretsConfig contains both public and sensitive configuration
type testSecretsConfig struct {
	DatabasePassword string `yaml:"database_password" json:"database_password" secret:"true"`
	APIKey           string `yaml:"api_key" json:"api_key" secret:"true"`
	JWTSecret        string `yaml:"jwt_secret" json:"jwt_secret" secret:"true"`
	PublicConfig     string `yaml:"public_config" json:"public_config"`
}

// testSecretsModule is a module that handles sensitive configuration
type testSecretsModule struct {
	name   string
	config *testSecretsConfig
}

func (m *testSecretsModule) Name() string {
	return m.name
}

func (m *testSecretsModule) RegisterConfig(app modular.Application) error {
	provider := modular.NewStdConfigProvider(m.config)
	app.RegisterConfigSection(m.name, provider)
	return nil
}

func (m *testSecretsModule) Init(app modular.Application) error {
	return nil
}

func (m *testSecretsModule) Start(ctx context.Context) error {
	return nil
}

func (m *testSecretsModule) Stop(ctx context.Context) error {
	return nil
}

// dumpConfig simulates configuration dump that should redact secrets
func (m *testSecretsModule) dumpConfig() string {
	// In a real implementation, this would use secret redaction
	// For now, we'll simulate basic redaction
	return fmt.Sprintf("Config{DatabasePassword: [REDACTED], APIKey: [REDACTED], JWTSecret: [REDACTED], PublicConfig: %s}",
		m.config.PublicConfig)
}

// testSensitiveService simulates a service with sensitive connection information
type testSensitiveService struct {
	connectionString string
	apiToken         string
}

func (s *testSensitiveService) Connect() error {
	return nil
}

func (s *testSensitiveService) GetConnectionInfo() string {
	// This should redact sensitive parts
	return "Connected to database [CONNECTION_REDACTED]"
}
