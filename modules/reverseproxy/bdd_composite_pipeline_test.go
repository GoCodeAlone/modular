package reverseproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"
)

// ============================================================================
// Pipeline Strategy BDD Steps
// ============================================================================

func (ctx *ReverseProxyBDDTestContext) iHaveAPipelineCompositeRouteWithTwoBackends() error {
	ctx.resetContext()

	// Backend 1: returns a list of items with IDs
	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []map[string]interface{}{
				{"id": "item-1", "name": "First Item"},
				{"id": "item-2", "name": "Second Item"},
			},
		})
	}))
	ctx.testServers = append(ctx.testServers, backend1)

	// Backend 2: returns details for given IDs
	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idsParam := r.URL.Query().Get("ids")
		details := make(map[string]interface{})
		if idsParam != "" {
			for _, id := range strings.Split(idsParam, ",") {
				if id == "item-1" {
					details[id] = map[string]interface{}{"category": "A", "priority": "high"}
				}
				if id == "item-2" {
					details[id] = map[string]interface{}{"category": "B", "priority": "low"}
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"details": details,
		})
	}))
	ctx.testServers = append(ctx.testServers, backend2)

	ctx.config = &ReverseProxyConfig{
		DefaultBackend: "items-backend",
		BackendServices: map[string]string{
			"items-backend":   backend1.URL,
			"details-backend": backend2.URL,
		},
		CompositeRoutes: map[string]CompositeRoute{
			"/api/pipeline": {
				Pattern:  "/api/pipeline",
				Backends: []string{"items-backend", "details-backend"},
				Strategy: "pipeline",
			},
		},
		HealthCheck: HealthCheckConfig{
			Enabled:  false,
			Interval: 30 * time.Second,
		},
		CircuitBreakerConfig: CircuitBreakerConfig{
			Enabled: false,
		},
	}

	// Capture backend2 URL for use in the closure
	backend2URL := backend2.URL

	if err := ctx.setupApplicationWithConfig(); err != nil {
		return err
	}

	// Set pipeline config on the module AFTER setup creates it
	ctx.module.SetPipelineConfig("/api/pipeline", PipelineConfig{
		RequestBuilder: func(rctx context.Context, originalReq *http.Request, previousResponses map[string][]byte, nextBackendID string) (*http.Request, error) {
			if nextBackendID == "details-backend" {
				var itemsResp struct {
					Items []struct {
						ID string `json:"id"`
					} `json:"items"`
				}
				if body, ok := previousResponses["items-backend"]; ok {
					if err := json.Unmarshal(body, &itemsResp); err != nil {
						return nil, err
					}
				}
				ids := make([]string, 0, len(itemsResp.Items))
				for _, item := range itemsResp.Items {
					ids = append(ids, item.ID)
				}
				url := backend2URL + "/details?ids=" + strings.Join(ids, ",")
				return http.NewRequestWithContext(rctx, "GET", url, nil)
			}
			return nil, fmt.Errorf("unknown backend: %s", nextBackendID)
		},
		ResponseMerger: func(rctx context.Context, originalReq *http.Request, allResponses map[string][]byte) (*http.Response, error) {
			var itemsResp struct {
				Items []map[string]interface{} `json:"items"`
			}
			json.Unmarshal(allResponses["items-backend"], &itemsResp)

			var detailsResp struct {
				Details map[string]interface{} `json:"details"`
			}
			json.Unmarshal(allResponses["details-backend"], &detailsResp)

			for i, item := range itemsResp.Items {
				if id, ok := item["id"].(string); ok {
					if detail, exists := detailsResp.Details[id]; exists {
						itemsResp.Items[i]["detail"] = detail
					}
				}
			}
			return MakeJSONResponse(http.StatusOK, map[string]interface{}{
				"items": itemsResp.Items,
			})
		},
	})

	return nil
}

