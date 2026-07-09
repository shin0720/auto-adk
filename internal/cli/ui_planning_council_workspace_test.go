package cli

import (
	"os"
	"testing"
)

func TestCreateReadOnlyCouncilWorkspace_CreatesEmptyDir(t *testing.T) {
	repoRoot := t.TempDir()
	ws, cleanup, err := createReadOnlyCouncilWorkspace(repoRoot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = cleanup() }()

	info, err := os.Stat(ws.Dir)
	if err != nil {
		t.Fatalf("workspace dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("workspace is not a directory: %s", ws.Dir)
	}
	entries, err := os.ReadDir(ws.Dir)
	if err != nil {
		t.Fatalf("read workspace: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("workspace should be empty, found %d entries", len(entries))
	}
}

func TestCreateReadOnlyCouncilWorkspace_DiffersFromRepoRoot(t *testing.T) {
	repoRoot := t.TempDir()
	ws, cleanup, err := createReadOnlyCouncilWorkspace(repoRoot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = cleanup() }()

	if ws.Dir == repoRoot {
		t.Fatalf("workspace must differ from repo root")
	}
	if same, _ := sameOrInside(ws.Dir, repoRoot); same {
		t.Fatalf("workspace must not be inside repo root: %s", ws.Dir)
	}
}

func TestReadOnlyCouncilWorkspace_CleanupRemovesDir(t *testing.T) {
	ws, cleanup, err := createReadOnlyCouncilWorkspace("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := cleanup(); err != nil {
		t.Fatalf("cleanup error: %v", err)
	}
	if _, err := os.Stat(ws.Dir); !os.IsNotExist(err) {
		t.Fatalf("expected dir removed, stat err = %v", err)
	}
}

func TestReadOnlyCouncilWorkspace_DoubleCleanupSafe(t *testing.T) {
	_, cleanup, err := createReadOnlyCouncilWorkspace("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := cleanup(); err != nil {
		t.Fatalf("first cleanup error: %v", err)
	}
	if err := cleanup(); err != nil {
		t.Fatalf("second cleanup must be safe, got: %v", err)
	}
}
