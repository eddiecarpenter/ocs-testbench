package manager

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/eddiecarpenter/ocs-testbench/internal/diameter"
	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// stubStoreLister is a minimal StoreLister fake used by the
// StorePeerProvider tests. It returns a fixed slice and an error
// flag so the projection / error-propagation paths are exercised
// without depending on an in-memory test store.
type stubStoreLister struct {
	rows []store.Peer
	err  error
}

func (s stubStoreLister) ListPeers(_ context.Context) ([]store.Peer, error) {
	return s.rows, s.err
}

// peerRow constructs a store.Peer with name = name and body = the
// JSON-encoded value of body.
func peerRow(t *testing.T, name string, body peerBody) store.Peer {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal peer body: %v", err)
	}
	return store.Peer{Name: name, Body: raw}
}

// Test 1 — happy-path projection: decoded body fields land on the
// PeerConfig with the expected types and conversions.
func TestStorePeerProvider_ListPeersHappyPath(t *testing.T) {
	t.Parallel()
	rows := []store.Peer{
		peerRow(t, "alpha", peerBody{
			Host: "ocs1.test", Port: 3868,
			OriginHost: "tb-01.test.local", OriginRealm: "test.local",
			Transport: "TCP", WatchdogIntervalSeconds: 30,
			AutoConnect: true,
		}),
		peerRow(t, "bravo", peerBody{
			Host: "ocs2.test", Port: 3869,
			OriginHost: "tb-02.test.local", OriginRealm: "test.local",
			Transport: "TLS", WatchdogIntervalSeconds: 60,
			AutoConnect: false,
		}),
	}
	prov := NewStorePeerProvider(stubStoreLister{rows: rows})
	got, err := prov.ListPeers(context.Background())
	if err != nil {
		t.Fatalf("ListPeers: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d configs; want 2", len(got))
	}

	a := got[0]
	if a.Name != "alpha" || a.Host != "ocs1.test" || a.Port != 3868 {
		t.Errorf("alpha mismatch: %+v", a)
	}
	if a.Transport != diameter.TransportTCP {
		t.Errorf("alpha transport = %q; want tcp", a.Transport)
	}
	if a.WatchdogInterval != 30*time.Second {
		t.Errorf("alpha watchdog = %s; want 30s", a.WatchdogInterval)
	}
	if !a.AutoConnect {
		t.Errorf("alpha autoConnect = false; want true")
	}

	b := got[1]
	if b.Transport != diameter.TransportTLS {
		t.Errorf("bravo transport = %q; want tls", b.Transport)
	}
	if b.AutoConnect {
		t.Errorf("bravo autoConnect = true; want false")
	}
	if b.WatchdogInterval != 60*time.Second {
		t.Errorf("bravo watchdog = %s; want 60s", b.WatchdogInterval)
	}
}

// Test 2 — empty store: ListPeers returns empty slice with no error.
func TestStorePeerProvider_EmptyStore(t *testing.T) {
	t.Parallel()
	prov := NewStorePeerProvider(stubStoreLister{rows: nil})
	got, err := prov.ListPeers(context.Background())
	if err != nil {
		t.Fatalf("ListPeers: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d; want 0", len(got))
	}
}

// Test 3 — store error propagates as-is.
func TestStorePeerProvider_StoreErrorPropagates(t *testing.T) {
	t.Parallel()
	want := errors.New("store boom")
	prov := NewStorePeerProvider(stubStoreLister{err: want})
	_, err := prov.ListPeers(context.Background())
	if !errors.Is(err, want) {
		t.Errorf("ListPeers err = %v; want %v", err, want)
	}
}

// Test 4 — a row whose body is invalid JSON aborts the list with a
// named error (no fail-soft).
func TestStorePeerProvider_InvalidBodyAborts(t *testing.T) {
	t.Parallel()
	rows := []store.Peer{
		peerRow(t, "good", peerBody{Host: "h", Port: 3868}),
		{Name: "broken", Body: []byte("{ this is not json ")},
	}
	prov := NewStorePeerProvider(stubStoreLister{rows: rows})
	_, err := prov.ListPeers(context.Background())
	if err == nil {
		t.Fatalf("ListPeers: expected decode error; got nil")
	}
	// The error should mention the offending peer's name so the
	// operator can find the row.
	if !contains(err.Error(), "broken") {
		t.Errorf("error message %q does not mention peer name 'broken'", err.Error())
	}
}

// Test 5 — empty transport string defaults to TCP per the openapi
// behaviour. Same for unknown casing.
func TestStorePeerProvider_TransportDefaulting(t *testing.T) {
	t.Parallel()
	cases := []struct {
		raw  string
		want string
	}{
		{"", diameter.TransportTCP},
		{"TCP", diameter.TransportTCP},
		{"tcp", diameter.TransportTCP},
		{"TLS", diameter.TransportTLS},
		{"tls", diameter.TransportTLS},
	}
	for _, tc := range cases {
		got := projectPeerBody("p", peerBody{Transport: tc.raw})
		if got.Transport != tc.want {
			t.Errorf("transport %q → %q; want %q", tc.raw, got.Transport, tc.want)
		}
	}
}

// Test 6 — NewStorePeerProvider panics on nil store.
func TestStorePeerProvider_NilStorePanics(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic; got nil")
		}
	}()
	NewStorePeerProvider(nil)
}

// contains is a tiny helper to assert substring presence without
// importing strings just for one check.
func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
