package reverseproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Pipeline Strategy Tests
// ============================================================================

func TestPipelineStrategy_BasicChaining(t *testing.T) {
	// Backend A returns a list of conversation IDs
	backendA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"conversations": []map[string]interface{}{
				{"id": "conv-1", "title": "First conversation"},
				{"id": "conv-2", "title": "Second conversation"},
				{"id": "conv-3", "title": "Third conversation"},
			},
		})
	}))
	defer backendA.Close()

	// Backend B returns follow-up details for given IDs (received via query params)
	backendB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ids := r.URL.Query().Get("ids")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Return follow-up details for the requested IDs
		followUps := map[string]interface{}{}
		for _, id := range strings.Split(ids, ",") {
			if id == "conv-1" {
				followUps[id] = map[string]interface{}{"is_followup": true, "original_id": "conv-0"}
			}
			// conv-2 and conv-3 have no follow-up data
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"follow_ups": followUps,
		})
	}))
	defer backendB.Close()

	backends := []*Backend{
		{ID: "conversations", URL: backendA.URL, Client: http.DefaultClient},
		{ID: "followups", URL: backendB.URL, Client: http.DefaultClient},
	}

	handler := NewCompositeHandler(backends, StrategyPipeline, 10*time.Second)
	handler.SetPipelineConfig(&PipelineConfig{
		RequestBuilder: func(ctx context.Context, originalReq *http.Request, previousResponses map[string][]byte, nextBackendID string) (*http.Request, error) {
			if nextBackendID == "followups" {
				// Parse conversation IDs from the previous response
				var convResp struct {
					Conversations []struct {
						ID string `json:"id"`
					} `json:"conversations"`
				}
				if convBody, ok := previousResponses["conversations"]; ok {
					if err := json.Unmarshal(convBody, &convResp); err != nil {
						return nil, fmt.Errorf("failed to parse conversations: %w", err)
					}
				}

				// Build query with IDs
				ids := make([]string, 0, len(convResp.Conversations))
				for _, c := range convResp.Conversations {
					ids = append(ids, c.ID)
				}

				url := backends[1].URL + "/followups?ids=" + strings.Join(ids, ",")
				return http.NewRequestWithContext(ctx, "GET", url, nil)
			}
			return nil, fmt.Errorf("unknown backend: %s", nextBackendID)
		},
		ResponseMerger: func(ctx context.Context, originalReq *http.Request, allResponses map[string][]byte) (*http.Response, error) {
			// Parse both responses
			var convResp struct {
				Conversations []map[string]interface{} `json:"conversations"`
			}
			if convBody, ok := allResponses["conversations"]; ok {
				json.Unmarshal(convBody, &convResp)
			}

			var followUpResp struct {
				FollowUps map[string]interface{} `json:"follow_ups"`
			}
			if fuBody, ok := allResponses["followups"]; ok {
				json.Unmarshal(fuBody, &followUpResp)
			}

			// Merge follow-up data into conversations
			for i, conv := range convResp.Conversations {
				if id, ok := conv["id"].(string); ok {
					if fu, exists := followUpResp.FollowUps[id]; exists {
						convResp.Conversations[i]["follow_up"] = fu
					}
				}
			}

			return MakeJSONResponse(http.StatusOK, map[string]interface{}{
				"conversations": convResp.Conversations,
			})
		},
	})

	req := httptest.NewRequest("GET", "/api/conversations", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)

	conversations, ok := result["conversations"].([]interface{})
	require.True(t, ok, "expected conversations array")
	assert.Len(t, conversations, 3)

	// Verify conv-1 has follow-up data
	conv1 := conversations[0].(map[string]interface{})
	assert.Equal(t, "conv-1", conv1["id"])
	followUp, hasFollowUp := conv1["follow_up"]
	assert.True(t, hasFollowUp, "conv-1 should have follow_up data")
	fuMap := followUp.(map[string]interface{})
	assert.Equal(t, true, fuMap["is_followup"])

	// Verify conv-2 has no follow-up data
	conv2 := conversations[1].(map[string]interface{})
	assert.Equal(t, "conv-2", conv2["id"])
	_, hasFollowUp2 := conv2["follow_up"]
	assert.False(t, hasFollowUp2, "conv-2 should not have follow_up data")
}