func (ctx *ReverseProxyBDDTestContext) iSendARequestToThePipelineRoute() error {
	if ctx.module == nil {
		return fmt.Errorf("module not initialized")
	}

	backends := []*Backend{
		{ID: "items-backend", URL: ctx.module.config.BackendServices["items-backend"], Client: http.DefaultClient},
		{ID: "details-backend", URL: ctx.module.config.BackendServices["details-backend"], Client: http.DefaultClient},
	}

	handler := NewCompositeHandler(backends, StrategyPipeline, 10*time.Second)

	if pipelineCfg, ok := ctx.module.pipelineConfigs["/api/pipeline"]; ok {
		handler.SetPipelineConfig(pipelineCfg)
	}

	req := httptest.NewRequest("GET", "/api/pipeline", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	ctx.lastResponse = resp
	body, _ := io.ReadAll(resp.Body)
	ctx.lastResponseBody = body
	resp.Body = io.NopCloser(strings.NewReader(string(body)))

	return nil
}

func (ctx *ReverseProxyBDDTestContext) theFirstBackendShouldBeCalledWithTheOriginalRequest() error {
	if ctx.lastResponse == nil {
		return fmt.Errorf("no response received")
	}
	if ctx.lastResponse.StatusCode != http.StatusOK {
		return fmt.Errorf("expected status 200, got %d", ctx.lastResponse.StatusCode)
	}
	return nil
}

func (ctx *ReverseProxyBDDTestContext) theSecondBackendShouldReceiveDataDerivedFromTheFirstResponse() error {
	if ctx.lastResponseBody == nil {
		return fmt.Errorf("no response body")
	}
	var result map[string]interface{}
	if err := json.Unmarshal(ctx.lastResponseBody, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	items, ok := result["items"].([]interface{})
	if !ok {
		return fmt.Errorf("expected items array in response")
	}
	if len(items) == 0 {
		return fmt.Errorf("expected at least one item")
	}
	// Check that items have detail data (proving second backend was called with IDs from first)
	item1 := items[0].(map[string]interface{})
	if _, hasDetail := item1["detail"]; !hasDetail {
		return fmt.Errorf("item should have detail from second backend")
	}
	return nil
}

func (ctx *ReverseProxyBDDTestContext) theFinalResponseShouldContainMergedDataFromAllStages() error {
	var result map[string]interface{}
	if err := json.Unmarshal(ctx.lastResponseBody, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	items := result["items"].([]interface{})
	if len(items) != 2 {
		return fmt.Errorf("expected 2 items, got %d", len(items))
	}

	// Verify item-1 has detail with category A
	item1 := items[0].(map[string]interface{})
	detail1 := item1["detail"].(map[string]interface{})
	if detail1["category"] != "A" {
		return fmt.Errorf("item-1 should have category A")
	}
	return nil
}

// ============================================================================
// Fan-Out-Merge Strategy BDD Steps
// ============================================================================

func (ctx *ReverseProxyBDDTestContext) iHaveAFanOutMergeCompositeRouteWithTwoBackends() error {
	ctx.resetContext()

	// Backend A: returns items
	backendA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"records": []map[string]interface{}{
				{"id": "r1", "title": "Record One"},
				{"id": "r2", "title": "Record Two"},
				{"id": "r3", "title": "Record Three"},
			},
		})
	}))
	ctx.testServers = append(ctx.testServers, backendA)

	// Backend B: returns tags keyed by ID
	backendB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"tags": map[string]interface{}{
				"r1": []string{"urgent", "new"},
				"r3": []string{"follow-up"},
			},
		})
	}))
	ctx.testServers = append(ctx.testServers, backendB)

	ctx.config = &ReverseProxyConfig{
		DefaultBackend: "records-backend",
		BackendServices: map[string]string{
			"records-backend": backendA.URL,
			"tags-backend":    backendB.URL,
		},
		CompositeRoutes: map[string]CompositeRoute{
			"/api/fanout": {
				Pattern:  "/api/fanout",
				Backends: []string{"records-backend", "tags-backend"},
				Strategy: "fan-out-merge",
			},
		},
		HealthCheck: HealthCheckConfig{
			Enabled:  false,
			Interval: 30 * time.Second,
		},
		CircuitBreakerConfig: CircuitBreakerConfig{
			Enabled: false,
		},
	}

	if err := ctx.setupApplicationWithConfig(); err != nil {
		return err
	}

	// Set fan-out merger AFTER setup creates the module
	ctx.module.SetFanOutMerger("/api/fanout", func(rctx context.Context, originalReq *http.Request, responses map[string][]byte) (*http.Response, error) {
		var recordsResp struct {
			Records []map[string]interface{} `json:"records"`
		}
		json.Unmarshal(responses["records-backend"], &recordsResp)

		var tagsResp struct {
			Tags map[string]interface{} `json:"tags"`
		}
		json.Unmarshal(responses["tags-backend"], &tagsResp)

		for i, record := range recordsResp.Records {
			if id, ok := record["id"].(string); ok {
				if tags, exists := tagsResp.Tags[id]; exists {
					recordsResp.Records[i]["tags"] = tags
				}
			}
		}

		return MakeJSONResponse(http.StatusOK, map[string]interface{}{
			"records": recordsResp.Records,
		})
	})

	return nil
}

