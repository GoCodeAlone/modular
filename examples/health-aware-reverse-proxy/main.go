package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/GoCodeAlone/modular"
	"github.com/GoCodeAlone/modular/feeders"
	"github.com/GoCodeAlone/modular/modules/chimux"
	"github.com/GoCodeAlone/modular/modules/httpserver"
	"github.com/GoCodeAlone/modular/modules/reverseproxy"
)

type AppConfig struct {
	// Empty config struct for the reverse proxy example
	// Configuration is handled by individual modules
}

func main() {
	// Start mock backend servers
	startMockBackends()

	// Create a new application and configure feeders per instance
	app := modular.NewStdApplication(
		modular.NewStdConfigProvider(&AppConfig{}),
		slog.New(slog.NewTextHandler(
			os.Stdout,
			&slog.HandlerOptions{Level: slog.LevelDebug},
		)),
	)
	if stdApp, ok := app.(*modular.StdApplication); ok {
		stdApp.SetConfigFeeders([]modular.Feeder{
			feeders.NewYamlFeeder("config.yaml"),
			feeders.NewEnvFeeder(),
		})
	}

	// Register the modules in dependency order
	app.RegisterModule(chimux.NewChiMuxModule())
	app.RegisterModule(&HealthModule{}) // Custom module to register health endpoint
	app.RegisterModule(reverseproxy.NewModule())
	app.RegisterModule(httpserver.NewHTTPServerModule())

	// Run application with lifecycle management
	if err := app.Run(); err != nil {
		app.Logger().Error("Application error", "error", err)
		os.Exit(1)
	}
}

// startMockBackends starts mock backend servers on different ports
func startMockBackends() {
	// Healthy API backend (port 9001)
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"backend":"healthy-api","path":"%s","method":"%s","timestamp":"%s"}`,
				r.URL.Path, r.Method, time.Now().Format(time.RFC3339))
		})
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"status":"healthy","service":"healthy-api","timestamp":"%s"}`,
				time.Now().Format(time.RFC3339))
		})
		fmt.Println("Starting healthy-api backend on :9001")
		if err := http.ListenAndServe(":9001", mux); err != nil { //nolint:gosec
			fmt.Printf("Backend server error on %s: %v\n", ":9001", err)
		} //nolint:gosec
	}()

	// Intermittent backend that sometimes fails (port 9002)
	go func() {
		mux := http.NewServeMux()
		requestCount := 0
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			requestCount++
			// Fail every 3rd request to trigger circuit breaker
			if requestCount%3 == 0 {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, `{"error":"simulated failure","backend":"intermittent-api","request":%d}`, requestCount)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"backend":"intermittent-api","path":"%s","method":"%s","request":%d}`,
				r.URL.Path, r.Method, requestCount)
		})
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			// Health endpoint is always available
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"status":"healthy","service":"intermittent-api","requests":%d}`, requestCount)
		})
		fmt.Println("Starting intermittent-api backend on :9002")
		if err := http.ListenAndServe(":9002", mux); err != nil { //nolint:gosec
			fmt.Printf("Backend server error on %s: %v\n", ":9002", err)
		} //nolint:gosec
	}()

	// Slow backend (port 9003)
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// Add delay to simulate slow backend
			time.Sleep(2 * time.Second)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"backend":"slow-api","path":"%s","method":"%s","delay":"2s"}`,
				r.URL.Path, r.Method)
		})
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			// Health check without delay
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"status":"healthy","service":"slow-api"}`)
		})
		fmt.Println("Starting slow-api backend on :9003")
		if err := http.ListenAndServe(":9003", mux); err != nil { //nolint:gosec
			fmt.Printf("Backend server error on %s: %v\n", ":9003", err)
		} //nolint:gosec
	}()

	// Unreachable backend simulation - we won't start this one
	// This will demonstrate DNS/connection failures
	fmt.Println("Unreachable backend (unreachable-api) will not be started - simulating unreachable service")
}

// HealthModule provides a simple application health endpoint
type HealthModule struct {
	app modular.Application
}

// Name implements modular.Module
func (h *HealthModule) Name() string {
	return "health"
}

// RegisterConfig implements modular.Configurable
func (h *HealthModule) RegisterConfig(app modular.Application) error {
	// No configuration needed for this simple module
	return nil
}

// Constructor implements modular.ModuleConstructor
func (h *HealthModule) Constructor() modular.ModuleConstructor {
	return func(app modular.Application, services map[string]any) (modular.Module, error) {
		return &HealthModule{
			app: app,
		}, nil
	}
}

// Init implements modular.Module
func (h *HealthModule) Init(app modular.Application) error {
	h.app = app
	return nil
}

// Start implements modular.Startable
func (h *HealthModule) Start(ctx context.Context) error {
	// Get the router service using the proper chimux interface
	var router chimux.BasicRouter
	if err := h.app.GetService("router", &router); err != nil {
		return fmt.Errorf("failed to get router service: %w", err)
	}

	// Register health endpoint that responds with application health, not backend health
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Simple health response indicating the reverse proxy application is running
		response := map[string]interface{}{
			"status":    "healthy",
			"service":   "health-aware-reverse-proxy",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"version":   "1.0.0",
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.app.Logger().Error("Failed to encode health response", "error", err)
		}
	})

	h.app.Logger().Info("Registered application health endpoint", "endpoint", "/health")
	return nil
}
