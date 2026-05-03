package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/eddiecarpenter/ocs-testbench/internal/diameter"
	"github.com/eddiecarpenter/ocs-testbench/internal/diameter/manager"
	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// mountPeers registers all /peers and /peers/{id} routes on r.
// CRUD handlers are implemented here. Connection-control handlers
// require a non-nil mgr; if mgr is nil they return 503.
func mountPeers(r chi.Router, s store.Store, mgr PeerManager) {
	r.Get("/peers", listPeers(s, mgr))
	r.Post("/peers", createPeer(s, mgr))
	r.Get("/peers/{id}", getPeer(s, mgr))
	r.Put("/peers/{id}", updatePeer(s, mgr))
	r.Delete("/peers/{id}", deletePeer(s))

	// Connection control.
	r.Post("/peers/{id}/connect", connectPeer(s, mgr))
	r.Post("/peers/{id}/disconnect", disconnectPeer(s, mgr))
	r.Post("/peers/{id}/start", startPeer(s, mgr))
	r.Post("/peers/{id}/stop", stopPeer(s, mgr))
	r.Get("/peers/{id}/status", peerStatus(s, mgr))
}

// peerRequest is the JSON request body for Peer create and update.
// Matches the PeerInput schema in api/openapi.yaml.
type peerRequest struct {
	Name                    string `json:"name"`
	Host                    string `json:"host"`
	Port                    int    `json:"port"`
	OriginHost              string `json:"originHost"`
	OriginRealm             string `json:"originRealm"`
	OriginIP                string `json:"originIp"`
	OriginPort              int    `json:"originPort"`
	Transport               string `json:"transport"`
	WatchdogIntervalSeconds int    `json:"watchdogIntervalSeconds"`
	AutoConnect             bool   `json:"autoConnect"`
}

// peerBody is the JSONB payload persisted in the store.
type peerBody struct {
	Host                    string `json:"host"`
	Port                    int    `json:"port"`
	OriginHost              string `json:"originHost"`
	OriginRealm             string `json:"originRealm"`
	OriginIP                string `json:"originIp"`
	OriginPort              int    `json:"originPort"`
	Transport               string `json:"transport"`
	WatchdogIntervalSeconds int    `json:"watchdogIntervalSeconds"`
	AutoConnect             bool   `json:"autoConnect"`
}

// peerResponse is the JSON response shape for a Peer.
// Matches the Peer schema in api/openapi.yaml.
type peerResponse struct {
	ID                      string `json:"id"`
	Name                    string `json:"name"`
	Host                    string `json:"host"`
	Port                    int    `json:"port"`
	OriginHost              string `json:"originHost"`
	OriginRealm             string `json:"originRealm"`
	OriginIP                string `json:"originIp"`
	OriginPort              int    `json:"originPort"`
	Transport               string `json:"transport"`
	WatchdogIntervalSeconds int    `json:"watchdogIntervalSeconds"`
	AutoConnect             bool   `json:"autoConnect"`
	Status                  string `json:"status"`
	LastChangeAt            string `json:"lastChangeAt,omitempty"`
}

// peerStatusResponse is the JSON response shape for the peer status
// endpoint.
type peerStatusResponse struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// toPeerResponse converts a store.Peer to a peerResponse.
// status is the live connection state string (e.g. "connected"); pass
// "stopped" when the manager has no record of the peer.
func toPeerResponse(p store.Peer, status string) peerResponse {
	resp := peerResponse{ID: uuidToString(p.ID), Name: p.Name, Status: status}
	var b peerBody
	if len(p.Body) > 0 {
		_ = json.Unmarshal(p.Body, &b)
	}
	resp.Host = b.Host
	resp.Port = b.Port
	resp.OriginHost = b.OriginHost
	resp.OriginRealm = b.OriginRealm
	resp.OriginIP = b.OriginIP
	resp.OriginPort = b.OriginPort
	resp.Transport = b.Transport
	resp.WatchdogIntervalSeconds = b.WatchdogIntervalSeconds
	resp.AutoConnect = b.AutoConnect
	return resp
}