func (ctx *ReverseProxyBDDTestContext) iSendARequestToTheFanOutMergeRoute() error {
	if ctx.module == nil {
		return fmt.Errorf("module not initialized")
	}

	backends := []*Backend{
		{ID: "records-backend", URL: ctx.module.config.BackendServices["records-backend"], Client: http.DefaultClient},
		{ID: "tags-backend", URL: ctx.module.config.BackendServices["tags-backend"], Client: http.DefaultClient},
	}

	handler := NewCompositeHandler(backends, StrategyFanOutMerge, 10*time.Second)

	if merger, ok := ctx.module.fanOutMergers["/api/fanout"]; ok {
		handler.SetFanOutMerger(merger)
	}

	req := httptest.NewRequest("GET", "/api/fanout", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	ctx.lastResponse = resp
	body, _ := io.ReadAll(resp.Body)
	ctx.lastResponseBody = body
	resp.Body = io.NopCloser(strings.NewReader(string(body)))

	return nil
}

func (ctx *ReverseProxyBDDTestContext) bothBackendsShouldBeCalledInParallel() error {
	if ctx.lastResponse == nil {
		return fmt.Errorf("no response received")
	}
	if ctx.lastResponse.StatusCode != http.StatusOK {
		return fmt.Errorf("expected status 200, got %d", ctx.lastResponse.StatusCode)
	}
	return nil
}

func (ctx *ReverseProxyBDDTestContext) theResponsesShouldBeMergedByMatchingIDs() error {
	var result map[string]interface{}
	if err := json.Unmarshal(ctx.lastResponseBody, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	records, ok := result["records"].([]interface{})
	if !ok {
		return fmt.Errorf("expected records array")
	}
	if len(records) != 3 {
		return fmt.Errorf("expected 3 records, got %d", len(records))
	}
	return nil
}

func (ctx *ReverseProxyBDDTestContext) itemsWithMatchingAncillaryDataShouldBeEnriched() error {
	var result map[string]interface{}
	json.Unmarshal(ctx.lastResponseBody, &result)

	records := result["records"].([]interface{})

	// r1 should have tags
	r1 := records[0].(map[string]interface{})
	if _, hasTags := r1["tags"]; !hasTags {
		return fmt.Errorf("r1 should have tags")
	}

	// r2 should NOT have tags
	r2 := records[1].(map[string]interface{})
	if _, hasTags := r2["tags"]; hasTags {
		return fmt.Errorf("r2 should NOT have tags")
	}

	// r3 should have tags
	r3 := records[2].(map[string]interface{})
	if _, hasTags := r3["tags"]; !hasTags {
		return fmt.Errorf("r3 should have tags")
	}

	return nil
}

// ============================================================================
// Empty Response Policy BDD Steps
// ============================================================================

func (ctx *ReverseProxyBDDTestContext) iHaveAPipelineRouteWithSkipEmptyPolicy() error {
	ctx.resetContext()

	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":"from-stage1","value":42}`))
	}))
	ctx.testServers = append(ctx.testServers, backend1)

	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`)) // Empty response
	}))
	ctx.testServers = append(ctx.testServers, backend2)

	// Capture URL for closure
	backend2URL := backend2.URL

	ctx.config = &ReverseProxyConfig{
		DefaultBackend: "data-backend",
		BackendServices: map[string]string{
			"data-backend":  backend1.URL,
			"empty-backend": backend2.URL,
		},
		CompositeRoutes: map[string]CompositeRoute{
			"/api/skip-empty": {
				Pattern:     "/api/skip-empty",
				Backends:    []string{"data-backend", "empty-backend"},
				Strategy:    "pipeline",
				EmptyPolicy: "skip-empty",
			},
		},
		HealthCheck:          HealthCheckConfig{Enabled: false, Interval: 30 * time.Second},
		CircuitBreakerConfig: CircuitBreakerConfig{Enabled: false},
	}

	if err := ctx.setupApplicationWithConfig(); err != nil {
		return err
	}

	// Set pipeline config and empty policy AFTER setup
	ctx.module.SetPipelineConfig("/api/skip-empty", PipelineConfig{
		RequestBuilder: func(rctx context.Context, originalReq *http.Request, previousResponses map[string][]byte, nextBackendID string) (*http.Request, error) {
			return http.NewRequestWithContext(rctx, "GET", backend2URL+"/test", nil)
		},
	})
	ctx.module.SetEmptyResponsePolicy("/api/skip-empty", EmptyResponseSkip)

	return nil
}

func (ctx *ReverseProxyBDDTestContext) iSendARequestAndABackendReturnsAnEmptyResponse() error {
	if ctx.module == nil {
		return fmt.Errorf("module not initialized")
	}

	var strategy CompositeStrategy
	var pattern string
	for p, route := range ctx.module.config.CompositeRoutes {
		strategy = CompositeStrategy(route.Strategy)
		pattern = p
		break
	}

	var backends []*Backend
	for _, name := range ctx.module.config.CompositeRoutes[pattern].Backends {
		backends = append(backends, &Backend{
			ID:     name,
			URL:    ctx.module.config.BackendServices[name],
			Client: http.DefaultClient,
		})
	}

	handler := NewCompositeHandler(backends, strategy, 10*time.Second)

	emptyPolicy := EmptyResponsePolicy(ctx.module.config.CompositeRoutes[pattern].EmptyPolicy)
	if policy, ok := ctx.module.emptyResponsePolicies[pattern]; ok {
		emptyPolicy = policy
	}
	handler.SetEmptyResponsePolicy(emptyPolicy)

	if pipelineCfg, ok := ctx.module.pipelineConfigs[pattern]; ok {
		handler.SetPipelineConfig(pipelineCfg)
	}
	if merger, ok := ctx.module.fanOutMergers[pattern]; ok {
		handler.SetFanOutMerger(merger)
	}

	req := httptest.NewRequest("GET", pattern, nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	ctx.lastResponse = resp
	body, _ := io.ReadAll(resp.Body)
	ctx.lastResponseBody = body

	return nil
}

func (ctx *ReverseProxyBDDTestContext) theEmptyResponseShouldBeExcludedFromTheResult() error {
	var result map[string]interface{}
	if err := json.Unmarshal(ctx.lastResponseBody, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// "empty-backend" should not be in the response
	if _, found := result["empty-backend"]; found {
		return fmt.Errorf("empty backend response should be excluded")
	}
	return nil
}

func (ctx *ReverseProxyBDDTestContext) theNonEmptyResponsesShouldStillBePresent() error {
	var result map[string]interface{}
	json.Unmarshal(ctx.lastResponseBody, &result)

	if _, found := result["data-backend"]; !found {
		return fmt.Errorf("data-backend response should be present")
	}
	return nil
}

func (ctx *ReverseProxyBDDTestContext) iHaveAFanOutMergeRouteWithFailOnEmptyPolicy() error {
	ctx.resetContext()

	backendA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":"ok"}`))
	}))
	ctx.testServers = append(ctx.testServers, backendA)

	backendB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(``)) // Completely empty
	}))
	ctx.testServers = append(ctx.testServers, backendB)

	ctx.config = &ReverseProxyConfig{
		DefaultBackend: "ok-backend",
		BackendServices: map[string]string{
			"ok-backend":    backendA.URL,
			"empty-backend": backendB.URL,
		},
		CompositeRoutes: map[string]CompositeRoute{
			"/api/fail-empty": {
				Pattern:     "/api/fail-empty",
				Backends:    []string{"ok-backend", "empty-backend"},
				Strategy:    "fan-out-merge",
				EmptyPolicy: "fail-on-empty",
			},
		},
		HealthCheck:          HealthCheckConfig{Enabled: false, Interval: 30 * time.Second},
		CircuitBreakerConfig: CircuitBreakerConfig{Enabled: false},
	}

	if err := ctx.setupApplicationWithConfig(); err != nil {
		return err
	}

	// Set merger and policy AFTER setup
	ctx.module.SetFanOutMerger("/api/fail-empty", func(rctx context.Context, originalReq *http.Request, responses map[string][]byte) (*http.Response, error) {
		return MakeJSONResponse(http.StatusOK, responses)
	})
	ctx.module.SetEmptyResponsePolicy("/api/fail-empty", EmptyResponseFail)

	return nil
}

