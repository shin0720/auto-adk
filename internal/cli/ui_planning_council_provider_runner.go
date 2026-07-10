package cli

import (
	"context"
	"fmt"
	"time"
)

// Terminal statuses of a provider runner invocation. Diagnostic only — they never
// influence approval/finalDecision state.
const (
	councilRunCompleted    = "completed"
	councilRunFailed       = "failed"
	councilRunTimeout      = "timeout"
	councilRunCanceled     = "canceled"
	councilRunAuthRequired = "authRequired"
	councilRunUnavailable  = "unavailable"
)

// councilMaxRawOutputBytes caps retained raw output so a runaway provider cannot
// bloat memory/state. Output beyond this is truncated and Truncated is set; what
// remains is still preserved. Secret filtering is NOT done here (a later concern);
// this PR simply never logs raw output or secrets.
const councilMaxRawOutputBytes = 256 * 1024

// councilProviderRunResult is the diagnostic outcome of one runner invocation. By
// construction it holds NO secret/token values, NO file contents, and NO prompt
// text — only (capped) raw provider output, timing and status. There is
// deliberately NO finalDecision field: a runner never touches approval state.
type councilProviderRunResult struct {
	ProviderID string
	Status     string
	RawText    string
	ExitCode   int
	DurationMs int64
	StartedAt  string
	FinishedAt string
	Error      string
	Truncated  bool
}

// councilProviderRunner abstracts "run this read-only invocation and return raw
// output". A fake implementation exists today; a gated REAL implementation is a
// LATER PR. Implementations MUST NOT (in this PR) start a provider process, read
// file contents, emit a dangerous flag, or run in the repo cwd. opts.Cwd is
// always an isolated temp workspace.
type councilProviderRunner interface {
	Run(ctx context.Context, opts readOnlyProviderOptions) (councilProviderRunResult, error)
}

// fakeCouncilProviderRunner is a test/seam-verification runner. It never starts a
// process, never touches the network, and never reads secrets. It plays back a
// preconfigured scenario while honoring ctx cancellation/deadline.
type fakeCouncilProviderRunner struct {
	// status selects the terminal outcome for the "ran to completion" branches
	// (completed/failed) and the "never ran" branches (authRequired/unavailable).
	// timeout/canceled are driven by ctx, not this field.
	status string
	// rawText is the output to report. It is preserved even on failure/timeout so
	// a partial provider response is never lost.
	rawText string
	exitCode int
	// work simulates in-flight duration; keep it short in tests. While working the
	// runner respects ctx: a deadline yields timeout, a cancel yields canceled.
	work time.Duration
	// errMsg is recorded on the failed branch.
	errMsg string
}

// Run executes the configured fake scenario. It returns a result (never a
// transport error) so the caller reads outcome from result.Status.
func (f fakeCouncilProviderRunner) Run(ctx context.Context, opts readOnlyProviderOptions) (councilProviderRunResult, error) {
	start := time.Now()
	res := councilProviderRunResult{ProviderID: opts.Provider, StartedAt: start.UTC().Format(time.RFC3339)}
	finish := func(status string) (councilProviderRunResult, error) {
		res.Status = status
		res.DurationMs = time.Since(start).Milliseconds()
		res.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		res.RawText, res.Truncated = capRawOutput(res.RawText)
		return res, nil
	}

	// Defensive: never proceed with a dangerous flag, even for a fake.
	if flag, bad := containsDangerousFlag(opts.Args); bad {
		res.Error = fmt.Sprintf("dangerous flag rejected: %s", flag)
		res.ExitCode = -1
		return finish(councilRunFailed)
	}
	switch opts.Provider {
	case "claude", "codex", "gemini":
	default:
		res.Error = fmt.Sprintf("unknown provider: %q", opts.Provider)
		res.ExitCode = -1
		return finish(councilRunFailed)
	}

	// authRequired / unavailable mean the runner never runs: no work, no output.
	switch f.status {
	case councilRunAuthRequired:
		return finish(councilRunAuthRequired)
	case councilRunUnavailable:
		return finish(councilRunUnavailable)
	}

	// Set rawText up front so a timeout/cancel still preserves partial output.
	res.RawText = f.rawText
	if f.work > 0 {
		select {
		case <-time.After(f.work):
		case <-ctx.Done():
			return finish(ctxStatus(ctx))
		}
	} else if ctx.Err() != nil {
		return finish(ctxStatus(ctx))
	}

	if f.status == councilRunFailed {
		res.ExitCode = f.exitCode
		if res.ExitCode == 0 {
			res.ExitCode = 1
		}
		res.Error = f.errMsg
		return finish(councilRunFailed)
	}
	res.ExitCode = 0
	return finish(councilRunCompleted)
}

// ctxStatus maps a done context to the corresponding terminal status.
func ctxStatus(ctx context.Context) string {
	if ctx.Err() == context.DeadlineExceeded {
		return councilRunTimeout
	}
	return councilRunCanceled
}

// capRawOutput truncates raw output to the cap, reporting whether truncation
// happened. Truncated output is still returned (never discarded entirely).
func capRawOutput(s string) (string, bool) {
	if len(s) <= councilMaxRawOutputBytes {
		return s, false
	}
	return s[:councilMaxRawOutputBytes], true
}
