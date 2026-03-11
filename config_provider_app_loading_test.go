package modular

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func Test_loadAppConfig(t *testing.T) {
	t.Parallel()
	// Tests now rely on per-application feeders (SetConfigFeeders) instead of mutating
	// the global ConfigFeeders slice to support safe parallelization.

	tests := []struct {
		name           string
		setupApp       func() *StdApplication
		setupFeeders   func() []Feeder
		expectError    bool
		validateResult func(t *testing.T, app *StdApplication)
	}{
		{
			name: "successful config load",
			setupApp: func() *StdApplication {
				mockLogger := new(MockLogger)
				mockLogger.On("Debug", "Added main config for loading", mock.Anything).Return()
				mockLogger.On("Debug", "Added section config for loading", mock.Anything).Return()
				mockLogger.On("Debug", "Updated main config", mock.Anything).Return()
				mockLogger.On("Debug", "Updated section config", mock.Anything).Return()

				app := &StdApplication{
					logger:      mockLogger,
					cfgProvider: NewStdConfigProvider(&testCfg{Str: "old", Num: 0}),
					cfgSections: make(map[string]ConfigProvider),
				}
				app.cfgSections["section1"] = NewStdConfigProvider(&testSectionCfg{Enabled: false, Name: "old"})
				return app
			},
			setupFeeders: func() []Feeder {
				feeder := new(MockComplexFeeder)
				// Setup to handle any Feed call - let the Run function determine the type
				feeder.On("Feed", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
					if cfg, ok := args.Get(0).(*testCfg); ok {
						cfg.Str = updatedValue
						cfg.Num = 42
					} else if cfg, ok := args.Get(0).(*testSectionCfg); ok {
						cfg.Enabled = true
						cfg.Name = "updated"
					}
				})
				// Setup for main config FeedKey calls
				feeder.On("FeedKey", "_main", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
					cfg := args.Get(1).(*testCfg)
					cfg.Str = updatedValue
					cfg.Num = 42
				})
				// Setup for section config FeedKey calls
				feeder.On("FeedKey", "section1", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
					cfg := args.Get(1).(*testSectionCfg)
					cfg.Enabled = true
					cfg.Name = "updated"
				})
				return []Feeder{feeder}
			},
			expectError: false,
			validateResult: func(t *testing.T, app *StdApplication) {
				mainCfg := app.cfgProvider.GetConfig().(*testCfg)
				assert.Equal(t, updatedValue, mainCfg.Str)
				assert.Equal(t, 42, mainCfg.Num)

				sectionCfg := app.cfgSections["section1"].GetConfig().(*testSectionCfg)
				assert.True(t, sectionCfg.Enabled)
				assert.Equal(t, "updated", sectionCfg.Name)
			},
		},
		{
			name: "feed error",
			setupApp: func() *StdApplication {
				mockLogger := new(MockLogger)
				mockLogger.On("Debug", "Added main config for loading", mock.Anything).Return()
				app := &StdApplication{
					logger:      mockLogger,
					cfgProvider: NewStdConfigProvider(&testCfg{Str: "old", Num: 0}),
					cfgSections: make(map[string]ConfigProvider),
				}
				return app
			},
			setupFeeders: func() []Feeder {
				feeder := new(MockComplexFeeder)
				feeder.On("Feed", mock.Anything).Return(ErrFeedFailed)
				return []Feeder{feeder}
			},
			expectError: true,
			validateResult: func(t *testing.T, app *StdApplication) {
				// Config should remain unchanged
				mainCfg := app.cfgProvider.GetConfig().(*testCfg)
				assert.Equal(t, "old", mainCfg.Str)
				assert.Equal(t, 0, mainCfg.Num)
			},
		},
		{
			name: "feedKey error",
			setupApp: func() *StdApplication {
				mockLogger := new(MockLogger)
				mockLogger.On("Debug", "Added main config for loading", mock.Anything).Return()
				mockLogger.On("Debug", "Added section config for loading", mock.Anything).Return()
				app := &StdApplication{
					logger:      mockLogger,
					cfgProvider: NewStdConfigProvider(&testCfg{Str: "old", Num: 0}),
					cfgSections: make(map[string]ConfigProvider),
				}
				app.cfgSections["section1"] = NewStdConfigProvider(&testSectionCfg{Enabled: false, Name: "old"})
				return app
			},
			setupFeeders: func() []Feeder {
				feeder := new(MockComplexFeeder)
				feeder.On("Feed", mock.Anything).Return(nil)
				// Due to map iteration order being random, either key could be processed first
				// If "section1" is processed first, it will fail and stop processing
				// If "_main" is processed first, it will succeed, then "section1" will fail
				feeder.On("FeedKey", "_main", mock.Anything).Return(nil).Maybe()
				feeder.On("FeedKey", "section1", mock.Anything).Return(ErrFeedKeyFailed)
				return []Feeder{feeder}
			},
			expectError: true,
			validateResult: func(t *testing.T, app *StdApplication) {
				// Configs should remain unchanged
				mainCfg := app.cfgProvider.GetConfig().(*testCfg)
				assert.Equal(t, "old", mainCfg.Str)

				sectionCfg := app.cfgSections["section1"].GetConfig().(*testSectionCfg)
				assert.False(t, sectionCfg.Enabled)
			},
		},
		{
			name: "non-pointer configs",
			setupApp: func() *StdApplication {
				mockLogger := new(MockLogger)
				mockLogger.On("Debug",
					"Creating new provider with updated config (original was non-pointer)",
					[]any(nil)).Return()
				mockLogger.On("Debug", "Creating new provider for section", []any{"section", "section1"}).Return()
				mockLogger.On("Debug", "Added main config for loading", mock.Anything).Return()
				mockLogger.On("Debug", "Added section config for loading", mock.Anything).Return()
				mockLogger.On("Debug", "Updated main config", mock.Anything).Return()
				mockLogger.On("Debug", "Updated section config", mock.Anything).Return()

				app := &StdApplication{
					logger:      mockLogger,
					cfgProvider: NewStdConfigProvider(testCfg{Str: "old", Num: 0}), // non-pointer
					cfgSections: make(map[string]ConfigProvider),
				}
				app.cfgSections["section1"] = NewStdConfigProvider(testSectionCfg{Enabled: false, Name: "old"}) // non-pointer
				return app
			},
			setupFeeders: func() []Feeder {
				feeder := new(MockComplexFeeder)
				feeder.On("Feed", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
					if cfg, ok := args.Get(0).(*testCfg); ok {
						cfg.Str = updatedValue
						cfg.Num = 42
					} else if cfg, ok := args.Get(0).(*testSectionCfg); ok {
						cfg.Enabled = true
						cfg.Name = "updated"
					}
				})
				feeder.On("FeedKey", "_main", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
					cfg := args.Get(1).(*testCfg)
					cfg.Str = updatedValue
					cfg.Num = 42
				})
				feeder.On("FeedKey", "section1", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
					cfg := args.Get(1).(*testSectionCfg)
					cfg.Enabled = true
					cfg.Name = "updated"
				})
				return []Feeder{feeder}
			},
			expectError: false,
			validateResult: func(t *testing.T, app *StdApplication) {
				mainCfg := app.cfgProvider.GetConfig()
				assert.Equal(t, updatedValue, mainCfg.(testCfg).Str)
				assert.Equal(t, 42, mainCfg.(testCfg).Num)

				sectionCfg := app.cfgSections["section1"].GetConfig()
				assert.True(t, sectionCfg.(testSectionCfg).Enabled)
				assert.Equal(t, "updated", sectionCfg.(testSectionCfg).Name)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := tt.setupApp()
			// Use per-app feeders; StdApplication exposes SetConfigFeeders directly.
			app.SetConfigFeeders(tt.setupFeeders())

			err := loadAppConfig(app)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				tt.validateResult(t, app)
			}

			// Assert that all mock expectations were met on the feeders we injected
			for _, feeder := range app.configFeeders {
				if mockFeeder, ok := feeder.(*MockComplexFeeder); ok {
					mockFeeder.AssertExpectations(t)
				}
			}
			if mockLogger, ok := app.logger.(*MockLogger); ok {
				mockLogger.AssertExpectations(t)
			}
		})
	}
}
