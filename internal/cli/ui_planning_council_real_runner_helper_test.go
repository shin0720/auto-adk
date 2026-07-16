package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// This file holds the test harness the real-runner tests share. Its defining
// property: the provider stand-in is THIS test binary re-executed, so the real
// claude/codex/gemini binaries are never invoked by any test in this package.

// councilHelperEnv gates the helper-process entrypoint below.
const councilHelperEnv = "GO_WANT_COUNCIL_HELPER_PROCESS"

// TestHelperProcess is not a real test. The runner tests re-execute this test
// binary with councilHelperEnv=1 so it stands in for a provider CLI.
//
// Behavior is driven entirely by environment variables so each scenario stays
// deterministic. It exits via os.Exit before the test framework prints anything,
// keeping the parent's captured output free of test scaffolding.
func TestHelperProcess(t *testing.T) {
	if os.Getenv(councilHelperEnv) != "1" {
		return
	}
	if os.Getenv("COUNCIL_HELPER_PRINT_CWD") == "1" {
		wd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "getwd: %v", err)
			os.Exit(90)
		}
		fmt.Fprintf(os.Stdout, "CWD:%s\n", wd)
	}
	if os.Getenv("COUNCIL_HELPER_ECHO_STDIN") == "1" {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read stdin: %v", err)
			os.Exit(91)
		}
		fmt.Fprintf(os.Stdout, "STDIN:%s\n", b)
	}
	if s := os.Getenv("COUNCIL_HELPER_STDOUT"); s != "" {
		fmt.Fprint(os.Stdout, s)
	}
	if n := os.Getenv("COUNCIL_HELPER_STDOUT_BYTES"); n != "" {
		count, err := strconv.Atoi(n)
		if err != nil {
			os.Exit(92)
		}
		fmt.Fprint(os.Stdout, strings.Repeat("a", count))
	}
	if s := os.Getenv("COUNCIL_HELPER_STDERR"); s != "" {
		fmt.Fprint(os.Stderr, s)
	}
	if d := os.Getenv("COUNCIL_HELPER_SLEEP"); d != "" {
		dur, err := time.ParseDuration(d)
		if err != nil {
			os.Exit(93)
		}
		time.Sleep(dur)
	}
	code := 0
	if c := os.Getenv("COUNCIL_HELPER_EXIT"); c != "" {
		parsed, err := strconv.Atoi(c)
		if err != nil {
			os.Exit(94)
		}
		code = parsed
	}
	os.Exit(code)
}

// helperCall records what the runner asked to execute, so a test can assert that
// nothing was started (or that the args carried no prompt).
type helperCall struct {
	name     string
	args     []string
	invoked  bool
	lookedUp bool
}

// newHelperRunner returns a runner whose binary lookup succeeds and whose command
// re-executes this test binary as the provider stand-in, plus the call record.
func newHelperRunner(t *testing.T, repoRoot string, env ...string) (realCouncilProviderRunner, *helperCall) {
	t.Helper()
	call := &helperCall{}
	r := realCouncilProviderRunner{
		repoRoot: repoRoot,
		lookPath: func(name string) (string, error) {
			call.lookedUp = true
			return name, nil
		},
		command: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			call.invoked = true
			call.name = name
			call.args = args
			helperArgs := append([]string{"-test.run=^TestHelperProcess$", "--"}, args...)
			cmd := exec.CommandContext(ctx, os.Args[0], helperArgs...)
			cmd.Env = append(os.Environ(), append([]string{councilHelperEnv + "=1"}, env...)...)
			return cmd
		},
	}
	return r, call
}

// mustOpts builds valid read-only options pinned to the given isolated cwd.
func mustOpts(t *testing.T, provider, prompt, cwd string) readOnlyProviderOptions {
	t.Helper()
	opts, err := buildReadOnlyProviderOptions(provider, prompt, cwd, "")
	if err != nil {
		t.Fatalf("build opts: %v", err)
	}
	return opts
}

// firstLine returns the first line of s, without its trailing newline.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
