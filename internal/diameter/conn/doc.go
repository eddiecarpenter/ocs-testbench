// Package conn implements the per-peer Diameter connection
// lifecycle.
//
// A PeerConnection wraps go-diameter's sm.Settings + sm.Client for
// exactly one peer. It owns:
//
//   - The connect / disconnect API the multi-peer manager (and
//     ultimately the REST control layer) drives.
//   - The state machine
//     disconnected → connecting → connected → disconnected
//     including the unexpected-drop branch.
//   - The CER/CEA capability exchange (delegated to sm.Client.Dial,
//     which blocks until CEA arrives).
//   - The DWR/DWA watchdog (delegated to sm.Client when its
//     EnableWatchdog flag is on).
//   - Bounded exponential reconnect backoff — 1s, 2s, 4s, … capped
//     at 60s — that runs unattended until the operator calls
//     Disconnect or the supplied context is cancelled.
//   - Per-peer state-event fan-out: subscribers register via
//     Subscribe() and each receives every transition.
//
// Plain TCP and TLS are supported. SCTP is out of scope.
//
// The package does not depend on net/http, internal/store, or any
// JSON schema — it consumes a typed PeerConfig from the parent
// diameter package. Wiring happens in the manager (task 3) and in
// the application bootstrap.
package conn
