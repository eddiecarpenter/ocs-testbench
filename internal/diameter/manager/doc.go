// Package manager is the multi-peer connection registry of the OCS
// Testbench's Diameter stack.
//
// The Manager owns a map of named peer connections (one per
// PeerConfig) keyed by peer name. Each entry composes a
// conn.PeerConnection from internal/diameter/conn — the manager does
// not inherit from or wrap the connection's lifecycle, it simply
// registers and dispatches.
//
// Responsibilities:
//
//   - Register peers on Start(ctx) from a PeerProvider — one
//     conn.PeerConnection per peer, each with the peer's own
//     Origin-Host, Origin-Realm and isolated session space.
//   - Auto-connect peers whose PeerConfig.AutoConnect is true.
//   - Hand out per-peer connection handles via Get(name) so the
//     messaging layer (Task 4) can resolve peerName → connection.
//   - Provide a manager-level subscriber channel via Subscribe()
//     that fans out StateEvent values from every PeerConnection
//     onto a single stream — the consumer subscribes once and sees
//     every peer's transitions. (Task 4 / Feature #9 will use this.)
//   - Drain reconnect goroutines and close transports cleanly on
//     Stop().
//
// The manager package does NOT import internal/store. The peer
// configs flow in through the PeerProvider interface; the
// orchestrator (cmd/ocs-testbench/main.go) is responsible for
// reading the peer rows out of the store, decoding them into
// diameter.PeerConfig, and feeding them via a PeerProvider
// implementation. This preserves the §14 core-separation invariant.
package manager
