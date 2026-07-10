package cli

import (
	"context"
	"strings"
	"testing"
	"time"
)

// runnerOpts builds valid non-dangerous options pinned to an isolated temp cwd.
func runnerOpts(t *testing.T, provider string) readOnlyProviderOptions {
	t.Helper()
	opts, err := buildReadOnlyProviderOptions(provider, "analyze", t.TempDir(), "")
	if err != nil {
		t.Fatalf("build opts for %s: %v", provider, err)
	}
	return opts
}

func TestCouncilProviderRunner_CompletedRawPreserved(t *testing.T) {
	r := fakeCouncilProviderRunner{status: councilRunCompleted, rawText: "MODEL OUTPUT"}
	res, err := r.Run(context.Background(), runnerOpts(t, "claude"))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res.Status != councilRunCompleted {
		t.Fatalf("status = %q, want completed", res.Status)
	}
	if res.RawText != "MODEL OUTPUT" {
		t.Fatalf("rawText not preserved: %q", res.RawText)
	}
	if res.ExitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", res.ExitCode)
	}
	if res.ProviderID != "claude" {
		t.Fatalf("providerID = %q, want claude", res.ProviderID)
	}
	if res.DurationMs < 0 {
		t.Fatalf("duration should be >= 0")
	}
}

func TestCouncilProviderRunner_FailedRawPreserved(t *testing.T) {
	r := fakeCouncilProviderRunner{status: councilRunFailed, rawText: "partial", errMsg: "boom", exitCode: 2}
	res, _ := r.Run(context.Background(), runnerOpts(t, "codex"))
	if res.Status != councilRunFailed {
		t.Fatalf("status = %q, want failed", res.Status)
	}
	if res.RawText != "partial" {
		t.Fatalf("rawText not preserved on failure: %q", res.RawText)
	}
	if res.ExitCode == 0 {
		t.Fatalf("exitCode should be non-zero on failure")
	}
	if res.Error == "" {
		t.Fatalf("error should be recorded")
	}
}

func TestCouncilProviderRunner_Timeout(t *testing.T) {
	r := fakeCouncilProviderRunner{status: councilRunCompleted, rawText: "partial", work: 50 * time.Millisecond}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	res, _ := r.Run(ctx, runnerOpts(t, "claude"))
	if res.Status != councilRunTimeout {
		t.Fatalf("status = %q, want timeout", res.Status)
	}
	if res.RawText != "partial" {
		t.Fatalf("partial rawText should be preserved on timeout: %q", res.RawText)
	}
}

func TestCouncilProviderRunner_Canceled(t *testing.T) {
	r := fakeCouncilProviderRunner{status: councilRunCompleted, rawText: "x", work: 50 * time.Millisecond}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-canceled
	res, _ := r.Run(ctx, runnerOpts(t, "claude"))
	if res.Status != councilRunCanceled {
		t.Fatalf("status = %q, want canceled", res.Status)
	}
}

func TestCouncilProviderRunner_AuthRequiredNoRun(t *testing.T) {
	r := fakeCouncilProviderRunner{status: councilRunAuthRequired, rawText: "should-not-appear"}
	res, _ := r.Run(context.Background(), runnerOpts(t, "gemini"))
	if res.Status != councilRunAuthRequired {
		t.Fatalf("status = %q, want authRequired", res.Status)
	}
	if res.RawText != "" {
		t.Fatalf("authRequired must not produce output, got %q", res.RawText)
	}
}

func TestCouncilProviderRunner_UnavailableNoRun(t *testing.T) {
	r := fakeCouncilProviderRunner{status: councilRunUnavailable, rawText: "should-not-appear"}
	res, _ := r.Run(context.Background(), runnerOpts(t, "gemini"))
	if res.Status != councilRunUnavailable {
		t.Fatalf("status = %q, want unavailable", res.Status)
	}
	if res.RawText != "" {
		t.Fatalf("unavailable must not produce output, got %q", res.RawText)
	}
}

func TestCouncilProviderRunner_UnknownProviderRejected(t *testing.T) {
	r := fakeCouncilProviderRunner{status: councilRunCompleted, rawText: "x"}
	// Build opts manually to bypass the builder's own provider check.
	opts := readOnlyProviderOptions{Provider: "gpt", Args: []string{"--print"}, Cwd: t.TempDir()}
	res, _ := r.Run(context.Background(), opts)
	if res.Status != councilRunFailed {
		t.Fatalf("status = %q, want failed for unknown provider", res.Status)
	}
}

func TestCouncilProviderRunner_DangerousArgsRejected(t *testing.T) {
	r := fakeCouncilProviderRunner{status: councilRunCompleted, rawText: "x"}
	opts := readOnlyProviderOptions{Provider: "claude", Args: []string{"--dangerously-skip-permissions"}, Cwd: t.TempDir()}
	res, _ := r.Run(context.Background(), opts)
	if res.Status != councilRunFailed {
		t.Fatalf("status = %q, want failed for dangerous args", res.Status)
	}
	if !strings.Contains(res.Error, "dangerous flag") {
		t.Fatalf("error should mention dangerous flag, got %q", res.Error)
	}
}

func TestCouncilProviderRunner_OutputTruncated(t *testing.T) {
	big := strings.Repeat("a", councilMaxRawOutputBytes+10)
	r := fakeCouncilProviderRunner{status: councilRunCompleted, rawText: big}
	res, _ := r.Run(context.Background(), runnerOpts(t, "claude"))
	if !res.Truncated {
		t.Fatalf("Truncated should be true for oversized output")
	}
	if len(res.RawText) != councilMaxRawOutputBytes {
		t.Fatalf("rawText len = %d, want %d", len(res.RawText), councilMaxRawOutputBytes)
	}
}

// Interface conformance: the fake satisfies councilProviderRunner.
func TestCouncilProviderRunner_InterfaceConformance(t *testing.T) {
	var _ councilProviderRunner = fakeCouncilProviderRunner{}
}
