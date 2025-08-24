package modular

import (
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoggerAsService(t *testing.T) {
	t.Run("Logger should be available as a service", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
		app := NewStdApplication(NewStdConfigProvider(&struct{}{}), logger)

		// The logger should be available as a service immediately after creation
		var retrievedLogger Logger
		err := app.GetService("logger", &retrievedLogger)
		require.NoError(t, err, "Logger service should be available")
		assert.Equal(t, logger, retrievedLogger, "Retrieved logger should match the original")
	})
}
