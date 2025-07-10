package database

import (
	"context"
	"testing"

	"github.com/GoCodeAlone/modular"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite" // Import pure Go sqlite driver for testing
)

// Mock implementations for testing
type MockApplication struct {
	configSections map[string]interface{}
	logger         modular.Logger
	services       map[string]any
}

func NewMockApplication() *MockApplication {
	return &MockApplication{
		configSections: make(map[string]interface{}),
		logger:         &MockLogger{},
		services:       make(map[string]any),
	}
}

func (a *MockApplication) RegisterConfigSection(name string, provider modular.ConfigProvider) {
	a.configSections[name] = provider
}

func (a *MockApplication) GetConfigSection(name string) (modular.ConfigProvider, error) {
	if provider, exists := a.configSections[name]; exists {
		if cp, ok := provider.(modular.ConfigProvider); ok {
			return cp, nil
		}
	}
	return nil, modular.ErrConfigSectionNotFound
}

func (a *MockApplication) ConfigProvider() modular.ConfigProvider { return &MockConfigProvider{} }
func (a *MockApplication) ConfigSections() map[string]modular.ConfigProvider {
	result := make(map[string]modular.ConfigProvider)
	for k, v := range a.configSections {
		if cp, ok := v.(modular.ConfigProvider); ok {
			result[k] = cp
		}
	}
	return result
}
func (a *MockApplication) Logger() modular.Logger               { return a.logger }
func (a *MockApplication) SetLogger(logger modular.Logger)      { a.logger = logger }
func (a *MockApplication) SvcRegistry() modular.ServiceRegistry { return a.services }
func (a *MockApplication) RegisterModule(module modular.Module) {}
func (a *MockApplication) RegisterService(name string, service any) error {
	a.services[name] = service
	return nil
}
func (a *MockApplication) GetService(name string, target any) error { return nil }
func (a *MockApplication) Init() error                              { return nil }
func (a *MockApplication) Start() error                             { return nil }
func (a *MockApplication) Stop() error                              { return nil }
func (a *MockApplication) Run() error                               { return nil }
func (a *MockApplication) IsVerboseConfig() bool                    { return false }
func (a *MockApplication) SetVerboseConfig(bool)                    {}

type MockConfigProvider struct {
	config interface{}
}

func (m *MockConfigProvider) GetConfig() any { return m.config }

type MockLogger struct{}

func (l *MockLogger) Debug(msg string, args ...any) {}
func (l *MockLogger) Info(msg string, args ...any)  {}
func (l *MockLogger) Warn(msg string, args ...any)  {}
func (l *MockLogger) Error(msg string, args ...any) {}

func TestNewModule(t *testing.T) {
	module := NewModule()
	assert.NotNil(t, module)
	assert.Equal(t, Name, module.Name())
	assert.Implements(t, (*modular.Module)(nil), module)
}

func TestModule_RegisterConfig(t *testing.T) {
	module := NewModule()
	app := NewMockApplication()

	err := module.RegisterConfig(app)
	require.NoError(t, err)

	// Verify config was registered
	configProvider, err := app.GetConfigSection("database")
	require.NoError(t, err)
	assert.NotNil(t, configProvider)

	config := configProvider.GetConfig().(*Config)
	assert.NotNil(t, config)
	assert.Equal(t, "default", config.Default)
}

func TestModule_Init_WithNoConfig(t *testing.T) {
	module := NewModule()
	app := NewMockApplication()

	// Test initialization without config section - should return error
	err := module.Init(app)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get config section")

	// When no config is provided, the module should not initialize any database services
	// Verify that GetDefaultConnection returns nil since no connections were configured
	defaultConn := module.GetDefaultConnection()
	assert.Nil(t, defaultConn)

	// Test getting named connection should also return false
	_, exists := module.GetConnection("test")
	assert.False(t, exists)
}

func TestModule_Init_WithEmptyConfig(t *testing.T) {
	module := NewModule()
	app := NewMockApplication()

	// Register empty config
	config := &Config{
		Default:     "default",
		Connections: map[string]*ConnectionConfig{},
	}
	app.RegisterConfigSection("database", &MockConfigProvider{config: config})

	err := module.RegisterConfig(app)
	require.NoError(t, err)

	err = module.Init(app)
	require.NoError(t, err) // Should succeed with empty connections

	// Verify services are still provided
	services := module.ProvidesServices()
	assert.Len(t, services, 2)
}

