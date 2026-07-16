package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Output beyond the cap is truncated and flagged, never silently dropped.
func TestCouncilRunEndpoint_OutputTruncated(t *testing.T) {
	big := strings.Repeat("x", councilMaxRawOutputBytes+1024)
	runner := &recordingCouncilRunner{inner: fakeCouncilProviderRunner{status: councilRunCompleted, rawText: big}}
	h := newPlanningCouncilProviderRunHandler(councilEndpointDeps(t, runner))

	_, resp := councilRunPost(t, h, councilRunBody)

	if !resp.Truncated {
		t.Error("truncated = false, want true")
	}
	if len(resp.RawText) != councilMaxRawOutputBytes {
		t.Errorf("rawText len = %d, want %d", len(resp.RawText), councilMaxRawOutputBytes)
	}
}

// A runner that mutates the guarded tree is reported as a scope violation, which
// outranks the runner's own success.
func TestCouncilRunEndpoint_MutationScopeViolation(t *testing.T) {
	repo := newCouncilHarnessRepo(t)
	runner := &recordingCouncilRunner{
		inner:   fakeCouncilProviderRunner{status: councilRunCompleted, rawText: "ok"},
		writeTo: filepath.Join(repo, "leaked.txt"),
	}
	h := newPlanningCouncilProviderRunHandler(planningCouncilProviderRunEndpointDeps{
		RepoRoot: repo, MutationDir: repo, Runner: runner, Enabled: true,
	})

	_, resp := councilRunPost(t, h, councilRunBody)

	if resp.Status != councilHarnessScopeViolation {
		t.Fatalf("status = %q, want scopeViolation", resp.Status)
	}
	if !councilContains(resp.MutationViolations, "leaked.txt") {
		t.Errorf("mutationViolations = %v, want leaked.txt", resp.MutationViolations)
	}
}

// A cleanup failure on an otherwise clean run surfaces as cleanupFailed.
func TestCouncilRunEndpoint_CleanupFailed(t *testing.T) {
	runner := &recordingCouncilRunner{inner: fakeCouncilProviderRunner{status: councilRunCompleted, rawText: "ok"}}
	deps := councilEndpointDeps(t, runner)
	deps.cleanupOverride = func() error { return errors.New("temp dir busy") }
	h := newPlanningCouncilProviderRunHandler(deps)

	// The override skips the real removal; clean up the leaked temp workspace
	// ourselves using the cwd the runner was handed.
	t.Cleanup(func() { _ = os.RemoveAll(runner.lastOpts.Cwd) })

	_, resp := councilRunPost(t, h, councilRunBody)

	if resp.Status != councilHarnessCleanupFailed {
		t.Errorf("status = %q, want cleanupFailed", resp.Status)
	}
	if resp.CleanupOK {
		t.Error("cleanupOK = true, want false")
	}
}

// A runner that cannot report at all degrades to failed rather than a false
// success.
func TestCouncilRunEndpoint_TransportErrorFails(t *testing.T) {
	runner := &recordingCouncilRunner{transportErr: errors.New("runner exploded")}
	h := newPlanningCouncilProviderRunHandler(councilEndpointDeps(t, runner))

	_, resp := councilRunPost(t, h, councilRunBody)

	if resp.Status != councilRunFailed {
		t.Errorf("status = %q, want failed", resp.Status)
	}
}

// The endpoint must never infer or expose an auth verdict: a runner claiming
// authRequired degrades to failed, and authRequired is not part of the contract.
func TestCouncilRunEndpoint_NeverReportsAuthRequired(t *testing.T) {
	runner := &recordingCouncilRunner{inner: fakeCouncilProviderRunner{status: councilRunAuthRequired}}
	h := newPlanningCouncilProviderRunHandler(councilEndpointDeps(t, runner))

	_, resp := councilRunPost(t, h, councilRunBody)

	if resp.Status == councilRunAuthRequired {
		t.Fatal("endpoint must not return authRequired")
	}
	if resp.Status != councilRunFailed {
		t.Errorf("status = %q, want failed", resp.Status)
	}
}

// The auth hint is advisory and comes from the status view, never from the run.
func TestCouncilRunEndpoint_AuthStateHintIsAdvisory(t *testing.T) {
	runner := &recordingCouncilRunner{inner: fakeCouncilProviderRunner{status: councilRunCompleted, rawText: "ok"}}
	deps := councilEndpointDeps(t, runner)
	deps.AuthStateHint = func(p string) string { return "authRequired" }
	h := newPlanningCouncilProviderRunHandler(deps)

	_, resp := councilRunPost(t, h, councilRunBody)

	if resp.AuthStateHint != "authRequired" {
		t.Errorf("authStateHint = %q", resp.AuthStateHint)
	}
	// The hint must not colour the run's own verdict.
	if resp.Status != councilRunCompleted {
		t.Errorf("status = %q, want completed: the hint is advisory only", resp.Status)
	}
}

// A harness-level failure (here: a MutationDir that is not a git repo, so the
// snapshot fails) degrades to failed rather than a false success.
func TestCouncilRunEndpoint_HarnessSnapshotFailureFails(t *testing.T) {
	runner := &recordingCouncilRunner{inner: fakeCouncilProviderRunner{status: councilRunCompleted, rawText: "ok"}}
	h := newPlanningCouncilProviderRunHandler(planningCouncilProviderRunEndpointDeps{
		RepoRoot: t.TempDir(), MutationDir: filepath.Join(t.TempDir(), "not-a-repo"),
		Runner: runner, Enabled: true,
	})

	_, resp := councilRunPost(t, h, councilRunBody)

	if resp.Status != councilRunFailed {
		t.Errorf("status = %q, want failed", resp.Status)
	}
	if resp.Error == "" {
		t.Error("error must explain the harness failure")
	}
}
