//go:build failing_test

package modular

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReloadWithValidationErrors(t *testing.T) {
	tests := []struct {
		name        string
		description string
		testFunc    func(t *testing.T)
	}{
		{
			name:        "should_handle_config_validation_errors_during_reload",
			description: "Reload should fail gracefully when config validation fails",
			testFunc: func(t *testing.T) {
				builder := NewApplicationBuilder()
				app, err := builder.
					WithOption(WithDynamicReload()).
					Build(context.Background())
				assert.NoError(t, err, "Should build application")

				// Create invalid config
				invalidConfig := map[string]interface{}{
					"invalid_field": "invalid_value",
				}

				// Attempt reload with invalid config
				err = app.TriggerReload(context.Background(), "validation-test", invalidConfig, ReloadTriggerManual)
				assert.Error(t, err, "Should fail with validation error")
				assert.Contains(t, err.Error(), "validation", "Error should mention validation")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}
