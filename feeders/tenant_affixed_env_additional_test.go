package feeders

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// localMockLogger implements the minimal Debug method expected (avoid clash with any other test definitions).
type localMockLogger struct{ mock.Mock }

func (m *localMockLogger) Debug(msg string, args ...any) { m.Called(msg, args) }

// TestTenantAffixedEnvFeeder_FeedKeyDynamic verifies dynamic prefix/suffix assignment inside FeedKey.
func TestTenantAffixedEnvFeeder_FeedKeyDynamic(t *testing.T) {
	prefixFunc := func(tenant string) string { return "APP_" + tenant + "_" }
	suffixFunc := func(tenant string) string { return "_" + tenant + "ENV" }
	feeder := NewTenantAffixedEnvFeeder(prefixFunc, suffixFunc)

	// Prepare env based on computed pattern: APP_TEN123__NAME__TEN123ENV
	os.Setenv("APP_TEN123__NAME__TEN123ENV", "dyn-name")
	defer os.Unsetenv("APP_TEN123__NAME__TEN123ENV")

	var cfg struct {
		Name string `env:"NAME"`
	}
	err := feeder.FeedKey("ten123", &cfg)
	assert.NoError(t, err)
	assert.Equal(t, "dyn-name", cfg.Name)
	// Underlying feeder stores prefix/suffix exactly as returned by provided funcs (case preserved)
	assert.Equal(t, "APP_ten123_", feeder.Prefix)
	assert.Equal(t, "_ten123ENV", feeder.Suffix)
}

// TestTenantAffixedEnvFeeder_FeedFallback ensures Feed() falls back to FeedKey with empty tenant when no prefix/suffix preset.
func TestTenantAffixedEnvFeeder_FeedFallback(t *testing.T) {
	prefixFunc := func(tenant string) string { return "P_" + tenant + "_" }
	suffixFunc := func(tenant string) string { return "_S" + tenant }
	feeder := NewTenantAffixedEnvFeeder(prefixFunc, suffixFunc)

	// Empty tenant means prefixFunc("") => "P__" and suffixFunc("") => "_S"
	// Expect env var pattern: P___NAME__S (double underscore due to affixed pattern logic)
	os.Setenv("P___NAME__S", "fallback")
	defer os.Unsetenv("P___NAME__S")

	var cfg struct {
		Name string `env:"NAME"`
	}
	err := feeder.Feed(&cfg)
	assert.NoError(t, err)
	assert.Equal(t, "fallback", cfg.Name)
}

// TestTenantAffixedEnvFeeder_VerboseDebugToggle ensures SetVerboseDebug propagates to underlying feeder.
func TestTenantAffixedEnvFeeder_VerboseDebugToggle(t *testing.T) {
	prefixFunc := func(tenant string) string { return tenant }
	suffixFunc := func(tenant string) string { return tenant }
	feeder := NewTenantAffixedEnvFeeder(prefixFunc, suffixFunc)

	ml := new(localMockLogger)
	ml.On("Debug", mock.Anything, mock.Anything).Return()
	feeder.SetVerboseDebug(true, ml)
	// No assertions on logs content; just ensure no panic and flag set
	assert.True(t, feeder.verboseDebug)
	feeder.SetVerboseDebug(false, ml)
	assert.False(t, feeder.verboseDebug)
}
