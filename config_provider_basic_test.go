package modular

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const (
	updatedValue = "updated"
)

type testSectionCfg struct {
	Enabled bool   `yaml:"enabled"`
	Name    string `yaml:"name"`
}

// Mock for ComplexFeeder
type MockComplexFeeder struct {
	mock.Mock
}

func (m *MockComplexFeeder) Feed(structure any) error {
	args := m.Called(structure)
	if err := args.Error(0); err != nil {
		return fmt.Errorf("mock feeder error: %w", err)
	}
	return nil
}

func (m *MockComplexFeeder) FeedKey(key string, target any) error {
	args := m.Called(key, target)
	if err := args.Error(0); err != nil {
		return fmt.Errorf("mock feeder key error: %w", err)
	}
	return nil
}

func TestNewStdConfigProvider(t *testing.T) {
	t.Parallel()
	cfg := &testCfg{Str: "test", Num: 42}
	provider := NewStdConfigProvider(cfg)

	assert.NotNil(t, provider)
	assert.Equal(t, cfg, provider.GetConfig())
}

func TestStdConfigProvider_GetConfig(t *testing.T) {
	t.Parallel()
	cfg := &testCfg{Str: "test", Num: 42}
	provider := &StdConfigProvider{cfg: cfg}

	assert.Equal(t, cfg, provider.GetConfig())
}

func TestNewConfig(t *testing.T) {
	t.Parallel()
	cfg := NewConfig()

	assert.NotNil(t, cfg)
	assert.NotNil(t, cfg.Feeders)
	assert.NotNil(t, cfg.StructKeys)
	assert.Empty(t, cfg.StructKeys)
}

func TestConfig_AddStructKey(t *testing.T) {
	t.Parallel()
	cfg := NewConfig()
	target := &testCfg{}

	result := cfg.AddStructKey("test", target)

	assert.Equal(t, cfg, result)
	assert.Len(t, cfg.StructKeys, 1)
	assert.Equal(t, target, cfg.StructKeys["test"])
}
