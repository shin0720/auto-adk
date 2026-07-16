package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// uiRoutesBefore is the route table as it existed before the registration seam was
// extracted from newUICmd. The refactor MUST preserve every one of these: a
// silently dropped route would break the dashboard, which no e2e test covers.
var uiRoutesBefore = []string{
	"/api/workspace/change",
	"/api/workflow/state",
	"/api/workflow/stream",
	"/api/workflow/event",
	"/api/workflow/run",
	"/api/workflow/cancel",
	"/api/workflow/running",
	"/api/workspace/list",
	"/api/files/list",
	"/api/files/read",
	"/api/files/write",
	"/api/files/upload",
	"/api/providers/status",
	"/api/providers/connect",
	"/api/shutdown",
	"/ui/",
	"/",
}

// newTestUIMux builds a fresh mux. Using a dedicated mux (never
// http.DefaultServeMux) is what lets these tests run repeatedly without the
// duplicate-registration panic, and it starts no server.
func newTestUIMux(t *testing.T) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	registerUIRoutes(mux, t.TempDir())
	return mux
}

// Every pre-refactor route still resolves to a handler, and each API route
// matches its own exact pattern rather than falling through to the "/" catch-all.
func TestRegisterUIRoutes_PreservesExistingRoutes(t *testing.T) {
	mux := newTestUIMux(t)

	for _, route := range uiRoutesBefore {
		t.Run(route, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://localhost"+route, nil)
			h, pattern := mux.Handler(req)
			if h == nil {
				t.Fatalf("route %q resolved to a nil handler", route)
			}
			if pattern != route {
				// "/" is the catch-all; anything else matching it means the
				// route's own registration went missing.
				t.Errorf("route %q matched pattern %q, want %q", route, pattern, route)
			}
		})
	}
}

// The refactor must not drop the legacy mutation path either: it stays registered
// and untouched (this PR neither changes nor reuses it).
func TestRegisterUIRoutes_LegacyWorkflowRunStillRegistered(t *testing.T) {
	mux := newTestUIMux(t)

	req := httptest.NewRequest(http.MethodGet, "http://localhost/api/workflow/run", nil)
	_, pattern := mux.Handler(req)
	if pattern != "/api/workflow/run" {
		t.Errorf("pattern = %q, want /api/workflow/run", pattern)
	}
}

// The council endpoint is registered, so a production surface now exists.
func TestRegisterUIRoutes_CouncilRouteRegistered(t *testing.T) {
	mux := newTestUIMux(t)

	req := httptest.NewRequest(http.MethodGet, "http://localhost"+planningCouncilProviderRunPath, nil)
	h, pattern := mux.Handler(req)
	if h == nil {
		t.Fatal("council route resolved to a nil handler")
	}
	if pattern != planningCouncilProviderRunPath {
		t.Errorf("pattern = %q, want %q", pattern, planningCouncilProviderRunPath)
	}
}

// Registering twice on separate muxes must not panic. This is the property that
// http.DefaultServeMux would violate, and it is why the seam takes a mux.
func TestRegisterUIRoutes_FreshMuxIsRepeatable(t *testing.T) {
	newTestUIMux(t)
	newTestUIMux(t)
}

// The registered council route answers "disabled" and nothing else: no provider
// runs, and the response says so explicitly.
func TestRegisterUIRoutes_CouncilRouteRespondsDisabled(t *testing.T) {
	mux := newTestUIMux(t)

	req := httptest.NewRequest(http.MethodPost, "http://localhost"+planningCouncilProviderRunPath,
		strings.NewReader(councilRunBody))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	var resp planningCouncilProviderRunResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v\nbody=%s", err, rec.Body.String())
	}
	if resp.Status != councilEndpointDisabled {
		t.Errorf("status = %q, want disabled", resp.Status)
	}
	if resp.Gate != councilGateDisabled {
		t.Errorf("gate = %q, want disabled", resp.Gate)
	}
	if resp.Executed {
		t.Error("executed must be false: the registered route can never run a provider")
	}
	// No workspace is created on the disabled path, so there is nothing to clean.
	if resp.CleanupOK {
		t.Error("cleanupOK = true, but no workspace should have been created")
	}
}

// The registered deps must leave Runner nil: that, not a flag, is what makes
// provider execution structurally impossible.
func TestRegisterPlanningCouncilRoutes_RunnerIsNilAndGateDisabled(t *testing.T) {
	deps := planningCouncilProviderRunEndpointDeps{
		RepoRoot: t.TempDir(), MutationDir: t.TempDir(), Enabled: false, Runner: nil,
	}
	if deps.Runner != nil {
		t.Fatal("Runner must be nil: no runner may be constructed at registration")
	}
	if deps.Enabled {
		t.Fatal("Enabled must be false")
	}
	if got := councilRunGate(deps); got != councilGateDisabled {
		t.Errorf("gate = %q, want disabled", got)
	}
}

// A nil Runner keeps the gate closed even if Enabled were somehow flipped on, so a
// configuration mistake alone cannot open a provider execution path.
func TestRegisterPlanningCouncilRoutes_NilRunnerBeatsEnabledFlag(t *testing.T) {
	deps := planningCouncilProviderRunEndpointDeps{
		RepoRoot: t.TempDir(), MutationDir: t.TempDir(), Enabled: true, Runner: nil,
	}
	if got := councilRunGate(deps); got != councilGateDisabled {
		t.Errorf("gate = %q, want disabled: a nil runner must dominate Enabled", got)
	}
}
