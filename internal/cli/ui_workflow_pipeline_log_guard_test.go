package cli

import (
	"strings"
	"testing"

	"github.com/shin0720/auto-adk/content"
)

// Static-invariant guard for the PR #62 workflow error-log fix. The dashboard is
// client-side JS with no JS test runner, so the invariant is asserted on the
// embedded asset: a missing error message must never render the literal
// "undefined" in the workflow terminal log.
func TestDashboardPipeline_NoUndefinedErrorLog(t *testing.T) {
	b, err := content.FS.ReadFile("ui/dashboard-pipeline.js")
	if err != nil {
		t.Fatalf("read embedded ui/dashboard-pipeline.js: %v", err)
	}
	js := string(b)

	// The old bare concatenation ('에러: ' + data.message) rendered "undefined"
	// when the response had no message field; it must be gone.
	if strings.Contains(js, "'❌ 에러: ' + data.message") {
		t.Fatalf("pipeline.js still concatenates data.message directly (renders 'undefined')")
	}
	// A fallback string must be present for the missing-message case.
	if !strings.Contains(js, "알 수 없는 오류") {
		t.Fatalf("pipeline.js lacks a fallback message for a missing error message")
	}
}
