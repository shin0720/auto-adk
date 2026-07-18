package cli

import "strings"

// councilAuthFailurePatterns are lower-cased substrings that STRONGLY indicate a
// provider refused the run because it is not authenticated. They are deliberately
// multi-word phrases: single generic words ("error", "failed", "denied",
// "permission") are excluded because a normal non-auth failure trips them and
// would misreport as authRequired.
//
// Each entry is a fixed constant, never derived from provider output, so using one
// as a classification reason cannot leak prompt or secret text.
var councilAuthFailurePatterns = []string{
	"not logged in",
	"not authenticated",
	"login required",
	"please login",
	"please log in",
	"please sign in",
	"sign in to",
	"authentication required",
	"authentication failed",
	"unauthorized",
	"401 unauthorized",
	"invalid credentials",
	"invalid api key",
	"missing api key",
	"expired token",
	"token expired",
	"session expired",
	"run `claude login`",
	"run 'claude login'",
}

// classifyCouncilProviderAuthFailure decides whether a FAILED provider run failed
// specifically because of authentication. It reports (true, reason) only on a
// strong match and (false, "") whenever there is any doubt, so an ambiguous
// failure always degrades to the plain failed status rather than a false
// authRequired.
//
// It reads only the strings already collected from the process (capped stdout/
// stderr and the run error) — it never opens a token/config file. The returned
// reason is one of the fixed patterns above, so it carries no provider output.
func classifyCouncilProviderAuthFailure(provider, rawText, errText string, exitCode int) (bool, string) {
	// Never guess auth for an unknown provider; the caller rejects those upstream,
	// but stay conservative here too.
	switch provider {
	case "claude", "codex", "gemini":
	default:
		return false, ""
	}
	// A clean exit is not a failure; nothing to classify.
	if exitCode == 0 {
		return false, ""
	}

	hay := strings.ToLower(rawText + "\n" + errText)
	for _, pat := range councilAuthFailurePatterns {
		if strings.Contains(hay, pat) {
			return true, "provider auth failure: " + pat
		}
	}
	return false, ""
}
