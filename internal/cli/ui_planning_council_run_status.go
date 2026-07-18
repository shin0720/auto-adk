package cli

// councilRunEndpointStatus resolves the reported status. A harness verdict about
// the real tree (scopeViolation) or teardown (cleanupFailed) outranks the runner:
// a mutated repo must never be reported as a clean success.
func councilRunEndpointStatus(
	hres councilReadOnlyHarnessResult,
	runRes councilProviderRunResult,
	runErr error,
) string {
	switch hres.Status {
	case councilHarnessScopeViolation:
		return councilHarnessScopeViolation
	case councilHarnessCleanupFailed:
		return councilHarnessCleanupFailed
	case councilHarnessInvalidOptions, councilHarnessFakeFailed:
		// Option-build, snapshot or transport failures: nothing usable ran.
		return councilRunFailed
	}
	if runErr != nil {
		return councilRunFailed
	}
	switch runRes.Status {
	case councilRunCompleted, councilRunFailed, councilRunTimeout, councilRunCanceled,
		councilRunUnavailable, councilRunAuthRequired:
		// authRequired is forwarded, not inferred: the runner derives it from the
		// provider's own output (see classifyCouncilProviderAuthFailure), so the
		// endpoint is reporting evidence rather than guessing.
		return runRes.Status
	default:
		// Any unset/unknown status degrades to failed rather than leaking through.
		return councilRunFailed
	}
}

// councilRunGate reports the active gate. A nil Runner reads as disabled so the
// handler never has a reason to build one itself. An enabled runner reports its
// GateLabel, defaulting to "fake" so a real run is never implied by accident.
func councilRunGate(deps planningCouncilProviderRunEndpointDeps) string {
	if !deps.Enabled || deps.Runner == nil {
		return councilGateDisabled
	}
	if deps.GateLabel != "" {
		return deps.GateLabel
	}
	return councilGateFake
}
