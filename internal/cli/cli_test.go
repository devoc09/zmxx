package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devoc09/zmxx/internal/gitx"
	"github.com/devoc09/zmxx/internal/workspace"
)

func TestRemoveStaleWorktreeEntry(t *testing.T) {
	if _, err := exec.LookPath("zmx"); err != nil {
		t.Skip("zmx binary required")
	}
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init")

	t.Chdir(dir)
	repo, err := gitx.FindRepo(dir)
	if err != nil {
		t.Fatalf("FindRepo: %v", err)
	}
	path := workspace.BranchDir(repo.ID, "feat-1")
	run("worktree", "add", "--quiet", "-b", "feat-1", path, "HEAD")

	// Simulate the directory being removed by hand.
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("remove dir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"remove", "feat-1", "--yes"}, &stdout, &stderr); code != 0 {
		t.Fatalf("remove failed (stdout=%q stderr=%q)", stdout.String(), stderr.String())
	}

	wts, err := repo.ListWorktrees()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, wt := range wts {
		if filepath.Clean(wt.Path) == filepath.Clean(path) {
			t.Fatal("stale worktree entry still present")
		}
	}
	if !strings.Contains(stdout.String(), "stale worktree entry") {
		t.Fatalf("unexpected output: %q", stdout.String())
	}
}

func TestListShowsMissingWorktree(t *testing.T) {
	if _, err := exec.LookPath("zmx"); err != nil {
		t.Skip("zmx binary required")
	}
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init")

	t.Chdir(dir)
	repo, err := gitx.FindRepo(dir)
	if err != nil {
		t.Fatalf("FindRepo: %v", err)
	}
	path := workspace.BranchDir(repo.ID, "feat-1")
	run("worktree", "add", "--quiet", "-b", "feat-1", path, "HEAD")
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("remove dir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"list"}, &stdout, &stderr); code != 0 {
		t.Fatalf("list failed (stderr=%q)", stderr.String())
	}
	if !strings.Contains(stdout.String(), "(missing)") {
		t.Fatalf("missing marker not shown: %q", stdout.String())
	}
	if strings.Contains(stderr.String(), "git status") {
		t.Fatalf("list must not fail on git status: %q", stderr.String())
	}
}
