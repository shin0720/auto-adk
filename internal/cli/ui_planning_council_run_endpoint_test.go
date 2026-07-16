package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// recordingCouncilRunner implements councilProviderRunner for the endpoint tests.
// It NEVER starts a process: there is no exec of claude/codex/gemini anywhere in
// these tests, and realCouncilProviderRunner is never constructed.
type recordingCouncilRunner struct {
	inner    fakeCouncilProviderRunner
	calls    int
	lastOpts readOnlyProviderOptions
	// writeTo, when set, creates a file at that path to simulate a provider that
	// mutated the guarded tree.
	writeTo string
	// transportErr simulates a runner that could not report a result at all.
	transportErr error
}

func (r *recordingCouncilRunner) Run(ctx context.Context, opts readOnlyProviderOptions) (councilProviderRunResult, error) {
	r.calls++
	r.lastOpts = opts
	if r.writeTo != "" {
		if err := os.WriteFile(r.writeTo, []byte("mutated\n"), 0o644); err != nil {
			return councilProviderRunResult{}, err
		}
	}
	if r.transportErr != nil {
		return councilProviderRunResult{ProviderID: opts.Provider}, r.transportErr
	}
	return r.inner.Run(ctx, opts)
}

// councilEndpointDeps builds enabled deps pointing at a throwaway git repo, so the
// user's real tree is never the mutation target.
func councilEndpointDeps(t *testing.T, runner councilProviderRunner) planningCouncilProviderRunEndpointDeps {
	t.Helper()
	repo := newCouncilHarnessRepo(t)
	return planningCouncilProviderRunEndpointDeps{
		RepoRoot:    repo,
		MutationDir: repo,
		Runner:      runner,
		Enabled:     true,
	}
}

// councilRunPost issues a POST and decodes the response.
func councilRunPost(t *testing.T, h http.HandlerFunc, body string) (*httptest.ResponseRecorder, planningCouncilProviderRunResponse) {
	t.Helper()
	return councilRunPostCtx(t, h, body, context.Background())
}

func councilRunPostCtx(t *testing.T, h http.HandlerFunc, body string, ctx context.Context) (*httptest.ResponseRecorder, planningCouncilProviderRunResponse) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, planningCouncilProviderRunPath, strings.NewReader(body))
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h(rec, req)

	var resp planningCouncilProviderRunResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v\nbody=%s", err, rec.Body.String())
	}
	return rec, resp
}

const councilRunBody = `{"provider":"claude","prompt":"analyze"}`

// A completed fake run reports the runner's output while still declaring that no
// actual provider process executed.
func TestCouncilRunEndpoint_FakeSuccess(t *testing.T) {
	runner := &recordingCouncilRunner{inner: fakeCouncilProviderRunner{status: councilRunCompleted, rawText: "verdict: ok"}}
	h := newPlanningCouncilProviderRunHandler(councilEndpointDeps(t, runner))

	rec, resp := councilRunPost(t, h, councilRunBody)

	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q", got)
	}
	if resp.Status != councilRunCompleted {
		t.Errorf("status = %q, want completed (error=%q)", resp.Status, resp.Error)
	}
	if resp.RawText != "verdict: ok" {
		t.Errorf("rawText = %q", resp.RawText)
	}
	if resp.Executed {
		t.Error("executed must stay false: no actual provider process ran")
	}
	if resp.Gate != councilGateFake {
		t.Errorf("gate = %q, want fake", resp.Gate)
	}
	if !resp.CleanupOK {
		t.Error("cleanupOK = false, want true")
	}
	if len(resp.MutationViolations) != 0 {
		t.Errorf("mutationViolations = %v, want none", resp.MutationViolations)
	}
	if resp.ProviderID != "claude" {
		t.Errorf("providerId = %q", resp.ProviderID)
	}
	if runner.calls != 1 {
		t.Errorf("runner calls = %d, want 1", runner.calls)
	}
	// The prompt must reach the provider via stdin, never via argv.
	if runner.lastOpts.Stdin != "analyze" {
		t.Errorf("stdin = %q, want the prompt", runner.lastOpts.Stdin)
	}
	for _, a := range runner.lastOpts.Args {
		if strings.Contains(a, "analyze") {
			t.Fatalf("prompt leaked into args: %v", runner.lastOpts.Args)
		}
	}
	// The harness must pin the run to an isolated temp cwd outside the repo.
	if runner.lastOpts.Cwd == "" {
		t.Error("cwd must be set to the isolated temp workspace")
	}
	if inside, err := sameOrInside(runner.lastOpts.Cwd, t.TempDir()); err == nil && inside {
		t.Error("cwd unexpectedly resolved inside the test repo tree")
	}
}

