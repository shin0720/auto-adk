package cli

import (
	"strings"
	"testing"

	"github.com/shin0720/auto-adk/content"
)

// Static-invariant guard for the PR #67 terminal log clear action. The dashboard
// is client-side JS with no JS test runner, so the invariants are asserted on the
// embedded assets: an explicit "로그 지우기" button must exist and its handler must
// clear ONLY workflowState.logs (never provider results / finalDecision /
// planningCouncil), must persist via saveState, and must not auto-run on load.
func embeddedUIAsset(t *testing.T, name string) string {
	t.Helper()
	b, err := content.FS.ReadFile(name)
	if err != nil {
		t.Fatalf("read embedded %s: %v", name, err)
	}
	return string(b)
}

func TestTerminalLogClear_ButtonAndHandlerExist(t *testing.T) {
	html := embeddedUIAsset(t, "ui/dashboard-body-2.html")
	if !strings.Contains(html, "로그 지우기") || !strings.Contains(html, `onclick="clearTerminalLogs()"`) {
		t.Fatalf("dashboard-body-2.html lacks the 로그 지우기 clear button wired to clearTerminalLogs()")
	}

	js := embeddedUIAsset(t, "ui/dashboard-stream.js")
	if !strings.Contains(js, "function clearTerminalLogs()") {
		t.Fatalf("dashboard-stream.js lacks clearTerminalLogs()")
	}
	// The handler must reset the log history and persist the cleared state.
	if !strings.Contains(js, "workflowState.logs = []") {
		t.Fatalf("clearTerminalLogs must reset workflowState.logs")
	}
	if !strings.Contains(js, "saveState") {
		t.Fatalf("clearTerminalLogs must persist via saveState")
	}
}

// The clear handler must not delete workflow results, approval, finalDecision, or
// planningCouncil data — only the log history.
func TestTerminalLogClear_DoesNotTouchResultsOrDecisions(t *testing.T) {
	js := embeddedUIAsset(t, "ui/dashboard-stream.js")
	start := strings.Index(js, "function clearTerminalLogs()")
	if start < 0 {
		t.Fatalf("clearTerminalLogs not found")
	}
	body := js[start:]
	if end := strings.Index(body[1:], "\n        function "); end >= 0 {
		body = body[:end+1]
	}
	for _, banned := range []string{"planningCouncil", "finalDecision", ".output", "providerRunResults", "workflowState.nodes", "workflowState.connections", "workflowState.approval"} {
		if strings.Contains(body, banned) {
			t.Fatalf("clearTerminalLogs must not touch %q (logs only)", banned)
		}
	}
}

// The clear must be user-triggered only: no auto-invocation on load or events.
func TestTerminalLogClear_NoAutoInvoke(t *testing.T) {
	for _, name := range []string{"ui/dashboard-stream.js", "ui/dashboard-main.js", "ui/dashboard-pipeline.js"} {
		js := embeddedUIAsset(t, name)
		// clearTerminalLogs may be DEFINED and wired via onclick, but never called
		// from JS (which would indicate an automatic purge path).
		if strings.Contains(js, "clearTerminalLogs();") {
			t.Fatalf("%s auto-invokes clearTerminalLogs() (must be user-click only)", name)
		}
	}
	// The PR62 undefined guard fallback must still be present.
	pipe := embeddedUIAsset(t, "ui/dashboard-pipeline.js")
	if !strings.Contains(pipe, "알 수 없는 오류") {
		t.Fatalf("PR62 fallback 알 수 없는 오류 missing from dashboard-pipeline.js")
	}
}
