package database

import (
	"context"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestModule_CollectMetrics(t *testing.T) {
	module := NewModule()
	app := NewMockApplication()

	config := &Config{
		Default: "primary",
		Connections: map[string]*ConnectionConfig{
			"primary": {
				Driver: "sqlite",
				DSN:    ":memory:",
			},
		},
	}

	err := module.RegisterConfig(app)
	require.NoError(t, err)
	app.RegisterConfigSection("database", &MockConfigProvider{config: config})

	err = module.Init(app)
	require.NoError(t, err)
	defer func() { _ = module.Stop(context.Background()) }()

	ctx := context.Background()
	metrics := module.CollectMetrics(ctx)

	assert.Equal(t, Name, metrics.Name)
	assert.NotNil(t, metrics.Values)

	// With a single connection, keys should not be prefixed
	expectedKeys := []string{
		"open_connections",
		"in_use",
		"idle",
		"wait_count",
		"wait_duration_ms",
		"max_open",
	}
	for _, key := range expectedKeys {
		_, exists := metrics.Values[key]
		assert.True(t, exists, "expected metric key %q to exist", key)
	}
}

func TestModule_CollectMetrics_MultipleConnections(t *testing.T) {
	module := NewModule()
	app := NewMockApplication()

	config := &Config{
		Default: "primary",
		Connections: map[string]*ConnectionConfig{
			"primary": {
				Driver: "sqlite",
				DSN:    ":memory:",
			},
			"secondary": {
				Driver: "sqlite",
				DSN:    ":memory:",
			},
		},
	}

	err := module.RegisterConfig(app)
	require.NoError(t, err)
	app.RegisterConfigSection("database", &MockConfigProvider{config: config})

	err = module.Init(app)
	require.NoError(t, err)
	defer func() { _ = module.Stop(context.Background()) }()

	ctx := context.Background()
	metrics := module.CollectMetrics(ctx)

	assert.Equal(t, Name, metrics.Name)

	// With multiple connections, keys should be prefixed with connection name
	for _, prefix := range []string{"primary.", "secondary."} {
		_, exists := metrics.Values[prefix+"open_connections"]
		assert.True(t, exists, "expected metric key %q to exist", prefix+"open_connections")
	}
}

func TestModule_CollectMetrics_NoConnections(t *testing.T) {
	module := NewModule()
	app := NewMockApplication()

	config := &Config{
		Default:     "default",
		Connections: map[string]*ConnectionConfig{},
	}

	err := module.RegisterConfig(app)
	require.NoError(t, err)
	app.RegisterConfigSection("database", &MockConfigProvider{config: config})

	err = module.Init(app)
	require.NoError(t, err)

	ctx := context.Background()
	metrics := module.CollectMetrics(ctx)

	assert.Equal(t, Name, metrics.Name)
	assert.Empty(t, metrics.Values)
}

func TestModule_PreStop(t *testing.T) {
	module := NewModule()
	app := NewMockApplication()

	config := &Config{
		Default: "primary",
		Connections: map[string]*ConnectionConfig{
			"primary": {
				Driver: "sqlite",
				DSN:    ":memory:",
			},
		},
	}

	err := module.RegisterConfig(app)
	require.NoError(t, err)
	app.RegisterConfigSection("database", &MockConfigProvider{config: config})

	err = module.Init(app)
	require.NoError(t, err)
	defer func() { _ = module.Stop(context.Background()) }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = module.PreStop(ctx)
	assert.NoError(t, err)
}

func TestModule_PreStop_NoConnections(t *testing.T) {
	module := NewModule()
	app := NewMockApplication()

	config := &Config{
		Default:     "default",
		Connections: map[string]*ConnectionConfig{},
	}

	err := module.RegisterConfig(app)
	require.NoError(t, err)
	app.RegisterConfigSection("database", &MockConfigProvider{config: config})

	err = module.Init(app)
	require.NoError(t, err)

	ctx := context.Background()
	err = module.PreStop(ctx)
	assert.NoError(t, err)
}

func TestModule_Reloadable(t *testing.T) {
	module := NewModule()
	app := NewMockApplication()

	config := &Config{
		Default: "primary",
		Connections: map[string]*ConnectionConfig{
			"primary": {
				Driver: "sqlite",
				DSN:    ":memory:",
			},
		},
	}

	err := module.RegisterConfig(app)
	require.NoError(t, err)
	app.RegisterConfigSection("database", &MockConfigProvider{config: config})

	err = module.Init(app)
	require.NoError(t, err)
	defer func() { _ = module.Stop(context.Background()) }()

	assert.True(t, module.CanReload())
	assert.Equal(t, 10*time.Second, module.ReloadTimeout())
}

func TestModule_Reload(t *testing.T) {
	module := NewModule()
	app := NewMockApplication()

	config := &Config{
		Default: "primary",
		Connections: map[string]*ConnectionConfig{
			"primary": {
				Driver:             "sqlite",
				DSN:                ":memory:",
				MaxOpenConnections: 10,
			},
		},
	}

	err := module.RegisterConfig(app)
	require.NoError(t, err)
	app.RegisterConfigSection("database", &MockConfigProvider{config: config})

	err = module.Init(app)
	require.NoError(t, err)
	defer func() { _ = module.Stop(context.Background()) }()

	ctx := context.Background()

	// Reload with MaxOpenConnections change
	changes := []modular.ConfigChange{
		{
			FieldPath: "MaxOpenConnections",
			NewValue:  "20",
		},
		{
			FieldPath: "MaxIdleConnections",
			NewValue:  "5",
		},
		{
			FieldPath: "ConnectionMaxLifetime",
			NewValue:  "30m",
		},
		{
			FieldPath: "ConnectionMaxIdleTime",
			NewValue:  "5m",
		},
	}

	err = module.Reload(ctx, changes)
	assert.NoError(t, err)

	// Verify the pool settings were applied
	db, exists := module.GetConnection("primary")
	require.True(t, exists)
	stats := db.Stats()
	assert.Equal(t, 20, stats.MaxOpenConnections)
}

func TestModule_Reload_ScopedConnection(t *testing.T) {
	module := NewModule()
	app := NewMockApplication()

	config := &Config{
		Default: "primary",
		Connections: map[string]*ConnectionConfig{
			"primary": {
				Driver:             "sqlite",
				DSN:                ":memory:",
				MaxOpenConnections: 10,
			},
			"secondary": {
				Driver:             "sqlite",
				DSN:                ":memory:",
				MaxOpenConnections: 10,
			},
		},
	}

	err := module.RegisterConfig(app)
	require.NoError(t, err)
	app.RegisterConfigSection("database", &MockConfigProvider{config: config})

	err = module.Init(app)
	require.NoError(t, err)
	defer func() { _ = module.Stop(context.Background()) }()

	ctx := context.Background()

	// Reload only the primary connection's MaxOpenConnections
	changes := []modular.ConfigChange{
		{
			FieldPath: "connections.primary.MaxOpenConnections",
			NewValue:  "25",
		},
	}

	err = module.Reload(ctx, changes)
	assert.NoError(t, err)

	primaryDB, _ := module.GetConnection("primary")
	assert.Equal(t, 25, primaryDB.Stats().MaxOpenConnections)

	secondaryDB, _ := module.GetConnection("secondary")
	assert.Equal(t, 10, secondaryDB.Stats().MaxOpenConnections)
}

func TestModule_Reload_UnrecognizedField(t *testing.T) {
	module := NewModule()
	app := NewMockApplication()

	config := &Config{
		Default: "primary",
		Connections: map[string]*ConnectionConfig{
			"primary": {
				Driver: "sqlite",
				DSN:    ":memory:",
			},
		},
	}

	err := module.RegisterConfig(app)
	require.NoError(t, err)
	app.RegisterConfigSection("database", &MockConfigProvider{config: config})

	err = module.Init(app)
	require.NoError(t, err)
	defer func() { _ = module.Stop(context.Background()) }()

	ctx := context.Background()

	// Unrecognized fields should be silently ignored
	changes := []modular.ConfigChange{
		{
			FieldPath: "SomeUnknownField",
			NewValue:  "value",
		},
	}

	err = module.Reload(ctx, changes)
	assert.NoError(t, err)
}

func TestModule_InterfaceCompliance(t *testing.T) {
	module := NewModule()
	assert.Implements(t, (*modular.MetricsProvider)(nil), module)
	assert.Implements(t, (*modular.Drainable)(nil), module)
	assert.Implements(t, (*modular.Reloadable)(nil), module)
}
