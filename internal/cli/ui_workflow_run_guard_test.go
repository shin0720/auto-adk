package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// TestLegacyWorkflowRunEnabled_StrictEquality pins the gate to an exact "1".
// This is a pure helper test — it never touches the handler or a provider path,
// so it cannot spawn a process. Anything other than the literal "1" must stay
// disabled so a stray value can never flip execution on by accident.
func TestLegacyWorkflowRunEnabled_StrictEquality(t *testing.T) {
	cases := []struct {
		val  string
		want bool
	}{
		{"1", true},
		{"", false},
		{"0", false},
		{"true", false},
		{"yes", false},
		{" 1", false},
		{"1 ", false},
		{"01", false},
		{"11", false},
		{"TRUE", false},
	}
	for _, c := range cases {
		got := legacyWorkflowRunEnabled(func(string) string { return c.val })
		if got != c.want {
			t.Errorf("legacyWorkflowRunEnabled(%q) = %v, want %v", c.val, got, c.want)
		}
	}
}

// TestHandleWorkflowRun_DisabledByDefault verifies that with the env unset the
// handler short-circuits to a clear disabled response and never reaches provider
// resolution or subprocess launch. No real POST is made — httptest only — and the
// gate is forced off via t.Setenv so no provider process can start.
func TestHandleWorkflowRun_DisabledByDefault(t *testing.T) {
	t.Setenv(legacyWorkflowRunEnv, "") // explicitly disabled: no provider run possible

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/workflow/run",
		strings.NewReader(`{"agentId":"planner","prompt":"noop"}`))
	handleWorkflowRun(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusForbidden)
	}

	body := rec.Body.String()
	if strings.Contains(body, "undefined") {
		t.Errorf("disabled response contains 'undefined': %s", body)
	}

	var resp struct {
		Success bool   `json:"success"`
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("decode disabled response: %v (body=%s)", err, body)
	}
	if resp.Success {
		t.Errorf("success = true, want false")
	}
	if resp.Status != "disabled" {
		t.Errorf("status = %q, want %q", resp.Status, "disabled")
	}
	if resp.Message == "" {
		t.Error("message is empty; frontend would render nothing")
	}
	if !strings.Contains(resp.Message, legacyWorkflowRunEnv) {
		t.Errorf("message does not name the enabling env var %q: %q", legacyWorkflowRunEnv, resp.Message)
	}
}

// TestHandleWorkflowRun_GuardPrecedesExecution is a static-order guard: the env
// gate MUST appear before any binary resolution or orchestra launch inside the
// handler source, so the disabled path is structurally unreachable to execution.
func TestHandleWorkflowRun_GuardPrecedesExecution(t *testing.T) {
	src, err := os.ReadFile("ui_workflow_run.go")
	if err != nil {
		t.Fatalf("read handler source: %v", err)
	}
	s := string(src)

	fnIdx := strings.Index(s, "func handleWorkflowRun(")
	if fnIdx < 0 {
		t.Fatal("handleWorkflowRun not found in source")
	}
	body := s[fnIdx:]

	guardIdx := strings.Index(body, "legacyWorkflowRunEnabled(os.Getenv)")
	if guardIdx < 0 {
		t.Fatal("env guard call not found inside handleWorkflowRun")
	}
	for _, marker := range []string{"resolveRunnableBinary", "orchestra.RunOrchestra", "RunOrchestra"} {
		if idx := strings.Index(body, marker); idx >= 0 && idx < guardIdx {
			t.Errorf("execution marker %q appears before the env guard (guard must be first)", marker)
		}
	}
}