func TestModule_Lifecycle(t *testing.T) {
	module := NewModule()
	app := NewMockApplication()

	// Register empty config to avoid initialization errors
	config := &Config{
		Default:     "default",
		Connections: map[string]*ConnectionConfig{},
	}
	app.RegisterConfigSection("database", &MockConfigProvider{config: config})

	// Register and initialize
	err := module.RegisterConfig(app)
	require.NoError(t, err)

	err = module.Init(app)
	require.NoError(t, err)

	// Test Start
	ctx := context.Background()
	err = module.Start(ctx)
	require.NoError(t, err)

	// Test Stop
	err = module.Stop(ctx)
	assert.NoError(t, err)
}

func TestModule_Services(t *testing.T) {
	module := NewModule()

	// Test RequiredServices
	required := module.RequiresServices()
	assert.Empty(t, required)

	// Test ProvidedServices after initialization
	app := NewMockApplication()
	config := &Config{
		Default:     "default",
		Connections: map[string]*ConnectionConfig{},
	}
	app.RegisterConfigSection("database", &MockConfigProvider{config: config})

	err := module.RegisterConfig(app)
	require.NoError(t, err)
	err = module.Init(app)
	require.NoError(t, err)

	provided := module.ProvidesServices()
	assert.Len(t, provided, 2)

	serviceMap := make(map[string]string)
	for _, svc := range provided {
		serviceMap[svc.Name] = svc.Description
	}

	// Use the correct service descriptions from the actual implementation
	assert.Equal(t, "Default database service", serviceMap["database.service"])
	assert.Equal(t, "Database connection manager", serviceMap["database.manager"])
}

func TestDatabaseServiceFactory(t *testing.T) {
	tests := []struct {
		name          string
		driver        string
		dsn           string
		shouldSucceed bool
	}{
		{
			name:          "sqlite service",
			driver:        "sqlite",
			dsn:           ":memory:",
			shouldSucceed: true,
		},
		{
			name:          "postgres service",
			driver:        "postgres",
			dsn:           "postgres://localhost/test",
			shouldSucceed: true,
		},
		{
			name:          "mysql service",
			driver:        "mysql",
			dsn:           "user:pass@tcp(localhost:3306)/db",
			shouldSucceed: true,
		},
		{
			name:          "empty driver",
			driver:        "",
			dsn:           "test://localhost",
			shouldSucceed: false,
		},
		{
			name:          "empty dsn",
			driver:        "sqlite",
			dsn:           "",
			shouldSucceed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ConnectionConfig{
				Driver: tt.driver,
				DSN:    tt.dsn,
			}

			service, err := NewDatabaseService(config)
			if tt.shouldSucceed {
				require.NoError(t, err)
				assert.NotNil(t, service)
				assert.Implements(t, (*DatabaseService)(nil), service)
			} else {
				require.Error(t, err)
				assert.Nil(t, service)
			}
		})
	}
}

