package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/GoCodeAlone/modular"
	_ "github.com/lib/pq"
)

// testLogger implements modular.Logger for this example
type testLogger struct{}

// logKV formats key-value pairs while redacting any sensitive header/content values.
// Keys that are considered sensitive: authorization, cookie, set-cookie, x-api-key, api-key, password, secret, token.
func (l *testLogger) logKV(prefix, msg string, args ...any) {
	sanitized := make([]any, 0, len(args))
	for i := 0; i < len(args); i += 2 {
		// If uneven args, just append remaining raw.
		if i+1 >= len(args) {
			sanitized = append(sanitized, args[i])
			break
		}
		k, v := args[i], args[i+1]
		keyStr, ok := k.(string)
		if !ok {
			sanitized = append(sanitized, k, v)
			continue
		}
		lower := strings.ToLower(keyStr)
		if lower == "authorization" || lower == "cookie" || lower == "set-cookie" || lower == "x-api-key" || strings.Contains(lower, "password") || strings.Contains(lower, "secret") || strings.HasSuffix(lower, "token") || lower == "api-key" {
			// Redact value preserving length for debugging.
			if s, ok := v.(string); ok && s != "" {
				v = fmt.Sprintf("[REDACTED len=%d]", len(s))
			} else if v != nil {
				v = "[REDACTED]"
			}
		}
		sanitized = append(sanitized, keyStr, v)
	}
	log.Printf("%s%s %v", prefix, msg, sanitized)
}

func (l *testLogger) Debug(msg string, args ...any) { l.logKV("[DEBUG] ", msg, args...) }
func (l *testLogger) Info(msg string, args ...any)  { l.logKV("[INFO] ", msg, args...) }
func (l *testLogger) Warn(msg string, args ...any)  { l.logKV("[WARN] ", msg, args...) }
func (l *testLogger) Error(msg string, args ...any) { l.logKV("[ERROR] ", msg, args...) }

// AppConfig represents the application configuration with dynamic reload support
type AppConfig struct {
	Server   ServerConfig   `json:"server"`
	Database DatabaseConfig `json:"database"`
	Cache    CacheConfig    `json:"cache"`
	Features FeatureFlags   `json:"features"`
}

type ServerConfig struct {
	Port            int           `json:"port" env:"SERVER_PORT" default:"8080"`
	ReadTimeout     time.Duration `json:"read_timeout" env:"READ_TIMEOUT" default:"10s" dynamic:"true"`
	WriteTimeout    time.Duration `json:"write_timeout" env:"WRITE_TIMEOUT" default:"10s" dynamic:"true"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout" env:"SHUTDOWN_TIMEOUT" default:"30s"`
}

type DatabaseConfig struct {
	Host            string        `json:"host" env:"DB_HOST" default:"localhost"`
	Port            int           `json:"port" env:"DB_PORT" default:"5432"`
	Database        string        `json:"database" env:"DB_NAME" default:"myapp"`
	MaxConnections  int           `json:"max_connections" env:"DB_MAX_CONNS" default:"25" dynamic:"true"`
	MaxIdleConns    int           `json:"max_idle_conns" env:"DB_MAX_IDLE" default:"5" dynamic:"true"`
	ConnMaxLifetime time.Duration `json:"conn_max_lifetime" env:"DB_CONN_LIFETIME" default:"1h" dynamic:"true"`
}

type CacheConfig struct {
	Enabled     bool          `json:"enabled" env:"CACHE_ENABLED" default:"true" dynamic:"true"`
	TTL         time.Duration `json:"ttl" env:"CACHE_TTL" default:"5m" dynamic:"true"`
	MaxEntries  int           `json:"max_entries" env:"CACHE_MAX_ENTRIES" default:"1000" dynamic:"true"`
	CleanupTime time.Duration `json:"cleanup_time" env:"CACHE_CLEANUP" default:"10m" dynamic:"true"`
}

type FeatureFlags struct {
	MaintenanceMode  bool   `json:"maintenance_mode" env:"MAINTENANCE_MODE" default:"false" dynamic:"true"`
	RateLimitEnabled bool   `json:"rate_limit_enabled" env:"RATE_LIMIT_ENABLED" default:"true" dynamic:"true"`
	LogLevel         string `json:"log_level" env:"LOG_LEVEL" default:"info" dynamic:"true"`
	DebugEndpoints   bool   `json:"debug_endpoints" env:"DEBUG_ENDPOINTS" default:"false" dynamic:"true"`
}

// DatabaseModule manages database connections with health checking
type DatabaseModule struct {
	config *DatabaseConfig
	db     *sql.DB
	app    modular.Application
}

func NewDatabaseModule(config *DatabaseConfig) *DatabaseModule {
	return &DatabaseModule{
		config: config,
	}
}

