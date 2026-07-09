package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// readOnlyCouncilWorkspace is an isolated, empty temporary directory intended as
// the cwd for a FUTURE Planning Council read-only provider run. This PR only
// builds the lifecycle helper: NO provider process is ever started here, NO
// endpoint is wired, and the legacy /api/workflow/run path is left untouched.
type readOnlyCouncilWorkspace struct {
	// Dir is the absolute path to the empty temp directory.
	Dir string

	cleanupOnce sync.Once
	cleanupErr  error
}

// councilWorkspacePrefix identifies temp dirs created for the read-only council
// workspace so they are recognizable on disk and never confused with the repo.
const councilWorkspacePrefix = "autopus-council-readonly-"

// createReadOnlyCouncilWorkspace creates a fresh, empty temporary directory to be
// used as a read-only cwd. It intentionally does NOT copy the repo and does NOT
// run any provider. The returned cleanup func is idempotent: calling it more than
// once is safe and only the first invocation performs removal.
func createReadOnlyCouncilWorkspace(repoRoot string) (*readOnlyCouncilWorkspace, func() error, error) {
	dir, err := os.MkdirTemp("", councilWorkspacePrefix)
	if err != nil {
		return nil, nil, fmt.Errorf("create read-only council workspace: %w", err)
	}

	abs, err := filepath.Abs(dir)
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, nil, fmt.Errorf("resolve workspace path: %w", err)
	}

	// Guard: the temp workspace MUST NOT be the repo root (or nested inside it).
	// If it somehow resolved into the repo, remove it and fail rather than risk a
	// future provider ever mutating the real tree.
	if repoRoot != "" {
		if same, relErr := sameOrInside(abs, repoRoot); relErr == nil && same {
			_ = os.RemoveAll(dir)
			return nil, nil, fmt.Errorf("refusing workspace inside repo root: %s", abs)
		}
	}

	ws := &readOnlyCouncilWorkspace{Dir: abs}
	return ws, ws.cleanup, nil
}

// cleanup removes the temp directory. It is idempotent via sync.Once, so double
// cleanup is safe and returns the same (possibly nil) error.
func (w *readOnlyCouncilWorkspace) cleanup() error {
	w.cleanupOnce.Do(func() {
		if w.Dir == "" {
			return
		}
		w.cleanupErr = os.RemoveAll(w.Dir)
	})
	return w.cleanupErr
}

// sameOrInside reports whether candidate equals root or is nested within it.
// A relative-path error (e.g. different Windows drives) is surfaced so callers
// can treat "cannot relate" as "not inside".
func sameOrInside(candidate, root string) (bool, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return false, err
	}
	candAbs, err := filepath.Abs(candidate)
	if err != nil {
		return false, err
	}
	rel, err := filepath.Rel(rootAbs, candAbs)
	if err != nil {
		return false, err
	}
	if rel == "." {
		return true, nil
	}
	// A rel path that neither starts with ".." nor is absolute means the
	// candidate lives inside root.
	if !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel) {
		return true, nil
	}
	return false, nil
}
