// Package web exposes the React/TypeScript frontend as an embedded
// filesystem. The build output lives at web/dist/; go:embed packages it
// into the binary at compile time so the application is a single
// distributable artefact (no separate frontend server, no runtime
// dependency on the source tree).
//
// The dist/ directory is rebuilt by Vite. Until Feature #22 wires the
// build pipeline, dist/ contains only a tracked placeholder
// index.html — the embedded handler in internal/appl serves that as
// a clear "frontend not yet built" page so the binary still runs end
// to end.
//
// The embed declaration must live at or above the embedded path in
// the source tree (Go's go:embed forbids ".." in patterns). main.go
// in cmd/ocs-testbench/ cannot reach web/dist directly, so this
// package exists as the embed pivot point and is imported by main
// for its FS export.
package web

import "embed"

// FS is the React/TypeScript build output. The root of the FS is the
// web/ directory, so callers want fs.Sub(FS, "dist") to land at the
// SPA's index.html.
//
//go:embed all:dist
var FS embed.FS
