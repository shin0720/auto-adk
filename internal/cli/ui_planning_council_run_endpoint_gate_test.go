package cli

import (
	"net/http"
	"testing"
)

// The gate defaults to disabled: a zero-value deps struct must never reach the
// runner or the harness.
func TestCouncilRunEndpoint_DefaultDisabled(t *testing.T) {
	runner := &recordingCouncilRunner{inner: fakeCouncilProviderRunner{status: councilRunCompleted, rawText: "ok"}}
	// Enabled is deliberately left at its zero value.
	h := newPlanningCouncilProviderRunHandler(planningCouncilProviderRunEndpointDeps{
		RepoRoot: t.TempDir(), MutationDir: t.TempDir(), Runner: runner,
	})

	rec, resp := councilRunPost(t, h, councilRunBody)

	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	if resp.Status != councilEndpointDisabled {
		t.Errorf("status = %q, want disabled", resp.Status)
	}
	if resp.Gate != councilGateDisabled {
		t.Errorf("gate = %q, want disabled", resp.Gate)
	}
	if resp.Executed {
		t.Error("executed must be false when disabled")
	}
	if runner.calls != 0 {
		t.Errorf("runner calls = %d, want 0: a disabled gate must not run anything", runner.calls)
	}
	// A disabled run never creates a workspace, so there is nothing to clean up.
	if resp.CleanupOK {
		t.Error("cleanupOK = true, but no workspace was created")
	}
	if resp.ProviderID != "claude" {
		t.Errorf("providerId = %q, want claude", resp.ProviderID)
	}
}

// A nil Runner reads as disabled: the handler must never construct a runner of its
// own (which is what would open a real provider execution path).
func TestCouncilRunEndpoint_NilRunnerIsDisabled(t *testing.T) {
	h := newPlanningCouncilProviderRunHandler(planningCouncilProviderRunEndpointDeps{
		RepoRoot: t.TempDir(), MutationDir: t.TempDir(), Enabled: true, Runner: nil,
	})

	_, resp := councilRunPost(t, h, councilRunBody)

	if resp.Status != councilEndpointDisabled {
		t.Errorf("status = %q, want disabled", resp.Status)
	}
	if resp.Gate != councilGateDisabled {
		t.Errorf("gate = %q, want disabled", resp.Gate)
	}
}

// The gate is reported on rejected requests too, so a caller can tell a disabled
// endpoint from a bad request.
func TestCouncilRunEndpoint_DisabledGateReportedOnInvalidRequest(t *testing.T) {
	h := newPlanningCouncilProviderRunHandler(planningCouncilProviderRunEndpointDeps{})

	rec, resp := councilRunPost(t, h, `{"provider":"nope","prompt":"x"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", rec.Code)
	}
	if resp.Status != councilEndpointInvalidRequest {
		t.Errorf("status = %q, want invalidRequest", resp.Status)
	}
	if resp.Gate != councilGateDisabled {
		t.Errorf("gate = %q, want disabled", resp.Gate)
	}
}

// An enabled gate reports "fake": this PR has no real gate value, so no response
// can ever claim a real provider ran.
func TestCouncilRunEndpoint_EnabledGateIsFake(t *testing.T) {
	runner := &recordingCouncilRunner{inner: fakeCouncilProviderRunner{status: councilRunCompleted, rawText: "ok"}}
	h := newPlanningCouncilProviderRunHandler(councilEndpointDeps(t, runner))

	_, resp := councilRunPost(t, h, councilRunBody)

	if resp.Gate != councilGateFake {
		t.Errorf("gate = %q, want fake", resp.Gate)
	}
	if resp.Executed {
		t.Error("executed must be false even on the enabled path")
	}
}