func TestPipelineStrategy_ThreeStageChain(t *testing.T) {
	// Stage 1: returns user IDs
	stage1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"users": []string{"user-1", "user-2"},
		})
	}))
	defer stage1.Close()

	// Stage 2: returns user profiles
	stage2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"profiles": map[string]interface{}{
				"user-1": map[string]interface{}{"name": "Alice", "dept": "eng"},
				"user-2": map[string]interface{}{"name": "Bob", "dept": "sales"},
			},
		})
	}))
	defer stage2.Close()

	// Stage 3: returns permissions
	stage3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"permissions": map[string]interface{}{
				"user-1": []string{"admin", "read", "write"},
				"user-2": []string{"read"},
			},
		})
	}))
	defer stage3.Close()

	backends := []*Backend{
		{ID: "users", URL: stage1.URL, Client: http.DefaultClient},
		{ID: "profiles", URL: stage2.URL, Client: http.DefaultClient},
		{ID: "permissions", URL: stage3.URL, Client: http.DefaultClient},
	}

	handler := NewCompositeHandler(backends, StrategyPipeline, 10*time.Second)
	handler.SetPipelineConfig(&PipelineConfig{
		RequestBuilder: func(ctx context.Context, originalReq *http.Request, previousResponses map[string][]byte, nextBackendID string) (*http.Request, error) {
			switch nextBackendID {
			case "profiles":
				url := backends[1].URL + "/profiles"
				return http.NewRequestWithContext(ctx, "GET", url, nil)
			case "permissions":
				url := backends[2].URL + "/permissions"
				return http.NewRequestWithContext(ctx, "GET", url, nil)
			}
			return nil, fmt.Errorf("unknown backend: %s", nextBackendID)
		},
		ResponseMerger: func(ctx context.Context, originalReq *http.Request, allResponses map[string][]byte) (*http.Response, error) {
			result := make(map[string]interface{})
			for k, v := range allResponses {
				var parsed interface{}
				json.Unmarshal(v, &parsed)
				result[k] = parsed
			}
			return MakeJSONResponse(http.StatusOK, result)
		},
	})

	req := httptest.NewRequest("GET", "/api/users", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	// All three stages should be present
	assert.Contains(t, result, "users")
	assert.Contains(t, result, "profiles")
	assert.Contains(t, result, "permissions")
}

func TestPipelineStrategy_NoPipelineConfig(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer backend.Close()

	backends := []*Backend{
		{ID: "b1", URL: backend.URL, Client: http.DefaultClient},
	}
	handler := NewCompositeHandler(backends, StrategyPipeline, 5*time.Second)
	// Intentionally not setting pipeline config

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestPipelineStrategy_DefaultMerger(t *testing.T) {
	// When no ResponseMerger is set, the default wraps responses by backend ID
	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"step":"one"}`))
	}))
	defer backend1.Close()

	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"step":"two"}`))
	}))
	defer backend2.Close()

	backends := []*Backend{
		{ID: "step1", URL: backend1.URL, Client: http.DefaultClient},
		{ID: "step2", URL: backend2.URL, Client: http.DefaultClient},
	}

	handler := NewCompositeHandler(backends, StrategyPipeline, 10*time.Second)
	handler.SetPipelineConfig(&PipelineConfig{
		RequestBuilder: func(ctx context.Context, originalReq *http.Request, previousResponses map[string][]byte, nextBackendID string) (*http.Request, error) {
			return http.NewRequestWithContext(ctx, "GET", backends[1].URL+"/step2", nil)
		},
		// No ResponseMerger - uses default
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	assert.Contains(t, result, "step1")
	assert.Contains(t, result, "step2")
}

func TestPipelineStrategy_SkipStage(t *testing.T) {
	// When PipelineRequestBuilder returns nil, the stage is skipped
	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":"from-stage1"}`))
	}))
	defer backend1.Close()

	callCount := 0
	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":"from-stage2"}`))
	}))
	defer backend2.Close()

	backends := []*Backend{
		{ID: "stage1", URL: backend1.URL, Client: http.DefaultClient},
		{ID: "stage2", URL: backend2.URL, Client: http.DefaultClient},
	}

	handler := NewCompositeHandler(backends, StrategyPipeline, 10*time.Second)
	handler.SetPipelineConfig(&PipelineConfig{
		RequestBuilder: func(ctx context.Context, originalReq *http.Request, previousResponses map[string][]byte, nextBackendID string) (*http.Request, error) {
			// Skip stage2 by returning nil
			return nil, nil
		},
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Stage 2 should not have been called
	assert.Equal(t, 0, callCount, "stage2 should not have been called")

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	assert.Contains(t, result, "stage1")
	assert.NotContains(t, result, "stage2")
}

func TestPipelineStrategy_BackendError(t *testing.T) {
	// First backend succeeds, second fails
	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":"from-stage1"}`))
	}))
	defer backend1.Close()

	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer backend2.Close()

	backends := []*Backend{
		{ID: "stage1", URL: backend1.URL, Client: http.DefaultClient},
		{ID: "stage2", URL: backend2.URL, Client: http.DefaultClient},
	}

	handler := NewCompositeHandler(backends, StrategyPipeline, 10*time.Second)
	handler.SetPipelineConfig(&PipelineConfig{
		RequestBuilder: func(ctx context.Context, originalReq *http.Request, previousResponses map[string][]byte, nextBackendID string) (*http.Request, error) {
			return http.NewRequestWithContext(ctx, "GET", backends[1].URL+"/test", nil)
		},
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	// Pipeline should still return stage1 results even if stage2 fails
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	assert.Contains(t, result, "stage1")
}

func TestPipelineStrategy_RequestBuilderError(t *testing.T) {
	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":"ok"}`))
	}))
	defer backend1.Close()

	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":"stage2"}`))
	}))
	defer backend2.Close()

	backends := []*Backend{
		{ID: "stage1", URL: backend1.URL, Client: http.DefaultClient},
		{ID: "stage2", URL: backend2.URL, Client: http.DefaultClient},
	}

	handler := NewCompositeHandler(backends, StrategyPipeline, 10*time.Second)
	handler.SetPipelineConfig(&PipelineConfig{
		RequestBuilder: func(ctx context.Context, originalReq *http.Request, previousResponses map[string][]byte, nextBackendID string) (*http.Request, error) {
			return nil, fmt.Errorf("intentional builder error")
		},
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	// Should still return stage1 data despite stage2 builder error
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// ============================================================================
// Fan-Out-Merge Strategy Tests
// ============================================================================

func TestFanOutMerge_IDBasedMerging(t *testing.T) {
	// Backend A returns conversations
	backendA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []map[string]interface{}{
				{"id": "item-1", "name": "Item One", "status": "active"},
				{"id": "item-2", "name": "Item Two", "status": "pending"},
				{"id": "item-3", "name": "Item Three", "status": "active"},
			},
		})
	}))
	defer backendA.Close()

	// Backend B returns ancillary details (some items may not be present)
	backendB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"details": map[string]interface{}{
				"item-1": map[string]interface{}{"priority": "high", "assignee": "Alice"},
				"item-3": map[string]interface{}{"priority": "low", "assignee": "Bob"},
			},
		})
	}))
	defer backendB.Close()

	backends := []*Backend{
		{ID: "items", URL: backendA.URL, Client: http.DefaultClient},
		{ID: "details", URL: backendB.URL, Client: http.DefaultClient},
	}

	handler := NewCompositeHandler(backends, StrategyFanOutMerge, 10*time.Second)
	handler.SetFanOutMerger(func(ctx context.Context, originalReq *http.Request, responses map[string][]byte) (*http.Response, error) {
		// Parse items
		var itemsResp struct {
			Items []map[string]interface{} `json:"items"`
		}
		if body, ok := responses["items"]; ok {
			json.Unmarshal(body, &itemsResp)
		}

		// Parse details
		var detailsResp struct {
			Details map[string]interface{} `json:"details"`
		}
		if body, ok := responses["details"]; ok {
			json.Unmarshal(body, &detailsResp)
		}

		// Merge by ID
		for i, item := range itemsResp.Items {
			if id, ok := item["id"].(string); ok {
				if detail, exists := detailsResp.Details[id]; exists {
					itemsResp.Items[i]["details"] = detail
				}
			}
		}

		return MakeJSONResponse(http.StatusOK, map[string]interface{}{
			"items": itemsResp.Items,
		})
	})

	req := httptest.NewRequest("GET", "/api/items", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	items := result["items"].([]interface{})
	assert.Len(t, items, 3)

	// item-1 should have details
	item1 := items[0].(map[string]interface{})
	assert.Equal(t, "item-1", item1["id"])
	assert.NotNil(t, item1["details"])

	// item-2 should NOT have details
	item2 := items[1].(map[string]interface{})
	assert.Equal(t, "item-2", item2["id"])
	_, hasDetails := item2["details"]
	assert.False(t, hasDetails)

	// item-3 should have details
	item3 := items[2].(map[string]interface{})
	assert.Equal(t, "item-3", item3["id"])
	assert.NotNil(t, item3["details"])
}

