package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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
	Thinking  bool   `json:"thinking"`
}

// aiConfigInput is the request body for PATCH /config/ai.
// When APIKey is blank the existing key is preserved.
type aiConfigInput struct {
	Endpoint string `json:"endpoint"`
	Model    string `json:"model"`
	APIKey   string `json:"apiKey"`
	Thinking bool   `json:"thinking"`
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
			Thinking:  cfg.Thinking,
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
			Thinking: input.Thinking,
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
			Thinking:  updated.Thinking,
		}
		respondJSON(w, http.StatusOK, resp)
	}
}

// anthropicModels is the fallback model list for api.anthropic.com when the
// endpoint rejects OAuth Bearer tokens on GET /v1/models (it requires x-api-key).
var anthropicModels = []string{
	"claude-opus-4-7",
	"claude-sonnet-4-6",
	"claude-haiku-4-5-20251001",
	"claude-3-5-sonnet-20241022",
	"claude-3-5-haiku-20241022",
	"claude-3-opus-20240229",
}

// listAIModels implements GET /v1/config/ai/models.
// Proxies {endpoint}/v1/models (OpenAI-compatible) with a 10-second timeout.
// An optional ?endpoint= query parameter overrides the saved config endpoint,
// allowing the frontend to fetch models for an endpoint that has not been saved
// yet (e.g. while the user is editing the provider field).
// For api.anthropic.com, falls back to a static model list when the upstream
// rejects the request (the /v1/models endpoint requires x-api-key; OAuth tokens
// from Claude Code are not accepted there).
// Returns 503 when agent is nil; 502 when the upstream call fails.
func listAIModels(agent *internalaiai.Agent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if agent == nil {
			respondError(w, http.StatusServiceUnavailable, CodeInternalError,
				"AI assistant not configured")
			return
		}

		cfg := agent.GetConfig()

		// Allow the caller to override endpoint and/or API key without saving
		// first — useful when the user is testing new credentials in the UI.
		if override := r.URL.Query().Get("endpoint"); override != "" {
			cfg.Endpoint = override
		}
		apiKey := agent.GetRawAPIKey()
		if override := r.URL.Query().Get("apiKey"); override != "" {
			apiKey = override
		}

		if cfg.Endpoint == "" {
			respondError(w, http.StatusServiceUnavailable, CodeInternalError,
				"AI endpoint not configured")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		url := cfg.Endpoint + "/v1/models"
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			respondError(w, http.StatusBadGateway, CodeInternalError,
				"failed to build models request: "+err.Error())
			return
		}
		// Delegate header logic to the same helper used by the LLM client.
		if apiKey != "" {
			internalaiai.SetAuthHeaders(req, cfg.Endpoint, apiKey)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			respondError(w, http.StatusBadGateway, CodeInternalError,
				"upstream models request failed: "+err.Error())
			return
		}
		defer resp.Body.Close() //nolint:errcheck

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			var buf bytes.Buffer
			_, _ = buf.ReadFrom(resp.Body)
			// Anthropic's /v1/models endpoint rejects OAuth Bearer tokens (Claude
			// Code credentials) with 401 — fall back to the static model list.
			if strings.Contains(cfg.Endpoint, "anthropic.com") {
				respondJSON(w, http.StatusOK, anthropicModels)
				return
			}
			respondError(w, http.StatusBadGateway, CodeInternalError,
				fmt.Sprintf("upstream returned HTTP %d: %s", resp.StatusCode, buf.String()))
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
