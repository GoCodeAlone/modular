package modular

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock for VerboseAwareFeeder
type MockVerboseAwareFeeder struct {
	mock.Mock
}

func (m *MockVerboseAwareFeeder) Feed(structure any) error {
	args := m.Called(structure)
	if err := args.Error(0); err != nil {
		return fmt.Errorf("mock feeder error: %w", err)
	}
	return nil
}

func (m *MockVerboseAwareFeeder) SetVerboseDebug(enabled bool, logger interface{ Debug(msg string, args ...any) }) {
	m.Called(enabled, logger)
}

func TestConfig_SetVerboseDebug(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name               string
		setVerbose         bool
		feeders            []Feeder
		expectVerboseCalls int
	}{
		{
			name:       "enable verbose debug with verbose-aware feeder",
			setVerbose: true,
			feeders: []Feeder{
				&MockVerboseAwareFeeder{},
				&MockComplexFeeder{}, // non-verbose aware feeder
			},
			expectVerboseCalls: 1,
		},
		{
			name:       "disable verbose debug with verbose-aware feeder",
			setVerbose: false,
			feeders: []Feeder{
				&MockVerboseAwareFeeder{},
			},
			expectVerboseCalls: 1,
		},
		{
			name:       "enable verbose debug with no verbose-aware feeders",
			setVerbose: true,
			feeders: []Feeder{
				&MockComplexFeeder{},
			},
			expectVerboseCalls: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLogger := new(MockLogger)

			// Set up the config with feeders already added (no verbose initially)
			cfg := NewConfig()
			for _, feeder := range tt.feeders {
				cfg.AddFeeder(feeder)
			}

			// Set up expectations for SetVerboseDebug call
			for _, feeder := range tt.feeders {
				if mockVerbose, ok := feeder.(*MockVerboseAwareFeeder); ok {
					mockVerbose.On("SetVerboseDebug", tt.setVerbose, mockLogger).Return()
				}
			}

			// Call SetVerboseDebug
			result := cfg.SetVerboseDebug(tt.setVerbose, mockLogger)

			// Assertions
			assert.Equal(t, cfg, result, "SetVerboseDebug should return the same config instance")
			assert.Equal(t, tt.setVerbose, cfg.VerboseDebug)
			assert.Equal(t, mockLogger, cfg.Logger)

			// Verify mock expectations
			for _, feeder := range tt.feeders {
				if mockVerbose, ok := feeder.(*MockVerboseAwareFeeder); ok {
					mockVerbose.AssertExpectations(t)
				}
			}
		})
	}
}

func TestConfig_AddFeeder_WithVerboseDebug(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		verboseEnabled    bool
		feeder            Feeder
		expectVerboseCall bool
	}{
		{
			name:              "add verbose-aware feeder with verbose enabled",
			verboseEnabled:    true,
			feeder:            &MockVerboseAwareFeeder{},
			expectVerboseCall: true,
		},
		{
			name:              "add verbose-aware feeder with verbose disabled",
			verboseEnabled:    false,
			feeder:            &MockVerboseAwareFeeder{},
			expectVerboseCall: false,
		},
		{
			name:              "add non-verbose-aware feeder",
			verboseEnabled:    true,
			feeder:            &MockComplexFeeder{},
			expectVerboseCall: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLogger := new(MockLogger)

			cfg := NewConfig()
			cfg.VerboseDebug = tt.verboseEnabled
			cfg.Logger = mockLogger

			// Set up expectations for verbose-aware feeders
			if tt.expectVerboseCall {
				if mockVerbose, ok := tt.feeder.(*MockVerboseAwareFeeder); ok {
					mockVerbose.On("SetVerboseDebug", true, mockLogger).Return()
				}
			}

			// Call AddFeeder
			result := cfg.AddFeeder(tt.feeder)

			// Assertions
			assert.Equal(t, cfg, result, "AddFeeder should return the same config instance")
			assert.Contains(t, cfg.Feeders, tt.feeder)

			// Verify mock expectations
			if mockVerbose, ok := tt.feeder.(*MockVerboseAwareFeeder); ok {
				mockVerbose.AssertExpectations(t)
			}
		})
	}
}

func TestConfig_Feed_VerboseDebug(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		enableVerbose bool
	}{
		{
			name:          "verbose debug enabled",
			enableVerbose: true,
		},
		{
			name:          "verbose debug disabled",
			enableVerbose: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLogger := new(MockLogger)

			cfg := NewConfig()
			if tt.enableVerbose {
				cfg.SetVerboseDebug(true, mockLogger)
				// Just allow any debug calls - we don't care about specific messages
				mockLogger.On("Debug", mock.Anything, mock.Anything).Return().Maybe()
			}

			cfg.AddStructKey("test", &testCfg{Str: "test", Num: 42})

			// Mock feeder that does nothing
			mockFeeder := new(MockComplexFeeder)
			mockFeeder.On("Feed", mock.Anything).Return(nil).Maybe()
			mockFeeder.On("FeedKey", mock.Anything, mock.Anything).Return(nil).Maybe()
			cfg.AddFeeder(mockFeeder)

			err := cfg.Feed()
			require.NoError(t, err)

			// Verify that verbose state was set correctly
			assert.Equal(t, tt.enableVerbose, cfg.VerboseDebug)
		})
	}
}
