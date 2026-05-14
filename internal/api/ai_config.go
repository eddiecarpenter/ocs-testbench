package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	internalaiai "github.com/eddiecarpenter/ocs-testbench/internal/ai"
	"github.com/eddiecarpenter/ocs-testbench/internal/baseconfig"
)

// mountAIConfig registers the AI configuration endpoints under /config/ai.
//
//	GET  /config/ai        — return current AI config (API key masked)
//	PATCH /config/ai       — update config and hot-reload the LLM client
//	GET  /config/ai/models — proxy {endpoint}/v1/models → list of model IDs
func mountAIConfig(r chi.Router, agent *internalaiai.Agent) {
	r.Get("/config/ai", getAIConfig(agent))
	r.Patch("/config/ai", updateAIConfig(agent))
	r.Get("/config/ai/models", listAIModels(agent))
}

// aiConfigResponse is the on-wire shape for GET and PATCH /config/ai.
// The API key is never returned; APIKeySet signals whether one is configured.
type aiConfigResponse struct {
	Endpoint  string `json:"endpoint"`
	Model     string `json:"model"`
	APIKeySet bool   `json:"apiKeySet"`
}

// aiConfigInput is the request body for PATCH /config/ai.
// When APIKey is blank the existing key is preserved.
type aiConfigInput struct {
	Endpoint string `json:"endpoint"`
	Model    string `json:"model"`
	APIKey   string `json:"apiKey"`
}

// getAIConfig implements GET /v1/config/ai.
// When agent is nil (endpoint not configured at startup) it returns a
// zero-value response — the frontend can display empty fields.
func getAIConfig(agent *internalaiai.Agent) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if agent == nil {
			respondJSON(w, http.StatusOK, aiConfigResponse{})
			return
		}
		cfg := agent.GetConfig()
		resp := aiConfigResponse{
			Endpoint:  cfg.Endpoint,
			Model:     cfg.Model,
			APIKeySet: cfg.APIKey != "",
		}
		respondJSON(w, http.StatusOK, resp)
	}
}

// updateAIConfig implements PATCH /v1/config/ai.
// Returns 503 when agent is nil — the binary was started with an empty
// endpoint; a restart with a valid endpoint is required to enable the
// feature from-disabled.
func updateAIConfig(agent *internalaiai.Agent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if agent == nil {
			respondError(w, http.StatusServiceUnavailable, CodeInternalError,
				"AI assistant not configured — restart with a valid endpoint to enable")
			return
		}

		var input aiConfigInput
		if !decodeJSON(w, r, &input) {
			return
		}

		// Preserve the existing API key when the caller did not supply one.
		// GetConfig masks the key as "••••" so we use GetRawAPIKey for the
		// round-trip value.
		apiKey := input.APIKey
		if apiKey == "" {
			apiKey = agent.GetRawAPIKey()
		}

		newCfg := baseconfig.AIConfig{
			Endpoint: input.Endpoint,
			Model:    input.Model,
			APIKey:   apiKey,
		}
		if err := agent.Reconfigure(newCfg); err != nil {
			respondError(w, http.StatusBadRequest, CodeInvalidRequest, err.Error())
			return
		}

		updated := agent.GetConfig()
		resp := aiConfigResponse{
			Endpoint:  updated.Endpoint,
			Model:     updated.Model,
			APIKeySet: updated.APIKey != "",
		}
		respondJSON(w, http.StatusOK, resp)
	}
}

// listAIModels implements GET /v1/config/ai/models.
// Proxies {endpoint}/v1/models (OpenAI-compatible) with a 5-second timeout.
// Returns 503 when agent is nil; 502 when the upstream call fails.
func listAIModels(agent *internalaiai.Agent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if agent == nil {
			respondError(w, http.StatusServiceUnavailable, CodeInternalError,
				"AI assistant not configured")
			return
		}

		cfg := agent.GetConfig()
		if cfg.Endpoint == "" {
			respondError(w, http.StatusServiceUnavailable, CodeInternalError,
				"AI endpoint not configured")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		url := cfg.Endpoint + "/v1/models"
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			respondError(w, http.StatusBadGateway, CodeInternalError,
				"failed to build models request: "+err.Error())
			return
		}
		// GetConfig masks the key; GetRawAPIKey provides it for the proxied request.
		if rawKey := agent.GetRawAPIKey(); rawKey != "" {
			req.Header.Set("Authorization", "Bearer "+rawKey)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			respondError(w, http.StatusBadGateway, CodeInternalError,
				"upstream models request failed: "+err.Error())
			return
		}
		defer resp.Body.Close() //nolint:errcheck

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			respondError(w, http.StatusBadGateway, CodeInternalError,
				"upstream models endpoint returned non-2xx status")
			return
		}

		// Parse OpenAI-compatible model list: {"data":[{"id":"..."},...]}
		var body struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			respondError(w, http.StatusBadGateway, CodeInternalError,
				"failed to parse upstream models response: "+err.Error())
			return
		}

		ids := make([]string, 0, len(body.Data))
		for _, m := range body.Data {
			ids = append(ids, m.ID)
		}
		respondJSON(w, http.StatusOK, ids)
	}
}
