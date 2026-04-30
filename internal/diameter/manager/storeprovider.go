package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/eddiecarpenter/ocs-testbench/internal/diameter"
	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// StoreLister is the slice of internal/store.Store the manager
// actually consumes. Defined as its own interface here so the
// adapter does not pull in the full Store interface — and so tests
// can pass a mock that satisfies just this method shape.
type StoreLister interface {
	ListPeers(ctx context.Context) ([]store.Peer, error)
}

// StorePeerProvider adapts the production internal/store.Store to
// the manager's PeerProvider interface.
//
// Each peer row's JSONB body is decoded into the PeerInput shape
// the openapi schema commits to (api/openapi.yaml `PeerInput` block)
// and projected onto a diameter.PeerConfig. The body is treated as
// a contract — the adapter reads what is there and never writes
// fields that the publisher (the REST layer) did not send.
//
// The adapter does NOT touch SQL directly — it goes through the
// Store interface, mirroring dictionary.StoreSource. This preserves
// the §14 core-separation invariant: the diameter package family
// depends on internal/store via interface, not on a database
// driver.
type StorePeerProvider struct {
	store StoreLister
}

// NewStorePeerProvider constructs a StorePeerProvider bound to the
// given store. Panics on a nil store — wiring the manager against a
// nil store is a programming error and should fail loudly at
// startup, not silently when ListPeers is called.
func NewStorePeerProvider(s StoreLister) *StorePeerProvider {
	if s == nil {
		panic("manager.NewStorePeerProvider: store must not be nil")
	}
	return &StorePeerProvider{store: s}
}

// peerBody is the on-disk JSONB shape of a peer row's body column.
// The field names match the openapi `PeerInput` schema verbatim so
// the adapter can decode any peer the REST layer (or any other
// well-behaved publisher) wrote without translation. New fields
// added to the openapi schema can be picked up here without changing
// the contract — the json decoder ignores unknown fields by default.
//
// Note: this struct is the projection of an existing contract, not a
// new one. Per the framework's Contract Rules, modifying any field
// here without explicit approval is a contract change.
type peerBody struct {
	// Name is duplicated in the row's name column; we tolerate the
	// duplication so a peerBody can be decoded standalone.
	Name string `json:"name,omitempty"`

	Host        string `json:"host"`
	Port        int    `json:"port"`
	OriginHost  string `json:"originHost"`
	OriginRealm string `json:"originRealm"`
	Transport   string `json:"transport"`

	// WatchdogIntervalSeconds is the openapi field name; converted
	// to time.Duration when projecting onto diameter.PeerConfig.
	WatchdogIntervalSeconds int `json:"watchdogIntervalSeconds"`

	// AutoConnect uses Go's default-zero of false when the field is
	// absent. The openapi default is true; the adapter preserves
	// "absent" as false rather than inventing a true the publisher
	// did not send.
	AutoConnect bool `json:"autoConnect"`
}

// ListPeers reads every peer row from the store, decodes each
// body into the manager's PeerConfig shape, and returns the result.
// Errors:
//
//   - Any store error is propagated as-is (callers can compare via
//     errors.Is against the store's sentinels).
//   - A peer whose body fails to decode aborts the whole list with
//     an error that names the offending peer. The adapter does not
//     fail-soft here — a malformed peer body is an integrity issue
//     in the database that should surface as a startup failure, not
//     a silent skip. The caller (manager.Start) treats a list error
//     as fatal, so the operator gets a clear log line.
func (p *StorePeerProvider) ListPeers(ctx context.Context) ([]diameter.PeerConfig, error) {
	rows, err := p.store.ListPeers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]diameter.PeerConfig, 0, len(rows))
	for _, row := range rows {
		var body peerBody
		if err := json.Unmarshal(row.Body, &body); err != nil {
			return nil, fmt.Errorf("manager: decode peer %q body: %w", row.Name, err)
		}
		out = append(out, projectPeerBody(row.Name, body))
	}
	return out, nil
}

// projectPeerBody maps the JSONB-decoded peerBody onto the
// diameter.PeerConfig the connection layer consumes. Centralised
// here so the body→config mapping is a single function the tests
// can exercise directly.
//
// The peer's name on the row (row.Name) is authoritative — the body
// may carry the same string as a convenience but the row column is
// the unique key.
func projectPeerBody(rowName string, body peerBody) diameter.PeerConfig {
	transport := strings.ToLower(strings.TrimSpace(body.Transport))
	switch transport {
	case "tcp", "":
		transport = diameter.TransportTCP
	case "tls":
		transport = diameter.TransportTLS
	}
	wd := time.Duration(body.WatchdogIntervalSeconds) * time.Second
	return diameter.PeerConfig{
		Name:             rowName,
		Host:             body.Host,
		Port:             body.Port,
		OriginHost:       body.OriginHost,
		OriginRealm:      body.OriginRealm,
		Transport:        transport,
		WatchdogInterval: wd,
		AutoConnect:      body.AutoConnect,
	}
}
