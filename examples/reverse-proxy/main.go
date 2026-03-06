package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
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

	// Create a new application and set feeders per instance (no global mutation)
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

	// Create reverse proxy module with composite route strategies and response transformers
	proxyModule := reverseproxy.NewModule()

	// Set a custom response header modifier to demonstrate dynamic CORS header consolidation
	proxyModule.SetResponseHeaderModifier(func(resp *http.Response, backendID string, tenantID modular.TenantID) error {
		// Add custom headers based on backend and tenant
		resp.Header.Set("X-Backend-Served-By", backendID)
		if tenantID != "" {
			resp.Header.Set("X-Tenant-Served", string(tenantID))
		}

		// Example: Dynamically set Cache-Control based on status code
		if resp.StatusCode == http.StatusOK {
			resp.Header.Set("Cache-Control", "public, max-age=300")
		} else {
			resp.Header.Set("Cache-Control", "no-cache, no-store, must-revalidate")
		}

		return nil
	})

	// CUSTOM RESPONSE TRANSFORMER EXAMPLE:
	// This transformer demonstrates how to merge data from one backend into another's response.
	// In this example, we fetch user profile data from one backend and enrich it with
	// analytics data from another backend, creating a unified response.
	proxyModule.SetResponseTransformer("/api/composite/profile-with-analytics", func(responses map[string]*http.Response) (*http.Response, error) {
		// Read responses from both backends
		var profileData map[string]interface{}
		var analyticsData map[string]interface{}
		var errors []string

		// Parse profile backend response
		if profileResp, ok := responses["profile-backend"]; ok {
			body, err := io.ReadAll(profileResp.Body)
			if err != nil {
				errors = append(errors, fmt.Sprintf("failed to read profile: %v", err))
			} else if err := json.Unmarshal(body, &profileData); err != nil {
				errors = append(errors, fmt.Sprintf("failed to parse profile: %v", err))
			}
		} else {
			errors = append(errors, "profile backend response missing")
		}

		// Parse analytics backend response
		if analyticsResp, ok := responses["analytics-backend"]; ok {
			body, err := io.ReadAll(analyticsResp.Body)
			if err != nil {
				errors = append(errors, fmt.Sprintf("failed to read analytics: %v", err))
			} else if err := json.Unmarshal(body, &analyticsData); err != nil {
				errors = append(errors, fmt.Sprintf("failed to parse analytics: %v", err))
			}
		} else {
			errors = append(errors, "analytics backend response missing")
		}

		// Create enriched response by merging analytics into profile
		enriched := make(map[string]interface{})
		if len(errors) > 0 {
			enriched["errors"] = errors
		}
		if profileData != nil {
			enriched["profile"] = profileData
		}
		if analyticsData != nil {
			// Augment profile with analytics data
			enriched["analytics"] = analyticsData
			// Example: Add analytics summary to the top level
			if views, ok := analyticsData["page_views"].(float64); ok {
				enriched["total_page_views"] = views
			}
		}
		enriched["enriched"] = true
		enriched["timestamp"] = time.Now().Format(time.RFC3339)

		// Create the response
		body, err := json.Marshal(enriched)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal enriched response: %w", err)
		}

		resp := &http.Response{
			Status:     "200 OK",
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader(body)),
		}

		return resp, nil
	})

	// PIPELINE STRATEGY EXAMPLE:
	// This demonstrates chained backend requests where backend B's request is constructed
	// using data from backend A's response. This is the map/reduce pattern.
	//
	// Use case: A list page shows queued conversations. Backend A returns conversation details,
	// then those conversation IDs are fed into Backend B to fetch follow-up information.
	// The responses are then merged to produce a unified view.
	proxyModule.SetPipelineConfig("/api/composite/pipeline", reverseproxy.PipelineConfig{
		RequestBuilder: func(ctx context.Context, originalReq *http.Request, previousResponses map[string][]byte, nextBackendID string) (*http.Request, error) {
			if nextBackendID == "followup-backend" {
				// Extract conversation IDs from the conversations backend response
				var convResp struct {
					Conversations []struct {
						ID string `json:"id"`
					} `json:"conversations"`
				}
				if body, ok := previousResponses["conversations-backend"]; ok {
					if err := json.Unmarshal(body, &convResp); err != nil {
						return nil, fmt.Errorf("failed to parse conversations: %w", err)
					}
				}

				// Build the follow-up request with those IDs
				ids := make([]string, 0, len(convResp.Conversations))
				for _, c := range convResp.Conversations {
					ids = append(ids, c.ID)
				}
				idsParam := ""
				for i, id := range ids {
					if i > 0 {
						idsParam += ","
					}
					idsParam += id
				}

				url := "http://localhost:9016/followups?ids=" + idsParam
				return http.NewRequestWithContext(ctx, "GET", url, nil)
			}
			return nil, fmt.Errorf("unknown pipeline backend: %s", nextBackendID)
		},
		ResponseMerger: func(ctx context.Context, originalReq *http.Request, allResponses map[string][]byte) (*http.Response, error) {
			// Parse conversations
			var convResp struct {
				Conversations []map[string]interface{} `json:"conversations"`
			}
			if body, ok := allResponses["conversations-backend"]; ok {
				json.Unmarshal(body, &convResp)
			}

			// Parse follow-ups
			var fuResp struct {
				FollowUps map[string]interface{} `json:"follow_ups"`
			}
			if body, ok := allResponses["followup-backend"]; ok {
				json.Unmarshal(body, &fuResp)
			}

			// Merge follow-up data into each conversation
			for i, conv := range convResp.Conversations {
				if id, ok := conv["id"].(string); ok {
					if fu, exists := fuResp.FollowUps[id]; exists {
						convResp.Conversations[i]["follow_up"] = fu
					}
				}
			}

			return reverseproxy.MakeJSONResponse(http.StatusOK, map[string]interface{}{
				"conversations": convResp.Conversations,
				"strategy":      "pipeline",
			})
		},
	})

	// FAN-OUT-MERGE STRATEGY EXAMPLE:
	// This demonstrates parallel requests to multiple backends with custom ID-based
	// response merging. Both backends are called simultaneously, then their responses
	// are correlated by matching IDs.
	//
	// Use case: Show a ticket dashboard where tickets come from one service and
	// priority/assignment data comes from another. The merger matches by ticket ID.
	proxyModule.SetFanOutMerger("/api/composite/fanout-merge", func(ctx context.Context, originalReq *http.Request, responses map[string][]byte) (*http.Response, error) {
		// Parse tickets from the tickets backend
		var ticketsResp struct {
			Tickets []map[string]interface{} `json:"tickets"`
		}
		if body, ok := responses["tickets-backend"]; ok {
			json.Unmarshal(body, &ticketsResp)
		}

		// Parse assignments from the assignments backend
		var assignResp struct {
			Assignments map[string]interface{} `json:"assignments"`
		}
		if body, ok := responses["assignments-backend"]; ok {
			json.Unmarshal(body, &assignResp)
		}

		// Merge assignments into tickets by ID
		for i, ticket := range ticketsResp.Tickets {
			if id, ok := ticket["id"].(string); ok {
				if assignment, exists := assignResp.Assignments[id]; exists {
					ticketsResp.Tickets[i]["assignment"] = assignment
				}
			}
		}

		return reverseproxy.MakeJSONResponse(http.StatusOK, map[string]interface{}{
			"tickets":  ticketsResp.Tickets,
			"strategy": "fan-out-merge",
		})
	})

	app.RegisterModule(proxyModule)
	app.RegisterModule(httpserver.NewHTTPServerModule())

	// Run application with lifecycle management
	if err := app.Run(); err != nil {
		app.Logger().Error("Application error", "error", err)
		os.Exit(1)
	}
}

