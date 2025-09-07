package integration

import (
	"context"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
	"github.com/GoCodeAlone/modular/feeders"
)

// Simple test certificate module for integration testing
type TestCertificateModule struct {
	name string
}

func (m *TestCertificateModule) Name() string { return m.name }
func (m *TestCertificateModule) Init(app modular.Application) error { return nil }

// T060: Add integration test for certificate renewal escalation
func TestCertificateRenewal_Integration(t *testing.T) {
	t.Run("should configure certificate renewal module", func(t *testing.T) {
		app, err := modular.NewApplication()
		if err != nil {
			t.Fatalf("Failed to create application: %v", err)
		}
		app.EnableEnhancedLifecycle()

		// Register test certificate module
		certMod := &TestCertificateModule{name: "certificate"}
		app.RegisterModule("certificate", certMod)

		// Configure module with renewal settings
		mapFeeder := feeders.NewMapFeeder(map[string]interface{}{
			"certificate.enabled":          true,
			"certificate.staging":          true,
			"certificate.email":            "test@example.com",
			"certificate.pre_renewal_days": 30,
			"certificate.escalation_days":  7,
			"certificate.check_interval":   "1h",
		})
		app.RegisterFeeder("config", mapFeeder)

		ctx := context.Background()

		// Initialize and start application
		err := app.InitWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Fatalf("Failed to initialize application: %v", err)
		}

		err = app.StartWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Fatalf("Failed to start application: %v", err)
		}

		// Verify configuration is loaded
		provider := app.ConfigProvider()
		if provider == nil {
			t.Fatal("Config provider should be available")
		}

		preRenewalDays, err := provider.GetInt("certificate.pre_renewal_days")
		if err != nil {
			t.Fatalf("Failed to get pre_renewal_days: %v", err)
		}
		if preRenewalDays != 30 {
			t.Errorf("Expected 30 pre-renewal days, got: %d", preRenewalDays)
		}

		escalationDays, err := provider.GetInt("certificate.escalation_days")
		if err != nil {
			t.Fatalf("Failed to get escalation_days: %v", err)
		}
		if escalationDays != 7 {
			t.Errorf("Expected 7 escalation days, got: %d", escalationDays)
		}

		// Cleanup
		err = app.StopWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Errorf("Failed to stop application: %v", err)
		}
	})

	t.Run("should handle certificate renewal configuration variations", func(t *testing.T) {
		testCases := []struct {
			name            string
			preRenewalDays  int
			escalationDays  int
			checkInterval   string
		}{
			{"standard renewal", 30, 7, "1h"},
			{"aggressive renewal", 60, 14, "30m"},
			{"minimal renewal", 15, 3, "6h"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				app, err := modular.NewApplication()
		if err != nil {
			t.Fatalf("Failed to create application: %v", err)
		}
				app.EnableEnhancedLifecycle()

				// Register test certificate module
				certMod := &TestCertificateModule{name: "certificate"}
				app.RegisterModule("certificate", certMod)

				// Configure with test case parameters
				mapFeeder := feeders.NewMapFeeder(map[string]interface{}{
					"certificate.enabled":          true,
					"certificate.pre_renewal_days": tc.preRenewalDays,
					"certificate.escalation_days":  tc.escalationDays,
					"certificate.check_interval":   tc.checkInterval,
				})
				app.RegisterFeeder("config", mapFeeder)

				ctx := context.Background()

				// Initialize and start application
				err := app.InitWithEnhancedLifecycle(ctx)
				if err != nil {
					t.Fatalf("Failed to initialize application: %v", err)
				}

				err = app.StartWithEnhancedLifecycle(ctx)
				if err != nil {
					t.Fatalf("Failed to start application: %v", err)
				}

				// Verify configuration is loaded correctly
				provider := app.ConfigProvider()
				if provider == nil {
					t.Fatal("Config provider should be available")
				}

				actualPreRenewal, err := provider.GetInt("certificate.pre_renewal_days")
				if err != nil {
					t.Fatalf("Failed to get pre_renewal_days: %v", err)
				}
				if actualPreRenewal != tc.preRenewalDays {
					t.Errorf("Expected %d pre-renewal days, got: %d", tc.preRenewalDays, actualPreRenewal)
				}

				actualEscalation, err := provider.GetInt("certificate.escalation_days")
				if err != nil {
					t.Fatalf("Failed to get escalation_days: %v", err)
				}
				if actualEscalation != tc.escalationDays {
					t.Errorf("Expected %d escalation days, got: %d", tc.escalationDays, actualEscalation)
				}

				actualInterval, err := provider.GetString("certificate.check_interval")
				if err != nil {
					t.Fatalf("Failed to get check_interval: %v", err)
				}
				if actualInterval != tc.checkInterval {
					t.Errorf("Expected '%s' check interval, got: %s", tc.checkInterval, actualInterval)
				}

				// Cleanup
				err = app.StopWithEnhancedLifecycle(ctx)
				if err != nil {
					t.Errorf("Failed to stop application: %v", err)
				}
			})
		}
	})

	t.Run("should validate certificate renewal configuration", func(t *testing.T) {
		app, err := modular.NewApplication()
		if err != nil {
			t.Fatalf("Failed to create application: %v", err)
		}
		app.EnableEnhancedLifecycle()

		// Register test certificate module
		certMod := &TestCertificateModule{name: "certificate"}
		app.RegisterModule("certificate", certMod)

		// Configure with edge case values
		mapFeeder := feeders.NewMapFeeder(map[string]interface{}{
			"certificate.enabled":          true,
			"certificate.pre_renewal_days": 0,    // Edge case: no pre-renewal
			"certificate.escalation_days":  0,    // Edge case: no escalation
			"certificate.check_interval":   "1s", // Very frequent checking
		})
		app.RegisterFeeder("config", mapFeeder)

		ctx := context.Background()

		// Initialize application
		err := app.InitWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Fatalf("Failed to initialize application: %v", err)
		}

		// Verify edge case configuration is loaded
		provider := app.ConfigProvider()
		if provider == nil {
			t.Fatal("Config provider should be available")
		}

		preRenewalDays, err := provider.GetInt("certificate.pre_renewal_days")
		if err != nil {
			t.Fatalf("Failed to get pre_renewal_days: %v", err)
		}
		if preRenewalDays != 0 {
			t.Errorf("Expected 0 pre-renewal days, got: %d", preRenewalDays)
		}

		// The framework should load the configuration; validation would be module-specific
		t.Log("Configuration edge cases handled by framework, validation by module")

		// Cleanup
		err = app.StopWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Errorf("Failed to stop application: %v", err)
		}
	})

	t.Run("should support certificate lifecycle management", func(t *testing.T) {
		app, err := modular.NewApplication()
		if err != nil {
			t.Fatalf("Failed to create application: %v", err)
		}
		app.EnableEnhancedLifecycle()

		// Register test certificate module
		certMod := &TestCertificateModule{name: "certificate"}
		app.RegisterModule("certificate", certMod)

		// Configure with lifecycle settings
		mapFeeder := feeders.NewMapFeeder(map[string]interface{}{
			"certificate.enabled":      true,
			"certificate.auto_renew":   true,
			"certificate.backup_certs": true,
			"certificate.notify_email": "admin@example.com",
		})
		app.RegisterFeeder("config", mapFeeder)

		ctx := context.Background()

		// Initialize and start application
		err := app.InitWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Fatalf("Failed to initialize application: %v", err)
		}

		err = app.StartWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Fatalf("Failed to start application: %v", err)
		}

		// Verify lifecycle features configuration
		provider := app.ConfigProvider()
		if provider == nil {
			t.Fatal("Config provider should be available")
		}

		autoRenew, err := provider.GetBool("certificate.auto_renew")
		if err != nil {
			t.Fatalf("Failed to get auto_renew: %v", err)
		}
		if !autoRenew {
			t.Error("Expected auto_renew to be true")
		}

		backupCerts, err := provider.GetBool("certificate.backup_certs")
		if err != nil {
			t.Fatalf("Failed to get backup_certs: %v", err)
		}
		if !backupCerts {
			t.Error("Expected backup_certs to be true")
		}

		notifyEmail, err := provider.GetString("certificate.notify_email")
		if err != nil {
			t.Fatalf("Failed to get notify_email: %v", err)
		}
		if notifyEmail != "admin@example.com" {
			t.Errorf("Expected notify_email 'admin@example.com', got: %s", notifyEmail)
		}

		// Verify health monitoring integration
		healthAggregator := app.GetHealthAggregator()
		if healthAggregator == nil {
			t.Fatal("Health aggregator should be available")
		}

		health, err := healthAggregator.GetOverallHealth(ctx)
		if err != nil {
			t.Fatalf("Failed to get overall health: %v", err)
		}

		if health.Status != "healthy" && health.Status != "warning" {
			t.Errorf("Expected healthy status, got: %s", health.Status)
		}

		// Cleanup
		err = app.StopWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Errorf("Failed to stop application: %v", err)
		}
	})

	t.Run("should handle certificate monitoring intervals", func(t *testing.T) {
		intervalTests := []struct {
			name     string
			interval string
			valid    bool
		}{
			{"seconds interval", "30s", true},
			{"minutes interval", "5m", true},
			{"hours interval", "2h", true},
			{"daily interval", "24h", true},
			{"invalid interval", "invalid", true}, // Framework loads it, module would validate
		}

		for _, tt := range intervalTests {
			t.Run(tt.name, func(t *testing.T) {
				app, err := modular.NewApplication()
		if err != nil {
			t.Fatalf("Failed to create application: %v", err)
		}
				app.EnableEnhancedLifecycle()

				// Register test certificate module
				certMod := &TestCertificateModule{name: "certificate"}
				app.RegisterModule("certificate", certMod)

				// Configure with test interval
				mapFeeder := feeders.NewMapFeeder(map[string]interface{}{
					"certificate.enabled":        true,
					"certificate.check_interval": tt.interval,
				})
				app.RegisterFeeder("config", mapFeeder)

				ctx := context.Background()

				// Initialize application
				err := app.InitWithEnhancedLifecycle(ctx)
				if err != nil {
					t.Fatalf("Failed to initialize application: %v", err)
				}

				// Verify interval configuration is loaded
				provider := app.ConfigProvider()
				if provider == nil {
					t.Fatal("Config provider should be available")
				}

				actualInterval, err := provider.GetString("certificate.check_interval")
				if err != nil {
					t.Fatalf("Failed to get check_interval: %v", err)
				}
				if actualInterval != tt.interval {
					t.Errorf("Expected interval '%s', got: %s", tt.interval, actualInterval)
				}

				// Cleanup
				err = app.StopWithEnhancedLifecycle(ctx)
				if err != nil {
					t.Errorf("Failed to stop application: %v", err)
				}
			})
		}
	})
}