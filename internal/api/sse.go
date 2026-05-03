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
// Peer SSE (GET /events/peers) is fully implemented. Execution SSE
// (GET /events/executions/{id}) requires a non-nil exec; when nil the
// endpoint returns 503.
func mountSSE(r chi.Router, s store.Store, mgr PeerManager, exec ExecutionEngine) {
	// Combined event stream — the frontend subscribes here for all event types.
	r.Get("/events", peerSSE(s, mgr))
	// Scoped sub-streams.
	r.Get("/events/peers", peerSSE(s, mgr))
	r.Get("/events/executions/{id}", executionSSE(exec))
}

// executionSSE handles GET /events/executions/{id}.
//
// Opens a long-lived Server-Sent Events stream for a specific execution
// session. As the session progresses the client receives events of the
// form:
//
//	event: execution.progress
//	data: {"type":"progress","sessionId":"...","state":"active","step":N,"metrics":{...}}
//
// The stream ends when the session reaches a terminal state or the
// client disconnects. When exec is nil the endpoint returns 503.
func executionSSE(exec ExecutionEngine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if exec == nil {
			executionUnavailable(w)
			return
		}

		sessionID := chi.URLParam(r, "id")
		if sessionID == "" {
			respondInvalidRequest(w, "id is required")
			return
		}

		// Set SSE headers before any write.
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			respondInternalError(w)
			return
		}

		ch, err := exec.Subscribe(r.Context(), sessionID)
		if err != nil {
			// Headers are already committed — we can only close.
			return
		}

		for {
			select {
			case evt, open := <-ch:
				if !open {
					// Session terminated; end the stream.
					return
				}
				data, err := json.Marshal(struct {
					Type      string            `json:"type"`
					SessionID string            `json:"sessionId"`
					State     string            `json:"state"`
					Step      int               `json:"step"`
					Metrics   executionMetricsJ `json:"metrics"`
				}{
					Type:      evt.Type,
					SessionID: evt.SessionID,
					State:     evt.State,
					Step:      evt.Step,
					Metrics: executionMetricsJ{
						TotalRequests: evt.Metrics.TotalRequests,
						SuccessCount:  evt.Metrics.SuccessCount,
						FailureCount:  evt.Metrics.FailureCount,
						MinRTTMs:      evt.Metrics.MinRTTMs,
						MaxRTTMs:      evt.Metrics.MaxRTTMs,
						AvgRTTMs:      evt.Metrics.AvgRTTMs,
					},
				})
				if err != nil {
					continue
				}
				fmt.Fprintf(w, "event: execution.progress\ndata: %s\n\n", data)
				flusher.Flush()

			case <-r.Context().Done():
				// Client disconnected.
				return
			}
		}
	}
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

		// Emit the current state of every known peer immediately so the
		// frontend does not miss transitions that happened before the
		// SSE stream was opened (e.g. auto-connect on startup).
		for _, p := range peers {
			resp := toPeerResponse(p, peerStateString(mgr, p.Name))
			data, err := json.Marshal(resp)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: peer.updated\ndata: %s\n\n", data)
		}
		flusher.Flush()

		// Build a name→store.Peer map so live events can build a full
		// peer response without an additional store round-trip.
		peerByName := make(map[string]store.Peer, len(peers))
		for _, p := range peers {
			peerByName[p.Name] = p
		}

		for {
			select {
			case evt, open := <-ch:
				if !open {
					// Manager stopped; end the stream.
					return
				}
				p, known := peerByName[evt.PeerName]
				if !known {
					continue
				}
				resp := toPeerResponse(p, evt.To.String())
				resp.LastChangeAt = evt.Time.UTC().Format(time.RFC3339)
				data, err := json.Marshal(resp)
				if err != nil {
					continue
				}
				fmt.Fprintf(w, "event: peer.updated\ndata: %s\n\n", data)
				flusher.Flush()

			case <-r.Context().Done():
				// Client disconnected.
				return
			}
		}
	}
}
