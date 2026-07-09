package cli

import (
	"bytes"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// councilStateExceptionPath is the single runtime-state file allowed to change
// during a (future) read-only council run without being treated as a mutation
// violation. It is written through the existing /api/workflow/state path and is
// not a source mutation. Every other change is a violation.
const councilStateExceptionPath = ".autopus/workflows/state.json"

// councilMutationSnapshot is a normalized `git status --porcelain` capture taken
// before and after a would-be run. Only path + status codes are retained; file
// CONTENTS are never read, so no secret/token/key material can enter the guard.
type councilMutationSnapshot struct {
	// entries maps a repo-relative (forward-slash) path to its porcelain status
	// code (e.g. " M", "??").
	entries map[string]string
}

// councilMutationResult is the outcome of comparing two snapshots.
type councilMutationResult struct {
	// Clean is true when no disallowed changes were introduced.
	Clean bool
	// Violations lists repo-relative paths that changed and are not exempt.
	Violations []string
	// ExemptedChanges lists changed paths that were allowed (e.g. state.json).
	ExemptedChanges []string
}

// snapshotCouncilMutations runs `git status --porcelain --untracked-files=all` at
// dir and records the path/status of each entry. It deliberately never reads file
// contents, keeping secrets out of the guard and its logs.
func snapshotCouncilMutations(dir string) (councilMutationSnapshot, error) {
	cmd := exec.Command("git", "-c", "gc.auto=0", "status", "--porcelain", "--untracked-files=all")
	cmd.Dir = dir

	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return councilMutationSnapshot{}, fmt.Errorf("git status --porcelain: %w\n%s", err, errBuf.String())
	}

	entries := map[string]string{}
	for _, line := range strings.Split(out.String(), "\n") {
		if len(line) < 4 {
			continue
		}
		// Porcelain v1 line: "XY path". Status code is the first two chars.
		code := line[:2]
		path := strings.TrimSpace(line[3:])
		// A rename is reported as "old -> new"; keep the destination path.
		if idx := strings.Index(path, " -> "); idx >= 0 {
			path = path[idx+4:]
		}
		path = strings.Trim(path, "\"")
		if path == "" {
			continue
		}
		entries[normalizeCouncilPath(path)] = code
	}
	return councilMutationSnapshot{entries: entries}, nil
}

// diffCouncilMutations compares before/after. A path is a violation when it is
// newly present, or its status changed relative to the baseline, AND it is not
// the exempt state.json file.
func diffCouncilMutations(before, after councilMutationSnapshot) councilMutationResult {
	res := councilMutationResult{Clean: true}
	for path, afterCode := range after.entries {
		if beforeCode, existed := before.entries[path]; existed && beforeCode == afterCode {
			continue // unchanged relative to the baseline snapshot
		}
		if path == councilStateExceptionPath {
			res.ExemptedChanges = append(res.ExemptedChanges, path)
			continue
		}
		res.Violations = append(res.Violations, path)
	}
	sort.Strings(res.Violations)
	sort.Strings(res.ExemptedChanges)
	if len(res.Violations) > 0 {
		res.Clean = false
	}
	return res
}

// normalizeCouncilPath normalizes separators to forward slashes so comparisons
// are stable across operating systems.
func normalizeCouncilPath(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}
