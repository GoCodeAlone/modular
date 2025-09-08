//go:build failing_test

package modular

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHealthWithOptionalModules(t *testing.T) {
	tests := []struct {
		name        string
		description string
		testFunc    func(t *testing.T)
	}{
		{
			name:        "should_handle_optional_modules_in_health_aggregation",
			description: "Health aggregation should handle optional modules gracefully",
			testFunc: func(t *testing.T) {
				builder := NewApplicationBuilder()
				app, err := builder.
					WithOption(WithHealthAggregator()).
					Build(context.Background())
				assert.NoError(t, err, "Should build application")

				healthService := app.GetHealthService()
				result := healthService.CheckHealth(context.Background())
				assert.NotNil(t, result, "Should return health result even with no modules")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}