func (m *DatabaseModule) Name() string {
	return "database"
}

func (m *DatabaseModule) Init(app modular.Application) error {
	m.app = app

	// Initialize database connection
	dsn := fmt.Sprintf("host=%s port=%d dbname=%s sslmode=disable",
		m.config.Host, m.config.Port, m.config.Database)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Apply initial configuration
	db.SetMaxOpenConns(m.config.MaxConnections)
	db.SetMaxIdleConns(m.config.MaxIdleConns)
	db.SetConnMaxLifetime(m.config.ConnMaxLifetime)

	m.db = db

	// Register as health provider (required component)
	if err := app.RegisterHealthProvider("database", m, false); err != nil {
		return fmt.Errorf("failed to register database health provider: %w", err)
	}
	return nil
}

func (m *DatabaseModule) Start(ctx context.Context) error {
	// Verify database connection
	if err := m.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}
	return nil
}

func (m *DatabaseModule) Stop(ctx context.Context) error {
	if err := m.db.Close(); err != nil {
		return fmt.Errorf("database close failed: %w", err)
	}
	return nil
}

// HealthCheck implements the HealthProvider interface
func (m *DatabaseModule) HealthCheck(ctx context.Context) ([]modular.HealthReport, error) {
	reports := []modular.HealthReport{}

	// Check basic connectivity
	startTime := time.Now()
	err := m.db.PingContext(ctx)
	latency := time.Since(startTime)

	if err != nil {
		reports = append(reports, modular.HealthReport{
			Module:        "database",
			Component:     "connectivity",
			Status:        modular.HealthStatusUnhealthy,
			Message:       fmt.Sprintf("Database unreachable: %v", err),
			CheckedAt:     time.Now(),
			ObservedSince: time.Now(),
		})
		return reports, nil
	}

	// Check connection pool health
	stats := m.db.Stats()
	poolUtilization := float64(stats.InUse) / float64(m.config.MaxConnections) * 100

	poolStatus := modular.HealthStatusHealthy
	poolMessage := "Connection pool healthy"

	if poolUtilization > 90 {
		poolStatus = modular.HealthStatusDegraded
		poolMessage = fmt.Sprintf("High connection pool utilization: %.1f%%", poolUtilization)
	} else if poolUtilization > 95 {
		poolStatus = modular.HealthStatusUnhealthy
		poolMessage = fmt.Sprintf("Critical connection pool utilization: %.1f%%", poolUtilization)
	}

	reports = append(reports,
		modular.HealthReport{
			Module:        "database",
			Component:     "connectivity",
			Status:        modular.HealthStatusHealthy,
			Message:       fmt.Sprintf("Database reachable (latency: %v)", latency),
			CheckedAt:     time.Now(),
			ObservedSince: time.Now(),
			Details: map[string]any{
				"latency_ms": latency.Milliseconds(),
			},
		},
		modular.HealthReport{
			Module:        "database",
			Component:     "connection_pool",
			Status:        poolStatus,
			Message:       poolMessage,
			CheckedAt:     time.Now(),
			ObservedSince: time.Now(),
			Details: map[string]any{
				"max_connections":   m.config.MaxConnections,
				"connections_open":  stats.OpenConnections,
				"connections_idle":  stats.Idle,
				"connections_inuse": stats.InUse,
				"utilization_pct":   poolUtilization,
			},
		},
	)

	return reports, nil
}

// Reload implements the Reloadable interface for dynamic configuration updates
func (m *DatabaseModule) CanReload() bool {
	return true
}

func (m *DatabaseModule) ReloadTimeout() time.Duration {
	return 5 * time.Second
}

func (m *DatabaseModule) Reload(ctx context.Context, changes []modular.ConfigChange) error {
	for _, change := range changes {
		switch change.FieldPath {
		case "database.max_connections":
			if val, ok := change.NewValue.(int); ok {
				m.db.SetMaxOpenConns(val)
				if m.app != nil && m.app.Logger() != nil {
					m.app.Logger().Info("updated database max connections to %d", val)
				}
			}
		case "database.max_idle_conns":
			if val, ok := change.NewValue.(int); ok {
				m.db.SetMaxIdleConns(val)
				if m.app != nil && m.app.Logger() != nil {
					m.app.Logger().Info("updated database max idle connections to %d", val)
				}
			}
		case "database.conn_max_lifetime":
			if val, ok := change.NewValue.(time.Duration); ok {
				m.db.SetConnMaxLifetime(val)
				if m.app != nil && m.app.Logger() != nil {
					m.app.Logger().Info("updated database connection lifetime to %v", val)
				}
			}
		}
	}
	return nil
}

