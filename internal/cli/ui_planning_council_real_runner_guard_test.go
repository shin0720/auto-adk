package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The tests below cover the runner's pre-flight refusals: each one must reject
// BEFORE a provider binary is looked up or a process is started. The shared
// helper-process harness lives in ui_planning_council_real_runner_test.go.

func TestRealCouncilProviderRunner_DangerousFlagRejectedBeforeStart(t *testing.T) {
	for _, flag := range []string{"--dangerously-skip-permissions", "--full-auto", "--approval-mode=full-auto"} {
		r, call := newHelperRunner(t, "")
		opts := readOnlyProviderOptions{Provider: "claude", Args: []string{flag}, Cwd: t.TempDir(), Stdin: "p"}

		res, _ := r.Run(context.Background(), opts)
		if res.Status != councilRunFailed {
			t.Fatalf("%s: status = %q, want failed", flag, res.Status)
		}
		if !strings.Contains(res.Error, "dangerous flag") {
			t.Fatalf("%s: error should name the dangerous flag, got %q", flag, res.Error)
		}
		if call.invoked || call.lookedUp {
			t.Fatalf("%s: nothing may be looked up or started for dangerous args", flag)
		}
	}
}

func TestRealCouncilProviderRunner_UnknownProviderRejectedBeforeStart(t *testing.T) {
	r, call := newHelperRunner(t, "")
	opts := readOnlyProviderOptions{Provider: "gpt", Args: []string{"--print"}, Cwd: t.TempDir(), Stdin: "p"}

	res, _ := r.Run(context.Background(), opts)
	if res.Status != councilRunFailed {
		t.Fatalf("status = %q, want failed", res.Status)
	}
	if !strings.Contains(res.Error, "unknown provider") {
		t.Fatalf("error should name the unknown provider, got %q", res.Error)
	}
	if call.invoked || call.lookedUp {
		t.Fatalf("nothing may be started for an unknown provider")
	}
}

func TestRealCouncilProviderRunner_RejectsEmptyCwd(t *testing.T) {
	r, call := newHelperRunner(t, "")
	opts := readOnlyProviderOptions{Provider: "claude", Args: []string{"--print"}, Cwd: "  ", Stdin: "p"}

	res, _ := r.Run(context.Background(), opts)
	if res.Status != councilRunFailed {
		t.Fatalf("status = %q, want failed", res.Status)
	}
	if !strings.Contains(res.Error, "temp cwd") {
		t.Fatalf("error should explain the cwd requirement, got %q", res.Error)
	}
	if call.invoked {
		t.Fatalf("nothing may be started without an isolated cwd")
	}
}

func TestRealCouncilProviderRunner_RejectsRepoRootCwd(t *testing.T) {
	repoRoot := t.TempDir()
	inside := filepath.Join(repoRoot, "sub")
	if err := os.MkdirAll(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, cwd := range []string{repoRoot, inside} {
		r, call := newHelperRunner(t, repoRoot)
		opts := readOnlyProviderOptions{Provider: "claude", Args: []string{"--print"}, Cwd: cwd, Stdin: "p"}

		res, _ := r.Run(context.Background(), opts)
		if res.Status != councilRunFailed {
			t.Fatalf("cwd %s: status = %q, want failed", cwd, res.Status)
		}
		if !strings.Contains(res.Error, "refusing repo-root cwd") {
			t.Fatalf("cwd %s: error = %q, want repo-root refusal", cwd, res.Error)
		}
		if call.invoked {
			t.Fatalf("cwd %s: nothing may be started inside the repo", cwd)
		}
	}
}

func TestRealCouncilProviderRunner_AllowsTempCwdOutsideRepoRoot(t *testing.T) {
	r, call := newHelperRunner(t, t.TempDir(), "COUNCIL_HELPER_STDOUT=ok")
	res, _ := r.Run(context.Background(), mustOpts(t, "claude", "p", t.TempDir()))

	if res.Status != councilRunCompleted {
		t.Fatalf("status = %q, want completed for a temp cwd outside the repo", res.Status)
	}
	if !call.invoked {
		t.Fatalf("a valid temp cwd should have started the child")
	}
}

func TestNewRealCouncilProviderRunner_Defaults(t *testing.T) {
	r := newRealCouncilProviderRunner("/repo")
	if r.lookPath == nil || r.command == nil {
		t.Fatalf("constructor must wire real exec lookups")
	}
	if r.repoRoot != "/repo" {
		t.Fatalf("repoRoot = %q, want /repo", r.repoRoot)
	}
	var _ councilProviderRunner = realCouncilProviderRunner{}
}

func TestStderrContext(t *testing.T) {
	base := errors.New("exit status 1")
	if got := stderrContext("   ", base); got != base.Error() {
		t.Fatalf("empty stderr should yield the bare error, got %q", got)
	}
	if got := stderrContext("boom", base); !strings.Contains(got, "boom") {
		t.Fatalf("stderr should be appended, got %q", got)
	}
	long := strings.Repeat("x", councilStderrContextBytes+500)
	got := stderrContext(long, base)
	// Only the tail is kept, so the message stays bounded.
	if len(got) > len(base.Error())+2+councilStderrContextBytes {
		t.Fatalf("stderr context should be trimmed to the tail, len = %d", len(got))
	}
}

func TestCappedWriter(t *testing.T) {
	w := &cappedWriter{limit: 4}
	n, err := w.Write([]byte("ab"))
	if n != 2 || err != nil {
		t.Fatalf("Write = (%d, %v), want (2, nil)", n, err)
	}
	if w.truncated {
		t.Fatalf("under limit must not truncate")
	}
	// Crosses the limit: the prefix is kept and truncation is recorded.
	if n, _ = w.Write([]byte("cdef")); n != 4 {
		t.Fatalf("Write must report the full length as consumed, got %d", n)
	}
	if w.String() != "abcd" {
		t.Fatalf("buf = %q, want abcd", w.String())
	}
	if !w.truncated {
		t.Fatalf("crossing the limit must set truncated")
	}
	// Already full: further writes are dropped but still reported as consumed.
	if n, _ = w.Write([]byte("gh")); n != 2 {
		t.Fatalf("Write after limit must report full length, got %d", n)
	}
	if w.String() != "abcd" {
		t.Fatalf("buf changed after limit: %q", w.String())
	}
	// A zero-length write on a full buffer is not a truncation event.
	w2 := &cappedWriter{limit: 0}
	if n, _ = w2.Write(nil); n != 0 {
		t.Fatalf("empty write = %d, want 0", n)
	}
	if w2.truncated {
		t.Fatalf("empty write must not record truncation")
	}
}
