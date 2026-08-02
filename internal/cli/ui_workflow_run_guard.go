package cli

import (
	"encoding/json"
	"net/http"
	"os"
)

// legacyWorkflowRunEnv gates whether the legacy /api/workflow/run handler may
// spawn provider subprocesses. It is opt-in and off by default: only the exact
// value "1" enables it, mirroring the Planning Council gate. A stray "true"/
// "yes"/"0" or a whitespace-padded value cannot flip execution on by accident.
const legacyWorkflowRunEnv = "AUTOPUS_LEGACY_WORKFLOW_RUN_ENABLED"

// legacyWorkflowRunEnabled reports whether the legacy run path is opted in. The
// match is intentionally strict — exactly "1", no trimming, no case folding — so
// the set of values that enable a real provider process is as small as possible.
func legacyWorkflowRunEnabled(getenv func(string) string) bool {
	if getenv == nil {
		getenv = os.Getenv
	}
	return getenv(legacyWorkflowRunEnv) == "1"
}

// writeLegacyWorkflowRunDisabled emits the default-disabled response. The shape
// carries explicit success/status/message fields so the frontend never renders
// "undefined": status "disabled" routes to the error branch, and message states
// how to opt in for an explicit maintenance run.
func writeLegacyWorkflowRunDisabled(w http.ResponseWriter) {
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(struct {
		Success bool   `json:"success"`
		Status  string `json:"status"`
		Message string `json:"message"`
	}{
		Success: false,
		Status:  "disabled",
		Message: "Legacy workflow run is disabled by default. Set AUTOPUS_LEGACY_WORKFLOW_RUN_ENABLED=1 only for an explicit maintenance run.",
	})
}
