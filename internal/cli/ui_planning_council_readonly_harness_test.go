package cli

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// gitInDir runs a git subcommand in dir, failing the test on error. This is the
// ONLY exec in these tests and never runs a provider CLI.
func gitInDir(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// newCouncilHarnessRepo creates a throwaway git repo with one committed file so
// its `git status --porcelain` starts clean. The user's real repo is never used.
func newCouncilHarnessRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitInDir(t, dir, "init", "-q")
	gitInDir(t, dir, "config", "user.email", "t@example.com")
	gitInDir(t, dir, "config", "user.name", "t")
	gitInDir(t, dir, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitInDir(t, dir, "add", ".")
	gitInDir(t, dir, "commit", "-q", "-m", "init")
	return dir
}

func councilContains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

// clean path: the fake runner only writes inside its isolated temp cwd, so the
// guarded repo is unchanged -> fakeCompleted with no violations, cleanup ok.
func TestCouncilHarness_CleanPath(t *testing.T) {
	repo := newCouncilHarnessRepo(t)
	res := runCouncilReadOnlyHarness(councilHarnessParams{
		Provider: "claude", Prompt: "analyze", RepoRoot: repo, MutationDir: repo,
		Runner: func(opts readOnlyProviderOptions) error {
			return os.WriteFile(filepath.Join(opts.Cwd, "scratch.txt"), []byte("x"), 0o644)
		},
	})
	if res.Status != councilHarnessFakeCompleted {
		t.Fatalf("status = %q, want fakeCompleted (err=%q)", res.Status, res.Error)
	}
	if len(res.MutationViolations) != 0 {
		t.Fatalf("unexpected violations: %v", res.MutationViolations)
	}
	if !res.CleanupOK {
		t.Fatalf("cleanup should succeed")
	}
	if _, err := os.Stat(res.TempCWD); !os.IsNotExist(err) {
		t.Fatalf("temp cwd should be removed, stat err=%v", err)
	}
}

// repo mutation: the runner modifies a tracked file in the guarded repo ->
// scopeViolation, and success is refused.
func TestCouncilHarness_RepoMutationScopeViolation(t *testing.T) {
	repo := newCouncilHarnessRepo(t)
	res := runCouncilReadOnlyHarness(councilHarnessParams{
		Provider: "codex", Prompt: "analyze", RepoRoot: repo, MutationDir: repo,
		Runner: func(opts readOnlyProviderOptions) error {
			return os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("mutated\n"), 0o644)
		},
	})
	if res.Status != councilHarnessScopeViolation {
		t.Fatalf("status = %q, want scopeViolation", res.Status)
	}
	if !councilContains(res.MutationViolations, "tracked.txt") {
		t.Fatalf("violations should include tracked.txt, got %v", res.MutationViolations)
	}
}

// state.json exception: the only change is the exempt runtime-state file ->
// fakeCompleted, recorded under AllowedStateChanges, not a violation.
func TestCouncilHarness_StateJSONExceptionAllowed(t *testing.T) {
	repo := newCouncilHarnessRepo(t)
	res := runCouncilReadOnlyHarness(councilHarnessParams{
		Provider: "gemini", Prompt: "analyze", RepoRoot: repo, MutationDir: repo,
		Runner: func(opts readOnlyProviderOptions) error {
			p := filepath.Join(repo, ".autopus", "workflows")
			if err := os.MkdirAll(p, 0o755); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(p, "state.json"), []byte("{}"), 0o644)
		},
	})
	if res.Status != councilHarnessFakeCompleted {
		t.Fatalf("status = %q, want fakeCompleted (err=%q)", res.Status, res.Error)
	}
	if !councilContains(res.AllowedStateChanges, councilStateExceptionPath) {
		t.Fatalf("state.json should be allowed, got %v", res.AllowedStateChanges)
	}
	if len(res.MutationViolations) != 0 {
		t.Fatalf("unexpected violations: %v", res.MutationViolations)
	}
}

// mixed: state.json (allowed) plus a normal file (violation) -> scopeViolation,
// with each change classified correctly.
func TestCouncilHarness_MixedViolation(t *testing.T) {
	repo := newCouncilHarnessRepo(t)
	res := runCouncilReadOnlyHarness(councilHarnessParams{
		Provider: "claude", Prompt: "analyze", RepoRoot: repo, MutationDir: repo,
		Runner: func(opts readOnlyProviderOptions) error {
			p := filepath.Join(repo, ".autopus", "workflows")
			if err := os.MkdirAll(p, 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(p, "state.json"), []byte("{}"), 0o644); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(repo, "other.txt"), []byte("y"), 0o644)
		},
	})
	if res.Status != councilHarnessScopeViolation {
		t.Fatalf("status = %q, want scopeViolation", res.Status)
	}
	if !councilContains(res.MutationViolations, "other.txt") {
		t.Fatalf("violations should include other.txt, got %v", res.MutationViolations)
	}
	if !councilContains(res.AllowedStateChanges, councilStateExceptionPath) {
		t.Fatalf("state.json should still be allowed, got %v", res.AllowedStateChanges)
	}
}

// cleanup failure seam: a clean run whose cleanup reports an error -> cleanupFailed.
func TestCouncilHarness_CleanupFailureSeam(t *testing.T) {
	repo := newCouncilHarnessRepo(t)
	res := runCouncilReadOnlyHarness(councilHarnessParams{
		Provider: "claude", Prompt: "analyze", RepoRoot: repo, MutationDir: repo,
		Runner:          func(opts readOnlyProviderOptions) error { return nil },
		cleanupOverride: func() error { return errors.New("cleanup boom") },
	})
	// The override skips the real removal; clean up the leaked temp dir ourselves.
	t.Cleanup(func() { _ = os.RemoveAll(res.TempCWD) })
	if res.Status != councilHarnessCleanupFailed {
		t.Fatalf("status = %q, want cleanupFailed", res.Status)
	}
	if res.CleanupOK {
		t.Fatalf("CleanupOK should be false")
	}
}
