package cli

import (
	"fmt"
	"strings"
)

// readOnlyProviderOptions describes how a FUTURE read-only council run would
// invoke a provider CLI. This PR only BUILDS the option set; it never starts a
// process. The defining property is that NO dangerous / auto-approve /
// unrestricted-write flag is present, and the cwd is always an isolated temp
// workspace — never the repo root.
type readOnlyProviderOptions struct {
	Provider string
	Args     []string
	// Cwd is the isolated temp workspace directory. It must be non-empty and must
	// not be (or be inside) the repo root; the builder enforces this.
	Cwd string
	// Stdin carries the prompt. The prompt is deliberately NOT placed in Args:
	// process arguments are world-readable (e.g. `ps`) and are bounded by an
	// OS-specific length limit, neither of which is acceptable for prompt text.
	Stdin string
}

// dangerousProviderFlags enumerates flags that grant unconfirmed file mutation or
// fully-automated execution. They MUST NOT appear in read-only options and are
// asserted against in the unit tests. The command guard remains a separate
// defense-in-depth layer, not the primary barrier (see command_guard_hook.go).
var dangerousProviderFlags = []string{
	"--dangerously-skip-permissions",
	"--full-auto",
	"--yolo",
	"--auto-edit",
	"--accept-edits",
	"--approval-mode",
}

// buildReadOnlyProviderOptions returns non-dangerous invocation options for a
// known provider, pinned to the given isolated temp cwd. The prompt is carried in
// Stdin, never in Args. It returns an error for an unknown provider, an empty
// prompt, or a cwd that is empty or equal to / inside the repo root. It does NOT
// execute anything.
func buildReadOnlyProviderOptions(provider, prompt, tempCwd, repoRoot string) (readOnlyProviderOptions, error) {
	if strings.TrimSpace(prompt) == "" {
		return readOnlyProviderOptions{}, fmt.Errorf("read-only provider options require a non-empty prompt")
	}
	if strings.TrimSpace(tempCwd) == "" {
		return readOnlyProviderOptions{}, fmt.Errorf("read-only provider options require an isolated temp cwd")
	}
	if repoRoot != "" {
		if same, err := sameOrInside(tempCwd, repoRoot); err == nil && same {
			return readOnlyProviderOptions{}, fmt.Errorf("refusing repo-root cwd for read-only provider: %s", tempCwd)
		}
	}

	var args []string
	switch provider {
	case "claude":
		// --print: non-interactive, prints the result and exits. Deliberately NO
		// --dangerously-skip-permissions, so file tools stay permission-gated.
		args = []string{"--print"}
	case "codex":
		// Codex runs from an isolated non-repo temp cwd; skip only the git-repo
		// precheck while pinning the sandbox to read-only. No --full-auto, no
		// auto-approved writes. Prompt remains on stdin.
		args = []string{"exec", "--skip-git-repo-check", "--sandbox", "read-only"}
	case "gemini":
		// -p prompt mode WITHOUT any auto-approve/write flag.
		args = []string{"-p"}
	default:
		return readOnlyProviderOptions{}, fmt.Errorf("unknown provider: %q", provider)
	}

	if flag, bad := containsDangerousFlag(args); bad {
		// Defensive: should be unreachable, but never emit a dangerous flag.
		return readOnlyProviderOptions{}, fmt.Errorf("internal: read-only args contained dangerous flag %s", flag)
	}

	return readOnlyProviderOptions{Provider: provider, Args: args, Cwd: tempCwd, Stdin: prompt}, nil
}

// containsDangerousFlag reports the first dangerous flag found in args, if any.
// It matches an exact flag or its "--flag=value" form.
func containsDangerousFlag(args []string) (string, bool) {
	for _, a := range args {
		low := strings.ToLower(strings.TrimSpace(a))
		for _, d := range dangerousProviderFlags {
			if low == d || strings.HasPrefix(low, d+"=") {
				return d, true
			}
		}
	}
	return "", false
}
