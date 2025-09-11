package modular

import (
	"errors"
	"testing"

	"github.com/GoCodeAlone/modular/feeders"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// failingModuleAwareFeeder implements ModuleAwareFeeder to force an error path
type failingModuleAwareFeeder struct{}

func (f *failingModuleAwareFeeder) Feed(_ interface{}) error {
	return errors.New("unexpected direct feed call")
}
func (f *failingModuleAwareFeeder) FeedWithModuleContext(_ interface{}, _ string) error {
	return errors.New("boom")
}

// TestFeedWithModuleContextSuccess ensures module-aware env feeder is used and validation/setup run.
func TestFeedWithModuleContextSuccess(t *testing.T) {
	type TestCfg struct {
		Name string `env:"TEST_FEATURE_NAME"`
	}
	cfgStruct := &TestCfg{}
	t.Setenv("TEST_FEATURE_NAME", "module-value")

	mockLogger := new(MockLogger)
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()

	cfg := NewConfig()
	cfg.SetVerboseDebug(true, mockLogger)
	// Use EnvFeeder which implements ModuleAwareFeeder
	envFeeder := feeders.NewEnvFeeder()
	envFeeder.SetVerboseDebug(true, mockLogger)
	cfg.AddFeeder(envFeeder)

	// Happy path: should populate value via FeedWithModuleContext
	err := cfg.FeedWithModuleContext(cfgStruct, "myModule")
	assert.NoError(t, err)
	assert.Equal(t, "module-value", cfgStruct.Name)
}

// TestFeedWithModuleContextError ensures errors from module-aware feeder are wrapped with ErrConfigFeederError.
func TestFeedWithModuleContextError(t *testing.T) {
	type BadCfg struct{}
	bad := &BadCfg{}
	mockLogger := new(MockLogger)
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()

	cfg := NewConfig()
	cfg.SetVerboseDebug(true, mockLogger)
	cfg.AddFeeder(&failingModuleAwareFeeder{})

	err := cfg.FeedWithModuleContext(bad, "badModule")
	if assert.Error(t, err) {
		// Expect wrapped error to contain sentinel ErrConfigFeederError
		assert.ErrorIs(t, err, ErrConfigFeederError)
	}
}
