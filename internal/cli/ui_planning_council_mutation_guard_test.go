package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// initCouncilTestRepo creates a throwaway git repo with a single committed file
// so mutation snapshots have a clean baseline. It never touches the user's repo.
func initCouncilTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-c", "gc.auto=0"}, args...)...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	runGit("init")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(dir, "seed.txt"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "-A")
	runGit("commit", "-m", "seed")
	return dir
}

func TestMutationGuard_CleanPath(t *testing.T) {
	dir := initCouncilTestRepo(t)
	before, err := snapshotCouncilMutations(dir)
	if err != nil {
		t.Fatalf("before snapshot: %v", err)
	}
	after, err := snapshotCouncilMutations(dir)
	if err != nil {
		t.Fatalf("after snapshot: %v", err)
	}
	if res := diffCouncilMutations(before, after); !res.Clean {
		t.Fatalf("expected clean, got violations %v", res.Violations)
	}
}

func TestMutationGuard_DirtyModifiedPath(t *testing.T) {
	dir := initCouncilTestRepo(t)
	before, _ := snapshotCouncilMutations(dir)
	if err := os.WriteFile(filepath.Join(dir, "seed.txt"), []byte("mutated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	after, _ := snapshotCouncilMutations(dir)
	res := diffCouncilMutations(before, after)
	if res.Clean {
		t.Fatalf("expected violation for modified file")
	}
	if len(res.Violations) != 1 || res.Violations[0] != "seed.txt" {
		t.Fatalf("unexpected violations: %v", res.Violations)
	}
}

func TestMutationGuard_UntrackedPath(t *testing.T) {
	dir := initCouncilTestRepo(t)
	before, _ := snapshotCouncilMutations(dir)
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	after, _ := snapshotCouncilMutations(dir)
	res := diffCouncilMutations(before, after)
	if res.Clean || len(res.Violations) != 1 || res.Violations[0] != "new.txt" {
		t.Fatalf("expected untracked violation, got clean=%v violations=%v", res.Clean, res.Violations)
	}
}

func TestMutationGuard_StateJSONException(t *testing.T) {
	dir := initCouncilTestRepo(t)
	if err := os.MkdirAll(filepath.Join(dir, ".autopus", "workflows"), 0o755); err != nil {
		t.Fatal(err)
	}
	before, _ := snapshotCouncilMutations(dir)
	statePath := filepath.Join(dir, ".autopus", "workflows", "state.json")
	if err := os.WriteFile(statePath, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	after, _ := snapshotCouncilMutations(dir)
	res := diffCouncilMutations(before, after)
	if !res.Clean {
		t.Fatalf("state.json change must be exempt, violations=%v", res.Violations)
	}
	if len(res.ExemptedChanges) != 1 || res.ExemptedChanges[0] != councilStateExceptionPath {
		t.Fatalf("expected state.json exemption recorded, got %v", res.ExemptedChanges)
	}
}

func TestMutationGuard_NonExceptionAlongsideStateIsViolation(t *testing.T) {
	dir := initCouncilTestRepo(t)
	if err := os.MkdirAll(filepath.Join(dir, ".autopus", "workflows"), 0o755); err != nil {
		t.Fatal(err)
	}
	before, _ := snapshotCouncilMutations(dir)
	if err := os.WriteFile(filepath.Join(dir, ".autopus", "workflows", "state.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "leak.txt"), []byte("bad\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	after, _ := snapshotCouncilMutations(dir)
	res := diffCouncilMutations(before, after)
	if res.Clean {
		t.Fatalf("expected violation when a non-exempt file also changed")
	}
	if len(res.Violations) != 1 || res.Violations[0] != "leak.txt" {
		t.Fatalf("expected leak.txt violation, got %v", res.Violations)
	}
	if len(res.ExemptedChanges) != 1 || res.ExemptedChanges[0] != councilStateExceptionPath {
		t.Fatalf("expected state.json still exempt, got %v", res.ExemptedChanges)
	}
}
