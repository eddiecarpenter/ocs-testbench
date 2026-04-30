// Common interface shared by the build-tag pair `devmode.go`
// (`!dev`) and `devmode_dev.go` (`dev`). Lives in its own file with
// no build tag so both builds can reference it.
//
// The interface is the slice of `appl.Lifecycle` that the dev hook
// needs (subprocess-shutdown registration). Defined locally instead
// of imported from `internal/appl` to keep this package's import
// graph identical between builds — neither flavour reaches into
// `appl` from devmode.go's signature.

package main

import "context"

type shutdownRegistrar interface {
	RegisterShutdown(name string, fn func(context.Context) error)
}
