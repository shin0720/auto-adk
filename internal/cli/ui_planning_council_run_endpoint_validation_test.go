package cli

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Only POST is accepted; anything else is refused before the body is read.
func TestCouncilRunEndpoint_MethodNotAllowed(t *testing.T) {
	runner := &recordingCouncilRunner{inner: fakeCouncilProviderRunner{status: councilRunCompleted}}
	h := newPlanningCouncilProviderRunHandler(councilEndpointDeps(t, runner))

	req := httptest.NewRequest(http.MethodGet, planningCouncilProviderRunPath, nil)
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("code = %d, want 405", rec.Code)
	}
	if runner.calls != 0 {
		t.Errorf("runner calls = %d, want 0", runner.calls)
	}
}

// A malformed body is rejected without quoting any of it back.
func TestCouncilRunEndpoint_InvalidJSON(t *testing.T) {
	runner := &recordingCouncilRunner{inner: fakeCouncilProviderRunner{status: councilRunCompleted}}
	h := newPlanningCouncilProviderRunHandler(councilEndpointDeps(t, runner))

	rec, resp := councilRunPost(t, h, `{"provider":`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", rec.Code)
	}
	if resp.Status != councilEndpointInvalidRequest {
		t.Errorf("status = %q, want invalidRequest", resp.Status)
	}
	if resp.Error != "invalid JSON body" {
		t.Errorf("error = %q: must be a fixed string, never echo the body", resp.Error)
	}
	if runner.calls != 0 {
		t.Errorf("runner calls = %d, want 0", runner.calls)
	}
}

// Every rejected request must stop before the runner and the harness.
func TestCouncilRunEndpoint_ValidationRejects(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"unknown provider", `{"provider":"gpt","prompt":"x"}`, "provider must be one of claude, codex, gemini"},
		{"empty provider", `{"prompt":"x"}`, "provider must be one of claude, codex, gemini"},
		{"empty prompt", `{"provider":"claude","prompt":""}`, "prompt must not be empty"},
		{"blank prompt", `{"provider":"claude","prompt":"   "}`, "prompt must not be empty"},
		{"timeout too small", `{"provider":"claude","prompt":"x","timeoutMs":999}`, "timeoutMs must be between 1000 and 300000"},
		{"timeout too large", `{"provider":"claude","prompt":"x","timeoutMs":300001}`, "timeoutMs must be between 1000 and 300000"},
		{"timeout negative", `{"provider":"claude","prompt":"x","timeoutMs":-1}`, "timeoutMs must be between 1000 and 300000"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner := &recordingCouncilRunner{inner: fakeCouncilProviderRunner{status: councilRunCompleted}}
			h := newPlanningCouncilProviderRunHandler(councilEndpointDeps(t, runner))

			rec, resp := councilRunPost(t, h, tc.body)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("code = %d, want 400", rec.Code)
			}
			if resp.Status != councilEndpointInvalidRequest {
				t.Errorf("status = %q, want invalidRequest", resp.Status)
			}
			if resp.Error != tc.want {
				t.Errorf("error = %q, want %q", resp.Error, tc.want)
			}
			if resp.ProviderID != "" {
				t.Errorf("providerId = %q: an unvalidated provider must not be echoed", resp.ProviderID)
			}
			if runner.calls != 0 {
				t.Errorf("runner calls = %d, want 0", runner.calls)
			}
		})
	}
}

// An oversized prompt is rejected by length, and the message never contains it.
func TestCouncilRunEndpoint_PromptTooLarge(t *testing.T) {
	runner := &recordingCouncilRunner{inner: fakeCouncilProviderRunner{status: councilRunCompleted}}
	h := newPlanningCouncilProviderRunHandler(councilEndpointDeps(t, runner))

	big := strings.Repeat("p", councilMaxPromptBytes+1)
	rec, resp := councilRunPost(t, h, `{"provider":"claude","prompt":"`+big+`"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", rec.Code)
	}
	if resp.Error != "prompt exceeds 131072 bytes" {
		t.Errorf("error = %q", resp.Error)
	}
	if strings.Contains(resp.Error, "pppp") {
		t.Error("prompt text leaked into the error message")
	}
	if runner.calls != 0 {
		t.Errorf("runner calls = %d, want 0", runner.calls)
	}
}

// A prompt exactly at the cap is accepted: the limit is inclusive.
func TestCouncilRunEndpoint_PromptAtCapAccepted(t *testing.T) {
	runner := &recordingCouncilRunner{inner: fakeCouncilProviderRunner{status: councilRunCompleted, rawText: "ok"}}
	h := newPlanningCouncilProviderRunHandler(councilEndpointDeps(t, runner))

	atCap := strings.Repeat("p", councilMaxPromptBytes)
	_, resp := councilRunPost(t, h, `{"provider":"claude","prompt":"`+atCap+`"}`)

	if resp.Status != councilRunCompleted {
		t.Errorf("status = %q, want completed", resp.Status)
	}
	if runner.calls != 1 {
		t.Errorf("runner calls = %d, want 1", runner.calls)
	}
}

// Each supported provider is accepted and forwarded unchanged.
func TestCouncilRunEndpoint_AcceptsKnownProviders(t *testing.T) {
	for _, p := range []string{"claude", "codex", "gemini"} {
		t.Run(p, func(t *testing.T) {
			runner := &recordingCouncilRunner{inner: fakeCouncilProviderRunner{status: councilRunCompleted, rawText: "ok"}}
			h := newPlanningCouncilProviderRunHandler(councilEndpointDeps(t, runner))

			_, resp := councilRunPost(t, h, `{"provider":"`+p+`","prompt":"analyze"}`)

			if resp.Status != councilRunCompleted {
				t.Errorf("status = %q, want completed", resp.Status)
			}
			if resp.ProviderID != p {
				t.Errorf("providerId = %q, want %q", resp.ProviderID, p)
			}
			if runner.lastOpts.Provider != p {
				t.Errorf("runner provider = %q, want %q", runner.lastOpts.Provider, p)
			}
		})
	}
}

// Timeout policy: an omitted timeoutMs falls back to the default, and deps can
// override the bounds.
func TestCouncilRunTimeoutBounds(t *testing.T) {
	min, max, def := councilRunTimeoutBounds(planningCouncilProviderRunEndpointDeps{})
	if min != councilMinTimeout || max != councilMaxTimeout || def != councilDefaultTimeout {
		t.Errorf("defaults = %v/%v/%v", min, max, def)
	}

	min, max, def = councilRunTimeoutBounds(planningCouncilProviderRunEndpointDeps{
		TimeoutMin: time.Millisecond, TimeoutMax: 2 * time.Second, TimeoutDefault: time.Second,
	})
	if min != time.Millisecond || max != 2*time.Second || def != time.Second {
		t.Errorf("overrides = %v/%v/%v", min, max, def)
	}
}

// An omitted timeoutMs is valid and resolves to the default.
func TestCouncilRunEndpoint_DefaultTimeoutApplied(t *testing.T) {
	d, verr := validateCouncilRunRequest(
		planningCouncilProviderRunRequest{Provider: "claude", Prompt: "x"},
		planningCouncilProviderRunEndpointDeps{},
	)
	if verr != "" {
		t.Fatalf("unexpected validation error: %s", verr)
	}
	if d != councilDefaultTimeout {
		t.Errorf("timeout = %v, want %v", d, councilDefaultTimeout)
	}
}
