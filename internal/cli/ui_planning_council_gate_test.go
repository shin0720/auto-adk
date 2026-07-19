package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// envMap returns a getenv func backed by a map, so tests never touch the real
// process environment (and never need to restore it).
func envMap(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// Only the exact value "1" enables the gate. Everything else — including
// whitespace-padded "1", other truthy words, and absence — is disabled.
func TestPlanningCouncilProviderRunEnabled_ExactlyOne(t *testing.T) {
	cases := []struct {
		val  string
		want bool
	}{
		{"1", true},
		{"", false},
		{"0", false},
		{"true", false},
		{"TRUE", false},
		{"yes", false},
		{"on", false},
		{" 1", false},
		{"1 ", false},
		{" 1 ", false},
		{"11", false},
		{"1\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.val, func(t *testing.T) {
			got := planningCouncilProviderRunEnabled(envMap(map[string]string{
				planningCouncilProviderRunEnv: tc.val,
			}))
			if got != tc.want {
				t.Errorf("enabled(%q) = %v, want %v", tc.val, got, tc.want)
			}
		})
	}
}

// An absent variable (key not present at all) is disabled.
func TestPlanningCouncilProviderRunEnabled_Absent(t *testing.T) {
	if planningCouncilProviderRunEnabled(envMap(map[string]string{})) {
		t.Error("absent env var must be disabled")
	}
}

// A nil getenv must not panic; it falls back to os.Getenv, which is unset in tests.
func TestPlanningCouncilProviderRunEnabled_NilGetenv(t *testing.T) {
	if planningCouncilProviderRunEnabled(nil) {
		t.Error("nil getenv must resolve to disabled when the real env is unset")
	}
}

// countingRunnerFactory records how many times it was asked to build a runner and
// hands back a harmless fake, so the enabled path can be exercised without ever
// constructing a real runner (let alone executing a provider).
func countingRunnerFactory(calls *int, gotRoot *string) func(string) councilProviderRunner {
	return func(root string) councilProviderRunner {
		*calls++
		*gotRoot = root
		return fakeCouncilProviderRunner{status: councilRunCompleted}
	}
}

// With the gate off, the deps are hard-disabled and the real-runner factory is
// never called.
func TestBuildDeps_DisabledByDefault(t *testing.T) {
	calls := 0
	var gotRoot string
	deps := buildPlanningCouncilProviderRunDeps(planningCouncilGateOptions{
		repoRoot:      "/repo",
		getenv:        envMap(map[string]string{}),
		newRealRunner: countingRunnerFactory(&calls, &gotRoot),
	})

	if deps.Runner != nil {
		t.Error("Runner must be nil when the gate is off")
	}
	if deps.Enabled {
		t.Error("Enabled must be false when the gate is off")
	}
	if calls != 0 {
		t.Errorf("factory calls = %d, want 0: no runner may be built when disabled", calls)
	}
	if got := councilRunGate(deps); got != councilGateDisabled {
		t.Errorf("gate = %q, want disabled", got)
	}
}

// An invalid env value likewise never builds a runner.
func TestBuildDeps_InvalidEnvNeverBuildsRunner(t *testing.T) {
	for _, val := range []string{"true", "0", " 1 ", "yes"} {
		t.Run(val, func(t *testing.T) {
			calls := 0
			var gotRoot string
			deps := buildPlanningCouncilProviderRunDeps(planningCouncilGateOptions{
				repoRoot:      "/repo",
				getenv:        envMap(map[string]string{planningCouncilProviderRunEnv: val}),
				newRealRunner: countingRunnerFactory(&calls, &gotRoot),
			})
			if deps.Runner != nil || deps.Enabled || calls != 0 {
				t.Errorf("val %q must stay disabled: runner=%v enabled=%v calls=%d",
					val, deps.Runner != nil, deps.Enabled, calls)
			}
		})
	}
}

