package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/eddiecarpenter/ocs-testbench/internal/diameter"
	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

func mountDashboard(r chi.Router, s store.Store, mgr PeerManager) {
	r.Get("/dashboard/kpis", getDashboardKPIs(s, mgr))
}

type dashboardKPIPeers struct {
	Connected int `json:"connected"`
	Total     int `json:"total"`
}

type dashboardKPIs struct {
	Peers       dashboardKPIPeers `json:"peers"`
	Subscribers int               `json:"subscribers"`
	Scenarios   int               `json:"scenarios"`
	ActiveRuns  int               `json:"activeRuns"`
}

func getDashboardKPIs(s store.Store, mgr PeerManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		peers, _ := s.ListPeers(ctx)
		subscribers, _ := s.ListSubscribers(ctx)
		scenarios, _ := s.ListScenarios(ctx)

		connected := 0
		if mgr != nil {
			for _, p := range peers {
				state, err := mgr.State(p.Name)
				if err == nil && state == diameter.StateConnected {
					connected++
				}
			}
		}

		respondJSON(w, http.StatusOK, dashboardKPIs{
			Peers:       dashboardKPIPeers{Connected: connected, Total: len(peers)},
			Subscribers: len(subscribers),
			Scenarios:   len(scenarios),
			ActiveRuns:  0, // execution engine not yet landed
		})
	}
}
