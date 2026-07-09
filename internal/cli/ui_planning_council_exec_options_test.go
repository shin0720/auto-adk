package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildReadOnlyProviderOptions_KnownProviders(t *testing.T) {
	tempCwd := t.TempDir()
	repoRoot := t.TempDir()
	for _, p := range []string{"claude", "codex", "gemini"} {
		opts, err := buildReadOnlyProviderOptions(p, "hello", tempCwd, repoRoot)
		if err != nil {
			t.Fatalf("provider %s: unexpected error: %v", p, err)
		}
		if opts.Cwd != tempCwd {
			t.Fatalf("provider %s: cwd should be temp cwd, got %s", p, opts.Cwd)
		}
		if opts.Provider != p {
			t.Fatalf("provider mismatch: want %s got %s", p, opts.Provider)
		}
		if flag, bad := containsDangerousFlag(opts.Args); bad {
			t.Fatalf("provider %s: dangerous flag present: %s", p, flag)
		}
	}
}

func TestBuildReadOnlyProviderOptions_NoDangerousFlags(t *testing.T) {
	tempCwd := t.TempDir()
	forbidden := []string{"--dangerously-skip-permissions", "--full-auto"}
	for _, p := range []string{"claude", "codex", "gemini"} {
		opts, err := buildReadOnlyProviderOptions(p, "hi", tempCwd, "")
		if err != nil {
			t.Fatalf("provider %s: %v", p, err)
		}
		for _, a := range opts.Args {
			for _, f := range forbidden {
				if a == f {
					t.Fatalf("provider %s emitted dangerous flag %s", p, a)
				}
			}
		}
	}
}

func TestBuildReadOnlyProviderOptions_UnknownProvider(t *testing.T) {
	tempCwd := t.TempDir()
	if _, err := buildReadOnlyProviderOptions("bogus", "hi", tempCwd, ""); err == nil {
		t.Fatalf("expected error for unknown provider")
	}
}

func TestBuildReadOnlyProviderOptions_EmptyPrompt(t *testing.T) {
	tempCwd := t.TempDir()
	if _, err := buildReadOnlyProviderOptions("claude", "   ", tempCwd, ""); err == nil {
		t.Fatalf("expected error for empty prompt")
	}
}

func TestBuildReadOnlyProviderOptions_EmptyCwd(t *testing.T) {
	if _, err := buildReadOnlyProviderOptions("claude", "hi", "", ""); err == nil {
		t.Fatalf("expected error for empty cwd")
	}
}

func TestBuildReadOnlyProviderOptions_RejectsRepoRootCwd(t *testing.T) {
	repoRoot := t.TempDir()
	if _, err := buildReadOnlyProviderOptions("claude", "hi", repoRoot, repoRoot); err == nil {
		t.Fatalf("expected rejection when cwd equals repo root")
	}
	inside := filepath.Join(repoRoot, "sub")
	if err := os.MkdirAll(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := buildReadOnlyProviderOptions("claude", "hi", inside, repoRoot); err == nil {
		t.Fatalf("expected rejection when cwd inside repo root")
	}
}

func TestContainsDangerousFlag_DetectsValueForm(t *testing.T) {
	if _, bad := containsDangerousFlag([]string{"--approval-mode=full-auto"}); !bad {
		t.Fatalf("expected detection of --approval-mode=full-auto")
	}
	if _, bad := containsDangerousFlag([]string{"--print"}); bad {
		t.Fatalf("--print must not be flagged as dangerous")
	}
}
