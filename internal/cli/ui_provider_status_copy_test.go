package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shin0720/auto-adk/content"
)

// TestProviderStatusCompactCopy guards PR #69: the stat-only "config detected"
// case must expose a SHORT, caveat-first label so the ".status-sub" ellipsis can
// never truncate the important part into a false "auth complete" reading. The
// fuller meaning lives in StatusDetail (surfaced via the frontend title tooltip).
//
// This is a copy/UX invariant, not a behavior change: AuthState, detection, and
// CanAutoRun are asserted UNCHANGED here so a future edit that quietly widens the
// label (or flips a state) fails loudly.
func TestProviderStatusCompactCopy(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/providers/status", nil)
	handleProviderStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var statuses []providerStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &statuses); err != nil {
		t.Fatalf("decode provider status: %v", err)
	}
	if len(statuses) == 0 {
		t.Fatal("expected at least one provider status")
	}

	// The old long label must not survive anywhere as a status label — its length
	// is exactly what caused the ellipsis to hide the "인증 미확인" caveat.
	const oldLongLabel = "설정 감지됨 · 인증 미확인"

	// Locate the config-detected branch by its authState, so the test does not
	// depend on which providers happen to be installed on the runner.
	var sawConfigDetected bool
	for _, st := range statuses {
		if st.StatusLabel == oldLongLabel {
			t.Errorf("provider %q still uses the old long label %q", st.ID, oldLongLabel)
		}
		// PR #29 invariant: no branch ever promises auto-run.
		if st.CanAutoRun {
			t.Errorf("provider %q has CanAutoRun=true; auto-run must stay disabled", st.ID)
		}
		if st.AuthState == "available" {
			sawConfigDetected = true
			if st.StatusLabel != "인증 미확인" {
				t.Errorf("config-detected provider %q label = %q, want %q",
					st.ID, st.StatusLabel, "인증 미확인")
			}
			if !strings.Contains(st.StatusDetail, "실제 실행 전 인증 미검증") {
				t.Errorf("config-detected provider %q detail = %q, want it to contain %q",
					st.ID, st.StatusDetail, "실제 실행 전 인증 미검증")
			}
		}
	}
	if !sawConfigDetected {
		t.Skip("no provider resolved to authState=available on this host; copy branch not exercised")
	}
}

// TestProviderStatusDetailWiredToTitle guards the frontend half of PR #69: the
// compact label is only safe because the fuller StatusDetail is reachable on
// hover. This asserts dashboard-providers.js wires statusDetail into a title.
func TestProviderStatusDetailWiredToTitle(t *testing.T) {
	raw, err := content.FS.ReadFile("ui/dashboard-providers.js")
	if err != nil {
		t.Fatalf("read embedded dashboard-providers.js: %v", err)
	}
	js := string(raw)

	if !strings.Contains(js, "provider.statusDetail") {
		t.Error("dashboard-providers.js does not reference provider.statusDetail")
	}
	if !strings.Contains(js, ".title =") {
		t.Error("dashboard-providers.js does not assign a .title (tooltip) anywhere")
	}
	// The specific wiring: detail must feed the title, not just be read.
	if !strings.Contains(js, "label.title = detailTitle") {
		t.Error("dashboard-providers.js does not connect the status detail to label.title")
	}
}