func TestFanOutMerge_FilterByAncillaryData(t *testing.T) {
	// Backend A returns all conversations
	backendA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"conversations": []map[string]interface{}{
				{"id": "c1", "title": "Conversation 1"},
				{"id": "c2", "title": "Conversation 2"},
				{"id": "c3", "title": "Conversation 3"},
			},
		})
	}))
	defer backendA.Close()

	// Backend B returns which conversations are follow-ups (acts as filter)
	backendB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"follow_up_ids": []string{"c1", "c3"},
		})
	}))
	defer backendB.Close()

	backends := []*Backend{
		{ID: "conversations", URL: backendA.URL, Client: http.DefaultClient},
		{ID: "followups", URL: backendB.URL, Client: http.DefaultClient},
	}

	handler := NewCompositeHandler(backends, StrategyFanOutMerge, 10*time.Second)
	handler.SetFanOutMerger(func(ctx context.Context, originalReq *http.Request, responses map[string][]byte) (*http.Response, error) {
		var convResp struct {
			Conversations []map[string]interface{} `json:"conversations"`
		}
		if body, ok := responses["conversations"]; ok {
			json.Unmarshal(body, &convResp)
		}

		var fuResp struct {
			FollowUpIDs []string `json:"follow_up_ids"`
		}
		if body, ok := responses["followups"]; ok {
			json.Unmarshal(body, &fuResp)
		}

		// Create lookup set
		followUpSet := make(map[string]bool)
		for _, id := range fuResp.FollowUpIDs {
			followUpSet[id] = true
		}

		// Filter: only include conversations that are follow-ups
		var filtered []map[string]interface{}
		for _, conv := range convResp.Conversations {
			if id, ok := conv["id"].(string); ok && followUpSet[id] {
				conv["is_follow_up"] = true
				filtered = append(filtered, conv)
			}
		}

		return MakeJSONResponse(http.StatusOK, map[string]interface{}{
			"follow_up_conversations": filtered,
		})
	})

	req := httptest.NewRequest("GET", "/api/followups", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	convs := result["follow_up_conversations"].([]interface{})
	assert.Len(t, convs, 2, "only c1 and c3 should be included")

	ids := make([]string, 0, len(convs))
	for _, c := range convs {
		ids = append(ids, c.(map[string]interface{})["id"].(string))
	}
	assert.Contains(t, ids, "c1")
	assert.Contains(t, ids, "c3")
}

