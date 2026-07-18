package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// realCouncilProviderRunner implements councilProviderRunner by starting a
// provider CLI as a child process.
//
// It is deliberately NOT wired to anything: there are zero production callers of
// this type. No endpoint, no UI, no state model, and no canAutoRun change accompany
// it. The legacy /api/workflow/run path is untouched and is never reused here.
//
// Safety properties, all enforced BEFORE any process starts:
//   - the provider must be one of claude/codex/gemini,
//   - no dangerous / auto-approve flag may appear in Args,
//   - Cwd must be non-empty and outside the repo root.
//
// The prompt travels via Stdin, never via Args, so it cannot leak through the
// process table. Nothing here reads a secret/token/config file, and no network
// access is performed.
type realCouncilProviderRunner struct {
	// lookPath resolves the provider binary. Injected so tests can stand in a
	// helper process and never touch a real provider binary.
	lookPath func(name string) (string, error)
	// command builds the child process. Injected for the same reason.
	command func(ctx context.Context, name string, args ...string) *exec.Cmd
	// repoRoot, when non-empty, is refused (along with anything inside it) as Cwd.
	repoRoot string
}

// newRealCouncilProviderRunner returns a runner backed by the real os/exec
// lookups. It starts nothing on its own; Run is the only entry point, and no
// production code calls it yet.
func newRealCouncilProviderRunner(repoRoot string) realCouncilProviderRunner {
	return realCouncilProviderRunner{
		lookPath: exec.LookPath,
		command:  exec.CommandContext,
		repoRoot: repoRoot,
	}
}

// councilStderrContextBytes caps how much stderr tail is echoed into Error. Raw
// output already lives in RawText; Error only needs enough to explain a failure.
const councilStderrContextBytes = 2048

// Run starts the provider and reports a diagnostic result. Like the fake, it
// returns a nil error and reports the outcome through result.Status.
//
// Status mapping: exit 0 -> completed, non-zero exit -> failed, deadline ->
// timeout, cancel -> canceled, binary not found -> unavailable. A non-zero exit
// whose output strongly matches an auth-failure pattern maps to authRequired
// instead of failed; anything ambiguous stays failed (see
// classifyCouncilProviderAuthFailure).
func (r realCouncilProviderRunner) Run(ctx context.Context, opts readOnlyProviderOptions) (councilProviderRunResult, error) {
	start := time.Now()
	res := councilProviderRunResult{ProviderID: opts.Provider, StartedAt: start.UTC().Format(time.RFC3339)}
	finish := func(status string) (councilProviderRunResult, error) {
		res.Status = status
		res.DurationMs = time.Since(start).Milliseconds()
		res.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		capped, truncated := capRawOutput(res.RawText)
		res.RawText = capped
		// Preserve a truncation already recorded while streaming.
		res.Truncated = res.Truncated || truncated
		return res, nil
	}
	reject := func(msg string) (councilProviderRunResult, error) {
		res.Error = msg
		res.ExitCode = -1
		return finish(councilRunFailed)
	}

	// --- Pre-flight. Every branch below returns before a process exists. ---
	switch opts.Provider {
	case "claude", "codex", "gemini":
	default:
		return reject(fmt.Sprintf("unknown provider: %q", opts.Provider))
	}
	if flag, bad := containsDangerousFlag(opts.Args); bad {
		return reject(fmt.Sprintf("dangerous flag rejected: %s", flag))
	}
	if strings.TrimSpace(opts.Cwd) == "" {
		return reject("read-only provider run requires an isolated temp cwd")
	}
	if r.repoRoot != "" {
		if same, err := sameOrInside(opts.Cwd, r.repoRoot); err == nil && same {
			return reject(fmt.Sprintf("refusing repo-root cwd for read-only provider: %s", opts.Cwd))
		}
	}

	bin, err := r.lookPath(opts.Provider)
	if err != nil {
		// No binary: nothing ran, so there is no output to report.
		res.Error = err.Error()
		return finish(councilRunUnavailable)
	}

	// --- Execution. ---
	cmd := r.command(ctx, bin, opts.Args...)
	cmd.Dir = opts.Cwd
	cmd.Stdin = strings.NewReader(opts.Stdin)
	// Separate writers: stdout and stderr each get their own goroutine inside
	// os/exec, so sharing one writer would race. Each is capped independently and
	// the combined text is capped again in finish, bounding retained output.
	outW := &cappedWriter{limit: councilMaxRawOutputBytes}
	errW := &cappedWriter{limit: councilMaxRawOutputBytes}
	cmd.Stdout = outW
	cmd.Stderr = errW

	// Past pre-flight and lookPath: a provider process is being launched, so the
	// run counts as executed even if it exits non-zero, times out or is canceled.
	res.ProcessStarted = true
	runErr := cmd.Run()

	// Preserve output on every path: a timed-out or failed provider may still have
	// produced a usable partial answer.
	res.RawText = outW.String() + errW.String()
	res.Truncated = outW.truncated || errW.truncated

	if ctx.Err() != nil {
		// CommandContext kills the child on ctx done, which surfaces as a generic
		// run error; the context is the authoritative reason.
		return finish(ctxStatus(ctx))
	}
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			res.ExitCode = exitErr.ExitCode()
			// A non-zero exit MAY mean the provider is not logged in. Classify
			// conservatively from the output already collected; an ambiguous
			// failure stays failed rather than becoming a false authRequired.
			if ok, reason := classifyCouncilProviderAuthFailure(opts.Provider, res.RawText, errW.String(), res.ExitCode); ok {
				res.Error = reason
				return finish(councilRunAuthRequired)
			}
		} else {
			// Failed to start (bad cwd, permissions, ...): no exit code exists, and a
			// start failure is not an auth signal.
			res.ExitCode = -1
		}
		res.Error = stderrContext(errW.String(), runErr)
		return finish(councilRunFailed)
	}
	res.ExitCode = 0
	return finish(councilRunCompleted)
}

// stderrContext explains a failure using the tail of stderr, falling back to the
// run error alone when the provider said nothing. Only provider output is used —
// no file is opened to enrich this message.
func stderrContext(stderr string, err error) string {
	s := strings.TrimSpace(stderr)
	if s == "" {
		return err.Error()
	}
	if len(s) > councilStderrContextBytes {
		s = s[len(s)-councilStderrContextBytes:]
	}
	return err.Error() + ": " + s
}

// cappedWriter retains at most limit bytes and records whether more arrived. It
// keeps a runaway provider from bloating memory while still reporting that output
// was cut. Each instance is written by a single os/exec goroutine.
type cappedWriter struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

// Write always reports the full length as consumed: the child must never observe
// a short write just because we stopped retaining bytes.
func (w *cappedWriter) Write(p []byte) (int, error) {
	remaining := w.limit - w.buf.Len()
	switch {
	case remaining <= 0:
		if len(p) > 0 {
			w.truncated = true
		}
	case len(p) > remaining:
		w.buf.Write(p[:remaining])
		w.truncated = true
	default:
		w.buf.Write(p)
	}
	return len(p), nil
}

// String returns the retained (possibly truncated) output.
func (w *cappedWriter) String() string { return w.buf.String() }
