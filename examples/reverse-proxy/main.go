package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/CrisisTextLine/modular"
	"github.com/CrisisTextLine/modular/feeders"
	"github.com/CrisisTextLine/modular/modules/chimux"
	"github.com/CrisisTextLine/modular/modules/httpserver"
	"github.com/CrisisTextLine/modular/modules/reverseproxy"
)

type AppConfig struct {
	// Empty config struct for the reverse proxy example
	// Configuration is handled by individual modules
}

func main() {
	// Start mock backend servers
	startMockBackends()

	// Configure feeders
	modular.ConfigFeeders = []modular.Feeder{
		feeders.NewYamlFeeder("config.yaml"),
		feeders.NewEnvFeeder(),
	}

	// Create a new application
	app := modular.NewStdApplication(
		modular.NewStdConfigProvider(&AppConfig{}),
		slog.New(slog.NewTextHandler(
			os.Stdout,
			&slog.HandlerOptions{Level: slog.LevelDebug},
		)),
	)

	// Create tenant service
	tenantService := modular.NewStandardTenantService(app.Logger())
	if err := app.RegisterService("tenantService", tenantService); err != nil {
		app.Logger().Error("Failed to register tenant service", "error", err)
		os.Exit(1)
	}

	// Register tenants with their configurations
	err := tenantService.RegisterTenant("tenant1", map[string]modular.ConfigProvider{
		"reverseproxy": modular.NewStdConfigProvider(&reverseproxy.ReverseProxyConfig{
			DefaultBackend: "tenant1-backend",
			BackendServices: map[string]string{
				"tenant1-backend": "http://localhost:9002",
			},
		}),
	})
	if err != nil {
		app.Logger().Error("Failed to register tenant1", "error", err)
		os.Exit(1)
	}

	err = tenantService.RegisterTenant("tenant2", map[string]modular.ConfigProvider{
		"reverseproxy": modular.NewStdConfigProvider(&reverseproxy.ReverseProxyConfig{
			DefaultBackend: "tenant2-backend",
			BackendServices: map[string]string{
				"tenant2-backend": "http://localhost:9003",
			},
		}),
	})
	if err != nil {
		app.Logger().Error("Failed to register tenant2", "error", err)
		os.Exit(1)
	}

	// Register the modules in dependency order
	app.RegisterModule(chimux.NewChiMuxModule())
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
	// Global default backend (port 9001)
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"backend":"global-default","path":"%s","method":"%s"}`, r.URL.Path, r.Method)
		})
		fmt.Println("Starting global-default backend on :9001")
		http.ListenAndServe(":9001", mux)
	}()

	// Tenant1 backend (port 9002)
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"backend":"tenant1-backend","path":"%s","method":"%s"}`, r.URL.Path, r.Method)
		})
		fmt.Println("Starting tenant1-backend on :9002")
		http.ListenAndServe(":9002", mux)
	}()

	// Tenant2 backend (port 9003)
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"backend":"tenant2-backend","path":"%s","method":"%s"}`, r.URL.Path, r.Method)
		})
		fmt.Println("Starting tenant2-backend on :9003")
		http.ListenAndServe(":9003", mux)
	}()

	// Specific API backend (port 9004)
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"backend":"specific-api","path":"%s","method":"%s"}`, r.URL.Path, r.Method)
		})
		fmt.Println("Starting specific-api backend on :9004")
		http.ListenAndServe(":9004", mux)
	}()
}
