package cli

import (
	"net/http"
	"testing"
)

// These tests pin the invariant the run-endpoint comments now describe: the route
// is a real production surface, yet in the default configuration it answers
// "disabled" without ever building or invoking a provider runner. They exercise the
// production wiring (buildPlanningCouncilProviderRunDeps) rather than a hand-built
// deps struct, so the factory-not-called guarantee is asserted at the layer that
// actually wires production.

// TestBuildPlanningCouncilProviderRunDeps_DefaultEnvNeverBuildsRunner asserts that
// with the gate variable unset, the deps builder does NOT call the real-runner
// factory, leaves Runner nil and Enabled false, and reports the gate as disabled.
// This is the construction-layer proof that no provider runner exists by default.
func TestBuildPlanningCouncilProviderRunDeps_DefaultEnvNeverBuildsRunner(t *testing.T) {
	factoryCalls := 0
	deps := buildPlanningCouncilProviderRunDeps(planningCouncilGateOptions{
		repoRoot: t.TempDir(),
		getenv:   envMap(nil), // default runtime: AUTOPUS_PLANNING_COUNCIL_PROVIDER_RUN unset
		newRealRunner: func(string) councilProviderRunner {
			factoryCalls++
			return &recordingCouncilRunner{}
		},
	})

	if factoryCalls != 0 {
		t.Fatalf("real-runner factory called %d times; must be 0 with the gate unset", factoryCalls)
	}
	if deps.Runner != nil {
		t.Error("Runner must be nil in the default configuration")
	}
	if deps.Enabled {
		t.Error("Enabled must be false in the default configuration")
	}
	if got := councilRunGate(deps); got != councilGateDisabled {
		t.Errorf("gate = %q, want %q", got, councilGateDisabled)
	}
}

// TestPlanningCouncilProviderRunHandler_DefaultDepsRespondDisabledWithoutRunner
// drives a POST through the handler built from the DEFAULT production deps and
// asserts it answers "disabled" while the real-runner factory and the runner's Run
// method are both never called: no provider subprocess or model call is reachable.
func TestPlanningCouncilProviderRunHandler_DefaultDepsRespondDisabledWithoutRunner(t *testing.T) {
	factoryCalls := 0
	spy := &recordingCouncilRunner{}
	deps := buildPlanningCouncilProviderRunDeps(planningCouncilGateOptions{
		repoRoot: t.TempDir(),
		getenv:   envMap(nil), // default runtime: gate off
		newRealRunner: func(string) councilProviderRunner {
			factoryCalls++
			return spy
		},
	})
	h := newPlanningCouncilProviderRunHandler(deps)

	rec, resp := councilRunPost(t, h, councilRunBody)

	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	if resp.Status != councilEndpointDisabled {
		t.Errorf("status = %q, want %q", resp.Status, councilEndpointDisabled)
	}
	if resp.Gate != councilGateDisabled {
		t.Errorf("gate = %q, want %q", resp.Gate, councilGateDisabled)
	}
	if resp.Executed {
		t.Error("executed must be false: the default route can never run a provider")
	}
	if factoryCalls != 0 {
		t.Errorf("real-runner factory called %d times; the disabled path must build no runner", factoryCalls)
	}
	if spy.calls != 0 {
		t.Errorf("runner.Run invoked %d times; no provider process may be reached", spy.calls)
	}
}

// TestPlanningCouncilProviderRunHandler_EnabledEnvWiresRealRunner is the negative
// control: when the gate env is exactly "1", the builder DOES call the factory and
// the gate stops reporting "disabled". It keeps the default-disabled tests honest by
// proving they assert a real gate rather than a permanently dead code path. No real
// provider runs here — the factory returns a recording spy, never a real runner.
func TestPlanningCouncilProviderRunHandler_EnabledEnvWiresRealRunner(t *testing.T) {
	factoryCalls := 0
	deps := buildPlanningCouncilProviderRunDeps(planningCouncilGateOptions{
		repoRoot: t.TempDir(),
		getenv:   envMap(map[string]string{planningCouncilProviderRunEnv: "1"}),
		newRealRunner: func(string) councilProviderRunner {
			factoryCalls++
			return &recordingCouncilRunner{}
		},
	})

	if factoryCalls != 1 {
		t.Fatalf("real-runner factory called %d times; want exactly 1 when the gate is \"1\"", factoryCalls)
	}
	if deps.Runner == nil {
		t.Error("Runner must be non-nil when the gate is enabled")
	}
	if got := councilRunGate(deps); got == councilGateDisabled {
		t.Errorf("gate = %q; enabled deps must not report disabled", got)
	}
}
