// Package dictionary loads the Diameter AVP dictionary used by the
// rest of the testbench's diameter package family.
//
// At application start the loader is invoked once. It guarantees:
//
//   - The built-in dictionaries shipped with go-diameter — RFC 6733
//     (Base), RFC 4006 (Credit-Control), and 3GPP TS 32.299 (Ro/Rf,
//     plus the additional 3GPP application dictionaries the library
//     bundles) — are loaded into the package-level dict.Default
//     parser. go-diameter's package init() does this for us; the
//     loader only has to verify that a sentinel base AVP is present.
//
//   - Every active row from the custom_dictionary store table is
//     parsed and applied on top of the base dictionary, extending
//     it with operator-supplied AVPs.
//
//   - Invalid custom XML is logged at WARN with the offending
//     dictionary's name and the parser error, then skipped — a
//     single bad row never blocks startup, and other custom rows
//     continue to load. This satisfies AC-9 of feature #17.
//
// The loader is the only place in the codebase that touches the
// Diameter dictionary; every later subsystem (connection manager,
// CCR/CCA Sender, protocol behaviour) reads from the same parser
// the loader populated.
//
// The store dependency is injected as a small Source interface so
// unit tests substitute a fake source in-process — no PostgreSQL is
// required to exercise the loader's branches.
package dictionary
