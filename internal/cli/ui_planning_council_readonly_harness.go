package cli

import (
	"fmt"
	"time"
)

// Terminal states of a fake read-only harness run. They are diagnostic only and
// never influence approval/finalDecision state.
const (
	councilHarnessFakeCompleted  = "fakeCompleted"
	councilHarnessScopeViolation = "scopeViolation"
	councilHarnessCleanupFailed  = "cleanupFailed"
	councilHarnessInvalidOptions = "invalidOptions"
	councilHarnessFakeFailed     = "fakeFailed"
)

// councilFakeRunner simulates the provider work of a FUTURE read-only run. It is
// injected by callers/tests and MUST NOT execute any provider CLI (no
// exec.Command for claude/codex/gemini). The harness hands it the built
// read-only options; a real implementation would only ever touch its own
// isolated temp cwd. Returning an error yields a fakeFailed result.
type councilFakeRunner func(opts readOnlyProviderOptions) error

// councilReadOnlyHarnessResult is the diagnostic outcome of one fake run. By
// construction it holds NO secret/token values, NO file contents, and NO prompt
// text — only non-sensitive args, paths and status. There is deliberately NO
// finalDecision field: the harness never reads or writes approval state.
type councilReadOnlyHarnessResult struct {
	ProviderID          string
	Status              string
	TempCWD             string
	StartedAt           string
	FinishedAt          string
	Args                []string
	CWD                 string
	MutationViolations  []string
	AllowedStateChanges []string
	Error               string
	CleanupOK           bool
}

// councilHarnessParams configures a single fake harness run. RepoRoot only guards
// temp-cwd placement (a run is never targeted at it). MutationDir is the
// repo/worktree whose mutations are snapshotted — intentionally SEPARATE from the
// isolated temp cwd where a provider would run.
type councilHarnessParams struct {
	Provider    string
	Prompt      string
	RepoRoot    string
	MutationDir string
	Runner      councilFakeRunner

	// cleanupOverride is a same-package test seam. When non-nil it replaces the
	// real workspace cleanup so a cleanup failure can be exercised without any OS
	// permission trickery. Production callers leave it nil.
	cleanupOverride func() error
}

// runCouncilReadOnlyHarness orchestrates the read-only helpers in integration
// order WITHOUT running any provider: create temp cwd -> before snapshot -> build
// non-dangerous options -> fake runner -> after snapshot -> mutation diff ->
// cleanup -> result. Any mutation outside the state.json exemption is a
// scopeViolation; a dangerous emitted flag is invalidOptions; a cleanup failure
// is cleanupFailed.
func runCouncilReadOnlyHarness(p councilHarnessParams) councilReadOnlyHarnessResult {
	res := councilReadOnlyHarnessResult{
		ProviderID: p.Provider,
		StartedAt:  time.Now().UTC().Format(time.RFC3339),
	}

	ws, cleanup, err := createReadOnlyCouncilWorkspace(p.RepoRoot)
	if err != nil {
		// Workspace creation cleans up after itself on failure; nothing to remove.
		res.Status = councilHarnessInvalidOptions
		res.Error = err.Error()
		res.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		return res
	}
	res.TempCWD = ws.Dir
	if p.cleanupOverride != nil {
		cleanup = p.cleanupOverride
	}

	opts, err := buildReadOnlyProviderOptions(p.Provider, p.Prompt, ws.Dir, p.RepoRoot)
	if err != nil {
		res.Status = councilHarnessInvalidOptions
		res.Error = err.Error()
		return finalizeCouncilHarness(res, cleanup)
	}
	res.Args = opts.Args
	res.CWD = opts.Cwd

	if flag, bad := containsDangerousFlag(opts.Args); bad {
		// Defensive: the builder should never emit these; refuse to run if it did.
		res.Status = councilHarnessInvalidOptions
		res.Error = fmt.Sprintf("dangerous flag emitted: %s", flag)
		return finalizeCouncilHarness(res, cleanup)
	}

	before, err := snapshotCouncilMutations(p.MutationDir)
	if err != nil {
		res.Status = councilHarnessFakeFailed
		res.Error = err.Error()
		return finalizeCouncilHarness(res, cleanup)
	}

	var runErr error
	if p.Runner != nil {
		runErr = p.Runner(opts)
	}

	after, err := snapshotCouncilMutations(p.MutationDir)
	if err != nil {
		res.Status = councilHarnessFakeFailed
		res.Error = err.Error()
		return finalizeCouncilHarness(res, cleanup)
	}

	diff := diffCouncilMutations(before, after)
	res.MutationViolations = diff.Violations
	res.AllowedStateChanges = diff.ExemptedChanges

	switch {
	case !diff.Clean:
		// A real-tree mutation dominates: never report success when the guarded
		// dir changed outside the exemption, even if the runner also errored.
		res.Status = councilHarnessScopeViolation
		if runErr != nil {
			res.Error = runErr.Error()
		}
	case runErr != nil:
		res.Status = councilHarnessFakeFailed
		res.Error = runErr.Error()
	default:
		res.Status = councilHarnessFakeCompleted
	}

	return finalizeCouncilHarness(res, cleanup)
}

// finalizeCouncilHarness runs cleanup once, records its outcome, and stamps the
// finish time. A cleanup failure sets CleanupOK=false and only promotes the
// status to cleanupFailed when the run had otherwise completed cleanly (a prior
// scopeViolation/fakeFailed/invalidOptions stays the primary story).
func finalizeCouncilHarness(res councilReadOnlyHarnessResult, cleanup func() error) councilReadOnlyHarnessResult {
	if cleanup != nil {
		if cErr := cleanup(); cErr != nil {
			res.CleanupOK = false
			if res.Status == councilHarnessFakeCompleted || res.Status == "" {
				res.Status = councilHarnessCleanupFailed
			}
			if res.Error == "" {
				res.Error = cErr.Error()
			}
		} else {
			res.CleanupOK = true
		}
	}
	res.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	return res
}