// peerBodyBytes serialises a peerRequest into the JSONB body stored in the database.
func peerBodyBytes(req peerRequest) ([]byte, error) {
	return json.Marshal(peerBody{
		Host:                    req.Host,
		Port:                    req.Port,
		OriginHost:              req.OriginHost,
		OriginRealm:             req.OriginRealm,
		OriginIP:                req.OriginIP,
		OriginPort:              req.OriginPort,
		Transport:               req.Transport,
		WatchdogIntervalSeconds: req.WatchdogIntervalSeconds,
		AutoConnect:             req.AutoConnect,
	})
}

// peerStateString resolves the live connection state for a named peer
// from the manager, returning "stopped" when the peer is not registered.
func peerStateString(mgr PeerManager, name string) string {
	if mgr == nil {
		return "stopped"
	}
	state, err := mgr.State(name)
	if err != nil {
		return "stopped"
	}
	return state.String()
}

// listPeers handles GET /peers.
func listPeers(s store.Store, mgr PeerManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		peers, err := s.ListPeers(r.Context())
		if err != nil {
			respondInternalError(w)
			return
		}
		out := make([]peerResponse, len(peers))
		for i, p := range peers {
			out[i] = toPeerResponse(p, peerStateString(mgr, p.Name))
		}
		respondJSON(w, http.StatusOK, out)
	}
}

// createPeer handles POST /peers.
func createPeer(s store.Store, mgr PeerManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req peerRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		if req.Name == "" {
			respondInvalidRequest(w, "name is required")
			return
		}
		body, err := peerBodyBytes(req)
		if err != nil {
			respondInternalError(w)
			return
		}
		peer, err := s.InsertPeer(r.Context(), req.Name, body)
		if mapStoreError(w, err) != nil {
			return
		}
		respondJSON(w, http.StatusCreated, toPeerResponse(peer, peerStateString(mgr, peer.Name)))
	}
}

// getPeer handles GET /peers/{id}.
func getPeer(s store.Store, mgr PeerManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseUUIDParam(w, r, "id")
		if !ok {
			return
		}
		peer, err := s.GetPeer(r.Context(), id)
		if mapStoreError(w, err) != nil {
			return
		}
		respondJSON(w, http.StatusOK, toPeerResponse(peer, peerStateString(mgr, peer.Name)))
	}
}

// updatePeer handles PUT /peers/{id}.
func updatePeer(s store.Store, mgr PeerManager) http.HandlerFunc {
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
		body, err := peerBodyBytes(req)
		if err != nil {
			respondInternalError(w)
			return
		}
		peer, err := s.UpdatePeer(r.Context(), id, req.Name, body)
		if mapStoreError(w, err) != nil {
			return
		}
		respondJSON(w, http.StatusOK, toPeerResponse(peer, peerStateString(mgr, peer.Name)))
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

// startPeer handles POST /peers/{id}/start.
// Equivalent to connect but returns a full Peer response body so the
// frontend can update its cache without a separate GET.
func startPeer(s store.Store, mgr PeerManager) http.HandlerFunc {
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
		if err := mgr.Connect(peer.Name); err != nil && !errors.Is(err, diameter.ErrPeerAlreadyConnected) {
			mapManagerError(w, err)
			return
		}
		// Return "connecting" immediately — the goroutine transitions
		// asynchronously and SSE delivers the settled state. Returning
		// the live state here would race and could give back "stopped".
		resp := toPeerResponse(peer, "connecting")
		respondJSON(w, http.StatusOK, resp)
	}
}

// stopPeer handles POST /peers/{id}/stop.
// Equivalent to disconnect but returns a full Peer response body.
func stopPeer(s store.Store, mgr PeerManager) http.HandlerFunc {
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
		// Return "stopped" immediately — Disconnect is synchronous but
		// the SSE event may arrive slightly after the HTTP response.
		respondJSON(w, http.StatusOK, toPeerResponse(peer, "stopped"))
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
