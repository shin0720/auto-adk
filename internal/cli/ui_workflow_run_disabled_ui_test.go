package cli

import (
	"strings"
	"testing"

	"github.com/shin0720/auto-adk/content"
)

// TestDashboardPipelineHandlesDisabledStatus guards PR #73: the frontend must
// treat a legacy /api/workflow/run "disabled" response as an intentional
// configuration state, NOT a generic runtime error. Enforced as a source-scan
// invariant over the embedded JS so a future edit that drops the branch (and
// falls back to the ❌ 에러 + error-recovery cascade) fails loudly.
func TestDashboardPipelineHandlesDisabledStatus(t *testing.T) {
	raw, err := content.FS.ReadFile("ui/dashboard-pipeline.js")
	if err != nil {
		t.Fatalf("read embedded dashboard-pipeline.js: %v", err)
	}
	js := string(raw)

	disabledIdx := strings.Index(js, "data.status === 'disabled'")
	if disabledIdx < 0 {
		t.Fatal("dashboard-pipeline.js does not handle data.status === 'disabled'")
	}

	// The disabled branch MUST come before the generic error branch, otherwise
	// the disabled response is swallowed as an error first.
	errBranchIdx := strings.Index(js, "data.status !== 'success'")
	if errBranchIdx < 0 {
		t.Fatal("generic error branch not found")
	}
	if disabledIdx > errBranchIdx {
		t.Error("disabled branch appears AFTER the generic error branch (must be first)")
	}

	// The disabled branch body: isolate it up to the generic error branch and
	// assert it (a) shows the Korean notice, (b) does not mark the node 'error',
	// and (c) does not enter the error-recovery loop.
	branch := js[disabledIdx:errBranchIdx]
	if !strings.Contains(branch, "실전 분석 실행은 기본 비활성화 상태입니다") {
		t.Error("disabled branch does not show the Korean disabled notice")
	}
	if strings.Contains(branch, "❌ 에러") {
		t.Error("disabled branch still uses the ❌ 에러 error tone")
	}
	if strings.Contains(branch, "setNodeStatus(agentId, 'error')") {
		t.Error("disabled branch marks the node as 'error'")
	}
	if strings.Contains(branch, "에러 복구 루프") || strings.Contains(branch, "errorRecoveryMap") {
		t.Error("disabled branch triggers the error-recovery loop")
	}
	// Undefined-log safety for the whole file is covered by
	// TestDashboardPipeline_NoUndefinedErrorLog; the disabled branch logs a fixed
	// Korean string so it cannot render a dynamic value in the first place.
}
