package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// planningCouncilProviderRunPath is the path this endpoint WILL be served at once
// a gated real runner exists. It is reserved as a constant only: ui.go does not
// register this handler, so there is no HTTP surface and no production caller
// today. The legacy /api/workflow/run path is untouched and is never reused here.
const planningCouncilProviderRunPath = "/api/planning-council/providers/run"

// Endpoint-level statuses that no runner can produce. The remaining statuses in
// the response enum are forwarded from the runner (completed/failed/timeout/
// canceled/unavailable/authRequired) or the harness (scopeViolation/cleanupFailed).
const (
	councilEndpointDisabled       = "disabled"
	councilEndpointInvalidRequest = "invalidRequest"
)

// Gate values. Production registration is still Runner==nil, so only "disabled" is
// reachable in production; "fake" and "real" are selected by deps.GateLabel when a
// runner is injected, so the response names the runner kind honestly.
const (
	councilGateDisabled = "disabled"
	councilGateFake     = "fake"
	councilGateReal     = "real"
)

// Request limits. The prompt cap bounds request memory; the timeout bounds keep a
// caller from pinning a run open forever or from a no-op zero deadline.
const (
	councilMaxPromptBytes = 128 * 1024
	councilDefaultTimeout = 60 * time.Second
	councilMinTimeout     = 1 * time.Second
	councilMaxTimeout     = 300 * time.Second
)

// planningCouncilProviderRunRequest is one ephemeral read-only run request. There
// is deliberately no approval/finalDecision field: this endpoint never reads or
// writes approval state.
type planningCouncilProviderRunRequest struct {
	Provider  string `json:"provider"`
	Prompt    string `json:"prompt"`
	TimeoutMs int    `json:"timeoutMs,omitempty"`
}

// planningCouncilProviderRunResponse is the diagnostic outcome of one request. It
// is ephemeral: nothing is persisted, and state.json is never written. It carries
// no secret/token values and no finalDecision field.
type planningCouncilProviderRunResponse struct {
	Status     string `json:"status"`
	ProviderID string `json:"providerId,omitempty"`
	// Executed reports whether an actual provider process was launched. It tracks
	// the runner result, so a fake run (and the disabled production route) reports
	// false while a real run would report true.
	Executed bool   `json:"executed"`
	Gate     string `json:"gate"`

	RawText    string `json:"rawText,omitempty"`
	Truncated  bool   `json:"truncated,omitempty"`
	ExitCode   int    `json:"exitCode,omitempty"`
	DurationMs int64  `json:"durationMs,omitempty"`
	StartedAt  string `json:"startedAt,omitempty"`
	FinishedAt string `json:"finishedAt,omitempty"`
	Error      string `json:"error,omitempty"`

	// AuthStateHint is advisory only, sourced from the provider-status view. The
	// endpoint never infers an auth verdict from a run outcome.
	AuthStateHint string `json:"authStateHint,omitempty"`

	MutationViolations []string `json:"mutationViolations,omitempty"`
	CleanupOK          bool     `json:"cleanupOK"`
}

// planningCouncilProviderRunEndpointDeps injects everything the handler touches so
// it can be exercised without a server, a real provider, or the user's repo.
type planningCouncilProviderRunEndpointDeps struct {
	RepoRoot    string
	MutationDir string
	// Runner performs the invocation. A nil Runner is treated as disabled so the
	// handler can never fall back to constructing a real one.
	Runner  councilProviderRunner
	Enabled bool
	// GateLabel names the injected runner kind ("fake" or "real") for the enabled
	// path. Empty defaults to "fake", so a real runner must opt in explicitly and a
	// fake run can never masquerade as real.
	GateLabel string
	// AuthStateHint supplies the advisory hint; nil omits the field.
	AuthStateHint  func(provider string) string
	TimeoutDefault time.Duration
	TimeoutMin     time.Duration
	TimeoutMax     time.Duration

	// cleanupOverride is a same-package test seam forwarded to the harness so a
	// cleanup failure can be exercised. Production callers leave it nil.
	cleanupOverride func() error
}

// newPlanningCouncilProviderRunHandler builds the handler. It is intentionally NOT
// registered on any mux; callers in this PR are tests only.
func newPlanningCouncilProviderRunHandler(deps planningCouncilProviderRunEndpointDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		var req planningCouncilProviderRunRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			// A decode error can quote the offending body; report a fixed string so
			// no prompt text can travel back out through the error field.
			writeCouncilRunJSON(w, http.StatusBadRequest, planningCouncilProviderRunResponse{
				Status: councilEndpointInvalidRequest, Gate: councilRunGate(deps), Error: "invalid JSON body",
			})
			return
		}

		// Validation precedes every side effect: no workspace, no snapshot, no
		// runner call happens for a rejected request.
		timeout, verr := validateCouncilRunRequest(req, deps)
		if verr != "" {
			// ProviderID is left empty: an unvalidated provider string is never
			// echoed back.
			writeCouncilRunJSON(w, http.StatusBadRequest, planningCouncilProviderRunResponse{
				Status: councilEndpointInvalidRequest, Gate: councilRunGate(deps), Error: verr,
			})
			return
		}

		if councilRunGate(deps) == councilGateDisabled {
			writeCouncilRunJSON(w, http.StatusOK, planningCouncilProviderRunResponse{
				Status: councilEndpointDisabled, ProviderID: req.Provider,
				Executed: false, Gate: councilGateDisabled,
			})
			return
		}

		writeCouncilRunJSON(w, http.StatusOK, runCouncilProviderEndpoint(r.Context(), req, deps, timeout))
	}
}