// With the gate on, the factory is called once with the repo root, and the deps
// report an enabled real gate. A FAKE factory stands in, so no real runner is
// constructed and no provider binary is ever touched.
func TestBuildDeps_EnabledWiresRunner(t *testing.T) {
	calls := 0
	var gotRoot string
	deps := buildPlanningCouncilProviderRunDeps(planningCouncilGateOptions{
		repoRoot:      "/repo",
		getenv:        envMap(map[string]string{planningCouncilProviderRunEnv: "1"}),
		newRealRunner: countingRunnerFactory(&calls, &gotRoot),
	})

	if deps.Runner == nil {
		t.Fatal("Runner must be wired when the gate is on")
	}
	if !deps.Enabled {
		t.Error("Enabled must be true when the gate is on")
	}
	if calls != 1 {
		t.Errorf("factory calls = %d, want 1", calls)
	}
	if gotRoot != "/repo" {
		t.Errorf("factory root = %q, want /repo", gotRoot)
	}
	if got := councilRunGate(deps); got != councilGateReal {
		t.Errorf("gate = %q, want real", got)
	}
}

// A nil factory keeps the gate closed even when the env says enabled: there is no
// way to build a runner, so execution stays impossible.
func TestBuildDeps_EnabledButNilFactoryStaysDisabled(t *testing.T) {
	deps := buildPlanningCouncilProviderRunDeps(planningCouncilGateOptions{
		repoRoot:      "/repo",
		getenv:        envMap(map[string]string{planningCouncilProviderRunEnv: "1"}),
		newRealRunner: nil,
	})
	if deps.Runner != nil || deps.Enabled {
		t.Error("a nil factory must leave the gate disabled")
	}
}

// The advisory auth hint is passed through but never enables execution.
func TestBuildDeps_AuthHintIsAdvisoryOnly(t *testing.T) {
	deps := buildPlanningCouncilProviderRunDeps(planningCouncilGateOptions{
		repoRoot:      "/repo",
		getenv:        envMap(map[string]string{}),
		authStateHint: func(string) string { return "authRequired" },
	})
	if deps.AuthStateHint == nil {
		t.Fatal("auth hint should be passed through")
	}
	if deps.Enabled || deps.Runner != nil {
		t.Error("an auth hint must not enable execution")
	}
}

// The production registration path is disabled by default: the registered route
// answers "disabled" and never launches anything. This uses the real
// registerUIRoutes wiring (which passes newRealCouncilProviderRunner as the
// factory) with the process env unset, proving default provider run 0.
func TestRegisterPlanningCouncilRoutes_DefaultDisabledEndToEnd(t *testing.T) {
	mux := http.NewServeMux()
	registerUIRoutes(mux, t.TempDir())

	req := httptest.NewRequest(http.MethodPost, planningCouncilProviderRunPath,
		strings.NewReader(`{"provider":"claude","prompt":"analyze"}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"status":"disabled"`) {
		t.Errorf("default registration must answer disabled, got %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"executed":true`) {
		t.Error("default registration must never report executed true")
	}
}

// Guard: the enabled path never depends on a live process. Even with a fake
// factory injected and the gate on, running through the handler uses the injected
// runner, not a real provider binary.
func TestBuildDeps_EnabledPathUsesInjectedRunnerOnly(t *testing.T) {
	deps := buildPlanningCouncilProviderRunDeps(planningCouncilGateOptions{
		repoRoot:      t.TempDir(),
		getenv:        envMap(map[string]string{planningCouncilProviderRunEnv: "1"}),
		newRealRunner: func(string) councilProviderRunner { return fakeCouncilProviderRunner{status: councilRunCompleted} },
	})
	// The injected fake never starts a process, so a direct Run reports not started.
	res, _ := deps.Runner.Run(context.Background(), readOnlyProviderOptions{Provider: "claude"})
	if res.ProcessStarted {
		t.Error("the injected fake must not report a launched process")
	}
}