func TestFanOutMerge_NoMerger(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer backend.Close()

	backends := []*Backend{
		{ID: "b1", URL: backend.URL, Client: http.DefaultClient},
	}

	handler := NewCompositeHandler(backends, StrategyFanOutMerge, 5*time.Second)
	// Intentionally not setting fan-out merger

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestFanOutMerge_ThreeBackends(t *testing.T) {
	// Three backends returning different types of data for the same entity
	backendUsers := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"users": []map[string]interface{}{
				{"id": "u1", "name": "Alice"},
				{"id": "u2", "name": "Bob"},
			},
		})
	}))
	defer backendUsers.Close()

	backendRoles := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"roles": map[string]string{
				"u1": "admin",
				"u2": "user",
			},
		})
	}))
	defer backendRoles.Close()

	backendActivity := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"activity": map[string]int{
				"u1": 42,
				"u2": 7,
			},
		})
	}))
	defer backendActivity.Close()

	backends := []*Backend{
		{ID: "users", URL: backendUsers.URL, Client: http.DefaultClient},
		{ID: "roles", URL: backendRoles.URL, Client: http.DefaultClient},
		{ID: "activity", URL: backendActivity.URL, Client: http.DefaultClient},
	}

	handler := NewCompositeHandler(backends, StrategyFanOutMerge, 10*time.Second)
	handler.SetFanOutMerger(func(ctx context.Context, originalReq *http.Request, responses map[string][]byte) (*http.Response, error) {
		var usersResp struct {
			Users []map[string]interface{} `json:"users"`
		}
		json.Unmarshal(responses["users"], &usersResp)

		var rolesResp struct {
			Roles map[string]string `json:"roles"`
		}
		json.Unmarshal(responses["roles"], &rolesResp)

		var activityResp struct {
			Activity map[string]float64 `json:"activity"`
		}
		json.Unmarshal(responses["activity"], &activityResp)

		// Enrich users with roles and activity
		for i, user := range usersResp.Users {
			id := user["id"].(string)
			if role, ok := rolesResp.Roles[id]; ok {
				usersResp.Users[i]["role"] = role
			}
			if count, ok := activityResp.Activity[id]; ok {
				usersResp.Users[i]["activity_count"] = count
			}
		}

		return MakeJSONResponse(http.StatusOK, map[string]interface{}{
			"enriched_users": usersResp.Users,
		})
	})

	req := httptest.NewRequest("GET", "/api/users", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	users := result["enriched_users"].([]interface{})
	assert.Len(t, users, 2)

	u1 := users[0].(map[string]interface{})
	assert.Equal(t, "Alice", u1["name"])
	assert.Equal(t, "admin", u1["role"])
	assert.Equal(t, float64(42), u1["activity_count"])

	u2 := users[1].(map[string]interface{})
	assert.Equal(t, "Bob", u2["name"])
	assert.Equal(t, "user", u2["role"])
	assert.Equal(t, float64(7), u2["activity_count"])
}

// ============================================================================
// Empty Response Policy Tests
// ============================================================================

