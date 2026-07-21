package cli

import (
	"os"
	"path/filepath"
	"strings"
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

func TestBuildReadOnlyProviderOptions_PromptGoesToStdinNotArgs(t *testing.T) {
	const prompt = "SECRET-LOOKING-PROMPT-TEXT"
	tempCwd := t.TempDir()
	for _, p := range []string{"claude", "codex", "gemini"} {
		opts, err := buildReadOnlyProviderOptions(p, prompt, tempCwd, "")
		if err != nil {
			t.Fatalf("provider %s: %v", p, err)
		}
		if opts.Stdin != prompt {
			t.Fatalf("provider %s: Stdin = %q, want the prompt", p, opts.Stdin)
		}
		// Args reach the process table; the prompt must never be there.
		for _, a := range opts.Args {
			if strings.Contains(a, prompt) {
				t.Fatalf("provider %s: prompt leaked into args %v", p, opts.Args)
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

func TestBuildReadOnlyProviderOptions_CodexReadOnlyNonRepoArgs(t *testing.T) {
	tempCwd := t.TempDir()
	opts, err := buildReadOnlyProviderOptions("codex", "hello", tempCwd, "")
	if err != nil {
		t.Fatalf("codex options: %v", err)
	}

	// Exact args order is pinned so a regression in flag order is caught.
	want := []string{"exec", "--skip-git-repo-check", "--sandbox", "read-only"}
	if len(opts.Args) != len(want) {
		t.Fatalf("codex args = %v, want %v", opts.Args, want)
	}
	for i := range want {
		if opts.Args[i] != want[i] {
			t.Fatalf("codex args[%d] = %q, want %q (full: %v)", i, opts.Args[i], want[i], opts.Args)
		}
	}

	// --sandbox must be immediately followed by read-only.
	sbIdx := -1
	for i, a := range opts.Args {
		if a == "--sandbox" {
			sbIdx = i
			break
		}
	}
	if sbIdx < 0 || sbIdx+1 >= len(opts.Args) || opts.Args[sbIdx+1] != "read-only" {
		t.Fatalf("--sandbox must be followed by read-only, got %v", opts.Args)
	}

	// --ephemeral is intentionally excluded from this patch.
	for _, a := range opts.Args {
		if a == "--ephemeral" {
			t.Fatalf("--ephemeral must not be present in this patch: %v", opts.Args)
		}
	}

	// The new flags must not be seen as dangerous by the guard.
	if flag, bad := containsDangerousFlag(opts.Args); bad {
		t.Fatalf("codex read-only args flagged as dangerous: %s", flag)
	}

	// Prompt stays on stdin, never in args.
	if opts.Stdin != "hello" {
		t.Fatalf("codex Stdin = %q, want the prompt", opts.Stdin)
	}
	for _, a := range opts.Args {
		if strings.Contains(a, "hello") {
			t.Fatalf("prompt leaked into codex args: %v", opts.Args)
		}
	}
}

func TestBuildReadOnlyProviderOptions_ClaudeAndGeminiArgsUnchanged(t *testing.T) {
	tempCwd := t.TempDir()
	claude, err := buildReadOnlyProviderOptions("claude", "hi", tempCwd, "")
	if err != nil {
		t.Fatalf("claude: %v", err)
	}
	if len(claude.Args) != 1 || claude.Args[0] != "--print" {
		t.Fatalf("claude args = %v, want [--print]", claude.Args)
	}
	gemini, err := buildReadOnlyProviderOptions("gemini", "hi", tempCwd, "")
	if err != nil {
		t.Fatalf("gemini: %v", err)
	}
	if len(gemini.Args) != 1 || gemini.Args[0] != "-p" {
		t.Fatalf("gemini args = %v, want [-p]", gemini.Args)
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