// CacheModule provides caching with health monitoring
type CacheModule struct {
	config  *CacheConfig
	enabled bool
	entries map[string]cacheEntry
	app     modular.Application
}

type cacheEntry struct {
	value      interface{} //nolint:unused // placeholder for future cache implementation
	expiration time.Time
}

func NewCacheModule(config *CacheConfig) *CacheModule {
	return &CacheModule{
		config:  config,
		enabled: config.Enabled,
		entries: make(map[string]cacheEntry),
	}
}

func (m *CacheModule) Name() string {
	return "cache"
}

func (m *CacheModule) Init(app modular.Application) error {
	m.app = app
	// Register as optional health provider
	if err := app.RegisterHealthProvider("cache", m, true); err != nil {
		return fmt.Errorf("failed to register cache health provider: %w", err)
	}
	return nil
}

func (m *CacheModule) Start(ctx context.Context) error {
	if m.config.Enabled {
		// Start cleanup goroutine
		go m.cleanupLoop(ctx)
	}
	return nil
}

func (m *CacheModule) Stop(ctx context.Context) error {
	return nil
}

func (m *CacheModule) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(m.config.CleanupTime)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.cleanup()
		}
	}
}

func (m *CacheModule) cleanup() {
	now := time.Now()
	for key, entry := range m.entries {
		if entry.expiration.Before(now) {
			delete(m.entries, key)
		}
	}
}

// HealthCheck implements the HealthProvider interface
func (m *CacheModule) HealthCheck(ctx context.Context) ([]modular.HealthReport, error) {
	if !m.enabled {
		return []modular.HealthReport{{
			Module:    "cache",
			Status:    modular.HealthStatusHealthy,
			Message:   "Cache disabled",
			CheckedAt: time.Now(),
			Optional:  true,
		}}, nil
	}

	entryCount := len(m.entries)
	utilization := float64(entryCount) / float64(m.config.MaxEntries) * 100

	status := modular.HealthStatusHealthy
	message := fmt.Sprintf("Cache operational (%d entries)", entryCount)

	if utilization > 80 {
		status = modular.HealthStatusDegraded
		message = fmt.Sprintf("Cache near capacity: %.1f%% utilized", utilization)
	} else if utilization > 95 {
		status = modular.HealthStatusUnhealthy
		message = fmt.Sprintf("Cache at capacity: %.1f%% utilized", utilization)
	}

	return []modular.HealthReport{{
		Module:    "cache",
		Status:    status,
		Message:   message,
		CheckedAt: time.Now(),
		Optional:  true,
		Details: map[string]any{
			"entries":         entryCount,
			"max_entries":     m.config.MaxEntries,
			"utilization_pct": utilization,
			"ttl":             m.config.TTL.String(),
		},
	}}, nil
}

// Reload implements the Reloadable interface
func (m *CacheModule) CanReload() bool {
	return true
}

func (m *CacheModule) ReloadTimeout() time.Duration {
	return 2 * time.Second
}

func (m *CacheModule) Reload(ctx context.Context, changes []modular.ConfigChange) error {
	for _, change := range changes {
		switch change.FieldPath {
		case "cache.enabled":
			if val, ok := change.NewValue.(bool); ok {
				m.enabled = val
				if m.app != nil && m.app.Logger() != nil {
					m.app.Logger().Info("cache enabled changed to %v", val)
				}
			}
		case "cache.ttl":
			if val, ok := change.NewValue.(time.Duration); ok {
				m.config.TTL = val
				if m.app != nil && m.app.Logger() != nil {
					m.app.Logger().Info("updated cache ttl to %v", val)
				}
			}
		case "cache.max_entries":
			if val, ok := change.NewValue.(int); ok {
				m.config.MaxEntries = val
				if m.app != nil && m.app.Logger() != nil {
					m.app.Logger().Info("updated cache max entries to %d", val)
				}
			}
		}
	}
	return nil
}

// HTTPServer provides the web interface with health endpoints
type HTTPServer struct {
	config *ServerConfig
	app    modular.Application
	server *http.Server
	mux    *http.ServeMux
}

func NewHTTPServer(config *ServerConfig, app modular.Application) *HTTPServer {
	return &HTTPServer{
		config: config,
		app:    app,
		mux:    http.NewServeMux(),
	}
}

func (s *HTTPServer) Name() string {
	return "httpserver"
}

func (s *HTTPServer) Init(app modular.Application) error {
	s.app = app

	// Setup routes
	s.mux.HandleFunc("/health", s.healthHandler)
	s.mux.HandleFunc("/ready", s.readinessHandler)
	s.mux.HandleFunc("/alive", s.livenessHandler)
	s.mux.HandleFunc("/reload", s.reloadHandler)
	s.mux.HandleFunc("/config", s.configHandler)

	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.Port),
		Handler:      s.mux,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
	}

	return nil
}

