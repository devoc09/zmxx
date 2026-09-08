package gitx

import (
	"os"
	"path/filepath"
	"testing"
)

func initRepo(t *testing.T) *Repo {
	t.Helper()
	dir := t.TempDir()
	if _, err := runGit(dir, "init", "-q", "-b", "main"); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if _, err := runGit(dir, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init"); err != nil {
		t.Fatalf("git commit: %v", err)
	}
	repo, err := FindRepo(dir)
	if err != nil {
		t.Fatalf("FindRepo: %v", err)
	}
	return repo
}

func findWorktree(worktrees []Worktree, path string) bool {
	for _, wt := range worktrees {
		if SamePath(wt.Path, path) {
			return true
		}
	}
	return false
}

func TestEnsureWorktreeCreatesAndReuses(t *testing.T) {
	repo := initRepo(t)
	path := filepath.Join(t.TempDir(), "wt")

	if err := repo.EnsureWorktree("feat/x", path, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("worktree missing after create: %v", err)
	}
	// idempotent: registered and present -> no-op
	if err := repo.EnsureWorktree("feat/x", path, ""); err != nil {
		t.Fatalf("reuse: %v", err)
	}
	wts, err := repo.ListWorktrees()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !findWorktree(wts, path) {
		t.Fatalf("worktree not registered: %#v", wts)
	}
}

func TestEnsureWorktreeRecreatesStaleEntry(t *testing.T) {
	repo := initRepo(t)
	path := filepath.Join(t.TempDir(), "wt")

	if err := repo.EnsureWorktree("feat/x", path, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	// Simulate the worktree directory being removed by hand: git still
	// lists it, but the path is gone.
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("remove dir: %v", err)
	}
	wts, err := repo.ListWorktrees()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !findWorktree(wts, path) {
		t.Fatal("expected stale registration to still be listed")
	}

	// Re-running new must prune the stale entry and recreate the worktree.
	if err := repo.EnsureWorktree("feat/x", path, ""); err != nil {
		t.Fatalf("recreate on stale: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("worktree not recreated: %v", err)
	}
	wts, err = repo.ListWorktrees()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !findWorktree(wts, path) {
		t.Fatal("worktree not registered after recreation")
	}
}
