package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/eddiecarpenter/ocs-testbench/internal/diameter/manager"
	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// mountPeers registers all /peers and /peers/{id} routes on r.
// CRUD handlers are implemented here. Connection-control handlers
// require a non-nil mgr; if mgr is nil they return 503.
func mountPeers(r chi.Router, s store.Store, mgr PeerManager) {
	r.Get("/peers", listPeers(s))
	r.Post("/peers", createPeer(s))
	r.Get("/peers/{id}", getPeer(s))
	r.Put("/peers/{id}", updatePeer(s))
	r.Delete("/peers/{id}", deletePeer(s))

	// Connection control — implemented in this task.
	r.Post("/peers/{id}/connect", connectPeer(s, mgr))
	r.Post("/peers/{id}/disconnect", disconnectPeer(s, mgr))
	r.Get("/peers/{id}/status", peerStatus(s, mgr))
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

// peerStatusResponse is the JSON response shape for the peer status
// endpoint.
type peerStatusResponse struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
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

// connectPeer handles POST /peers/{id}/connect.
// Looks up the peer name from the store, then delegates to the Manager.
func connectPeer(s store.Store, mgr PeerManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if mgr == nil {
			managerUnavailable(w)
			return
		}
		id, ok := parseUUIDParam(w, r, "id")
		if !ok {
			return
		}
		peer, err := s.GetPeer(r.Context(), id)
		if mapStoreError(w, err) != nil {
			return
		}
		if err := mgr.Connect(peer.Name); err != nil {
			mapManagerError(w, err)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}
}

// disconnectPeer handles POST /peers/{id}/disconnect.
func disconnectPeer(s store.Store, mgr PeerManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if mgr == nil {
			managerUnavailable(w)
			return
		}
		id, ok := parseUUIDParam(w, r, "id")
		if !ok {
			return
		}
		peer, err := s.GetPeer(r.Context(), id)
		if mapStoreError(w, err) != nil {
			return
		}
		if err := mgr.Disconnect(peer.Name); err != nil {
			mapManagerError(w, err)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

// peerStatus handles GET /peers/{id}/status.
func peerStatus(s store.Store, mgr PeerManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if mgr == nil {
			managerUnavailable(w)
			return
		}
		id, ok := parseUUIDParam(w, r, "id")
		if !ok {
			return
		}
		peer, err := s.GetPeer(r.Context(), id)
		if mapStoreError(w, err) != nil {
			return
		}
		state, err := mgr.State(peer.Name)
		if err != nil {
			// Peer exists in store but not registered in Manager —
			// treat as stopped (not managed by the live stack).
			if errors.Is(err, manager.ErrUnknownPeer) {
				respondJSON(w, http.StatusOK, peerStatusResponse{
					ID:     uuidToString(id),
					Name:   peer.Name,
					Status: "stopped",
				})
				return
			}
			mapManagerError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, peerStatusResponse{
			ID:     uuidToString(id),
			Name:   peer.Name,
			Status: state.String(),
		})
	}
}

// mapManagerError maps a manager error to the appropriate HTTP response.
func mapManagerError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, manager.ErrUnknownPeer):
		respondNotFoundMsg(w, "peer not found in connection manager")
	case errors.Is(err, manager.ErrNotStarted):
		respondError(w, http.StatusServiceUnavailable, CodeInternalError, "manager not started")
	case errors.Is(err, manager.ErrStopped):
		respondError(w, http.StatusServiceUnavailable, CodeInternalError, "manager stopped")
	default:
		respondInternalError(w)
	}
}
