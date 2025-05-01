package modular

import (
	"context"
	"reflect"
	"testing"
)

// TestComplexDependencies tests a more comprehensive scenario with both explicit and
// interface-based service dependencies in a complex dependency graph.
func TestComplexDependencies(t *testing.T) {
	// Create a simple logger for testing
	testLogger := &logger{t}

	// Setup test application
	app := &StdApplication{
		cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
		cfgSections:    make(map[string]ConfigProvider),
		svcRegistry:    make(ServiceRegistry),
		moduleRegistry: make(ModuleRegistry),
		logger:         testLogger,
	}

	// Create and register modules (in random order to ensure dependency resolution works)
	apiModule := &APIModule{name: "api-module", t: t}
	databaseModule := &DatabaseModule{name: "database-module", t: t}
	cacheModule := &CacheModule{name: "cache-module", t: t}
	loggerModule := &LoggerModule{name: "logger-module", t: t}
	authModule := &AuthModule{name: "auth-module", t: t}

	// Register modules in an order that would cause issues if dependency resolution didn't work
	app.RegisterModule(apiModule)      // Depends on cache and database services
	app.RegisterModule(authModule)     // Depends on logger service and database module
	app.RegisterModule(loggerModule)   // No dependencies
	app.RegisterModule(cacheModule)    // Explicitly depends on database module
	app.RegisterModule(databaseModule) // No dependencies

	// Resolve dependencies
	order, err := app.resolveDependencies()
	if err != nil {
		t.Fatalf("Failed to resolve dependencies: %v", err)
	}

	// Verify the resulting order meets all dependency constraints
	// This is complex with multiple valid orders, so we'll verify constraints rather than an exact order
	validateDependencyConstraints(t, order, map[string][]string{
		"api-module":      {"cache-module", "database-module"},  // API depends on both cache and database
		"cache-module":    {"database-module"},                  // Cache explicitly depends on database
		"auth-module":     {"database-module", "logger-module"}, // Auth depends on database and logger
		"logger-module":   {},                                   // Logger has no dependencies
		"database-module": {},                                   // Database has no dependencies
	})

	// Now run the full initialization to make sure modules actually initialize without panicking
	err = app.Init()
	if err != nil {
		t.Fatalf("App initialization failed: %v", err)
	}

	// Verify that services were correctly injected
	if !apiModule.cacheServiceInjected {
		t.Error("Cache service was not injected into API module")
	}
	if !apiModule.databaseServiceInjected {
		t.Error("Database service was not injected into API module")
	}
	if !authModule.loggerServiceInjected {
		t.Error("Logger service was not injected into Auth module")
	}
}

// Helper function to validate that dependency constraints are satisfied
// For each module in 'order', ensure all its dependencies appear before it
func validateDependencyConstraints(t *testing.T, order []string, constraints map[string][]string) {
	// Build a map of module positions in the order
	positions := make(map[string]int)
	for i, module := range order {
		positions[module] = i
	}

	// Check that all modules are present
	for module := range constraints {
		if _, found := positions[module]; !found {
			t.Errorf("Module %s is missing from initialization order", module)
		}
	}

	// For each module, check that all its dependencies come before it
	for module, deps := range constraints {
		modulePos, ok := positions[module]
		if !ok {
			continue // Already reported above
		}

		for _, dep := range deps {
			depPos, ok := positions[dep]
			if !ok {
				t.Errorf("Dependency %s of module %s is missing from initialization order", dep, module)
				continue
			}

			if depPos >= modulePos {
				t.Errorf("Module %s is initialized before its dependency %s", module, dep)
			}
		}
	}
}

// Service interfaces for testing

type DatabaseService interface {
	Query(query string) string
}

type CacheService interface {
	Get(key string) string
	Set(key, value string)
}

type LoggingService interface {
	Log(message string)
}

// Simple implementation of the DatabaseService
type SimpleDatabaseService struct{}

func (s *SimpleDatabaseService) Query(query string) string {
	return "database-result-for-" + query
}

// Simple implementation of the CacheService
type SimpleCacheService struct {
	database DatabaseService
	cache    map[string]string
}

func (s *SimpleCacheService) Get(key string) string {
	if value, ok := s.cache[key]; ok {
		return value
	}
	// Fall back to database
	result := s.database.Query(key)
	s.cache[key] = result
	return result
}

func (s *SimpleCacheService) Set(key, value string) {
	s.cache[key] = value
}

// Simple implementation of the LoggingService
type SimpleLoggerService struct {
	logs []string
}

func (s *SimpleLoggerService) Log(message string) {
	s.logs = append(s.logs, message)
}

// Module implementations

// DatabaseModule provides a DatabaseService with no dependencies
type DatabaseModule struct {
	name string
	t    *testing.T
}

func (m *DatabaseModule) Name() string {
	return m.name
}

func (m *DatabaseModule) Init(app Application) error {
	m.t.Logf("Initializing DatabaseModule")
	return nil
}

func (m *DatabaseModule) Dependencies() []string {
	return nil // No dependencies
}

func (m *DatabaseModule) ProvidesServices() []ServiceProvider {
	return []ServiceProvider{
		{
			Name:        "database",
			Description: "Database service for testing",
			Instance:    &SimpleDatabaseService{},
		},
	}
}

func (m *DatabaseModule) RequiresServices() []ServiceDependency {
	return nil // No required services
}

// CacheModule provides a CacheService and explicitly depends on DatabaseModule
type CacheModule struct {
	name             string
	databaseService  DatabaseService
	databaseInjected bool
	t                *testing.T
}

