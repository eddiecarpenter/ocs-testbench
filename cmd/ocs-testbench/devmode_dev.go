//go:build dev

package main

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"time"

	"github.com/eddiecarpenter/ocs-testbench/internal/appl"
)

// Hard-coded for the noodle. If we ever want to flex these (different
// port, different working dir for monorepo cousins), promote to
// config.yaml or env vars. Today the dev workflow is one shape.
const (
	viteAddr        = "http://localhost:5173"
	viteWorkDir     = "web"
	viteStartupWait = 30 * time.Second
)

// devFrontendHandler — dev build. Spawns / attaches to a Vite dev
// server, returns a reverse-proxy handler aimed at it, and registers
// the supervisor's Stop as a shutdown callback so Vite dies cleanly
// when the Go process gets SIGTERM. The embedded `_dist` is unused
// in dev mode (Vite serves the SPA live from source).
func devFrontendHandler(lc shutdownRegistrar, _ fs.FS) (http.Handler, error) {
	sup := appl.NewViteSupervisor(viteWorkDir, viteAddr)
	if err := sup.Start(viteStartupWait); err != nil {
		return nil, fmt.Errorf("dev: vite supervisor: %w", err)
	}
	lc.RegisterShutdown("vite", func(_ context.Context) error {
		return sup.Stop()
	})
	return appl.DevFrontendHandler(viteAddr)
}
