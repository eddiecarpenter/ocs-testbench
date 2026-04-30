package appl

import (
	"fmt"
	"os/exec"
	"runtime"
)

// browserExec is the exec.Command hook the browser-open helper uses.
// Tests override it so OpenBrowser can be exercised without spawning a
// real subprocess.
var browserExec = exec.Command

// OpenBrowser launches the operating system's default browser at the
// given URL. Stdlib only — no third-party dependency. Returns an
// error if the URL is empty, if the platform is unsupported, or if
// the launched helper cannot be started.
//
// Supported platforms: linux (xdg-open), darwin (open),
// windows (rundll32 url.dll,FileProtocolHandler).
func OpenBrowser(url string) error {
	spec, err := browserCommandSpec(runtime.GOOS, url)
	if err != nil {
		return err
	}
	return browserExec(spec.name, spec.args...).Start()
}

// browserCmdSpec is the (name, args) tuple used to launch the helper.
// Pulled into its own type so unit tests can assert the per-OS
// command shape without invoking exec at all.
type browserCmdSpec struct {
	name string
	args []string
}

// browserCommandSpec resolves the platform-specific helper for url.
// goos is a runtime.GOOS-shaped string so the function is testable
// across platforms without compile-time conditional builds.
func browserCommandSpec(goos, url string) (browserCmdSpec, error) {
	if url == "" {
		return browserCmdSpec{}, fmt.Errorf("appl: OpenBrowser: empty URL")
	}
	switch goos {
	case "linux", "freebsd", "openbsd", "netbsd":
		return browserCmdSpec{name: "xdg-open", args: []string{url}}, nil
	case "darwin":
		return browserCmdSpec{name: "open", args: []string{url}}, nil
	case "windows":
		return browserCmdSpec{name: "rundll32", args: []string{"url.dll,FileProtocolHandler", url}}, nil
	default:
		return browserCmdSpec{}, fmt.Errorf("appl: OpenBrowser unsupported on %s", goos)
	}
}
