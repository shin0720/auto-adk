package cli

import (
	"strings"
	"testing"

	"github.com/shin0720/auto-adk/content"
)

// These are STATIC-INVARIANT guard tests over the embedded PR #53 UI asset. The
// provider run result panel is client-side JS (the repo ships no JS test runner),
// so the safety envelope is enforced here by asserting invariants on the served
// bytes. No provider process is ever started by these tests.
const runResultAssetPath = "ui/dashboard-provider-run-result.js"

func readRunResultAsset(t *testing.T) string {
	t.Helper()
	b, err := content.FS.ReadFile(runResultAssetPath)
	if err != nil {
		t.Fatalf("read embedded %s: %v", runResultAssetPath, err)
	}
	return string(b)
}

// The new script must be registered in the dashboard shell, and the render seam
// must reference the section builder — otherwise the panel never mounts.
func TestProviderRunResultUI_RegisteredInDashboard(t *testing.T) {
	html, err := content.FS.ReadFile("ui/dashboard.html")
	if err != nil {
		t.Fatalf("read dashboard.html: %v", err)
	}
	if !strings.Contains(string(html), "dashboard-provider-run-result.js") {
		t.Fatalf("dashboard.html does not register dashboard-provider-run-result.js")
	}
	council, err := content.FS.ReadFile("ui/dashboard-planning-council.js")
	if err != nil {
		t.Fatalf("read dashboard-planning-council.js: %v", err)
	}
	if !strings.Contains(string(council), "renderProviderRunResult") {
		t.Fatalf("planning-council render seam does not call renderProviderRunResult")
	}
}

// The run endpoint must be POSTed only from an explicit click handler, never from
// a page-load / DOMContentLoaded / window.onload / addEventListener auto path.
func TestProviderRunResultUI_NoAutoRunOnLoad(t *testing.T) {
	js := readRunResultAsset(t)
	if !strings.Contains(js, planningCouncilProviderRunPath) {
		t.Fatalf("asset does not reference the run endpoint path %q", planningCouncilProviderRunPath)
	}
	if !strings.Contains(js, `onclick="runProviderRealResult`) {
		t.Fatalf("run must be wired to an explicit onclick handler")
	}
	for _, banned := range []string{"DOMContentLoaded", "window.onload", "addEventListener"} {
		if strings.Contains(js, banned) {
			t.Fatalf("asset contains auto-execution path token %q (page-load run is forbidden)", banned)
		}
	}
}

// The legacy workflow path must never be called or reused from this panel.
func TestProviderRunResultUI_NoWorkflowRunReuse(t *testing.T) {
	js := readRunResultAsset(t)
	if strings.Contains(js, "/api/workflow/run") {
		t.Fatalf("asset reuses /api/workflow/run, which is forbidden")
	}
}

// rawText and error CONTENT must never be read from the response or persisted.
func TestProviderRunResultUI_NoRawTextOrErrorPersistence(t *testing.T) {
	js := readRunResultAsset(t)
	for _, banned := range []string{".rawText", "rawText:", "resp.rawText", ".error", "resp.error", "error:"} {
		if strings.Contains(js, banned) {
			t.Fatalf("asset touches raw provider output/error content via %q (must be summary-only)", banned)
		}
	}
}

// canAutoRun must stay false: the panel may mention it, but must never set it true.
func TestProviderRunResultUI_CanAutoRunStaysFalse(t *testing.T) {
	js := readRunResultAsset(t)
	for _, banned := range []string{"canAutoRun = true", "canAutoRun: true", "canAutoRun:true"} {
		if strings.Contains(js, banned) {
			t.Fatalf("asset sets canAutoRun true via %q", banned)
		}
	}
}

// A single-run in-flight guard and the mutation/cleanup safety fields must exist.
func TestProviderRunResultUI_InFlightGuardAndSafetyFields(t *testing.T) {
	js := readRunResultAsset(t)
	if !strings.Contains(js, "providerRunInFlight") {
		t.Fatalf("asset lacks the in-flight (one-at-a-time) guard")
	}
	for _, tok := range []string{"mutationCount", "cleanupOK", "mutationViolations"} {
		if !strings.Contains(js, tok) {
			t.Fatalf("asset does not surface safety field %q", tok)
		}
	}
	if !strings.Contains(js, "blocked") {
		t.Fatalf("asset does not distinguish a BLOCKED kind for mutation/cleanup")
	}
}

// Each provider run status must have a distinct display branch, with authRequired
// treated as action-needed rather than a failure.
func TestProviderRunResultUI_StatusBranches(t *testing.T) {
	js := readRunResultAsset(t)
	for _, tok := range []string{"completed", "authRequired", "failed", "timeout", "disabled", "unavailable"} {
		if !strings.Contains(js, tok) {
			t.Fatalf("asset does not handle status %q", tok)
		}
	}
	if !strings.Contains(js, "action-needed") {
		t.Fatalf("authRequired must map to an action-needed kind, not a failure")
	}
}
