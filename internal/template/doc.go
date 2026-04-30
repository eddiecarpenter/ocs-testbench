// Package template implements the AVP rendering engine for the OCS
// Testbench.
//
// The package is split into two layers:
//
//   - Loader (loader.go) — reads an avp_template row from the store,
//     assembles a resolved value map from static values, execution-
//     context variables, generated values, and step-level overrides,
//     and produces an EngineInput ready for the engine to consume.
//
//   - Engine (engine.go) — consumes an EngineInput (produced by the
//     loader or constructed directly by any caller), validates every
//     AVP name against the dictionary, substitutes {{PLACEHOLDER}}
//     tokens, applies vendor-id inheritance, encodes values into the
//     correct Diameter data types, and builds the complete AVP tree
//     returned as []*diam.AVP.
//
// The split keeps the engine pure and stateless: it accepts an input
// struct and produces an AVP tree with no store dependency. Any caller
// can build an EngineInput directly (e.g. for inline-template
// execution) and hand it to the engine without going through the
// loader.
//
// Errors returned by both layers are typed TemplateError values;
// callers can match on the Code field via errors.As.
package template
