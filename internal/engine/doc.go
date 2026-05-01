// Package engine is the scenario execution runtime for the OCS Testbench.
//
// The engine implements three layers:
//
//  1. Session context (context.go) — the stateful execution thread that binds
//     a scenario run to a peer, carries context variables across steps, wraps
//     the Sender in a MeasuredSender for metrics capture, and accumulates
//     per-step metrics with a query API.
//
//  2. Step executor (step_executor.go) — a stateless processor that takes a
//     single step, a session context, and the previous CCA response, and
//     produces a StepResult. Guards, extractions, derived values, assertions,
//     and result-code handlers are evaluated here.
//
//  3. Scenario orchestrator (orchestrator.go) — iterates the step list,
//     manages interactive/continuous mode semantics, handles result-code
//     dispatch (retry, pause, terminate), and stop conditions.
//
// The engine depends on:
//   - internal/diameter/messaging — Sender interface and CCR/CCA types
//   - internal/template — AVP tree renderer
//   - github.com/eddiecarpenter/ruleevaluator — expression evaluation
//
// The engine does NOT import internal/store or any HTTP/SSE layer. This
// boundary is intentional and must be maintained (ARCHITECTURE.md §14
// core-separation invariant).
package engine
