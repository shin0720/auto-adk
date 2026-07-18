package cli

import (
	"context"
	"net/http"
	"testing"
)

// processStartedRunner is a fake that reports a launched process WITHOUT running
// one, so the endpoint's Executed plumbing can be exercised with no provider
// binary. It stands in for a real runner's ProcessStarted signal only.
type processStartedRunner struct{}

func (processStartedRunner) Run(context.Context, readOnlyProviderOptions) (councilProviderRunResult, error) {
	return councilProviderRunResult{Status: councilRunCompleted, ProcessStarted: true}, nil
}

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

// An enabled gate with no explicit label defaults to "fake", and a fake run
// reports executed false because no process was launched.
func TestCouncilRunEndpoint_EnabledGateDefaultsToFake(t *testing.T) {
	runner := &recordingCouncilRunner{inner: fakeCouncilProviderRunner{status: councilRunCompleted, rawText: "ok"}}
	h := newPlanningCouncilProviderRunHandler(councilEndpointDeps(t, runner))

	_, resp := councilRunPost(t, h, councilRunBody)

	if resp.Gate != councilGateFake {
		t.Errorf("gate = %q, want fake", resp.Gate)
	}
	if resp.Executed {
		t.Error("executed must be false for a fake runner: no process launched")
	}
}

// Gate is no longer hard-coded: an explicit GateLabel flows through to the
// response, so a future real wiring reports "real" instead of masquerading as fake.
func TestCouncilRunEndpoint_GateLabelIsHonoured(t *testing.T) {
	runner := &recordingCouncilRunner{inner: fakeCouncilProviderRunner{status: councilRunCompleted, rawText: "ok"}}
	deps := councilEndpointDeps(t, runner)
	deps.GateLabel = councilGateReal
	h := newPlanningCouncilProviderRunHandler(deps)

	_, resp := councilRunPost(t, h, councilRunBody)

	if resp.Gate != councilGateReal {
		t.Errorf("gate = %q, want real", resp.Gate)
	}
}

// Executed is no longer hard-coded false: a runner reporting ProcessStarted flows
// through to the response, so a real launch would be reported honestly. This uses
// a fake stand-in that sets ProcessStarted — no actual provider binary runs.
func TestCouncilRunEndpoint_ExecutedReflectsProcessStarted(t *testing.T) {
	runner := &processStartedRunner{}
	deps := councilEndpointDeps(t, runner)
	deps.GateLabel = councilGateReal
	h := newPlanningCouncilProviderRunHandler(deps)

	_, resp := councilRunPost(t, h, councilRunBody)

	if !resp.Executed {
		t.Error("executed must be true when the runner reports a launched process")
	}
	if resp.Gate != councilGateReal {
		t.Errorf("gate = %q, want real", resp.Gate)
	}
}
