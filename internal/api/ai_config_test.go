package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eddiecarpenter/ocs-testbench/internal/ai"
	"github.com/eddiecarpenter/ocs-testbench/internal/api"
)

// newAIConfigRouter builds a minimal chi router with only the AI config
// endpoints wired. agent may be nil to test the disabled state.
func newAIConfigRouter(agent *ai.Agent) http.Handler {
	r := chi.NewRouter()
	r.Route("/v1", func(v1 chi.Router) {
		api.MountAIConfig(v1, agent)
	})
	return r
}

// newModelsServer creates an httptest.Server that responds to GET /v1/models
// with an OpenAI-compatible model list. Returns the server and its base URL.
func newModelsServer(t *testing.T, modelIDs []string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		data := make([]map[string]any, 0, len(modelIDs))
		for _, id := range modelIDs {
			data = append(data, map[string]any{"id": id})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"data": data}) //nolint:errcheck
	}))
	t.Cleanup(srv.Close)
	return srv
}

// ---- GET /v1/config/ai -------------------------------------------------------

// TestGetAIConfig_NilAgent returns 200 with a zero-value response when the
// agent is not configured.
func TestGetAIConfig_NilAgent(t *testing.T) {
	r := newAIConfigRouter(nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/config/ai", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var body map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Equal(t, "", body["endpoint"])
	assert.Equal(t, "", body["model"])
	assert.Equal(t, false, body["apiKeySet"])
}

// TestGetAIConfig_WithAgent returns the configured endpoint and model with the
// API key masked as a boolean flag.
func TestGetAIConfig_WithAgent(t *testing.T) {
	// Spin up a fake LLM server so NewAgent succeeds (non-empty endpoint).
	llmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(llmSrv.Close)

	cfg := ai.MakeAIConfigWithKey(llmSrv.URL, "test-api-key")
	agent := ai.NewAgent(cfg, "http://localhost:8080")
	require.NotNil(t, agent)

	r := newAIConfigRouter(agent)

	req := httptest.NewRequest(http.MethodGet, "/v1/config/ai", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var body map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Equal(t, llmSrv.URL, body["endpoint"])
	assert.Equal(t, true, body["apiKeySet"], "apiKeySet must be true when a key is configured")
	// The raw key must NOT appear anywhere in the response body.
	assert.NotContains(t, rr.Body.String(), "test-api-key", "API key must not be returned in the response")
}

// ---- PATCH /v1/config/ai -----------------------------------------------------

// TestUpdateAIConfig_NilAgent returns 503 when the agent is not configured.
func TestUpdateAIConfig_NilAgent(t *testing.T) {
	r := newAIConfigRouter(nil)

	body, _ := json.Marshal(map[string]any{"endpoint": "http://llm.example.com", "model": "gpt-4o"})
	req := httptest.NewRequest(http.MethodPatch, "/v1/config/ai", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}

// TestUpdateAIConfig_ValidEndpoint returns 200 and updates the agent config.
func TestUpdateAIConfig_ValidEndpoint(t *testing.T) {
	llmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(llmSrv.Close)

	agent := ai.NewAgent(ai.MakeAIConfig(llmSrv.URL), "http://localhost:8080")
	require.NotNil(t, agent)

	// New endpoint for the update.
	newEndpoint := llmSrv.URL + "/v2" // still points at the same test server
	body, _ := json.Marshal(map[string]any{
		"endpoint": newEndpoint,
		"model":    "gpt-4o-mini",
	})
	r := newAIConfigRouter(agent)
	req := httptest.NewRequest(http.MethodPatch, "/v1/config/ai", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, newEndpoint, resp["endpoint"])
	assert.Equal(t, "gpt-4o-mini", resp["model"])
}

// TestUpdateAIConfig_EmptyEndpointFails returns 400 when endpoint is empty.
func TestUpdateAIConfig_EmptyEndpointFails(t *testing.T) {
	llmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(llmSrv.Close)

	agent := ai.NewAgent(ai.MakeAIConfig(llmSrv.URL), "http://localhost:8080")
	require.NotNil(t, agent)

	body, _ := json.Marshal(map[string]any{"endpoint": "", "model": "gpt-4o"})
	r := newAIConfigRouter(agent)
	req := httptest.NewRequest(http.MethodPatch, "/v1/config/ai", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestUpdateAIConfig_BlankAPIKeyPreservesExisting verifies that sending a
// blank apiKey field does not clear the existing key.
func TestUpdateAIConfig_BlankAPIKeyPreservesExisting(t *testing.T) {
	llmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(llmSrv.Close)

	agent := ai.NewAgent(ai.MakeAIConfigWithKey(llmSrv.URL, "secret-key"), "http://localhost:8080")
	require.NotNil(t, agent)

	// PATCH without apiKey — should preserve "secret-key".
	body, _ := json.Marshal(map[string]any{
		"endpoint": llmSrv.URL,
		"model":    "gpt-4o",
		// apiKey intentionally omitted (zero value → blank)
	})
	r := newAIConfigRouter(agent)
	req := httptest.NewRequest(http.MethodPatch, "/v1/config/ai", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, true, resp["apiKeySet"], "existing API key must be preserved when blank is sent")

	// Verify the raw key is still set internally.
	assert.Equal(t, "secret-key", agent.GetRawAPIKey())
}

// ---- GET /v1/config/ai/models ------------------------------------------------

// TestListAIModels_NilAgent returns 503 when the agent is not configured.
func TestListAIModels_NilAgent(t *testing.T) {
	r := newAIConfigRouter(nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/config/ai/models", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}

// TestListAIModels_Success proxies the upstream model list and returns IDs.
func TestListAIModels_Success(t *testing.T) {
	modelIDs := []string{"gpt-4o", "gpt-4o-mini", "gpt-3.5-turbo"}
	modelsSrv := newModelsServer(t, modelIDs)

	agent := ai.NewAgent(ai.MakeAIConfig(modelsSrv.URL), "http://localhost:8080")
	require.NotNil(t, agent)

	r := newAIConfigRouter(agent)
	req := httptest.NewRequest(http.MethodGet, "/v1/config/ai/models", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var ids []string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&ids))
	assert.Equal(t, modelIDs, ids)
}

// TestListAIModels_UpstreamFailure returns 502 when the upstream server errors.
func TestListAIModels_UpstreamFailure(t *testing.T) {
	// Use a server that immediately returns 500.
	badSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(badSrv.Close)

	agent := ai.NewAgent(ai.MakeAIConfig(badSrv.URL), "http://localhost:8080")
	require.NotNil(t, agent)

	r := newAIConfigRouter(agent)
	req := httptest.NewRequest(http.MethodGet, "/v1/config/ai/models", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadGateway, rr.Code)
}
