package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"regexp"

	"github.com/CrisisTextLine/modular"
	"github.com/CrisisTextLine/modular/feeders"
	"github.com/CrisisTextLine/modular/modules/chimux"
	"github.com/CrisisTextLine/modular/modules/httpserver"
	"github.com/CrisisTextLine/modular/modules/reverseproxy"
)

type AppConfig struct {
	// Empty config struct for the feature flag example
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

	// Feature flag evaluator service will be automatically provided by the reverseproxy module
	// when feature flags are enabled in configuration. No manual registration needed.

	// Create tenant service for multi-tenancy support
	tenantService := modular.NewStandardTenantService(app.Logger())
	if err := app.RegisterService("tenantService", tenantService); err != nil {
		app.Logger().Error("Failed to register tenant service", "error", err)
		os.Exit(1)
	}

	// Register tenant config loader to load tenant configurations from files
	tenantConfigLoader := modular.NewFileBasedTenantConfigLoader(modular.TenantConfigParams{
		ConfigNameRegex: regexp.MustCompile(`^[\w-]+\.yaml$`), // Allow hyphens in tenant names
		ConfigDir:       "tenants",
		ConfigFeeders: []modular.Feeder{
			// Add tenant-specific environment variable support
			feeders.NewTenantAffixedEnvFeeder(func(tenantId string) string {
				return fmt.Sprintf("%s_", tenantId)
			}, func(s string) string { return "" }),
		},
	})
	if err := app.RegisterService("tenantConfigLoader", tenantConfigLoader); err != nil {
		app.Logger().Error("Failed to register tenant config loader", "error", err)
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
	// Default backend (port 9001)
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"backend":"default","path":"%s","method":"%s","feature":"stable"}`, r.URL.Path, r.Method)
		})
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"status":"healthy","backend":"default"}`)
		})
		fmt.Println("Starting default backend on :9001")
		if err := http.ListenAndServe(":9001", mux); err != nil { //nolint:gosec
			fmt.Printf("Backend server error on %s: %v\n", ":9001", err)
		}
	}()

	// Alternative backend when feature flags are disabled (port 9002)
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"backend":"alternative","path":"%s","method":"%s","feature":"fallback"}`, r.URL.Path, r.Method)
		})
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"status":"healthy","backend":"alternative"}`)
		})
		fmt.Println("Starting alternative backend on :9002")
		if err := http.ListenAndServe(":9002", mux); err != nil { //nolint:gosec
			fmt.Printf("Backend server error on %s: %v\n", ":9002", err)
		}
	}()

	// New feature backend (port 9003)
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"backend":"new-feature","path":"%s","method":"%s","feature":"new"}`, r.URL.Path, r.Method)
		})
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"status":"healthy","backend":"new-feature"}`)
		})
		fmt.Println("Starting new-feature backend on :9003")
		if err := http.ListenAndServe(":9003", mux); err != nil { //nolint:gosec
			fmt.Printf("Backend server error on %s: %v\n", ":9003", err)
		}
	}()

	// API backend for composite routes (port 9004)
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"backend":"api","path":"%s","method":"%s","data":"api-data"}`, r.URL.Path, r.Method)
		})
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"status":"healthy","backend":"api"}`)
		})
		fmt.Println("Starting api backend on :9004")
		if err := http.ListenAndServe(":9004", mux); err != nil { //nolint:gosec
			fmt.Printf("Backend server error on %s: %v\n", ":9004", err)
		}
	}()

	// Beta tenant backend (port 9005)
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"backend":"beta-backend","path":"%s","method":"%s","feature":"beta-enabled"}`, r.URL.Path, r.Method)
		})
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"status":"healthy","backend":"beta-backend"}`)
		})
		fmt.Println("Starting beta-backend on :9005")
		if err := http.ListenAndServe(":9005", mux); err != nil { //nolint:gosec
			fmt.Printf("Backend server error on %s: %v\n", ":9005", err)
		}
	}()

	// Premium API backend for beta tenant (port 9006)
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"backend":"premium-api","path":"%s","method":"%s","feature":"premium-enabled"}`, r.URL.Path, r.Method)
		})
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"status":"healthy","backend":"premium-api"}`)
		})
		fmt.Println("Starting premium-api backend on :9006")
		if err := http.ListenAndServe(":9006", mux); err != nil { //nolint:gosec
			fmt.Printf("Backend server error on %s: %v\n", ":9006", err)
		}
	}()

	// Enterprise backend (port 9007)
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"backend":"enterprise-backend","path":"%s","method":"%s","feature":"enterprise-enabled"}`, r.URL.Path, r.Method)
		})
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"status":"healthy","backend":"enterprise-backend"}`)
		})
		fmt.Println("Starting enterprise-backend on :9007")
		if err := http.ListenAndServe(":9007", mux); err != nil { //nolint:gosec
			fmt.Printf("Backend server error on %s: %v\n", ":9007", err)
		}
	}()

	// Analytics API backend for enterprise tenant (port 9008)
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"backend":"analytics-api","path":"%s","method":"%s","data":"analytics-data"}`, r.URL.Path, r.Method)
		})
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"status":"healthy","backend":"analytics-api"}`)
		})
		fmt.Println("Starting analytics-api backend on :9008")
		if err := http.ListenAndServe(":9008", mux); err != nil { //nolint:gosec
			fmt.Printf("Backend server error on %s: %v\n", ":9008", err)
		}
	}()
}
