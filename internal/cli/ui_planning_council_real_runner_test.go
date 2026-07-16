package cli

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The tests below cover the runner's execution outcomes. The provider stand-in is
// the helper process defined in ui_planning_council_real_runner_helper_test.go;
// no real provider binary is ever started.

func TestRealCouncilProviderRunner_CompletedDeliversPromptViaStdin(t *testing.T) {
	const prompt = "REVIEW-THIS-PROMPT"
	r, call := newHelperRunner(t, "", "COUNCIL_HELPER_ECHO_STDIN=1", "COUNCIL_HELPER_EXIT=0")
	opts := mustOpts(t, "claude", prompt, t.TempDir())

	res, err := r.Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != councilRunCompleted {
		t.Fatalf("status = %q, want completed (raw=%q)", res.Status, res.RawText)
	}
	if res.ExitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", res.ExitCode)
	}
	if !strings.Contains(res.RawText, "STDIN:"+prompt) {
		t.Fatalf("prompt did not reach stdin, raw = %q", res.RawText)
	}
	if res.Truncated {
		t.Fatalf("small output must not be truncated")
	}
	if res.ProviderID != "claude" {
		t.Fatalf("providerID = %q, want claude", res.ProviderID)
	}
	// The prompt must never be visible in the process arguments.
	for _, a := range call.args {
		if strings.Contains(a, prompt) {
			t.Fatalf("prompt leaked into args: %v", call.args)
		}
	}
	if opts.Stdin != prompt {
		t.Fatalf("opts.Stdin = %q, want the prompt", opts.Stdin)
	}
}

func TestRealCouncilProviderRunner_RunsInGivenTempCwd(t *testing.T) {
	cwd := t.TempDir()
	r, _ := newHelperRunner(t, "", "COUNCIL_HELPER_PRINT_CWD=1")
	res, _ := r.Run(context.Background(), mustOpts(t, "codex", "p", cwd))

	if res.Status != councilRunCompleted {
		t.Fatalf("status = %q, want completed", res.Status)
	}
	// Resolve symlinks: a temp dir can report a different path from inside.
	want, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(strings.TrimPrefix(firstLine(res.RawText), "CWD:"))
	gotResolved, err := filepath.EvalSymlinks(got)
	if err != nil {
		t.Fatalf("resolve reported cwd %q: %v", got, err)
	}
	if gotResolved != want {
		t.Fatalf("child cwd = %q, want %q", gotResolved, want)
	}
}

func TestRealCouncilProviderRunner_FailedRecordsExitCodeAndStderr(t *testing.T) {
	r, _ := newHelperRunner(t, "",
		"COUNCIL_HELPER_STDOUT=partial-answer",
		"COUNCIL_HELPER_STDERR=provider-blew-up",
		"COUNCIL_HELPER_EXIT=3",
	)
	res, _ := r.Run(context.Background(), mustOpts(t, "claude", "p", t.TempDir()))

	if res.Status != councilRunFailed {
		t.Fatalf("status = %q, want failed", res.Status)
	}
	if res.ExitCode != 3 {
		t.Fatalf("exitCode = %d, want 3", res.ExitCode)
	}
	if !strings.Contains(res.RawText, "partial-answer") {
		t.Fatalf("partial stdout must be preserved on failure: %q", res.RawText)
	}
	if !strings.Contains(res.RawText, "provider-blew-up") {
		t.Fatalf("stderr must be preserved in raw output: %q", res.RawText)
	}
	if !strings.Contains(res.Error, "provider-blew-up") {
		t.Fatalf("stderr must inform the error, got %q", res.Error)
	}
}

func TestRealCouncilProviderRunner_UnavailableWhenBinaryMissing(t *testing.T) {
	started := false
	r := realCouncilProviderRunner{
		lookPath: func(string) (string, error) { return "", errors.New("executable file not found in $PATH") },
		command: func(context.Context, string, ...string) *exec.Cmd {
			started = true
			t.Error("command must not be built when the binary is missing")
			return nil
		},
	}
	res, err := r.Run(context.Background(), mustOpts(t, "gemini", "p", t.TempDir()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != councilRunUnavailable {
		t.Fatalf("status = %q, want unavailable", res.Status)
	}
	if res.RawText != "" {
		t.Fatalf("unavailable must produce no output, got %q", res.RawText)
	}
	if started {
		t.Fatalf("no process may be started when the binary is missing")
	}
}

func TestRealCouncilProviderRunner_Timeout(t *testing.T) {
	r, _ := newHelperRunner(t, "", "COUNCIL_HELPER_STDOUT=partial", "COUNCIL_HELPER_SLEEP=30s")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	res, _ := r.Run(ctx, mustOpts(t, "claude", "p", t.TempDir()))
	if res.Status != councilRunTimeout {
		t.Fatalf("status = %q, want timeout", res.Status)
	}
}

func TestRealCouncilProviderRunner_Canceled(t *testing.T) {
	r, _ := newHelperRunner(t, "", "COUNCIL_HELPER_SLEEP=30s")
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-canceled

	res, _ := r.Run(ctx, mustOpts(t, "claude", "p", t.TempDir()))
	if res.Status != councilRunCanceled {
		t.Fatalf("status = %q, want canceled", res.Status)
	}
}

func TestRealCouncilProviderRunner_OutputTruncated(t *testing.T) {
	over := strconv.Itoa(councilMaxRawOutputBytes + 4096)
	r, _ := newHelperRunner(t, "", "COUNCIL_HELPER_STDOUT_BYTES="+over)
	res, _ := r.Run(context.Background(), mustOpts(t, "claude", "p", t.TempDir()))

	if res.Status != councilRunCompleted {
		t.Fatalf("status = %q, want completed", res.Status)
	}
	if !res.Truncated {
		t.Fatalf("oversized output must set Truncated")
	}
	if len(res.RawText) > councilMaxRawOutputBytes {
		t.Fatalf("rawText len = %d, want <= %d", len(res.RawText), councilMaxRawOutputBytes)
	}
}

func TestRealCouncilProviderRunner_StartFailureHasNoExitCode(t *testing.T) {
	// A cwd that no longer exists makes the child fail to start: there is no exit
	// status to report, only a start error.
	gone := filepath.Join(t.TempDir(), "removed")
	if err := os.MkdirAll(gone, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}
	r, _ := newHelperRunner(t, "")
	res, _ := r.Run(context.Background(), mustOpts(t, "claude", "p", gone))

	if res.Status != councilRunFailed {
		t.Fatalf("status = %q, want failed", res.Status)
	}
	if res.ExitCode != -1 {
		t.Fatalf("exitCode = %d, want -1 for a start failure", res.ExitCode)
	}
	if res.Error == "" {
		t.Fatalf("a start failure must be explained")
	}
}

func TestRealCouncilProviderRunner_NeverReportsAuthRequired(t *testing.T) {
	// Auth detection is out of scope: an auth-shaped exit is just a failure.
	r, _ := newHelperRunner(t, "", "COUNCIL_HELPER_STDERR=not logged in", "COUNCIL_HELPER_EXIT=1")
	res, _ := r.Run(context.Background(), mustOpts(t, "claude", "p", t.TempDir()))

	if res.Status == councilRunAuthRequired {
		t.Fatalf("runner must not infer authRequired in this PR")
	}
	if res.Status != councilRunFailed {
		t.Fatalf("status = %q, want failed", res.Status)
	}
}
