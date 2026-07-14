//go:build e2e

package e2e

import (
	"regexp"
	"strings"
)

// versionLineRe matches the "auto <version> (commit: <c>, built: <d>)" line.
// The version, commit, and build date all vary between the local temp build
// and the CI make build, so the whole value payload is replaced.
var versionLineRe = regexp.MustCompile(`^auto .+ \(commit: .+, built: .+\)$`)

// scrubVersionOutput normalizes build-specific dynamic values in the `auto
// version` output so the golden comparison is stable across environments.
//
// It preserves the static structure — banner, line presence, and field labels —
// and only replaces the dynamic value on each line with a "<dynamic>"
// placeholder. This keeps the golden a meaningful signal for structural
// regressions (a missing/added line still fails) while ignoring values that
// legitimately differ between local (no ldflags) and CI (make build with
// ldflags + absolute binary path) runs.
//
// The function is idempotent: applying it to already-scrubbed output yields the
// same result, so it can be applied to both the expected golden and the actual
// output.
func scrubVersionOutput(s string) string {
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		switch {
		case versionLineRe.MatchString(ln):
			// Line 3: "auto <version> (commit: <c>, built: <d>)".
			lines[i] = "auto <dynamic> (commit: <dynamic>, built: <dynamic>)"
		case strings.HasPrefix(ln, "path: "):
			// Canonical binary path — temp dir locally, absolute in CI.
			lines[i] = "path: <dynamic>"
		case strings.HasPrefix(ln, "invoked via: "):
			// Present only when the binary path is a symlink.
			lines[i] = "invoked via: <dynamic>"
		case strings.HasPrefix(ln, "   ") && strings.TrimSpace(ln) != "":
			// Banner version line: "   <version>".
			lines[i] = "   <dynamic>"
		}
	}
	return strings.Join(lines, "\n")
}