func TestDatabaseService_Operations(t *testing.T) {
	// Test with SQLite in-memory database
	config := ConnectionConfig{
		Driver:             "sqlite",
		DSN:                ":memory:",
		MaxOpenConnections: 5, // Allow multiple connections for parallel subtests
	}

	service, err := NewDatabaseService(config)
	require.NoError(t, err)
	require.NotNil(t, service)

	// Connect to the database first
	err = service.Connect()
	require.NoError(t, err)
	defer func() {
		if closeErr := service.Close(); closeErr != nil {
			t.Logf("Failed to close service: %v", closeErr)
		}
	}()

	t.Run("Ping", func(t *testing.T) {
		ctx := context.Background()
		err := service.Ping(ctx)
		assert.NoError(t, err)
	})

	t.Run("DB", func(t *testing.T) {
		db := service.DB()
		assert.NotNil(t, db)
	})

	t.Run("Stats", func(t *testing.T) {
		stats := service.Stats()
		assert.NotNil(t, stats)
	})

	t.Run("Exec", func(t *testing.T) {
		_, err := service.Exec("CREATE TABLE IF NOT EXISTS test_table (id INTEGER PRIMARY KEY, name TEXT)")
		require.NoError(t, err)

		ctx := context.Background()
		result, err := service.ExecContext(ctx, "INSERT INTO test_table (name) VALUES (?)", "test")
		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("Query", func(t *testing.T) {
		// Ensure table exists for this test
		_, err := service.Exec("CREATE TABLE IF NOT EXISTS test_table (id INTEGER PRIMARY KEY, name TEXT)")
		require.NoError(t, err)

		// Insert some test data
		_, err = service.Exec("INSERT OR IGNORE INTO test_table (name) VALUES ('test')")
		require.NoError(t, err)

		rows1, err := service.Query("SELECT * FROM test_table")
		require.NoError(t, err)
		assert.NotNil(t, rows1)
		require.NoError(t, rows1.Err())
		rows1.Close() // nolint:sqlclosecheck // Close explicitly before next query to avoid connection pool deadlock

		ctx := context.Background()
		rows2, err := service.QueryContext(ctx, "SELECT * FROM test_table WHERE name = ?", "test")
		require.NoError(t, err)
		require.NotNil(t, rows2)
		require.NoError(t, rows2.Err())
		defer rows2.Close()
	})

	t.Run("QueryRow", func(t *testing.T) {
		// Ensure table exists for this test
		_, err := service.Exec("CREATE TABLE IF NOT EXISTS test_table (id INTEGER PRIMARY KEY, name TEXT)")
		require.NoError(t, err)

		// First QueryRow - consume the result to release connection
		row := service.QueryRow("SELECT COUNT(*) FROM test_table")
		assert.NotNil(t, row)
		var count int
		err = row.Scan(&count)
		require.NoError(t, err)

		// Second QueryRow - now safe to execute
		ctx := context.Background()
		row = service.QueryRowContext(ctx, "SELECT name FROM test_table WHERE id = ?", 1)
		assert.NotNil(t, row)
	})

	t.Run("Transactions", func(t *testing.T) {
		// Test Begin() - ensure transaction is rolled back before next test
		tx1, err := service.Begin()
		require.NoError(t, err)
		require.NotNil(t, tx1)
		rollbackErr := tx1.Rollback()
		require.NoError(t, rollbackErr) // Ensure rollback succeeds to free connection

		// Test BeginTx() - now safe to start second transaction
		ctx := context.Background()
		tx2, err := service.BeginTx(ctx, nil)
		require.NoError(t, err)
		require.NotNil(t, tx2)
		rollbackErr = tx2.Rollback()
		require.NoError(t, rollbackErr) // Ensure rollback succeeds
	})
}

func TestDatabaseService_ErrorHandling(t *testing.T) {
	t.Run("OperationsWithoutConnection", func(t *testing.T) {
		config := ConnectionConfig{
			Driver: "sqlite",
			DSN:    ":memory:",
		}
		service, err := NewDatabaseService(config)
		require.NoError(t, err)

		ctx := context.Background()

		// Test operations without connecting first
		err = service.Ping(ctx)
		require.Error(t, err)
		require.Equal(t, ErrDatabaseNotConnected, err)
		_, err = service.ExecContext(ctx, "SELECT 1")
		require.Error(t, err)
		require.Equal(t, ErrDatabaseNotConnected, err)

		// Test QueryContext without connection
		rows, err := service.QueryContext(ctx, "SELECT 1")
		require.Error(t, err)
		require.Equal(t, ErrDatabaseNotConnected, err)
		if rows != nil {
			defer rows.Close()
			if rows.Err() != nil {
				t.Errorf("rows error: %v", rows.Err())
			}
		}

		// Test that we can check rows errors - this should also fail since not connected
		rows2, err := service.QueryContext(ctx, "SELECT 1")
		require.Error(t, err)
		require.Equal(t, ErrDatabaseNotConnected, err)
		if rows2 != nil {
			defer rows2.Close()
			if rows2.Err() != nil {
				t.Errorf("rows error: %v", rows2.Err())
			}
		}
		_, err = service.BeginTx(ctx, nil)
		require.Error(t, err)
		require.Equal(t, ErrDatabaseNotConnected, err)
		_, err = service.Begin()
		require.Error(t, err)
		require.Equal(t, ErrDatabaseNotConnected, err)
	})

	t.Run("InvalidDriver", func(t *testing.T) {
		config := ConnectionConfig{
			Driver: "invalid_driver",
			DSN:    "test://localhost",
		}

		service, err := NewDatabaseService(config)
		require.NoError(t, err) // Service creation should succeed
		assert.NotNil(t, service)

		// But connection should fail
		err = service.Connect()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown driver")
	})
}

// Test connection management with actual SQLite connections
func TestModule_ConnectionManagement_SQLite(t *testing.T) {
	module := NewModule()
	app := NewMockApplication()

	// Set up configuration with multiple SQLite in-memory connections
	config := &Config{
		Default: "primary",
		Connections: map[string]*ConnectionConfig{
			"primary": &ConnectionConfig{
				Driver: "sqlite",
				DSN:    ":memory:",
			},
			"secondary": &ConnectionConfig{
				Driver: "sqlite",
				DSN:    ":memory:",
			},
		},
	}

	// First register the default config, then override with test config
	err := module.RegisterConfig(app)
	require.NoError(t, err)

	// Now override with our test config
	app.RegisterConfigSection("database", &MockConfigProvider{config: config})

	err = module.Init(app)
	require.NoError(t, err)

	t.Run("GetConnection", func(t *testing.T) {
		conn, exists := module.GetConnection("primary")
		assert.True(t, exists)
		assert.NotNil(t, conn)

		conn, exists = module.GetConnection("nonexistent")
		assert.False(t, exists)
		assert.Nil(t, conn)
	})

	t.Run("GetDefaultConnection", func(t *testing.T) {
		defaultConn := module.GetDefaultConnection()
		assert.NotNil(t, defaultConn)
	})

	t.Run("GetConnections", func(t *testing.T) {
		connections := module.GetConnections()
		assert.Len(t, connections, 2)
		assert.Contains(t, connections, "primary")
		assert.Contains(t, connections, "secondary")
	})

	// Clean up
	if stopErr := module.Stop(context.Background()); stopErr != nil {
		t.Logf("Failed to stop module: %v", stopErr)
	}
}

func TestModule_ConfigErrors(t *testing.T) {
	t.Run("MissingConfigSection", func(t *testing.T) {
		module := NewModule()
		app := NewMockApplication()

		// Don't register config section
		err := module.Init(app)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get config section")
	})

	t.Run("InvalidConnectionConfig", func(t *testing.T) {
		module := NewModule()
		app := NewMockApplication()

		// Register config with invalid driver
		config := &Config{
			Connections: map[string]*ConnectionConfig{
				"invalid": &ConnectionConfig{
					Driver: "nonexistent_driver",
					DSN:    "invalid://dsn",
				},
			},
			Default: "invalid",
		}

		// First register the default config, then override with test config
		err := module.RegisterConfig(app)
		require.NoError(t, err)

		// Now override with our test config
		app.RegisterConfigSection("database", &MockConfigProvider{config: config})

		err = module.Init(app)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to connect to database")
	})
}

// Benchmark tests
func BenchmarkDatabaseService_Connect(b *testing.B) {
	config := ConnectionConfig{
		Driver: "sqlite",
		DSN:    ":memory:",
	}

	for i := 0; i < b.N; i++ {
		service, err := NewDatabaseService(config)
		if err != nil {
			b.Skipf("Skipping benchmark - SQLite3 requires CGO: %v", err)
			return
		}

		err = service.Connect()
		if err != nil {
			b.Skipf("Skipping benchmark - SQLite3 requires CGO: %v", err)
			return
		}
		if closeErr := service.Close(); closeErr != nil {
			b.Logf("Failed to close service: %v", closeErr)
		}
	}
}

func BenchmarkDatabaseService_Query(b *testing.B) {
	config := ConnectionConfig{
		Driver: "sqlite",
		DSN:    ":memory:",
	}

	service, err := NewDatabaseService(config)
	if err != nil {
		b.Skipf("Skipping benchmark - SQLite3 requires CGO: %v", err)
		return
	}

	err = service.Connect()
	if err != nil {
		b.Skipf("Skipping benchmark - SQLite3 requires CGO: %v", err)
		return
	}
	defer func() {
		if closeErr := service.Close(); closeErr != nil {
			b.Logf("Failed to close service: %v", closeErr)
		}
	}()

	ctx := context.Background()
	db := service.DB()

	// Setup test table
	_, err = db.ExecContext(ctx, `
		CREATE TABLE bench_test (
			id INTEGER PRIMARY KEY,
			value TEXT
		)
	`)
	if err != nil {
		b.Fatal(err)
	}

	// Insert test data
	for i := 0; i < 100; i++ {
		_, err = db.ExecContext(ctx, "INSERT INTO bench_test (value) VALUES (?)", "test_value")
		if err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rows, err := db.QueryContext(ctx, "SELECT id, value FROM bench_test LIMIT 10")
		if err != nil {
			b.Fatal(err)
		}
		defer func() {
			if closeErr := rows.Close(); closeErr != nil {
				b.Logf("Failed to close rows: %v", closeErr)
			}
		}()

		for rows.Next() {
			var id int
			var value string
			err := rows.Scan(&id, &value)
			if err != nil {
				b.Fatal(err)
			}
		}

		if rows.Err() != nil {
			b.Fatal(rows.Err())
		}
	}
}
