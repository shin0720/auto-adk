package cli

import "os"

// planningCouncilProviderRunEnv gates whether the Planning Council endpoint wires a
// REAL provider runner. It is opt-in and off by default: only the exact value "1"
// enables it, so a stray "true"/"yes"/"0" or a whitespace-padded value cannot flip
// execution on by accident.
const planningCouncilProviderRunEnv = "AUTOPUS_PLANNING_COUNCIL_PROVIDER_RUN"

// planningCouncilProviderRunEnabled reports whether the real runner is opted in.
// The match is intentionally strict — exactly "1", no trimming, no case folding —
// so the set of values that enable a real provider process is as small as possible.
func planningCouncilProviderRunEnabled(getenv func(string) string) bool {
	if getenv == nil {
		getenv = os.Getenv
	}
	return getenv(planningCouncilProviderRunEnv) == "1"
}

// planningCouncilGateOptions configures how the endpoint deps are built. Every
// external dependency is injected so the wiring can be tested without reading the
// real environment or constructing a real runner.
type planningCouncilGateOptions struct {
	repoRoot string
	// getenv reads the gate variable; nil falls back to os.Getenv.
	getenv func(string) string
	// newRealRunner constructs the real runner. It is called ONLY when the gate is
	// enabled, so with the gate off no runner is ever built and execution stays
	// structurally impossible. Constructing a runner does not start a process — only
	// its Run method does, and nothing calls Run on the disabled path.
	newRealRunner func(root string) councilProviderRunner
	// authStateHint is advisory only; it never enables execution.
	authStateHint func(provider string) string
}

// buildPlanningCouncilProviderRunDeps builds the endpoint deps, wiring a real
// runner only when the gate is enabled.
//
// Disabled (the default, and always in CI): Runner stays nil and Enabled stays
// false, so councilRunGate reports "disabled" and the handler returns before any
// runner, workspace or provider process is touched. newRealRunner is NOT called.
//
// Enabled (env exactly "1"): Runner is the constructed real runner, Enabled is
// true, and GateLabel is "real" so the response reports the runner kind honestly.
func buildPlanningCouncilProviderRunDeps(o planningCouncilGateOptions) planningCouncilProviderRunEndpointDeps {
	deps := planningCouncilProviderRunEndpointDeps{
		RepoRoot:      o.repoRoot,
		MutationDir:   o.repoRoot,
		Enabled:       false,
		Runner:        nil,
		AuthStateHint: o.authStateHint,
	}
	if planningCouncilProviderRunEnabled(o.getenv) && o.newRealRunner != nil {
		deps.Runner = o.newRealRunner(o.repoRoot)
		deps.Enabled = true
		deps.GateLabel = councilGateReal
	}
	return deps
}
