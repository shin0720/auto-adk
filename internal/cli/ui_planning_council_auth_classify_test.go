package cli

import (
	"strings"
	"testing"
)

// Strong, unambiguous auth-failure phrases classify as authRequired.
func TestClassifyCouncilProviderAuthFailure_StrongMatches(t *testing.T) {
	cases := []struct {
		name   string
		stderr string
	}{
		{"not logged in", "Error: you are not logged in. Run login first."},
		{"login required", "login required to continue"},
		{"authentication required", "authentication required"},
		{"authentication failed", "authentication failed for user"},
		{"unauthorized", "request failed: Unauthorized"},
		{"401 unauthorized", "HTTP 401 Unauthorized"},
		{"invalid credentials", "invalid credentials provided"},
		{"invalid api key", "Invalid API key supplied"},
		{"expired token", "your expired token must be refreshed"},
		{"please sign in", "please sign in to use this tool"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, reason := classifyCouncilProviderAuthFailure("claude", "", tc.stderr, 1)
			if !ok {
				t.Fatalf("expected authRequired for stderr %q", tc.stderr)
			}
			if !strings.HasPrefix(reason, "provider auth failure: ") {
				t.Errorf("reason = %q, want a fixed prefixed reason", reason)
			}
			// The reason must not echo the provider's surrounding output.
			if strings.Contains(reason, "user") || strings.Contains(reason, "HTTP") {
				t.Errorf("reason %q leaked provider output", reason)
			}
		})
	}
}

// A match in stdout (rawText) rather than stderr is still caught.
func TestClassifyCouncilProviderAuthFailure_MatchesRawText(t *testing.T) {
	ok, _ := classifyCouncilProviderAuthFailure("codex", "please login and retry", "", 2)
	if !ok {
		t.Fatal("expected authRequired when the phrase is in rawText")
	}
}

// Gemini's missing-auth-method failure (exitCode 41) must classify as authRequired.
// The samples are SANITIZED: no real path, email, API key, or full output — only
// the distinctive auth phrases with a [REDACTED_PATH] placeholder.
func TestClassifyCouncilProviderAuthFailure_GeminiMissingAuthMethod(t *testing.T) {
	cases := []struct {
		name   string
		stderr string
	}{
		{"set an auth method", "Please set an Auth method in [REDACTED_PATH]/.gemini/settings.json or specify GEMINI_API_KEY before running"},
		{"gemini_api_key only", "no auth configured; specify GEMINI_API_KEY environment variable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, reason := classifyCouncilProviderAuthFailure("gemini", "", tc.stderr, 41)
			if !ok {
				t.Fatalf("expected authRequired for gemini stderr %q", tc.stderr)
			}
			if !strings.HasPrefix(reason, "provider auth failure: ") {
				t.Errorf("reason = %q, want a fixed prefixed reason", reason)
			}
			// The reason is a fixed pattern; it must not echo the sample output.
			if strings.Contains(reason, "REDACTED_PATH") || strings.Contains(reason, "settings.json") {
				t.Errorf("reason %q leaked sample output", reason)
			}
		})
	}
}

// Ordinary non-auth failures, and generic single words, must NOT classify as auth.
func TestClassifyCouncilProviderAuthFailure_NoFalsePositives(t *testing.T) {
	cases := []struct {
		name   string
		stderr string
	}{
		{"generic error", "error: something went wrong"},
		{"generic failed", "failed to compile prompt"},
		{"generic denied", "connection denied by peer"},
		{"generic permission", "permission to write file was refused"},
		{"rate limit", "rate limit exceeded, try later"},
		{"network", "dial tcp: connection refused"},
		{"empty", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, reason := classifyCouncilProviderAuthFailure("gemini", "", tc.stderr, 1)
			if ok {
				t.Errorf("stderr %q must not classify as authRequired (reason=%q)", tc.stderr, reason)
			}
		})
	}
}

// A clean exit is never an auth failure, even if the output mentions login.
func TestClassifyCouncilProviderAuthFailure_CleanExitNeverAuth(t *testing.T) {
	ok, _ := classifyCouncilProviderAuthFailure("claude", "you are not logged in", "", 0)
	if ok {
		t.Error("exit code 0 must never classify as authRequired")
	}
}

// An unknown provider is never guessed as auth, even with a matching phrase.
func TestClassifyCouncilProviderAuthFailure_UnknownProviderNeverAuth(t *testing.T) {
	ok, _ := classifyCouncilProviderAuthFailure("gpt", "unauthorized", "", 1)
	if ok {
		t.Error("unknown provider must not classify as authRequired")
	}
}
