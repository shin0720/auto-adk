package cli

import (
	"io/fs"
	"net/http"
	"os"

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
// endpoint. It is DISABLED by default: buildPlanningCouncilProviderRunDeps only
// wires a real runner when AUTOPUS_PLANNING_COUNCIL_PROVIDER_RUN is exactly "1".
//
// With the gate off (the default, and always in CI) Runner stays nil and Enabled
// stays false, so councilRunGate reports "disabled" and no provider process can
// start. newRealCouncilProviderRunner is passed as a factory but is CALLED only on
// the enabled path, so constructing this registration launches nothing.
//
// The legacy /api/workflow/run path is untouched and never reused here.
func registerPlanningCouncilRoutes(mux *http.ServeMux, root string) {
	deps := buildPlanningCouncilProviderRunDeps(planningCouncilGateOptions{
		repoRoot:      root,
		getenv:        os.Getenv,
		newRealRunner: func(r string) councilProviderRunner { return newRealCouncilProviderRunner(r) },
	})
	mux.HandleFunc(planningCouncilProviderRunPath, newPlanningCouncilProviderRunHandler(deps))
}
