//go:build !dev

package main

import (
	"io/fs"
	"net/http"
)

// devFrontendHandler is the dev-mode hook into runWith. The non-dev
// build returns (nil, nil), telling runWith to fall through to the
// embedded-FS handler. The corresponding `//go:build dev` file in
// devmode_dev.go returns a real handler (a Vite reverse proxy) and
// registers the supervisor's Stop on lifecycle shutdown.
//
// The signature is identical in both builds so runWith stays
// build-tag agnostic. The shutdownRegistrar interface lives in
// devmode_iface.go so both builds can reference it.
func devFrontendHandler(_ shutdownRegistrar, _ fs.FS) (http.Handler, error) {
	return nil, nil
}
