// Package messaging implements the Gy credit-control message
// surface of the Diameter stack.
//
// Three deliverables:
//
//   - CCR (Credit-Control-Request) and CCA
//     (Credit-Control-Answer) Go-native types that hold the fields
//     scenarios and the engine layer typically need (Session-Id,
//     Origin-Host, Origin-Realm, Destination-Realm, Auth-
//     Application-Id, Service-Context-Id, CC-Request-Type / Number,
//     Subscription-Id, MSCC blocks, …) plus a slice of arbitrary
//     extra AVPs the caller can inject.
//
//   - Encoder (BuildCCRMessage) and decoder (DecodeCCAMessage)
//     for the wire format. Encoding uses the package-level
//     dictionary singleton (dict.Default after the loader has run);
//     decoding produces a *CCA shaped for both engine-layer
//     consumption and the protocol-mandated behaviour layer
//     (task 5).
//
//   - Sender — the §14 Sender contract (`Send(ctx, peerName, ccr)
//     -> (*CCA, error)`). The concrete implementation resolves
//     `peerName` via the manager.Manager, encodes the CCR,
//     correlates the CCA response by end-to-end / hop-by-hop ID,
//     and decodes it. A peer in `disconnected` state fails fast
//     with diameter.ErrPeerNotConnected — no queueing, no retry.
//
// The messaging package does NOT implement protocol-mandated CCA
// behaviour (FUI-TERMINATE → CCR-T, Validity-Time → CCR-U, 5xxx
// → terminate). Those live in internal/diameter/protocol (task 5)
// as a Sender decorator over this package's concrete sender.
package messaging