func (m *CacheModule) Name() string {
	return m.name
}

func (m *CacheModule) Init(app Application) error {
	m.t.Logf("Initializing CacheModule")
	// Verify that dependencies are satisfied
	if !m.databaseInjected {
		m.t.Error("CacheModule initialized before database service was injected")
	}
	return nil
}

func (m *CacheModule) Dependencies() []string {
	return []string{"database-module"} // Explicitly depends on database module
}

func (m *CacheModule) ProvidesServices() []ServiceProvider {
	// Create and return the cache service, which uses the injected database service
	cacheService := &SimpleCacheService{
		database: m.databaseService,
		cache:    make(map[string]string),
	}
	return []ServiceProvider{
		{
			Name:        "cache",
			Description: "Cache service for testing",
			Instance:    cacheService,
		},
	}
}

func (m *CacheModule) RequiresServices() []ServiceDependency {
	return []ServiceDependency{
		{
			Name:     "database",
			Required: true,
		},
	}
}

func (m *CacheModule) Constructor() ModuleConstructor {
	return func(app Application, services map[string]any) (Module, error) {
		if dbService, ok := services["database"].(DatabaseService); ok {
			m.databaseService = dbService
			m.databaseInjected = true
		}
		return m, nil
	}
}

// APIModule requires both CacheService and DatabaseService through interfaces
type APIModule struct {
	name                    string
	cacheService            CacheService
	databaseService         DatabaseService
	cacheServiceInjected    bool
	databaseServiceInjected bool
	t                       *testing.T
}

func (m *APIModule) Name() string {
	return m.name
}

func (m *APIModule) Init(app Application) error {
	m.t.Logf("Initializing APIModule")
	// Verify that dependencies are satisfied
	if !m.cacheServiceInjected {
		m.t.Error("APIModule initialized before cache service was injected")
	}
	if !m.databaseServiceInjected {
		m.t.Error("APIModule initialized before database service was injected")
	}
	return nil
}

func (m *APIModule) Dependencies() []string {
	return nil // No explicit module dependencies, only service dependencies
}

func (m *APIModule) ProvidesServices() []ServiceProvider {
	return nil // Doesn't provide any services
}

func (m *APIModule) RequiresServices() []ServiceDependency {
	return []ServiceDependency{
		{
			Name:               "cache",
			Required:           true,
			MatchByInterface:   true,
			SatisfiesInterface: reflect.TypeOf((*CacheService)(nil)).Elem(),
		},
		{
			Name:               "database",
			Required:           true,
			MatchByInterface:   true,
			SatisfiesInterface: reflect.TypeOf((*DatabaseService)(nil)).Elem(),
		},
	}
}

func (m *APIModule) Constructor() ModuleConstructor {
	return func(app Application, services map[string]any) (Module, error) {
		if cacheService, ok := services["cache"].(CacheService); ok {
			m.cacheService = cacheService
			m.cacheServiceInjected = true
		}
		if dbService, ok := services["database"].(DatabaseService); ok {
			m.databaseService = dbService
			m.databaseServiceInjected = true
		}
		return m, nil
	}
}

func (m *APIModule) Start(ctx context.Context) error {
	return nil
}

func (m *APIModule) Stop(ctx context.Context) error {
	return nil
}

// LoggerModule provides a LoggingService with no dependencies
type LoggerModule struct {
	name string
	t    *testing.T
}

func (m *LoggerModule) Name() string {
	return m.name
}

func (m *LoggerModule) Init(app Application) error {
	m.t.Logf("Initializing LoggerModule")
	return nil
}

func (m *LoggerModule) Dependencies() []string {
	return nil // No dependencies
}

func (m *LoggerModule) ProvidesServices() []ServiceProvider {
	return []ServiceProvider{
		{
			Name:        "logger-service",
			Description: "Logging service for testing",
			Instance:    &SimpleLoggerService{logs: make([]string, 0)},
		},
	}
}

func (m *LoggerModule) RequiresServices() []ServiceDependency {
	return nil
}

// AuthModule requires a LoggingService and explicitly depends on DatabaseModule
type AuthModule struct {
	name                  string
	loggerService         LoggingService
	loggerServiceInjected bool
	t                     *testing.T
}

func (m *AuthModule) Name() string {
	return m.name
}

func (m *AuthModule) Init(app Application) error {
	m.t.Logf("Initializing AuthModule")
	// Verify that dependencies are satisfied
	if !m.loggerServiceInjected {
		m.t.Error("AuthModule initialized before logger service was injected")
	}
	return nil
}

func (m *AuthModule) Dependencies() []string {
	return []string{"database-module"} // Explicitly depends on database module
}

func (m *AuthModule) ProvidesServices() []ServiceProvider {
	return nil // Doesn't provide any services
}

func (m *AuthModule) RequiresServices() []ServiceDependency {
	return []ServiceDependency{
		{
			Name:               "logger-service",
			Required:           true,
			MatchByInterface:   true,
			SatisfiesInterface: reflect.TypeOf((*LoggingService)(nil)).Elem(),
		},
	}
}

func (m *AuthModule) Constructor() ModuleConstructor {
	return func(app Application, services map[string]any) (Module, error) {
		if loggerService, ok := services["logger-service"].(LoggingService); ok {
			m.loggerService = loggerService
			m.loggerServiceInjected = true
		}
		return m, nil
	}
}

func (m *AuthModule) Start(ctx context.Context) error {
	return nil
}

func (m *AuthModule) Stop(ctx context.Context) error {
	return nil
}