func TestEmptyResponsePolicy_AllowEmpty_Pipeline(t *testing.T) {
	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":"stage1"}`))
	}))
	defer backend1.Close()

	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`)) // Empty JSON object
	}))
	defer backend2.Close()

	backends := []*Backend{
		{ID: "s1", URL: backend1.URL, Client: http.DefaultClient},
		{ID: "s2", URL: backend2.URL, Client: http.DefaultClient},
	}

	handler := NewCompositeHandler(backends, StrategyPipeline, 10*time.Second)
	handler.SetEmptyResponsePolicy(EmptyResponseAllow)
	handler.SetPipelineConfig(&PipelineConfig{
		RequestBuilder: func(ctx context.Context, originalReq *http.Request, previousResponses map[string][]byte, nextBackendID string) (*http.Request, error) {
			return http.NewRequestWithContext(ctx, "GET", backends[1].URL+"/test", nil)
		},
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	// Both stages should be present
	assert.Contains(t, result, "s1")
	assert.Contains(t, result, "s2")
}

func TestEmptyResponsePolicy_SkipEmpty_Pipeline(t *testing.T) {
	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":"stage1"}`))
	}))
	defer backend1.Close()

	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`)) // Empty JSON object
	}))
	defer backend2.Close()

	backends := []*Backend{
		{ID: "s1", URL: backend1.URL, Client: http.DefaultClient},
		{ID: "s2", URL: backend2.URL, Client: http.DefaultClient},
	}

	handler := NewCompositeHandler(backends, StrategyPipeline, 10*time.Second)
	handler.SetEmptyResponsePolicy(EmptyResponseSkip)
	handler.SetPipelineConfig(&PipelineConfig{
		RequestBuilder: func(ctx context.Context, originalReq *http.Request, previousResponses map[string][]byte, nextBackendID string) (*http.Request, error) {
			return http.NewRequestWithContext(ctx, "GET", backends[1].URL+"/test", nil)
		},
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	// Only s1 should be present; s2 was empty and skipped
	assert.Contains(t, result, "s1")
	assert.NotContains(t, result, "s2")
}

func TestEmptyResponsePolicy_FailOnEmpty_Pipeline(t *testing.T) {
	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":"stage1"}`))
	}))
	defer backend1.Close()

	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`)) // Empty JSON array
	}))
	defer backend2.Close()

	backends := []*Backend{
		{ID: "s1", URL: backend1.URL, Client: http.DefaultClient},
		{ID: "s2", URL: backend2.URL, Client: http.DefaultClient},
	}

	handler := NewCompositeHandler(backends, StrategyPipeline, 10*time.Second)
	handler.SetEmptyResponsePolicy(EmptyResponseFail)
	handler.SetPipelineConfig(&PipelineConfig{
		RequestBuilder: func(ctx context.Context, originalReq *http.Request, previousResponses map[string][]byte, nextBackendID string) (*http.Request, error) {
			return http.NewRequestWithContext(ctx, "GET", backends[1].URL+"/test", nil)
		},
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	// Should fail because stage 2 returned empty
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
}

func TestEmptyResponsePolicy_SkipEmpty_FanOutMerge(t *testing.T) {
	backendA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []string{"a", "b", "c"},
		})
	}))
	defer backendA.Close()

	backendB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`null`)) // Empty/null response
	}))
	defer backendB.Close()

	backends := []*Backend{
		{ID: "primary", URL: backendA.URL, Client: http.DefaultClient},
		{ID: "ancillary", URL: backendB.URL, Client: http.DefaultClient},
	}

	handler := NewCompositeHandler(backends, StrategyFanOutMerge, 10*time.Second)
	handler.SetEmptyResponsePolicy(EmptyResponseSkip)
	handler.SetFanOutMerger(func(ctx context.Context, originalReq *http.Request, responses map[string][]byte) (*http.Response, error) {
		// ancillary should be skipped
		_, hasAncillary := responses["ancillary"]
		return MakeJSONResponse(http.StatusOK, map[string]interface{}{
			"has_ancillary": hasAncillary,
			"backends":      len(responses),
		})
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	assert.Equal(t, false, result["has_ancillary"])
	assert.Equal(t, float64(1), result["backends"])
}

func TestEmptyResponsePolicy_FailOnEmpty_FanOutMerge(t *testing.T) {
	backendA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":"ok"}`))
	}))
	defer backendA.Close()

	backendB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(``)) // Completely empty body
	}))
	defer backendB.Close()

	backends := []*Backend{
		{ID: "primary", URL: backendA.URL, Client: http.DefaultClient},
		{ID: "ancillary", URL: backendB.URL, Client: http.DefaultClient},
	}

	handler := NewCompositeHandler(backends, StrategyFanOutMerge, 10*time.Second)
	handler.SetEmptyResponsePolicy(EmptyResponseFail)
	handler.SetFanOutMerger(func(ctx context.Context, originalReq *http.Request, responses map[string][]byte) (*http.Response, error) {
		return MakeJSONResponse(http.StatusOK, responses)
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
}

// ============================================================================
// isEmptyBody Tests
// ============================================================================

func TestIsEmptyBody(t *testing.T) {
	tests := []struct {
		name     string
		body     []byte
		expected bool
	}{
		{"nil body", nil, true},
		{"empty body", []byte{}, true},
		{"whitespace only", []byte("   \n\t  "), true},
		{"null JSON", []byte("null"), true},
		{"empty object", []byte("{}"), true},
		{"empty array", []byte("[]"), true},
		{"null with whitespace", []byte("  null  "), true},
		{"non-empty object", []byte(`{"key":"value"}`), false},
		{"non-empty array", []byte(`[1,2,3]`), false},
		{"string value", []byte(`"hello"`), false},
		{"number value", []byte(`42`), false},
		{"boolean true", []byte(`true`), false},
		{"object with empty string", []byte(`{"key":""}`), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isEmptyBody(tt.body)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ============================================================================
// MakeJSONResponse Helper Tests
// ============================================================================

func TestMakeJSONResponse(t *testing.T) {
	data := map[string]interface{}{
		"key": "value",
		"num": 42,
	}

	resp, err := MakeJSONResponse(http.StatusOK, data)
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var result map[string]interface{}
	json.Unmarshal(body, &result)
	assert.Equal(t, "value", result["key"])
	assert.Equal(t, float64(42), result["num"])
}

func TestMakeJSONResponse_CustomStatusCode(t *testing.T) {
	resp, err := MakeJSONResponse(http.StatusCreated, map[string]string{"status": "created"})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()
}

// ============================================================================
// Complex Scenario Tests
// ============================================================================

func TestPipelineStrategy_ConversationListWithFollowUps(t *testing.T) {
	// Scenario from the issue: list page with queued conversations
	// Backend A has general conversation details
	// Backend B has ancillary details (follow-ups)

	conversationsBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"conversations": []map[string]interface{}{
				{"id": "c-100", "status": "queued", "counselor": "Alice", "created_at": "2024-01-01T10:00:00Z"},
				{"id": "c-101", "status": "queued", "counselor": "Bob", "created_at": "2024-01-01T10:05:00Z"},
				{"id": "c-102", "status": "active", "counselor": "Carol", "created_at": "2024-01-01T10:10:00Z"},
				{"id": "c-103", "status": "queued", "counselor": nil, "created_at": "2024-01-01T10:15:00Z"},
			},
			"total": 4,
			"page":  1,
		})
	}))
	defer conversationsBackend.Close()

	followUpBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This backend receives specific conversation IDs to check
		idsParam := r.URL.Query().Get("conversation_ids")
		ids := strings.Split(idsParam, ",")

		followUps := make(map[string]interface{})
		// c-100 is a follow-up to c-50
		for _, id := range ids {
			if id == "c-100" {
				followUps[id] = map[string]interface{}{
					"is_follow_up":     true,
					"original_conv_id": "c-50",
					"follow_up_count":  2,
				}
			}
			if id == "c-103" {
				followUps[id] = map[string]interface{}{
					"is_follow_up":     true,
					"original_conv_id": "c-90",
					"follow_up_count":  1,
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"follow_ups": followUps,
		})
	}))
	defer followUpBackend.Close()

	backends := []*Backend{
		{ID: "conversations", URL: conversationsBackend.URL, Client: http.DefaultClient},
		{ID: "followups", URL: followUpBackend.URL, Client: http.DefaultClient},
	}

	handler := NewCompositeHandler(backends, StrategyPipeline, 10*time.Second)
	handler.SetPipelineConfig(&PipelineConfig{
		RequestBuilder: func(ctx context.Context, originalReq *http.Request, previousResponses map[string][]byte, nextBackendID string) (*http.Request, error) {
			if nextBackendID == "followups" {
				// Extract conversation IDs from the first response
				var convResp struct {
					Conversations []struct {
						ID string `json:"id"`
					} `json:"conversations"`
				}
				if body, ok := previousResponses["conversations"]; ok {
					if err := json.Unmarshal(body, &convResp); err != nil {
						return nil, err
					}
				}

				ids := make([]string, 0, len(convResp.Conversations))
				for _, c := range convResp.Conversations {
					ids = append(ids, c.ID)
				}

				url := followUpBackend.URL + "/followups?conversation_ids=" + strings.Join(ids, ",")
				return http.NewRequestWithContext(ctx, "GET", url, nil)
			}
			return nil, fmt.Errorf("unknown backend: %s", nextBackendID)
		},
		ResponseMerger: func(ctx context.Context, originalReq *http.Request, allResponses map[string][]byte) (*http.Response, error) {
			var convResp struct {
				Conversations []map[string]interface{} `json:"conversations"`
				Total         int                      `json:"total"`
				Page          int                      `json:"page"`
			}
			json.Unmarshal(allResponses["conversations"], &convResp)

			var fuResp struct {
				FollowUps map[string]interface{} `json:"follow_ups"`
			}
			json.Unmarshal(allResponses["followups"], &fuResp)

			// Enrich conversations with follow-up data
			for i, conv := range convResp.Conversations {
				if id, ok := conv["id"].(string); ok {
					if fu, exists := fuResp.FollowUps[id]; exists {
						convResp.Conversations[i]["follow_up_info"] = fu
					}
				}
			}

			return MakeJSONResponse(http.StatusOK, map[string]interface{}{
				"conversations": convResp.Conversations,
				"total":         convResp.Total,
				"page":          convResp.Page,
			})
		},
	})

	req := httptest.NewRequest("GET", "/api/conversations?status=queued", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	conversations := result["conversations"].([]interface{})
	assert.Len(t, conversations, 4)

	// c-100 should have follow_up_info
	c100 := conversations[0].(map[string]interface{})
	assert.Equal(t, "c-100", c100["id"])
	fuInfo, hasFU := c100["follow_up_info"]
	assert.True(t, hasFU)
	fuMap := fuInfo.(map[string]interface{})
	assert.Equal(t, true, fuMap["is_follow_up"])
	assert.Equal(t, "c-50", fuMap["original_conv_id"])

	// c-101 should NOT have follow_up_info
	c101 := conversations[1].(map[string]interface{})
	_, hasFU101 := c101["follow_up_info"]
	assert.False(t, hasFU101)

	// c-103 should have follow_up_info
	c103 := conversations[3].(map[string]interface{})
	assert.Equal(t, "c-103", c103["id"])
	_, hasFU103 := c103["follow_up_info"]
	assert.True(t, hasFU103)
}

