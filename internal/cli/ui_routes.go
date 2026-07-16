package cli

import (
	"io/fs"
	"net/http"

	"github.com/shin0720/auto-adk/content"
)

// registerUIRoutes wires every dashboard route onto mux.
//
// It is deliberately separate from newUICmd so the route table can be asserted in
// tests without starting a server. It also takes an explicit mux rather than using
// the global http.DefaultServeMux: registering the same pattern there twice panics,
// which would make the route table untestable across repeated test runs.
//
// root is the workspace directory captured at server startup.
func registerUIRoutes(mux *http.ServeMux, root string) {
	mux.HandleFunc("/api/workspace/change", handleWorkspaceChange)
	mux.HandleFunc("/api/workflow/state", handleWorkflowState)
	mux.HandleFunc("/api/workflow/stream", handleWorkflowStream)
	mux.HandleFunc("/api/workflow/event", handleWorkflowEvent)
	mux.HandleFunc("/api/workflow/run", handleWorkflowRun)
	mux.HandleFunc("/api/workflow/cancel", handleWorkflowCancel)
	mux.HandleFunc("/api/workflow/running", handleWorkflowRunning)
	mux.HandleFunc("/api/workspace/list", handleWorkspaceList)
	mux.HandleFunc("/api/files/list", handleFileList)
	mux.HandleFunc("/api/files/read", handleFileRead)
	mux.HandleFunc("/api/files/write", handleFileWrite)
	mux.HandleFunc("/api/files/upload", handleFileUpload)
	mux.HandleFunc("/api/providers/status", handleProviderStatus)
	mux.HandleFunc("/api/providers/connect", handleProviderConnect)
	mux.HandleFunc("/api/shutdown", handleShutdown)

	registerPlanningCouncilRoutes(mux, root)

	// Serve split static assets (CSS/JS) from the embedded ui directory.
	// Registered before the root handler so /ui/* routes resolve first.
	if staticFS, err := fs.Sub(content.FS, "ui"); err == nil {
		mux.Handle("/ui/", http.StripPrefix("/ui/", http.FileServer(http.FS(staticFS))))
	}
	mux.HandleFunc("/", handleDashboard)
}

// registerPlanningCouncilRoutes registers the Planning Council provider run
// endpoint in a HARD-DISABLED state.
//
// Runner is nil and Enabled is false, so councilRunGate reports "disabled" and the
// handler returns before any workspace, harness or runner is touched. Execution is
// impossible as a STRUCTURAL fact rather than a flag that configuration could flip:
// no runner exists to call, and this package constructs none here. In particular
// newRealCouncilProviderRunner is never called, so no provider process can start.
//
// Injecting a real runner behind a gate is a LATER change that requires separate
// approval. The legacy /api/workflow/run path is untouched and never reused here.
func registerPlanningCouncilRoutes(mux *http.ServeMux, root string) {
	mux.HandleFunc(planningCouncilProviderRunPath, newPlanningCouncilProviderRunHandler(
		planningCouncilProviderRunEndpointDeps{
			RepoRoot:    root,
			MutationDir: root,
			Enabled:     false,
			Runner:      nil,
		}))
}
