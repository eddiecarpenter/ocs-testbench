package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// mountPeers registers all /peers and /peers/{id} routes on r.
// CRUD handlers are implemented here; connection-control handlers are
// added by Task 7 when the Manager dependency is wired in.
func mountPeers(r chi.Router, s store.Store) {
	r.Get("/peers", listPeers(s))
	r.Post("/peers", createPeer(s))
	r.Get("/peers/{id}", getPeer(s))
	r.Put("/peers/{id}", updatePeer(s))
	r.Delete("/peers/{id}", deletePeer(s))

	// Connection control — implemented by Task 7
	r.Post("/peers/{id}/connect", notImplemented)
	r.Post("/peers/{id}/disconnect", notImplemented)
	r.Get("/peers/{id}/status", notImplemented)
}

// peerRequest is the JSON request body for Peer create and update.
type peerRequest struct {
	Name string          `json:"name"`
	Body json.RawMessage `json:"body"`
}

// peerResponse is the JSON response shape for a Peer.
type peerResponse struct {
	ID   string          `json:"id"`
	Name string          `json:"name"`
	Body json.RawMessage `json:"body"`
}

// toPeerResponse converts a store.Peer to a peerResponse.
func toPeerResponse(p store.Peer) peerResponse {
	return peerResponse{
		ID:   uuidToString(p.ID),
		Name: p.Name,
		Body: p.Body,
	}
}

// listPeers handles GET /peers.
func listPeers(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		peers, err := s.ListPeers(r.Context())
		if err != nil {
			respondInternalError(w)
			return
		}
		out := make([]peerResponse, len(peers))
		for i, p := range peers {
			out[i] = toPeerResponse(p)
		}
		respondJSON(w, http.StatusOK, out)
	}
}

// createPeer handles POST /peers.
func createPeer(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req peerRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		if req.Name == "" {
			respondInvalidRequest(w, "name is required")
			return
		}
		peer, err := s.InsertPeer(r.Context(), req.Name, req.Body)
		if mapStoreError(w, err) != nil {
			return
		}
		respondJSON(w, http.StatusCreated, toPeerResponse(peer))
	}
}

// getPeer handles GET /peers/{id}.
func getPeer(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseUUIDParam(w, r, "id")
		if !ok {
			return
		}
		peer, err := s.GetPeer(r.Context(), id)
		if mapStoreError(w, err) != nil {
			return
		}
		respondJSON(w, http.StatusOK, toPeerResponse(peer))
	}
}

// updatePeer handles PUT /peers/{id}.
func updatePeer(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseUUIDParam(w, r, "id")
		if !ok {
			return
		}
		var req peerRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		if req.Name == "" {
			respondInvalidRequest(w, "name is required")
			return
		}
		peer, err := s.UpdatePeer(r.Context(), id, req.Name, req.Body)
		if mapStoreError(w, err) != nil {
			return
		}
		respondJSON(w, http.StatusOK, toPeerResponse(peer))
	}
}

// deletePeer handles DELETE /peers/{id}.
func deletePeer(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseUUIDParam(w, r, "id")
		if !ok {
			return
		}
		err := s.DeletePeer(r.Context(), id)
		if mapStoreError(w, err) != nil {
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
