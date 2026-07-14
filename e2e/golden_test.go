//go:build e2e

package e2e

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// updateGolden controls whether golden files are refreshed on this run.
// Use: go test -tags e2e -run TestGolden ./e2e/... -update
var updateGolden = flag.Bool("update", false, "update golden files")

// identity returns the input unchanged. It is used for command outputs that
// contain no build-specific dynamic values (e.g. --help), so their golden
// files act as a strict regression signal for command/flag drift.
func identity(s string) string { return s }

// TestGolden validates CLI output against golden files stored in testdata/.
//
// Dynamic, build-specific values (version, commit, build date, binary path)
// differ between the local temp build (no ldflags) and the CI make build
// (ldflags + absolute binary path). Each case supplies a scrub function that
// normalizes those values to stable placeholders; the SAME scrub is applied to
// both the expected golden and the actual output before comparison, and golden
// files are stored in already-scrubbed form.
func TestGolden(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)

	cases := []struct {
		name   string
		args   []string
		golden string
		scrub  func(string) string
	}{
		{
			name:   "version output",
			args:   []string{"version"},
			golden: "version_output.golden",
			scrub:  scrubVersionOutput,
		},
		{
			name:   "help output",
			args:   []string{"--help"},
			golden: "help_output.golden",
			scrub:  identity,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := runBinary(t, bin, tc.args...)
			got := tc.scrub(r.Stdout + r.Stderr)

			goldenPath := filepath.Join("testdata", tc.golden)

			if *updateGolden {
				require.NoError(t, os.MkdirAll("testdata", 0o755))
				require.NoError(t, os.WriteFile(goldenPath, []byte(got), 0o644))
				t.Logf("updated golden file: %s", goldenPath)
				return
			}

			data, err := os.ReadFile(goldenPath)
			if os.IsNotExist(err) {
				t.Skipf("golden file missing: %s (run with -update to create)", goldenPath)
				return
			}
			require.NoError(t, err)

			// Scrub both sides. The golden is stored scrubbed and the scrub is
			// idempotent, so this only normalizes the actual output while
			// keeping the comparison symmetric.
			want := tc.scrub(string(data))
			assert.Equal(t, want, got, "output does not match golden file %s", goldenPath)
		})
	}
}
