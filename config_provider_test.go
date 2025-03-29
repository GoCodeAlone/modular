package modular

import (
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type testCfg struct {
	Str string `yaml:"str"`
	Num int    `yaml:"num"`
}

type testSectionCfg struct {
	Enabled bool   `yaml:"enabled"`
	Name    string `yaml:"name"`
}

// Mock for ComplexFeeder
type MockComplexFeeder struct {
	mock.Mock
}

func (m *MockComplexFeeder) Feed(structure interface{}) error {
	args := m.Called(structure)
	return args.Error(0)
}

func (m *MockComplexFeeder) FeedKey(key string, target interface{}) error {
	args := m.Called(key, target)
	return args.Error(0)
}

func TestNewStdConfigProvider(t *testing.T) {
	cfg := &testCfg{Str: "test", Num: 42}
	provider := NewStdConfigProvider(cfg)

	assert.NotNil(t, provider)
	assert.Equal(t, cfg, provider.GetConfig())
}

func TestStdConfigProvider_GetConfig(t *testing.T) {
	cfg := &testCfg{Str: "test", Num: 42}
	provider := &StdConfigProvider{cfg: cfg}

	assert.Equal(t, cfg, provider.GetConfig())
}

func TestNewConfig(t *testing.T) {
	cfg := NewConfig()

	assert.NotNil(t, cfg)
	assert.NotNil(t, cfg.Config)
	assert.NotNil(t, cfg.StructKeys)
	assert.Empty(t, cfg.StructKeys)
}

func TestConfig_AddStructKey(t *testing.T) {
	cfg := NewConfig()
	target := &testCfg{}

	result := cfg.AddStructKey("test", target)

	assert.Equal(t, cfg, result)
	assert.Len(t, cfg.StructKeys, 1)
	assert.Equal(t, target, cfg.StructKeys["test"])
}

// Test implementation of ConfigSetup
type testSetupCfg struct {
	Value       string `yaml:"value"`
	setupCalled bool
	shouldError bool
}

func (t *testSetupCfg) Setup() error {
	t.setupCalled = true
	if t.shouldError {
		return errors.New("setup error")
	}
	return nil
}

func TestConfig_Feed(t *testing.T) {
	tests := []struct {
		name           string
		setupConfig    func() (*Config, *MockComplexFeeder)
		expectFeedErr  bool
		expectKeyErr   bool
		expectedErrMsg string
	}{
		{
			name: "successful feed",
			setupConfig: func() (*Config, *MockComplexFeeder) {
				cfg := NewConfig()
				feeder := new(MockComplexFeeder)
				feeder.On("Feed", mock.Anything).Return(nil)
				feeder.On("FeedKey", "test", mock.Anything).Return(nil)
				cfg.AddFeeder(feeder)
				cfg.AddStruct(&testCfg{})
				cfg.AddStructKey("test", &testCfg{})
				return cfg, feeder
			},
			expectFeedErr: false,
		},
		{
			name: "feed error",
			setupConfig: func() (*Config, *MockComplexFeeder) {
				cfg := NewConfig()
				feeder := new(MockComplexFeeder)
				feeder.On("Feed", mock.Anything).Return(errors.New("feed error"))
				cfg.AddFeeder(feeder)
				cfg.AddStruct(&testCfg{})
				return cfg, feeder
			},
			expectFeedErr:  true,
			expectedErrMsg: "feed error",
		},
		{
			name: "feedKey error",
			setupConfig: func() (*Config, *MockComplexFeeder) {
				cfg := NewConfig()
				feeder := new(MockComplexFeeder)
				feeder.On("Feed", mock.Anything).Return(nil)
				feeder.On("FeedKey", "test", mock.Anything).Return(errors.New("feedKey error"))
				cfg.AddFeeder(feeder)
				cfg.AddStruct(&testCfg{})
				cfg.AddStructKey("test", &testCfg{})
				return cfg, feeder
			},
			expectFeedErr:  true,
			expectKeyErr:   true,
			expectedErrMsg: "config: feeder error: feedKey error",
		},
		{
			name: "setup success",
			setupConfig: func() (*Config, *MockComplexFeeder) {
				cfg := NewConfig()
				feeder := new(MockComplexFeeder)
				feeder.On("Feed", mock.Anything).Return(nil)
				feeder.On("FeedKey", "test", mock.Anything).Return(nil)
				cfg.AddFeeder(feeder)
				cfg.AddStruct(&testCfg{})
				cfg.AddStructKey("test", &testSetupCfg{})
				return cfg, feeder
			},
			expectFeedErr: false,
		},
		{
			name: "setup error",
			setupConfig: func() (*Config, *MockComplexFeeder) {
				cfg := NewConfig()
				feeder := new(MockComplexFeeder)
				feeder.On("Feed", mock.Anything).Return(nil)
				feeder.On("FeedKey", "test", mock.Anything).Return(nil)
				cfg.AddFeeder(feeder)
				cfg.AddStruct(&testCfg{})
				cfg.AddStructKey("test", &testSetupCfg{shouldError: true})
				return cfg, feeder
			},
			expectFeedErr:  true,
			expectedErrMsg: "config: setup error for test: setup error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, feeder := tt.setupConfig()

			err := cfg.Feed()

			if tt.expectFeedErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrMsg)
			} else {
				assert.NoError(t, err)
				// Check if setup was called when using testSetupCfg
				if setupCfg, ok := cfg.StructKeys["test"].(*testSetupCfg); ok {
					assert.True(t, setupCfg.setupCalled)
				}
			}

			feeder.AssertExpectations(t)
		})
	}
}