func (ctx *ReverseProxyBDDTestContext) theRequestShouldFailWithABadGatewayError() error {
	if ctx.lastResponse == nil {
		return fmt.Errorf("no response received")
	}
	if ctx.lastResponse.StatusCode != http.StatusBadGateway {
		return fmt.Errorf("expected status 502, got %d", ctx.lastResponse.StatusCode)
	}
	return nil
}

// ============================================================================
// Pipeline Filter BDD Steps
// ============================================================================

func (ctx *ReverseProxyBDDTestContext) iHaveAPipelineRouteThatFiltersByAncillaryBackendData() error {
	ctx.resetContext()

	// Backend A: returns all conversations
	backendA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"conversations": []map[string]interface{}{
				{"id": "c1", "title": "Conv 1", "status": "queued"},
				{"id": "c2", "title": "Conv 2", "status": "queued"},
				{"id": "c3", "title": "Conv 3", "status": "active"},
				{"id": "c4", "title": "Conv 4", "status": "queued"},
			},
		})
	}))
	ctx.testServers = append(ctx.testServers, backendA)

	// Backend B: returns which conversations are flagged
	backendB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"flagged_ids": []string{"c1", "c4"},
		})
	}))
	ctx.testServers = append(ctx.testServers, backendB)

	// Capture URL for closure
	backendBURL := backendB.URL

	ctx.config = &ReverseProxyConfig{
		DefaultBackend: "conv-backend",
		BackendServices: map[string]string{
			"conv-backend":  backendA.URL,
			"flags-backend": backendB.URL,
		},
		CompositeRoutes: map[string]CompositeRoute{
			"/api/filter": {
				Pattern:  "/api/filter",
				Backends: []string{"conv-backend", "flags-backend"},
				Strategy: "pipeline",
			},
		},
		HealthCheck:          HealthCheckConfig{Enabled: false, Interval: 30 * time.Second},
		CircuitBreakerConfig: CircuitBreakerConfig{Enabled: false},
	}

	if err := ctx.setupApplicationWithConfig(); err != nil {
		return err
	}

	// Set pipeline config AFTER setup
	ctx.module.SetPipelineConfig("/api/filter", PipelineConfig{
		RequestBuilder: func(rctx context.Context, originalReq *http.Request, previousResponses map[string][]byte, nextBackendID string) (*http.Request, error) {
			return http.NewRequestWithContext(rctx, "GET", backendBURL+"/flags", nil)
		},
		ResponseMerger: func(rctx context.Context, originalReq *http.Request, allResponses map[string][]byte) (*http.Response, error) {
			var convResp struct {
				Conversations []map[string]interface{} `json:"conversations"`
			}
			json.Unmarshal(allResponses["conv-backend"], &convResp)

			var flagsResp struct {
				FlaggedIDs []string `json:"flagged_ids"`
			}
			json.Unmarshal(allResponses["flags-backend"], &flagsResp)

			flagSet := make(map[string]bool)
			for _, id := range flagsResp.FlaggedIDs {
				flagSet[id] = true
			}

			var filtered []map[string]interface{}
			for _, conv := range convResp.Conversations {
				if id, ok := conv["id"].(string); ok && flagSet[id] {
					conv["flagged"] = true
					filtered = append(filtered, conv)
				}
			}

			return MakeJSONResponse(http.StatusOK, map[string]interface{}{
				"filtered_conversations": filtered,
				"total_filtered":         len(filtered),
			})
		},
	})

	return nil
}

