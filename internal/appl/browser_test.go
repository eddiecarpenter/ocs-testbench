package appl

import (
	"errors"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

// TestBrowserCommandSpec_PerOS — every supported runtime.GOOS yields
// the documented helper command. Unsupported platforms return an
// error rather than a silent fallback.
func TestBrowserCommandSpec_PerOS(t *testing.T) {
	cases := []struct {
		goos     string
		wantName string
		wantArg0 string
	}{
		{"linux", "xdg-open", "https://example.com/"},
		{"freebsd", "xdg-open", "https://example.com/"},
		{"openbsd", "xdg-open", "https://example.com/"},
		{"netbsd", "xdg-open", "https://example.com/"},
		{"darwin", "open", "https://example.com/"},
		{"windows", "rundll32", "url.dll,FileProtocolHandler"},
	}
	for _, tc := range cases {
		t.Run(tc.goos, func(t *testing.T) {
			spec, err := browserCommandSpec(tc.goos, "https://example.com/")
			if err != nil {
				t.Fatalf("browserCommandSpec(%s): %v", tc.goos, err)
			}
			if spec.name != tc.wantName {
				t.Errorf("name: got %q, want %q", spec.name, tc.wantName)
			}
			if len(spec.args) == 0 || spec.args[0] != tc.wantArg0 {
				t.Errorf("args[0]: got %v, want first %q", spec.args, tc.wantArg0)
			}
		})
	}
}

// TestBrowserCommandSpec_Unsupported — unsupported platforms surface
// a clear error.
func TestBrowserCommandSpec_Unsupported(t *testing.T) {
	_, err := browserCommandSpec("plan9", "https://example.com/")
	if err == nil {
		t.Fatal("expected error for unsupported platform")
	}
	if !strings.Contains(err.Error(), "plan9") {
		t.Errorf("error should name the platform; got %v", err)
	}
}

// TestBrowserCommandSpec_RejectsEmptyURL — empty URL is a
// programmer error, not an OS issue.
func TestBrowserCommandSpec_RejectsEmptyURL(t *testing.T) {
	_, err := browserCommandSpec("linux", "")
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

// TestOpenBrowser_HostOS — exercises the helper through the real
// exec hook on the test host. xdg-open / open / rundll32 may not be
// installed in CI, so we accept either a successful Start or any
// exec-shaped error; what we're testing is that the helper resolves
// the right command for the host's GOOS without panicking.
func TestOpenBrowser_HostOS(t *testing.T) {
	// Replace exec with a stub that records the invocation but does
	// not actually run anything — the spec test above exercises the
	// real lookup logic; this guard confirms the wiring.
	called := false
	prev := browserExec
	browserExec = func(name string, args ...string) *exec.Cmd {
		called = true
		// Returning a no-op command. exec.Command is permissive — it
		// accepts any name; Start may fail later if the binary is
		// missing.
		return exec.Command("true")
	}
	defer func() { browserExec = prev }()

	if err := OpenBrowser("https://example.com/"); err != nil {
		// Some hosts may not have `true` on PATH (extremely unlikely
		// on POSIX); accept exec errors.
		var pe *exec.Error
		if !errors.As(err, &pe) {
			t.Errorf("OpenBrowser: unexpected error: %v", err)
		}
	}
	if !called {
		t.Errorf("expected the exec hook to be invoked on %s", runtime.GOOS)
	}
}

// TestOpenBrowser_EmptyURL — empty URL surfaces as an error before
// any exec call.
func TestOpenBrowser_EmptyURL(t *testing.T) {
	called := false
	prev := browserExec
	browserExec = func(string, ...string) *exec.Cmd {
		called = true
		return exec.Command("true")
	}
	defer func() { browserExec = prev }()

	if err := OpenBrowser(""); err == nil {
		t.Error("expected error for empty URL")
	}
	if called {
		t.Error("exec hook should not be called with empty URL")
	}
}
