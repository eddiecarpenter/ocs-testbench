// Package protocol implements the protocol-mandated CCA behaviour
// of the Diameter stack.
//
// docs/ARCHITECTURE.md §10.1 names three CCA-side behaviours that
// are *protocol-mandated, not testing choices*:
//
//   - Final-Unit-Indication action TERMINATE → emit a CCR-Terminate
//     for the same Session-Id automatically, without the engine
//     having to ask.
//   - Validity-Time → schedule a CCR-Update before the validity
//     expires so the grant can be refreshed.
//   - Top-level Result-Code in the 5xxx range → permanent failure;
//     mark the session terminated so subsequent Send calls for
//     that session fail fast with ErrSessionTerminated.
//
// This package implements those behaviours as a *decorator* over
// the messaging.Sender interface. Wiring in
// cmd/ocs-testbench/main.go produces a Behaviour-wrapped Sender
// as the one the engine layer eventually consumes — the engine
// sees the original CCA verbatim, and any out-of-band CCRs the
// Behaviour issues happen behind the scenes.
//
// What this package explicitly does NOT handle:
//
//   - Per-MSCC 4010 / 4011 / 4012 result codes — those are the
//     engine's per-Rating-Group state derivation per
//     ARCHITECTURE §7.5. The Behaviour reads only the top-level
//     Result-Code; per-MSCC codes flow through to the engine
//     untouched.
//   - Retry policy on transient failures — those are the engine's
//     responsibility, surfaced as result-handler `retry` actions
//     per ARCHITECTURE §10.2.
//
// Session state lives inside the Behaviour as a small
// `sessions map[Session-Id]*sessionState`. It is internal — the
// engine sees only the post-protocol effects (a session that is
// terminated is terminated; a CCR-T already fired; a re-auth
// timer is in flight).
package protocol