func (ctx *ReverseProxyBDDTestContext) iSendARequestToFetchFilteredResults() error {
	if ctx.module == nil {
		return fmt.Errorf("module not initialized")
	}

	backends := []*Backend{
		{ID: "conv-backend", URL: ctx.module.config.BackendServices["conv-backend"], Client: http.DefaultClient},
		{ID: "flags-backend", URL: ctx.module.config.BackendServices["flags-backend"], Client: http.DefaultClient},
	}

	handler := NewCompositeHandler(backends, StrategyPipeline, 10*time.Second)
	if pipelineCfg, ok := ctx.module.pipelineConfigs["/api/filter"]; ok {
		handler.SetPipelineConfig(pipelineCfg)
	}

	req := httptest.NewRequest("GET", "/api/filter", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	ctx.lastResponse = resp
	body, _ := io.ReadAll(resp.Body)
	ctx.lastResponseBody = body

	return nil
}

func (ctx *ReverseProxyBDDTestContext) onlyItemsMatchingTheAncillaryCriteriaShouldBeReturned() error {
	var result map[string]interface{}
	if err := json.Unmarshal(ctx.lastResponseBody, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	filtered := result["filtered_conversations"].([]interface{})
	totalFiltered := result["total_filtered"].(float64)

	if int(totalFiltered) != 2 {
		return fmt.Errorf("expected 2 filtered conversations, got %v", totalFiltered)
	}
	if len(filtered) != 2 {
		return fmt.Errorf("expected 2 filtered conversations in array, got %d", len(filtered))
	}

	// Verify only c1 and c4 are present
	ids := make(map[string]bool)
	for _, item := range filtered {
		m := item.(map[string]interface{})
		ids[m["id"].(string)] = true
		if m["flagged"] != true {
			return fmt.Errorf("expected flagged=true on filtered item")
		}
	}

	if !ids["c1"] || !ids["c4"] {
		return fmt.Errorf("expected c1 and c4 in filtered results, got %v", ids)
	}

	return nil
}
