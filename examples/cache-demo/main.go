package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/GoCodeAlone/modular"
	"github.com/GoCodeAlone/modular/feeders"
	"github.com/GoCodeAlone/modular/modules/cache"
	"github.com/GoCodeAlone/modular/modules/chimux"
	"github.com/GoCodeAlone/modular/modules/httpserver"
	"github.com/go-chi/chi/v5"
)

type AppConfig struct {
	Name string `yaml:"name" default:"Cache Demo"`
}

type CacheSetRequest struct {
	Value interface{} `json:"value"`
	TTL   int         `json:"ttl,omitempty"` // TTL in seconds
}

type CacheResponse struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
	Found bool        `json:"found"`
}

type CacheStatsResponse struct {
	Backend string `json:"backend"`
	Status  string `json:"status"`
}

// CacheProvider defines the interface we expect from the cache module
type CacheProvider interface {
	Get(ctx context.Context, key string) (interface{}, bool)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	GetMulti(ctx context.Context, keys []string) (map[string]interface{}, error)
	SetMulti(ctx context.Context, items map[string]interface{}, ttl time.Duration) error
	DeleteMulti(ctx context.Context, keys []string) error
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Set up configuration feeders
	modular.ConfigFeeders = []modular.Feeder{
		feeders.NewYamlFeeder("config.yaml"),
		feeders.NewEnvFeeder(),
	}

	// Create config provider
	appConfig := &AppConfig{}
	configProvider := modular.NewStdConfigProvider(appConfig)

	// Create application
	app := modular.NewStdApplication(configProvider, logger)

	// Register modules
	app.RegisterModule(cache.NewModule())
	app.RegisterModule(chimux.NewChiMuxModule())
	app.RegisterModule(httpserver.NewHTTPServerModule())

	// Register API routes module
	app.RegisterModule(NewCacheAPIModule())

	// Run the application
	if err := app.Run(); err != nil {
		logger.Error("Application error", "error", err)
		os.Exit(1)
	}
}

// CacheAPIModule provides HTTP routes for cache operations
type CacheAPIModule struct {
	router    chi.Router
	cache     CacheProvider
	logger    modular.Logger
}

func NewCacheAPIModule() modular.Module {
	return &CacheAPIModule{}
}

func (m *CacheAPIModule) Name() string {
	return "cache-api"
}

func (m *CacheAPIModule) Dependencies() []string {
	return []string{"cache", "chimux"}
}

func (m *CacheAPIModule) RegisterConfig(app modular.Application) error {
	// No additional config needed
	return nil
}

func (m *CacheAPIModule) Init(app modular.Application) error {
	m.logger = app.Logger()

	// Get cache service
	if err := app.GetService("cache.provider", &m.cache); err != nil {
		return fmt.Errorf("failed to get cache service: %w", err)
	}

	// Get router
	if err := app.GetService("chi.router", &m.router); err != nil {
		return fmt.Errorf("failed to get router service: %w", err)
	}

	m.setupRoutes()
	return nil
}

func (m *CacheAPIModule) setupRoutes() {
	// Add health endpoint
	m.router.Get("/health", m.handleHealth)
	
	m.router.Route("/api/cache", func(r chi.Router) {
		r.Post("/{key}", m.handleSetCache)
		r.Get("/{key}", m.handleGetCache)
		r.Delete("/{key}", m.handleDeleteCache)
		r.Delete("/", m.handleClearCache)
		r.Get("/stats", m.handleCacheStats)
	})
}

func (m *CacheAPIModule) handleSetCache(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if key == "" {
		http.Error(w, "Key parameter is required", http.StatusBadRequest)
		return
	}

	var req CacheSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Convert TTL from seconds to time.Duration
	var ttl time.Duration
	if req.TTL > 0 {
		ttl = time.Duration(req.TTL) * time.Second
	}

	// Set value in cache
	if err := m.cache.Set(r.Context(), key, req.Value, ttl); err != nil {
		m.logger.Error("Failed to set cache value", "key", key, "error", err)
		http.Error(w, "Failed to set cache value", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"key":     key,
		"ttl":     req.TTL,
		"message": "Value cached successfully",
	})
}

func (m *CacheAPIModule) handleGetCache(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if key == "" {
		http.Error(w, "Key parameter is required", http.StatusBadRequest)
		return
	}

	value, found := m.cache.Get(r.Context(), key)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(CacheResponse{
		Key:   key,
		Value: value,
		Found: found,
	})
}

func (m *CacheAPIModule) handleDeleteCache(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if key == "" {
		http.Error(w, "Key parameter is required", http.StatusBadRequest)
		return
	}

	if err := m.cache.Delete(r.Context(), key); err != nil {
		m.logger.Error("Failed to delete cache value", "key", key, "error", err)
		http.Error(w, "Failed to delete cache value", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"key":     key,
		"message": "Value deleted successfully",
	})
}

func (m *CacheAPIModule) handleClearCache(w http.ResponseWriter, r *http.Request) {
	// For the demo, we'll implement a simple clear by deleting known keys
	// In a real implementation, you might have a Clear() method
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Note: This demo doesn't implement clear all. Delete individual keys instead.",
	})
}

func (m *CacheAPIModule) handleCacheStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(CacheStatsResponse{
		Backend: "configured-backend",
		Status:  "active",
	})
}

// Advanced endpoint for batch operations
func (m *CacheAPIModule) handleBatchSet(w http.ResponseWriter, r *http.Request) {
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	ttlParam := r.URL.Query().Get("ttl")
	var ttl time.Duration
	if ttlParam != "" {
		if ttlSeconds, err := strconv.Atoi(ttlParam); err == nil {
			ttl = time.Duration(ttlSeconds) * time.Second
		}
	}

	if err := m.cache.SetMulti(r.Context(), req, ttl); err != nil {
		m.logger.Error("Failed to set multiple cache values", "error", err)
		http.Error(w, "Failed to set cache values", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"count":   len(req),
		"message": "Values cached successfully",
	})
}

func (m *CacheAPIModule) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok","service":"cache"}`))
}

func (m *CacheAPIModule) Start(ctx context.Context) error {
	m.logger.Info("Cache API module started")
	return nil
}

func (m *CacheAPIModule) Stop(ctx context.Context) error {
	m.logger.Info("Cache API module stopped")
	return nil
}

func (m *CacheAPIModule) ProvidesServices() []modular.ServiceProvider {
	return []modular.ServiceProvider{}
}

func (m *CacheAPIModule) RequiresServices() []modular.ServiceDependency {
	return []modular.ServiceDependency{
		{Name: "cache.provider", Required: true},
		{Name: "chi.router", Required: true},
	}
}