func TestFanOutMerge_ComplexNestedResponses(t *testing.T) {
	// Complex scenario: merging nested JSON structures
	backendA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"orders": []map[string]interface{}{
				{
					"id":     "ord-1",
					"amount": 99.99,
					"items": []map[string]interface{}{
						{"sku": "SKU-001", "qty": 2},
						{"sku": "SKU-002", "qty": 1},
					},
				},
				{
					"id":     "ord-2",
					"amount": 149.50,
					"items": []map[string]interface{}{
						{"sku": "SKU-003", "qty": 3},
					},
				},
			},
		})
	}))
	defer backendA.Close()

	backendB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"shipping": map[string]interface{}{
				"ord-1": map[string]interface{}{
					"status":   "shipped",
					"tracking": "TRACK-12345",
					"carrier":  "FedEx",
				},
				"ord-2": map[string]interface{}{
					"status":   "processing",
					"tracking": "",
					"carrier":  "",
				},
			},
		})
	}))
	defer backendB.Close()

	backends := []*Backend{
		{ID: "orders", URL: backendA.URL, Client: http.DefaultClient},
		{ID: "shipping", URL: backendB.URL, Client: http.DefaultClient},
	}

	handler := NewCompositeHandler(backends, StrategyFanOutMerge, 10*time.Second)
	handler.SetFanOutMerger(func(ctx context.Context, originalReq *http.Request, responses map[string][]byte) (*http.Response, error) {
		var ordersResp struct {
			Orders []map[string]interface{} `json:"orders"`
		}
		json.Unmarshal(responses["orders"], &ordersResp)

		var shippingResp struct {
			Shipping map[string]interface{} `json:"shipping"`
		}
		json.Unmarshal(responses["shipping"], &shippingResp)

		for i, order := range ordersResp.Orders {
			if id, ok := order["id"].(string); ok {
				if shipping, exists := shippingResp.Shipping[id]; exists {
					ordersResp.Orders[i]["shipping"] = shipping
				}
			}
		}

		return MakeJSONResponse(http.StatusOK, map[string]interface{}{
			"orders": ordersResp.Orders,
		})
	})

	req := httptest.NewRequest("GET", "/api/orders", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	orders := result["orders"].([]interface{})
	assert.Len(t, orders, 2)

	ord1 := orders[0].(map[string]interface{})
	shipping := ord1["shipping"].(map[string]interface{})
	assert.Equal(t, "shipped", shipping["status"])
	assert.Equal(t, "TRACK-12345", shipping["tracking"])
}

