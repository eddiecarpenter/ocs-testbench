package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/eddiecarpenter/ocs-testbench/internal/diameter"
	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// mountSSE registers the Server-Sent Events streaming endpoints.
//
// Peer SSE (GET /events/peers) is implemented here.
// Execution SSE (GET /events/executions/{id}) is not yet implemented;
// it depends on Feature #19 (internal/engine/) which has not yet landed.
func mountSSE(r chi.Router, s store.Store, mgr PeerManager) {
	r.Get("/events/peers", peerSSE(s, mgr))
	r.Get("/events/executions/{id}", notImplemented) // blocked on Feature #19
}

// peerSSEEvent is the JSON payload emitted for each peer state
// transition in the peer SSE stream.
type peerSSEEvent struct {
	PeerID string `json:"peerId"`
	Name   string `json:"peerName"`
	Status string `json:"status"`
	At     string `json:"at"`
}

// peerSSE handles GET /events/peers.
//
// Opens a long-lived Server-Sent Events stream. When a peer's
// connection state changes, the client receives an event of the form:
//
//	event: peer-status
//	data: {"peerId":"...","peerName":"...","status":"connected","at":"..."}
//
// The stream runs until the client disconnects or the Manager's
// subscriber channel closes (which happens when the Manager stops).
//
// If the Manager is nil or has no registered peers, the stream opens
// and stays silent until the client disconnects.
func peerSSE(s store.Store, mgr PeerManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set SSE headers before any write.
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			respondInternalError(w)
			return
		}

		if mgr == nil {
			// No manager — hold the stream open until the client
			// disconnects.
			<-r.Context().Done()
			return
		}

		ch := mgr.Subscribe()
		defer func() {
			// Best-effort unsubscribe if the Manager exposes it. The
			// PeerManager interface does not require Unsubscribe, so
			// we type-assert to the optional extended interface.
			type unsubscriber interface {
				Unsubscribe(ch <-chan diameter.StateEvent)
			}
			if u, ok := mgr.(unsubscriber); ok {
				u.Unsubscribe(ch)
			}
		}()

		// Resolve peer ID from name for event payloads. Peer IDs are
		// needed by the frontend to correlate events with stored rows.
		// We build the name→id map once on connection to avoid a
		// per-event store lookup.
		peers, _ := s.ListPeers(r.Context())
		peerIDByName := make(map[string]string, len(peers))
		for _, p := range peers {
			peerIDByName[p.Name] = uuidToString(p.ID)
		}

		for {
			select {
			case evt, open := <-ch:
				if !open {
					// Manager stopped; end the stream.
					return
				}
				peerID := peerIDByName[evt.PeerName]
				payload := peerSSEEvent{
					PeerID: peerID,
					Name:   evt.PeerName,
					Status: evt.To.String(),
					At:     evt.Time.UTC().Format(time.RFC3339),
				}
				data, err := json.Marshal(payload)
				if err != nil {
					continue
				}
				fmt.Fprintf(w, "event: peer-status\ndata: %s\n\n", data)
				flusher.Flush()

			case <-r.Context().Done():
				// Client disconnected.
				return
			}
		}
	}
}