func Test_createTempConfig(t *testing.T) {
	t.Run("with pointer", func(t *testing.T) {
		originalCfg := &testCfg{Str: "test", Num: 42}
		tempCfg, info, err := createTempConfig(originalCfg)

		assert.NotNil(t, tempCfg)
		assert.NoError(t, err)
		assert.True(t, info.isPtr)
		assert.Equal(t, reflect.ValueOf(originalCfg).Type(), info.tempVal.Type())
	})

	t.Run("with non-pointer", func(t *testing.T) {
		originalCfg := testCfg{Str: "test", Num: 42}
		tempCfg, info, err := createTempConfig(originalCfg)

		assert.NotNil(t, tempCfg)
		assert.NoError(t, err)
		assert.False(t, info.isPtr)
		assert.Equal(t, reflect.PtrTo(reflect.ValueOf(originalCfg).Type()), info.tempVal.Type())
	})
}

func Test_updateConfig(t *testing.T) {
	t.Run("with pointer config", func(t *testing.T) {
		originalCfg := &testCfg{Str: "old", Num: 0}
		tempCfg := &testCfg{Str: "new", Num: 42}

		mockLogger := new(MockLogger)
		app := &Application{logger: mockLogger}

		provider := ConfigProvider(NewStdConfigProvider(originalCfg))
		origInfo := configInfo{
			originalVal: reflect.ValueOf(originalCfg),
			tempVal:     reflect.ValueOf(tempCfg),
			isPtr:       true,
		}

		updateConfig(app, &provider, origInfo)

		// Check the original config was updated
		assert.Equal(t, "new", originalCfg.Str)
		assert.Equal(t, 42, originalCfg.Num)
	})

	t.Run("with non-pointer config", func(t *testing.T) {
		originalCfg := testCfg{Str: "old", Num: 0}
		tempCfgPtr, origInfo, err := createTempConfig(originalCfg)
		assert.NoError(t, err)
		tempCfgPtr.(*testCfg).Str = "new"
		tempCfgPtr.(*testCfg).Num = 42

		mockLogger := new(MockLogger)
		mockLogger.On("Info", "Creating new provider with updated config (original was non-pointer)", []interface{}(nil)).Return()
		app := &Application{logger: mockLogger}

		provider := ConfigProvider(NewStdConfigProvider(originalCfg))

		updateConfig(app, &provider, origInfo)

		// Check a new provider was created with the expected values
		updated := provider.GetConfig()
		assert.True(t, reflect.ValueOf(updated).Kind() == reflect.Struct)
		assert.Equal(t, "new", updated.(testCfg).Str)
		assert.Equal(t, 42, updated.(testCfg).Num)
		mockLogger.AssertExpectations(t)
	})
}

func Test_updateSectionConfig(t *testing.T) {
	t.Run("with pointer section config", func(t *testing.T) {
		originalCfg := &testSectionCfg{Enabled: false, Name: "old"}
		tempCfg := &testSectionCfg{Enabled: true, Name: "new"}

		mockLogger := new(MockLogger)
		app := &Application{
			logger:      mockLogger,
			cfgSections: make(map[string]ConfigProvider),
		}
		app.cfgSections["test"] = NewStdConfigProvider(originalCfg)

		sectionInfo := configInfo{
			originalVal: reflect.ValueOf(originalCfg),
			tempVal:     reflect.ValueOf(tempCfg),
			isPtr:       true,
		}

		updateSectionConfig(app, "test", sectionInfo)

		// Check the original config was updated
		assert.True(t, originalCfg.Enabled)
		assert.Equal(t, "new", originalCfg.Name)
	})

	t.Run("with non-pointer section config", func(t *testing.T) {
		originalCfg := testSectionCfg{Enabled: false, Name: "old"}
		tempCfgPtr, sectionInfo, err := createTempConfig(originalCfg)
		assert.NoError(t, err)

		// Cast and update the temp config
		tempCfgPtr.(*testSectionCfg).Enabled = true
		tempCfgPtr.(*testSectionCfg).Name = "new"

		mockLogger := new(MockLogger)
		mockLogger.On("Info", "Creating new provider for section", []interface{}{"section", "test"}).Return()

		app := &Application{
			logger:      mockLogger,
			cfgSections: make(map[string]ConfigProvider),
		}
		app.cfgSections["test"] = NewStdConfigProvider(originalCfg)

		updateSectionConfig(app, "test", sectionInfo)

		// Check a new provider was created
		sectCfg := app.cfgSections["test"].GetConfig()
		assert.True(t, sectCfg.(testSectionCfg).Enabled)
		assert.Equal(t, "new", sectCfg.(testSectionCfg).Name)
		mockLogger.AssertExpectations(t)
	})
}

