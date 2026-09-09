package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/devoc09/zmxx/internal/gitx"
	"github.com/devoc09/zmxx/internal/workspace"
	"github.com/devoc09/zmxx/internal/zmx"
)

// initRepo creates a git repository with one empty commit in dir.
func initRepo(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init")
}

func TestRemoveSessionRemovesWorktree(t *testing.T) {
	if _, err := exec.LookPath("zmx"); err != nil {
		t.Skip("zmx binary required")
	}
	// Isolate zmx state (sockets, sessions) from the developer's machine.
	// /tmp keeps the socket path short: zmx bounds the session name by the
	// remaining room under ZMX_DIR, and $TMPDIR on macOS is too long.
	zmxDir, err := os.MkdirTemp("/tmp", "zmxx-test-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(zmxDir) })
	t.Setenv("ZMX_DIR", zmxDir)
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	repoA := t.TempDir()
	repoB := t.TempDir()
	initRepo(t, repoA)
	initRepo(t, repoB)

	a, err := gitx.FindRepo(repoA)
	if err != nil {
		t.Fatalf("FindRepo: %v", err)
	}
	path := workspace.BranchDir(a.ID, "feat-1")
	cmd := exec.Command("git", "-C", repoA, "worktree", "add", "--quiet", "-b", "feat-1", path, "HEAD")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("worktree add: %v: %s", err, out)
	}

	sessionName := workspace.SessionName(a.ID, "feat-1")
	labelStr := workspace.LabelString(workspace.BuildLabels(a.ID, a.DisplayName(), "feat-1", path))
	if err := zmx.EnsureSession(sessionName, labelStr, path, []string{"bash", "-c", "exec sleep 30"}); err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}

	// Remove by session name from the unrelated repository repoB.
	t.Chdir(repoB)
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"remove", sessionName, "--yes"}, &stdout, &stderr); code != 0 {
		t.Fatalf("remove failed (stdout=%q stderr=%q)", stdout.String(), stderr.String())
	}
	if zmx.SessionExists(sessionName) {
		t.Fatal("session still exists")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("worktree still on disk")
	}
	wts, err := a.ListWorktrees()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, wt := range wts {
		if filepath.Clean(wt.Path) == filepath.Clean(path) {
			t.Fatal("worktree entry still registered")
		}
	}
}