func TestFanOutMerge_EmptyAncillaryData_AllowPolicy(t *testing.T) {
	// Backend A returns data, Backend B returns empty (no ancillary data exists)
	// With allow-empty policy, merger should handle gracefully
	backendA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []map[string]interface{}{
				{"id": "x1", "name": "X1"},
			},
		})
	}))
	defer backendA.Close()

	backendB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`)) // Empty response - no ancillary data
	}))
	defer backendB.Close()

	backends := []*Backend{
		{ID: "primary", URL: backendA.URL, Client: http.DefaultClient},
		{ID: "ancillary", URL: backendB.URL, Client: http.DefaultClient},
	}

	handler := NewCompositeHandler(backends, StrategyFanOutMerge, 10*time.Second)
	handler.SetEmptyResponsePolicy(EmptyResponseAllow)
	handler.SetFanOutMerger(func(ctx context.Context, originalReq *http.Request, responses map[string][]byte) (*http.Response, error) {
		// Both responses should be present
		_, hasPrimary := responses["primary"]
		_, hasAncillary := responses["ancillary"]

		return MakeJSONResponse(http.StatusOK, map[string]interface{}{
			"primary_present":   hasPrimary,
			"ancillary_present": hasAncillary,
		})
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	assert.Equal(t, true, result["primary_present"])
	assert.Equal(t, true, result["ancillary_present"])
}

func TestPipelineStrategy_WithRequestBody(t *testing.T) {
	// Test pipeline with POST request containing body
	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var input map[string]interface{}
		json.Unmarshal(body, &input)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"processed": true,
			"input":     input,
			"result_id": "res-123",
		})
	}))
	defer backend1.Close()

	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var input map[string]interface{}
		json.Unmarshal(body, &input)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"stored":    true,
			"result_id": input["result_id"],
		})
	}))
	defer backend2.Close()

	backends := []*Backend{
		{ID: "process", URL: backend1.URL, Client: http.DefaultClient},
		{ID: "store", URL: backend2.URL, Client: http.DefaultClient},
	}

	handler := NewCompositeHandler(backends, StrategyPipeline, 10*time.Second)
	handler.SetPipelineConfig(&PipelineConfig{
		RequestBuilder: func(ctx context.Context, originalReq *http.Request, previousResponses map[string][]byte, nextBackendID string) (*http.Request, error) {
			if nextBackendID == "store" {
				// Parse the result_id from process response
				var processResp struct {
					ResultID string `json:"result_id"`
				}
				json.Unmarshal(previousResponses["process"], &processResp)

				// Build store request with result_id
				storeBody, _ := json.Marshal(map[string]interface{}{
					"result_id": processResp.ResultID,
					"action":    "save",
				})
				req, _ := http.NewRequestWithContext(ctx, "POST", backends[1].URL+"/store",
					bytes.NewReader(storeBody))
				req.Header.Set("Content-Type", "application/json")
				return req, nil
			}
			return nil, fmt.Errorf("unknown backend: %s", nextBackendID)
		},
		ResponseMerger: func(ctx context.Context, originalReq *http.Request, allResponses map[string][]byte) (*http.Response, error) {
			result := make(map[string]interface{})
			for k, v := range allResponses {
				var parsed interface{}
				json.Unmarshal(v, &parsed)
				result[k] = parsed
			}
			return MakeJSONResponse(http.StatusOK, result)
		},
	})

	inputBody := `{"data":"test-payload"}`
	req := httptest.NewRequest("POST", "/api/process", strings.NewReader(inputBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	// Verify process stage
	processResult := result["process"].(map[string]interface{})
	assert.Equal(t, true, processResult["processed"])
	assert.Equal(t, "res-123", processResult["result_id"])

	// Verify store stage received the result_id from process stage
	storeResult := result["store"].(map[string]interface{})
	assert.Equal(t, true, storeResult["stored"])
	assert.Equal(t, "res-123", storeResult["result_id"])
}

// ============================================================================
// Module Integration Tests
// ============================================================================

func TestModuleSetPipelineConfig(t *testing.T) {
	module := NewModule()

	config := PipelineConfig{
		RequestBuilder: func(ctx context.Context, originalReq *http.Request, previousResponses map[string][]byte, nextBackendID string) (*http.Request, error) {
			return nil, nil
		},
	}

	module.SetPipelineConfig("/api/pipeline", config)
	assert.NotNil(t, module.pipelineConfigs["/api/pipeline"])
}

func TestModuleSetFanOutMerger(t *testing.T) {
	module := NewModule()

	merger := func(ctx context.Context, originalReq *http.Request, responses map[string][]byte) (*http.Response, error) {
		return MakeJSONResponse(http.StatusOK, responses)
	}

	module.SetFanOutMerger("/api/fanout", merger)
	assert.NotNil(t, module.fanOutMergers["/api/fanout"])
}

func TestModuleSetEmptyResponsePolicy(t *testing.T) {
	module := NewModule()

	module.SetEmptyResponsePolicy("/api/test", EmptyResponseSkip)
	assert.Equal(t, EmptyResponseSkip, module.emptyResponsePolicies["/api/test"])
}

// ============================================================================
// Edge Case Tests
// ============================================================================

func TestPipeline_AllBackendsFail(t *testing.T) {
	// When all backends fail, we should get a bad gateway
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer backend.Close()

	backends := []*Backend{
		{ID: "b1", URL: "http://localhost:1", Client: &http.Client{Timeout: 100 * time.Millisecond}}, // Will fail
	}

	handler := NewCompositeHandler(backends, StrategyPipeline, 5*time.Second)
	handler.SetPipelineConfig(&PipelineConfig{
		RequestBuilder: func(ctx context.Context, originalReq *http.Request, previousResponses map[string][]byte, nextBackendID string) (*http.Request, error) {
			return nil, nil
		},
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
}

func TestFanOutMerge_SingleBackend(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"single": true})
	}))
	defer backend.Close()

	backends := []*Backend{
		{ID: "only", URL: backend.URL, Client: http.DefaultClient},
	}

	handler := NewCompositeHandler(backends, StrategyFanOutMerge, 10*time.Second)
	handler.SetFanOutMerger(func(ctx context.Context, originalReq *http.Request, responses map[string][]byte) (*http.Response, error) {
		var data interface{}
		json.Unmarshal(responses["only"], &data)
		return MakeJSONResponse(http.StatusOK, data)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	assert.Equal(t, true, result["single"])
}

func TestFanOutMerge_MergerError(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer backend.Close()

	backends := []*Backend{
		{ID: "b1", URL: backend.URL, Client: http.DefaultClient},
	}

	handler := NewCompositeHandler(backends, StrategyFanOutMerge, 10*time.Second)
	handler.SetFanOutMerger(func(ctx context.Context, originalReq *http.Request, responses map[string][]byte) (*http.Response, error) {
		return nil, fmt.Errorf("intentional merger error")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestPipelineStrategy_ResponseMergerError(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer backend.Close()

	backends := []*Backend{
		{ID: "b1", URL: backend.URL, Client: http.DefaultClient},
	}

	handler := NewCompositeHandler(backends, StrategyPipeline, 10*time.Second)
	handler.SetPipelineConfig(&PipelineConfig{
		RequestBuilder: func(ctx context.Context, originalReq *http.Request, previousResponses map[string][]byte, nextBackendID string) (*http.Request, error) {
			return nil, nil
		},
		ResponseMerger: func(ctx context.Context, originalReq *http.Request, allResponses map[string][]byte) (*http.Response, error) {
			return nil, fmt.Errorf("intentional merger error")
		},
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}