func Test_loadAppConfig(t *testing.T) {
	// Save original ConfigFeeders and restore after test
	originalFeeders := ConfigFeeders
	defer func() { ConfigFeeders = originalFeeders }()

	tests := []struct {
		name           string
		setupApp       func() *Application
		setupFeeders   func() []Feeder
		expectError    bool
		validateResult func(t *testing.T, app *Application)
	}{
		{
			name: "successful config load",
			setupApp: func() *Application {
				mockLogger := new(MockLogger)
				app := &Application{
					logger:      mockLogger,
					cfgProvider: NewStdConfigProvider(&testCfg{Str: "old", Num: 0}),
					cfgSections: make(map[string]ConfigProvider),
				}
				app.cfgSections["section1"] = NewStdConfigProvider(&testSectionCfg{Enabled: false, Name: "old"})
				return app
			},
			setupFeeders: func() []Feeder {
				feeder := new(MockComplexFeeder)
				// Setup for main config
				feeder.On("Feed", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
					cfg := args.Get(0).(*testCfg)
					cfg.Str = "updated"
					cfg.Num = 42
				})
				// Setup for section config
				feeder.On("FeedKey", "section1", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
					cfg := args.Get(1).(*testSectionCfg)
					cfg.Enabled = true
					cfg.Name = "updated"
				})
				return []Feeder{feeder}
			},
			expectError: false,
			validateResult: func(t *testing.T, app *Application) {
				mainCfg := app.cfgProvider.GetConfig().(*testCfg)
				assert.Equal(t, "updated", mainCfg.Str)
				assert.Equal(t, 42, mainCfg.Num)

				sectionCfg := app.cfgSections["section1"].GetConfig().(*testSectionCfg)
				assert.True(t, sectionCfg.Enabled)
				assert.Equal(t, "updated", sectionCfg.Name)
			},
		},
		{
			name: "feed error",
			setupApp: func() *Application {
				mockLogger := new(MockLogger)
				app := &Application{
					logger:      mockLogger,
					cfgProvider: NewStdConfigProvider(&testCfg{Str: "old", Num: 0}),
					cfgSections: make(map[string]ConfigProvider),
				}
				return app
			},
			setupFeeders: func() []Feeder {
				feeder := new(MockComplexFeeder)
				feeder.On("Feed", mock.Anything).Return(errors.New("feed error"))
				return []Feeder{feeder}
			},
			expectError: true,
			validateResult: func(t *testing.T, app *Application) {
				// Config should remain unchanged
				mainCfg := app.cfgProvider.GetConfig().(*testCfg)
				assert.Equal(t, "old", mainCfg.Str)
				assert.Equal(t, 0, mainCfg.Num)
			},
		},
		{
			name: "feedKey error",
			setupApp: func() *Application {
				mockLogger := new(MockLogger)
				app := &Application{
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
				feeder.On("FeedKey", "section1", mock.Anything).Return(errors.New("feedKey error"))
				return []Feeder{feeder}
			},
			expectError: true,
			validateResult: func(t *testing.T, app *Application) {
				// Configs should remain unchanged
				mainCfg := app.cfgProvider.GetConfig().(*testCfg)
				assert.Equal(t, "old", mainCfg.Str)

				sectionCfg := app.cfgSections["section1"].GetConfig().(*testSectionCfg)
				assert.False(t, sectionCfg.Enabled)
			},
		},
		{
			name: "non-pointer configs",
			setupApp: func() *Application {
				mockLogger := new(MockLogger)
				mockLogger.On("Info", "Creating new provider with updated config (original was non-pointer)", []interface{}(nil)).Return()
				mockLogger.On("Info", "Creating new provider for section", []interface{}{"section", "section1"}).Return()

				app := &Application{
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
					cfg := args.Get(0).(*testCfg)
					cfg.Str = "updated"
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
			validateResult: func(t *testing.T, app *Application) {
				mainCfg := app.cfgProvider.GetConfig()
				assert.Equal(t, "updated", mainCfg.(testCfg).Str)
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
			ConfigFeeders = tt.setupFeeders()

			err := loadAppConfig(app)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				tt.validateResult(t, app)
			}

			// Assert that all mock expectations were met
			for _, feeder := range ConfigFeeders {
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
