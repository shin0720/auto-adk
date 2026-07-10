package cli

import (
	"errors"
	"os"
	"testing"
)

// The harness must never surface a dangerous flag in its emitted Args, for any
// provider, and buildReadOnlyProviderOptions must agree.
func TestCouncilHarness_DangerousArgsAbsent(t *testing.T) {
	repo := newCouncilHarnessRepo(t)
	for _, provider := range []string{"claude", "codex", "gemini"} {
		res := runCouncilReadOnlyHarness(councilHarnessParams{
			Provider: provider, Prompt: "analyze", RepoRoot: repo, MutationDir: repo,
			Runner: func(opts readOnlyProviderOptions) error { return nil },
		})
		if res.Status != councilHarnessFakeCompleted {
			t.Fatalf("%s: status = %q, want fakeCompleted (err=%q)", provider, res.Status, res.Error)
		}
		if flag, bad := containsDangerousFlag(res.Args); bad {
			t.Fatalf("%s: emitted dangerous flag %s in %v", provider, flag, res.Args)
		}
	}
}

// A fake runner that errors (without mutating the repo) yields fakeFailed, and
// success is not reported.
func TestCouncilHarness_FakeRunnerError(t *testing.T) {
	repo := newCouncilHarnessRepo(t)
	res := runCouncilReadOnlyHarness(councilHarnessParams{
		Provider: "claude", Prompt: "analyze", RepoRoot: repo, MutationDir: repo,
		Runner: func(opts readOnlyProviderOptions) error { return errors.New("fake failed") },
	})
	if res.Status != councilHarnessFakeFailed {
		t.Fatalf("status = %q, want fakeFailed", res.Status)
	}
	if res.Error == "" {
		t.Fatalf("error should be recorded")
	}
	if !res.CleanupOK {
		t.Fatalf("cleanup should still succeed on runner error")
	}
}

// An unknown provider is rejected before any snapshot or runner call.
func TestCouncilHarness_UnknownProviderInvalidOptions(t *testing.T) {
	repo := newCouncilHarnessRepo(t)
	called := false
	res := runCouncilReadOnlyHarness(councilHarnessParams{
		Provider: "gpt", Prompt: "analyze", RepoRoot: repo, MutationDir: repo,
		Runner: func(opts readOnlyProviderOptions) error { called = true; return nil },
	})
	if res.Status != councilHarnessInvalidOptions {
		t.Fatalf("status = %q, want invalidOptions", res.Status)
	}
	if called {
		t.Fatalf("runner must not be called for an unknown provider")
	}
	if !res.CleanupOK {
		t.Fatalf("temp cwd should still be cleaned up")
	}
}

// An empty prompt is rejected as invalidOptions.
func TestCouncilHarness_EmptyPromptInvalidOptions(t *testing.T) {
	repo := newCouncilHarnessRepo(t)
	res := runCouncilReadOnlyHarness(councilHarnessParams{
		Provider: "claude", Prompt: "   ", RepoRoot: repo, MutationDir: repo,
		Runner: func(opts readOnlyProviderOptions) error { return nil },
	})
	if res.Status != councilHarnessInvalidOptions {
		t.Fatalf("status = %q, want invalidOptions", res.Status)
	}
}

// The underlying workspace cleanup stays idempotent: a second call is safe.
func TestCouncilHarness_CleanupIdempotent(t *testing.T) {
	ws, cleanup, err := createReadOnlyCouncilWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if err := cleanup(); err != nil {
		t.Fatalf("first cleanup: %v", err)
	}
	if err := cleanup(); err != nil {
		t.Fatalf("second cleanup should be safe: %v", err)
	}
	if _, err := os.Stat(ws.Dir); !os.IsNotExist(err) {
		t.Fatalf("workspace should be gone, stat err=%v", err)
	}
}
