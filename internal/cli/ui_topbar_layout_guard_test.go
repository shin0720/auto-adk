package cli

import (
	"regexp"
	"strings"
	"testing"

	"github.com/shin0720/auto-adk/content"
)

// Static-invariant guard for the PR #64 topbar layout fix. The dashboard CSS is
// embedded (no CSS test runner), so the invariants are asserted on the served
// asset: the mode-toggle labels must stay horizontal (nowrap) and the right-side
// controls must not be compressed by a wider provider-status block (flex-shrink:0).
func TestDashboardLayout_TopbarModeControlsStayHorizontal(t *testing.T) {
	b, err := content.FS.ReadFile("ui/dashboard-layout.css")
	if err != nil {
		t.Fatalf("read embedded ui/dashboard-layout.css: %v", err)
	}
	css := string(b)

	// The primary `.mode-toggle button` rule (not the responsive override) must
	// carry white-space:nowrap so "빌더"/"모니터링" never wrap vertically.
	ruleRe := regexp.MustCompile(`\.mode-toggle button \{[^}]*\}`)
	base := ""
	for _, m := range ruleRe.FindAllString(css, -1) {
		if strings.Contains(m, "cursor:pointer") { // the base rule, not the compact override
			base = m
			break
		}
	}
	if base == "" {
		t.Fatalf("could not locate the base .mode-toggle button rule")
	}
	if !strings.Contains(base, "white-space:nowrap") {
		t.Fatalf(".mode-toggle button lacks white-space:nowrap: %q", base)
	}

	// The mode-toggle and zoom controls must resist flex compression.
	if !regexp.MustCompile(`\.mode-toggle \{[^}]*flex-shrink:0`).MatchString(css) {
		t.Fatalf(".mode-toggle lacks flex-shrink:0")
	}
	if !regexp.MustCompile(`\.zoom-compact \{[^}]*flex-shrink:0`).MatchString(css) {
		t.Fatalf(".zoom-compact lacks flex-shrink:0")
	}
}
