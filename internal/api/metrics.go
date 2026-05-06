package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func mountMetrics(r chi.Router, exec ExecutionEngine) {
	r.Get("/metrics/response-time", getResponseTimeSeries(exec))
}

func getResponseTimeSeries(exec ExecutionEngine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if exec == nil {
			respondError(w, http.StatusServiceUnavailable, CodeInternalError, "execution engine not available")
			return
		}
		window := r.URL.Query().Get("window")
		if window == "" {
			window = "PT1H"
		}
		series, err := exec.ResponseTimeSeries(r.Context(), window)
		if err != nil {
			respondError(w, http.StatusBadRequest, CodeInvalidRequest, err.Error())
			return
		}
		respondJSON(w, http.StatusOK, series)
	}
}
