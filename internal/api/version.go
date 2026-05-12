package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// VersionInfo is the response body for GET /v1/version.
type VersionInfo struct {
	Version string `json:"version"`
}

func mountVersion(r chi.Router, version string) {
	r.Get("/version", func(w http.ResponseWriter, req *http.Request) {
		respondJSON(w, http.StatusOK, VersionInfo{Version: version})
	})
}