// A non-zero provider exit is a runner status, not a harness failure: the exit
// code and error propagate while cleanup still succeeds.
func TestCouncilRunEndpoint_FakeFailure(t *testing.T) {
	runner := &recordingCouncilRunner{inner: fakeCouncilProviderRunner{
		status: councilRunFailed, rawText: "partial", exitCode: 3, errMsg: "boom",
	}}
	h := newPlanningCouncilProviderRunHandler(councilEndpointDeps(t, runner))

	_, resp := councilRunPost(t, h, councilRunBody)

	if resp.Status != councilRunFailed {
		t.Errorf("status = %q, want failed", resp.Status)
	}
	if resp.ExitCode != 3 {
		t.Errorf("exitCode = %d, want 3", resp.ExitCode)
	}
	if resp.Error != "boom" {
		t.Errorf("error = %q, want boom", resp.Error)
	}
	if resp.RawText != "partial" {
		t.Errorf("rawText = %q: partial output must be preserved", resp.RawText)
	}
	if !resp.CleanupOK {
		t.Error("a failed provider must still clean up")
	}
}

// A missing binary never runs anything and reports unavailable.
func TestCouncilRunEndpoint_Unavailable(t *testing.T) {
	runner := &recordingCouncilRunner{inner: fakeCouncilProviderRunner{status: councilRunUnavailable}}
	h := newPlanningCouncilProviderRunHandler(councilEndpointDeps(t, runner))

	_, resp := councilRunPost(t, h, councilRunBody)

	if resp.Status != councilRunUnavailable {
		t.Errorf("status = %q, want unavailable", resp.Status)
	}
	if resp.Executed {
		t.Error("executed must stay false")
	}
}

// An exceeded deadline reports timeout. TimeoutMin is lowered so the test drives a
// real deadline without a real wait.
func TestCouncilRunEndpoint_Timeout(t *testing.T) {
	runner := &recordingCouncilRunner{inner: fakeCouncilProviderRunner{
		status: councilRunCompleted, work: 2 * time.Second,
	}}
	deps := councilEndpointDeps(t, runner)
	deps.TimeoutMin = time.Millisecond
	h := newPlanningCouncilProviderRunHandler(deps)

	_, resp := councilRunPost(t, h, `{"provider":"codex","prompt":"analyze","timeoutMs":1}`)

	if resp.Status != councilRunTimeout {
		t.Errorf("status = %q, want timeout", resp.Status)
	}
	if !resp.CleanupOK {
		t.Error("a timed-out run must still clean up its temp workspace")
	}
}

// A canceled request context reports canceled, not timeout.
func TestCouncilRunEndpoint_Canceled(t *testing.T) {
	runner := &recordingCouncilRunner{inner: fakeCouncilProviderRunner{
		status: councilRunCompleted, work: 2 * time.Second,
	}}
	h := newPlanningCouncilProviderRunHandler(councilEndpointDeps(t, runner))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, resp := councilRunPostCtx(t, h, councilRunBody, ctx)

	if resp.Status != councilRunCanceled {
		t.Errorf("status = %q, want canceled", resp.Status)
	}
}
