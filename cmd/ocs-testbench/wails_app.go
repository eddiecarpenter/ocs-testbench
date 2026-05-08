package main

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// wailsApp is the Wails application host. All business logic lives in
// runWith(); this struct exists solely to satisfy Wails' OnStartup
// callback contract and to provide the context used by menu callbacks.
type wailsApp struct {
	ctx context.Context
}

func newWailsApp() *wailsApp { return &wailsApp{} }

func (a *wailsApp) startup(ctx context.Context) { a.ctx = ctx }

// buildMenu returns the macOS menu bar:
//   - Application menu — Preferences…, separator, Quit.
//   - Edit menu — standard clipboard items so WKWebView's responder chain
//     routes Cmd+C/V/X/A through the OS accessibility layer.
//   - View menu — Reload.
func (a *wailsApp) buildMenu() *menu.Menu {
	m := menu.NewMenu()

	// Application menu (first entry = the process name on macOS).
	appSub := m.AddSubmenu("OCS Testbench")
	appSub.AddText("Preferences…", keys.CmdOrCtrl(","), func(_ *menu.CallbackData) {
		runtime.WindowExecJS(a.ctx, "window.location.href='/settings'")
	})
	appSub.AddSeparator()
	appSub.AddText("Quit OCS Testbench", keys.CmdOrCtrl("q"), func(_ *menu.CallbackData) {
		runtime.Quit(a.ctx)
	})

	// Edit menu — the items themselves are no-ops in Go; their presence
	// is what tells WKWebView/macOS to route the shortcuts through the
	// responder chain so clipboard, undo, and select-all work inside the
	// web content.
	editSub := m.AddSubmenu("Edit")
	editSub.AddText("Undo", keys.CmdOrCtrl("z"), nil)
	editSub.AddText("Redo", keys.Combo("z", keys.CmdOrCtrlKey, keys.ShiftKey), nil)
	editSub.AddSeparator()
	editSub.AddText("Cut", keys.CmdOrCtrl("x"), nil)
	editSub.AddText("Copy", keys.CmdOrCtrl("c"), nil)
	editSub.AddText("Paste", keys.CmdOrCtrl("v"), nil)
	editSub.AddSeparator()
	editSub.AddText("Select All", keys.CmdOrCtrl("a"), nil)

	// View menu.
	viewSub := m.AddSubmenu("View")
	viewSub.AddText("Reload", keys.CmdOrCtrl("r"), func(_ *menu.CallbackData) {
		runtime.WindowReload(a.ctx)
	})

	return m
}
