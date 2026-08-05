package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/shin0720/auto-adk/content"
)

// TestDashboardPipelinePreflightCopyIsNeutral guards PR #74: the frontend
// pre-flight log in runAgent() fires BEFORE the /api/workflow/run response, so it
// must not claim the engine already started. Otherwise a default-disabled run
// reads as "분석 엔진을 가동합니다 → then disabled". The real start message is emitted
// by the backend over SSE (handleWorkflowRun) only after the guard passes.
func TestDashboardPipelinePreflightCopyIsNeutral(t *testing.T) {
	raw, err := content.FS.ReadFile("ui/dashboard-pipeline.js")
	if err != nil {
		t.Fatalf("read embedded dashboard-pipeline.js: %v", err)
	}
	js := string(raw)

	// Isolate runAgent() up to the fetch call: the pre-flight log lives here and
	// must be neutral. Anything after the fetch (or in the backend) is out of scope.
	fnIdx := strings.Index(js, "async function runAgent(")
	if fnIdx < 0 {
		t.Fatal("runAgent not found")
	}
	fetchIdx := strings.Index(js[fnIdx:], "await fetch('/api/workflow/run'")
	if fetchIdx < 0 {
		t.Fatal("fetch to /api/workflow/run not found in runAgent")
	}
	preflight := js[fnIdx : fnIdx+fetchIdx]

	if strings.Contains(preflight, "분석 엔진을 가동합니다") {
		t.Error("pre-flight (pre-fetch) log still claims the engine started; must be neutral")
	}
	if !strings.Contains(preflight, "실행 가능 여부를 확인합니다") {
		t.Error("pre-flight log does not use the neutral '실행 가능 여부를 확인합니다' copy")
	}
}

// TestBackendStartedMessageAfterGuard pins that the backend's SSE "started"
// copy is unchanged AND still sits after the default-disabled guard. PR #74 is
// frontend-only: the real "분석 엔진을 가동합니다" must keep coming from the backend,
// and only on the enabled path, so it never precedes a disabled 403.
func TestBackendStartedMessageAfterGuard(t *testing.T) {
	src, err := os.ReadFile("ui_workflow_run.go")
	if err != nil {
		t.Fatalf("read handler source: %v", err)
	}
	s := string(src)

	startedIdx := strings.Index(s, "분석 엔진을 가동합니다. 잠시만 기다려주세요...")
	if startedIdx < 0 {
		t.Fatal("backend SSE start copy was removed (must stay unchanged)")
	}
	guardIdx := strings.Index(s, "legacyWorkflowRunEnabled(os.Getenv)")
	if guardIdx < 0 {
		t.Fatal("default-disabled guard not found")
	}
	if startedIdx < guardIdx {
		t.Error("backend start copy appears before the disabled guard (must stay after)")
	}
}
