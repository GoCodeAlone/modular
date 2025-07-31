package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

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
		http.ListenAndServe(":9001", mux)
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
		http.ListenAndServe(":9002", mux)
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
		http.ListenAndServe(":9003", mux)
	}()

	// Unreachable backend simulation - we won't start this one
	// This will demonstrate DNS/connection failures
	fmt.Println("Unreachable backend (unreachable-api) will not be started - simulating unreachable service")
}