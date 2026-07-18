package cli

import (
	"context"
	"errors"
	"os/exec"
	"testing"
)

// A non-zero exit whose stderr strongly signals a login problem maps to
// authRequired, driven by the helper process — no real provider binary runs.
func TestRealCouncilProviderRunner_AuthRequiredFromStderr(t *testing.T) {
	r, call := newHelperRunner(t, "",
		"COUNCIL_HELPER_STDERR=Error: you are not logged in",
		"COUNCIL_HELPER_EXIT=1",
	)
	res, _ := r.Run(context.Background(), mustOpts(t, "claude", "p", t.TempDir()))

	if res.Status != councilRunAuthRequired {
		t.Fatalf("status = %q, want authRequired", res.Status)
	}
	if !res.ProcessStarted {
		t.Error("a process was launched, so ProcessStarted must be true")
	}
	if !call.invoked {
		t.Error("the helper stand-in should have been invoked")
	}
	// The error is the fixed classification reason, not the raw stderr line.
	if res.Error == "" {
		t.Error("authRequired should carry a classification reason")
	}
}

// A non-zero exit with ordinary stderr stays failed, never a false authRequired.
func TestRealCouncilProviderRunner_NonAuthFailureStaysFailed(t *testing.T) {
	r, _ := newHelperRunner(t, "",
		"COUNCIL_HELPER_STDERR=error: rate limit exceeded",
		"COUNCIL_HELPER_EXIT=1",
	)
	res, _ := r.Run(context.Background(), mustOpts(t, "codex", "p", t.TempDir()))

	if res.Status != councilRunFailed {
		t.Fatalf("status = %q, want failed", res.Status)
	}
	if !res.ProcessStarted {
		t.Error("a process was launched, so ProcessStarted must be true")
	}
}

// A clean exit reports completed with ProcessStarted true.
func TestRealCouncilProviderRunner_CompletedSetsProcessStarted(t *testing.T) {
	r, _ := newHelperRunner(t, "",
		"COUNCIL_HELPER_STDOUT=verdict",
		"COUNCIL_HELPER_EXIT=0",
	)
	res, _ := r.Run(context.Background(), mustOpts(t, "gemini", "p", t.TempDir()))

	if res.Status != councilRunCompleted {
		t.Fatalf("status = %q, want completed", res.Status)
	}
	if !res.ProcessStarted {
		t.Error("ProcessStarted must be true after a real launch")
	}
}

// A missing binary never launches a process: unavailable, ProcessStarted false,
// and the auth classifier is never consulted.
func TestRealCouncilProviderRunner_UnavailableHasNoProcessStarted(t *testing.T) {
	r := realCouncilProviderRunner{
		lookPath: func(string) (string, error) { return "", errors.New("not found") },
		command: func(context.Context, string, ...string) *exec.Cmd {
			t.Error("command must not be built when the binary is missing")
			return nil
		},
	}
	res, _ := r.Run(context.Background(), mustOpts(t, "claude", "p", t.TempDir()))

	if res.Status != councilRunUnavailable {
		t.Fatalf("status = %q, want unavailable", res.Status)
	}
	if res.ProcessStarted {
		t.Error("no process was launched, so ProcessStarted must be false")
	}
}

// A pre-flight rejection (dangerous flag) never launches a process and is never
// classified as auth.
func TestRealCouncilProviderRunner_DangerousFlagNotAuthNoProcess(t *testing.T) {
	r := realCouncilProviderRunner{
		lookPath: func(name string) (string, error) { return name, nil },
		command: func(context.Context, string, ...string) *exec.Cmd {
			t.Error("command must not be built when a dangerous flag is present")
			return nil
		},
	}
	opts := readOnlyProviderOptions{Provider: "claude", Args: []string{"--dangerously-skip-permissions"}, Cwd: t.TempDir()}
	res, _ := r.Run(context.Background(), opts)

	if res.Status != councilRunFailed {
		t.Fatalf("status = %q, want failed", res.Status)
	}
	if res.ProcessStarted {
		t.Error("a rejected run must not report ProcessStarted")
	}
}