func (s *HTTPServer) Start(ctx context.Context) error {
	go func() {
		if s.app != nil && s.app.Logger() != nil {
			s.app.Logger().Info("http server starting on port %d", s.config.Port)
		}
		if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			if s.app != nil && s.app.Logger() != nil {
				s.app.Logger().Error("http server error: %v", err)
			}
		}
	}()
	return nil
}

func (s *HTTPServer) Stop(ctx context.Context) error {
	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("HTTP server shutdown failed: %w", err)
	}
	return nil
}

func (s *HTTPServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	health, err := s.app.Health()
	if err != nil {
		http.Error(w, "Health service unavailable", http.StatusServiceUnavailable)
		return
	}

	aggregated, err := health.Collect(r.Context())
	if err != nil {
		http.Error(w, "Failed to collect health", http.StatusInternalServerError)
		return
	}

	// Set appropriate status code
	statusCode := http.StatusOK
	if aggregated.Health == modular.HealthStatusUnhealthy {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(aggregated); err != nil {
		if s.app != nil && s.app.Logger() != nil {
			s.app.Logger().Error("failed to encode health response: %v", err)
		}
	}
}

func (s *HTTPServer) readinessHandler(w http.ResponseWriter, r *http.Request) {
	health, err := s.app.Health()
	if err != nil {
		http.Error(w, "Health service unavailable", http.StatusServiceUnavailable)
		return
	}

	aggregated, err := health.Collect(r.Context())
	if err != nil {
		http.Error(w, "Failed to collect health", http.StatusInternalServerError)
		return
	}

	ready := aggregated.Readiness == modular.HealthStatusHealthy
	response := map[string]interface{}{
		"ready":     ready,
		"status":    aggregated.Readiness.String(),
		"timestamp": time.Now(),
	}

	statusCode := http.StatusOK
	if !ready {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		if s.app != nil && s.app.Logger() != nil {
			s.app.Logger().Error("failed to encode readiness response: %v", err)
		}
	}
}

func (s *HTTPServer) livenessHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"alive":     true,
		"timestamp": time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		if s.app != nil && s.app.Logger() != nil {
			s.app.Logger().Error("failed to encode liveness response: %v", err)
		}
	}
}

func (s *HTTPServer) reloadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := s.app.RequestReload()
	if err != nil {
		http.Error(w, fmt.Sprintf("Reload failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Configuration reload initiated",
	}); err != nil {
		if s.app != nil && s.app.Logger() != nil {
			s.app.Logger().Error("failed to encode reload response: %v", err)
		}
	}
}

func (s *HTTPServer) configHandler(w http.ResponseWriter, r *http.Request) {
	// This would normally return the current configuration
	// For demo purposes, return a simple status
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"server": map[string]interface{}{
			"port":          s.config.Port,
			"read_timeout":  s.config.ReadTimeout.String(),
			"write_timeout": s.config.WriteTimeout.String(),
		},
	}); err != nil {
		if s.app != nil && s.app.Logger() != nil {
			s.app.Logger().Error("failed to encode config response: %v", err)
		}
	}
}

func main() {
	// Load configuration
	config := &AppConfig{}
	// In a real app, load from file/env

	// Create application with dynamic reload and health aggregation
	configProvider := modular.NewStdConfigProvider(config)
	logger := &testLogger{}

	app := modular.NewStdApplicationWithOptions(
		configProvider,
		logger,
		modular.WithDynamicReload(
			modular.DynamicReloadConfig{
				Enabled:       true,
				ReloadTimeout: 10 * time.Second,
			},
		),
		modular.WithHealthAggregator(
			modular.HealthAggregatorConfig{
				Enabled:       true,
				CheckInterval: 30 * time.Second,
				CheckTimeout:  200 * time.Millisecond,
			},
		),
	)

	// Create and register modules
	dbModule := NewDatabaseModule(&config.Database)
	cacheModule := NewCacheModule(&config.Cache)
	httpServer := NewHTTPServer(&config.Server, app)

	app.RegisterModule(dbModule)
	app.RegisterModule(cacheModule)
	app.RegisterModule(httpServer)

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		if app.Logger() != nil {
			app.Logger().Info("shutting down...")
		}
		_, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := app.Stop(); err != nil {
			if app.Logger() != nil {
				app.Logger().Error("error during shutdown: %v", err)
			}
		}
	}()

	// Start the application
	if app.Logger() != nil {
		app.Logger().Info("starting Dynamic Health Application...")
	}
	if err := app.Run(); err != nil {
		if app.Logger() != nil {
			app.Logger().Error("application failed: %v", err)
		}
		os.Exit(1)
	}
}