// startMockBackends starts mock backend servers on different ports to demonstrate composite routing strategies
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
		if err := http.ListenAndServe(":9001", mux); err != nil { //nolint:gosec
			fmt.Printf("Backend server error on :9001: %v\n", err)
		}
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
		if err := http.ListenAndServe(":9002", mux); err != nil { //nolint:gosec
			fmt.Printf("Backend server error on :9002: %v\n", err)
		}
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
		if err := http.ListenAndServe(":9003", mux); err != nil { //nolint:gosec
			fmt.Printf("Backend server error on :9003: %v\n", err)
		}
	}()

	// Specific API backend (port 9004) - simulates a backend with CORS headers
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// Backend sets its own CORS headers (which will be overridden by proxy)
			w.Header().Set("Access-Control-Allow-Origin", "http://old-domain.com")
			w.Header().Set("Access-Control-Allow-Methods", "GET")
			w.Header().Set("X-Internal-Header", "internal-value")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"backend":"specific-api","path":"%s","method":"%s","note":"backend CORS headers will be overridden"}`, r.URL.Path, r.Method)
		})
		fmt.Println("Starting specific-api backend on :9004")
		if err := http.ListenAndServe(":9004", mux); err != nil { //nolint:gosec
			fmt.Printf("Backend server error on :9004: %v\n", err)
		}
	}()

	// ========================================
	// Backends for demonstrating FIRST-SUCCESS strategy
	// ========================================

	// Primary backend (port 9005) - Sometimes fails to demonstrate fallback
	go func() {
		requestCount := 0
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			requestCount++
			// Fail every 3rd request to demonstrate fallback
			if requestCount%3 == 0 {
				w.WriteHeader(http.StatusServiceUnavailable)
				fmt.Fprintf(w, `{"error":"primary backend unavailable"}`)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"backend":"primary-backend","status":"success","request_count":%d}`, requestCount)
		})
		fmt.Println("Starting primary-backend (first-success demo) on :9005")
		if err := http.ListenAndServe(":9005", mux); err != nil { //nolint:gosec
			fmt.Printf("Backend server error on :9005: %v\n", err)
		}
	}()

	// Fallback backend (port 9006) - Always succeeds
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"backend":"fallback-backend","status":"success","message":"fallback activated"}`)
		})
		fmt.Println("Starting fallback-backend (first-success demo) on :9006")
		if err := http.ListenAndServe(":9006", mux); err != nil { //nolint:gosec
			fmt.Printf("Backend server error on :9006: %v\n", err)
		}
	}()

	// ========================================
	// Backends for demonstrating MERGE strategy
	// ========================================

	// Users backend (port 9007) - Returns user data
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"user_id":123,"username":"john_doe","email":"john@example.com"}`)
		})
		fmt.Println("Starting users-backend (merge demo) on :9007")
		if err := http.ListenAndServe(":9007", mux); err != nil { //nolint:gosec
			fmt.Printf("Backend server error on :9007: %v\n", err)
		}
	}()

	// Orders backend (port 9008) - Returns order data
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"total_orders":42,"recent_orders":[{"id":1,"amount":99.99},{"id":2,"amount":149.99}]}`)
		})
		fmt.Println("Starting orders-backend (merge demo) on :9008")
		if err := http.ListenAndServe(":9008", mux); err != nil { //nolint:gosec
			fmt.Printf("Backend server error on :9008: %v\n", err)
		}
	}()

	// Preferences backend (port 9009) - Returns user preferences
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"theme":"dark","notifications":true,"language":"en"}`)
		})
		fmt.Println("Starting preferences-backend (merge demo) on :9009")
		if err := http.ListenAndServe(":9009", mux); err != nil { //nolint:gosec
			fmt.Printf("Backend server error on :9009: %v\n", err)
		}
	}()

	// ========================================
	// Backends for demonstrating SEQUENTIAL strategy
	// ========================================

	// Auth backend (port 9010) - First in sequence, validates request
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"auth":"validated","token":"abc123","step":"1_auth"}`)
		})
		fmt.Println("Starting auth-backend (sequential demo) on :9010")
		if err := http.ListenAndServe(":9010", mux); err != nil { //nolint:gosec
			fmt.Printf("Backend server error on :9010: %v\n", err)
		}
	}()

	// Processing backend (port 9011) - Second in sequence, processes request
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"processing":"complete","job_id":"job-456","step":"2_process"}`)
		})
		fmt.Println("Starting processing-backend (sequential demo) on :9011")
		if err := http.ListenAndServe(":9011", mux); err != nil { //nolint:gosec
			fmt.Printf("Backend server error on :9011: %v\n", err)
		}
	}()

	// Finalization backend (port 9012) - Last in sequence, returns final result
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"status":"completed","result":"success","message":"All steps completed","step":"3_finalize"}`)
		})
		fmt.Println("Starting finalization-backend (sequential demo) on :9012")
		if err := http.ListenAndServe(":9012", mux); err != nil { //nolint:gosec
			fmt.Printf("Backend server error on :9012: %v\n", err)
		}
	}()

	// ========================================
	// Backends for CUSTOM TRANSFORMER demonstration
	// ========================================

	// Profile backend (port 9013) - Returns user profile
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"user_id":789,"name":"Alice Smith","bio":"Software Engineer","joined":"2023-01-15"}`)
		})
		fmt.Println("Starting profile-backend (transformer demo) on :9013")
		if err := http.ListenAndServe(":9013", mux); err != nil { //nolint:gosec
			fmt.Printf("Backend server error on :9013: %v\n", err)
		}
	}()

	// Analytics backend (port 9014) - Returns user analytics
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"page_views":1523,"session_duration":"45m","last_login":"2024-01-20T10:30:00Z"}`)
		})
		fmt.Println("Starting analytics-backend (transformer demo) on :9014")
		if err := http.ListenAndServe(":9014", mux); err != nil { //nolint:gosec
			fmt.Printf("Backend server error on :9014: %v\n", err)
		}
	}()

	// ========================================
	// Backends for PIPELINE strategy demonstration
	// ========================================

	// Conversations backend (port 9015) - Returns queued conversations
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"conversations":[{"id":"conv-1","status":"queued","counselor":"Alice","created_at":"2024-01-01T10:00:00Z"},{"id":"conv-2","status":"queued","counselor":"Bob","created_at":"2024-01-01T10:05:00Z"},{"id":"conv-3","status":"active","counselor":"Carol","created_at":"2024-01-01T10:10:00Z"}]}`)
		})
		fmt.Println("Starting conversations-backend (pipeline demo) on :9015")
		if err := http.ListenAndServe(":9015", mux); err != nil { //nolint:gosec
			fmt.Printf("Backend server error on :9015: %v\n", err)
		}
	}()

	// Follow-up backend (port 9016) - Returns follow-up details for given conversation IDs
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			idsParam := r.URL.Query().Get("ids")
			followUps := make(map[string]interface{})
			if idsParam != "" {
				for _, id := range strings.Split(idsParam, ",") {
					switch id {
					case "conv-1":
						followUps[id] = map[string]interface{}{
							"is_follow_up":     true,
							"original_conv_id": "conv-50",
							"follow_up_count":  2,
						}
					case "conv-3":
						followUps[id] = map[string]interface{}{
							"is_follow_up":     true,
							"original_conv_id": "conv-90",
							"follow_up_count":  1,
						}
					}
				}
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			resp, _ := json.Marshal(map[string]interface{}{"follow_ups": followUps})
			w.Write(resp) //nolint:errcheck
		})
		fmt.Println("Starting followup-backend (pipeline demo) on :9016")
		if err := http.ListenAndServe(":9016", mux); err != nil { //nolint:gosec
			fmt.Printf("Backend server error on :9016: %v\n", err)
		}
	}()

	// ========================================
	// Backends for FAN-OUT-MERGE strategy demonstration
	// ========================================

	// Tickets backend (port 9017) - Returns support tickets
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"tickets":[{"id":"ticket-1","subject":"Login issue","status":"open","created":"2024-01-15"},{"id":"ticket-2","subject":"Billing question","status":"open","created":"2024-01-16"},{"id":"ticket-3","subject":"Feature request","status":"pending","created":"2024-01-17"}]}`)
		})
		fmt.Println("Starting tickets-backend (fan-out-merge demo) on :9017")
		if err := http.ListenAndServe(":9017", mux); err != nil { //nolint:gosec
			fmt.Printf("Backend server error on :9017: %v\n", err)
		}
	}()

	// Assignments backend (port 9018) - Returns ticket assignments and priorities
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"assignments":{"ticket-1":{"assignee":"Alice","priority":"high","sla_deadline":"2024-01-16T12:00:00Z"},"ticket-3":{"assignee":"Bob","priority":"low","sla_deadline":"2024-01-20T12:00:00Z"}}}`)
		})
		fmt.Println("Starting assignments-backend (fan-out-merge demo) on :9018")
		if err := http.ListenAndServe(":9018", mux); err != nil { //nolint:gosec
			fmt.Printf("Backend server error on :9018: %v\n", err)
		}
	}()
}