// runCouncilProviderEndpoint drives the harness (temp cwd, mutation snapshot/diff,
// cleanup) and lets the injected runner do only the invocation. The harness is
// never bypassed, so the mutation guard and cleanup always apply.
func runCouncilProviderEndpoint(
	parent context.Context,
	req planningCouncilProviderRunRequest,
	deps planningCouncilProviderRunEndpointDeps,
	timeout time.Duration,
) planningCouncilProviderRunResponse {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	var runRes councilProviderRunResult
	var runErr error
	// The harness runner seam is func(opts) error while councilProviderRunner is
	// Run(ctx, opts) (result, error). A closure absorbs the mismatch: it captures
	// ctx and the result, and returns only a transport error. A non-zero provider
	// exit is NOT an error here — that is a runner status, not a harness failure.
	hres := runCouncilReadOnlyHarness(councilHarnessParams{
		Provider:        req.Provider,
		Prompt:          req.Prompt,
		RepoRoot:        deps.RepoRoot,
		MutationDir:     deps.MutationDir,
		cleanupOverride: deps.cleanupOverride,
		Runner: func(opts readOnlyProviderOptions) error {
			runRes, runErr = deps.Runner.Run(ctx, opts)
			return runErr
		},
	})

	resp := planningCouncilProviderRunResponse{
		ProviderID: req.Provider,
		// Report the truth, not a constant: Executed reflects whether a process was
		// actually launched, and Gate names the injected runner kind.
		Executed:           runRes.ProcessStarted,
		Gate:               councilRunGate(deps),
		RawText:            runRes.RawText,
		Truncated:          runRes.Truncated,
		ExitCode:           runRes.ExitCode,
		DurationMs:         runRes.DurationMs,
		StartedAt:          runRes.StartedAt,
		FinishedAt:         runRes.FinishedAt,
		Error:              runRes.Error,
		MutationViolations: hres.MutationViolations,
		CleanupOK:          hres.CleanupOK,
	}
	if deps.AuthStateHint != nil {
		resp.AuthStateHint = deps.AuthStateHint(req.Provider)
	}
	resp.Status = councilRunEndpointStatus(hres, runRes, runErr)
	if resp.Error == "" && hres.Error != "" {
		resp.Error = hres.Error
	}
	return resp
}

// validateCouncilRunRequest returns the effective timeout, or a non-empty message
// describing the first problem. Messages never quote the prompt.
func validateCouncilRunRequest(
	req planningCouncilProviderRunRequest,
	deps planningCouncilProviderRunEndpointDeps,
) (time.Duration, string) {
	switch req.Provider {
	case "claude", "codex", "gemini":
	default:
		return 0, "provider must be one of claude, codex, gemini"
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return 0, "prompt must not be empty"
	}
	if len(req.Prompt) > councilMaxPromptBytes {
		return 0, fmt.Sprintf("prompt exceeds %d bytes", councilMaxPromptBytes)
	}

	min, max, def := councilRunTimeoutBounds(deps)
	if req.TimeoutMs == 0 {
		return def, ""
	}
	d := time.Duration(req.TimeoutMs) * time.Millisecond
	if d < min || d > max {
		return 0, fmt.Sprintf("timeoutMs must be between %d and %d",
			min.Milliseconds(), max.Milliseconds())
	}
	return d, ""
}

// councilRunTimeoutBounds resolves the timeout policy, letting deps override the
// defaults so tests can drive a deadline without a real wait.
func councilRunTimeoutBounds(deps planningCouncilProviderRunEndpointDeps) (min, max, def time.Duration) {
	min, max, def = councilMinTimeout, councilMaxTimeout, councilDefaultTimeout
	if deps.TimeoutMin > 0 {
		min = deps.TimeoutMin
	}
	if deps.TimeoutMax > 0 {
		max = deps.TimeoutMax
	}
	if deps.TimeoutDefault > 0 {
		def = deps.TimeoutDefault
	}
	return min, max, def
}

// writeCouncilRunJSON emits the response. Nothing is logged here: the prompt and
// raw provider output must never reach a log sink.
func writeCouncilRunJSON(w http.ResponseWriter, code int, resp planningCouncilProviderRunResponse) {
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(resp)
}
