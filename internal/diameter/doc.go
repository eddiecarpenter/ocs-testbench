// Package diameter is the Diameter-protocol stack of the OCS Testbench.
//
// The package owns the testbench's transport-and-protocol layer: the
// AVP dictionary, per-peer client connections (CER/CEA, DWR/DWA), the
// multi-peer registry and auto-connect orchestration, the CCR/CCA
// Sender abstraction, and the protocol-mandated CCA behaviour
// (FUI-TERMINATE, Validity-Time, 5xxx). It sits below every higher
// layer (engine, REST API, WebSocket) and consumes the configuration
// rows owned by internal/store read-only.
//
// The package does NOT touch SQL or HTTP — it operates on a
// store-shaped Source interface that the orchestrator implements,
// keeping the Diameter package free of database and web dependencies
// per docs/ARCHITECTURE.md §14 core-separation invariant.
//
// Sub-packages:
//
//   - dictionary — the AVP dictionary loader (built-in 6733/4006/32.299
//     plus active custom XML records from a Source).
//
// Subsequent feature tasks add per-peer connection management, the
// multi-peer manager, the Sender, and the protocol-behaviour layer
// to this package family.
package diameter
