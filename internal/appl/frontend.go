package appl

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// FrontendHandler returns an http.Handler that serves files from rootFS
// (a sub-FS rooted at the embedded frontend's build output directory)
// with single-page-app fallback semantics: any GET/HEAD request whose
// path does not resolve to an existing regular file is served the
// contents of "index.html" so client-side routing can take over.
//
// The returned handler is intended to be mounted as the chi router's
// NotFound handler: API routes attached to the router earlier take
// precedence; everything else falls through to this handler.
//
// Returns an error when "index.html" is not present in rootFS — this
// is a build-time problem (the //go:embed target is missing the
// placeholder), not a runtime one.
func FrontendHandler(rootFS fs.FS) (http.Handler, error) {
	indexBytes, err := fs.ReadFile(rootFS, "index.html")
	if err != nil {
		return nil, fmt.Errorf("appl: read index.html from frontend FS: %w", err)
	}

	serveIndex := func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(indexBytes)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Normalise the path. Empty / root is treated as a request
		// for index.html.
		clean := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if clean == "" {
			serveIndex(w)
			return
		}

		// Try to serve the file directly. If it does not exist or is
		// a directory, fall through to index.html.
		f, err := rootFS.Open(clean)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				serveIndex(w)
				return
			}
			// Any other error (permission, etc.) is genuine; surface it.
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		stat, statErr := f.Stat()
		_ = f.Close()
		if statErr != nil {
			http.Error(w, statErr.Error(), http.StatusInternalServerError)
			return
		}
		if stat.IsDir() {
			serveIndex(w)
			return
		}

		// The file exists and is a regular file — let http.ServeFileFS
		// handle MIME detection, byte-range requests, and ETag.
		http.ServeFileFS(w, r, rootFS, clean)
	}), nil
}
