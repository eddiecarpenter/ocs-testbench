package appl

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func sampleFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":            {Data: []byte("<!doctype html><html><body>placeholder</body></html>")},
		"assets/app.js":         {Data: []byte("console.log('app');")},
		"assets/style.css":      {Data: []byte("body { color: blue; }")},
		"static/data/info.json": {Data: []byte(`{"ok":true}`)},
	}
}

// TestFrontendHandler_ServesRootIndex — GET / returns index.html.
func TestFrontendHandler_ServesRootIndex(t *testing.T) {
	h, err := FrontendHandler(sampleFS())
	if err != nil {
		t.Fatalf("FrontendHandler: %v", err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "placeholder") {
		t.Errorf("body should contain index.html marker; got: %s", body)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("content-type: got %q, want text/html prefix", ct)
	}
}

// TestFrontendHandler_ServesExistingAsset — a real file in the FS is
// served verbatim with a sensible content-type.
func TestFrontendHandler_ServesExistingAsset(t *testing.T) {
	h, err := FrontendHandler(sampleFS())
	if err != nil {
		t.Fatalf("FrontendHandler: %v", err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/assets/app.js")
	if err != nil {
		t.Fatalf("GET asset: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "console.log('app');" {
		t.Errorf("body: got %q, want %q", body, "console.log('app');")
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/javascript") && !strings.HasPrefix(ct, "application/javascript") {
		t.Errorf("content-type: got %q, want js prefix", ct)
	}
}

// TestFrontendHandler_SPAFallback — an unknown path that doesn't
// resolve to a file falls back to index.html.
func TestFrontendHandler_SPAFallback(t *testing.T) {
	h, err := FrontendHandler(sampleFS())
	if err != nil {
		t.Fatalf("FrontendHandler: %v", err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/some/spa/path")
	if err != nil {
		t.Fatalf("GET SPA path: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "placeholder") {
		t.Errorf("SPA fallback should serve index.html; got: %s", body)
	}
}

// TestFrontendHandler_DirectoryFallsBack — a request that resolves
// to a directory (rather than a file) also falls back to index.html.
func TestFrontendHandler_DirectoryFallsBack(t *testing.T) {
	h, err := FrontendHandler(sampleFS())
	if err != nil {
		t.Fatalf("FrontendHandler: %v", err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/assets")
	if err != nil {
		t.Fatalf("GET dir: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "placeholder") {
		t.Errorf("directory should fall back to index.html; got: %s", body)
	}
}

// TestFrontendHandler_RejectsNonGET — POST/PUT/DELETE return 405.
func TestFrontendHandler_RejectsNonGET(t *testing.T) {
	h, err := FrontendHandler(sampleFS())
	if err != nil {
		t.Fatalf("FrontendHandler: %v", err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/", strings.NewReader("body"))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("POST status: got %d, want 405", resp.StatusCode)
	}
}

// TestFrontendHandler_MissingIndex — constructor fails when the FS
// does not contain index.html.
func TestFrontendHandler_MissingIndex(t *testing.T) {
	fsys := fstest.MapFS{"foo.txt": {Data: []byte("x")}}
	_, err := FrontendHandler(fsys)
	if err == nil {
		t.Error("expected constructor error when index.html is missing")
	}
}
