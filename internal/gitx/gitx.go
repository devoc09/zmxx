// Package gitx wraps the subset of git commands zmxx needs.
package gitx

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Worktree describes one entry from `git worktree list`.
type Worktree struct {
	Path     string
	Branch   string // short branch name; "" when detached
	Detached bool
	Bare     bool
}

// Repo is the repository (shared git dir) zmxx is operating on, together
// with the worktree the user invoked zmxx from.
type Repo struct {
	Root      string // absolute top-level path of the current worktree
	CommonDir string // absolute path of the common git dir
	OriginURL string
	ID        string // stable identifier (12 hex chars)
}

// FindRepo locates the git repository containing cwd.
func FindRepo(cwd string) (*Repo, error) {
	root, err := runGit(cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("not a git repository (or unable to find one): %w", err)
	}
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("git returned an empty repository root")
	}

	common, err := runGit(cwd, "rev-parse", "--git-common-dir")
	if err != nil {
		return nil, fmt.Errorf("cannot resolve git common dir: %w", err)
	}
	common = strings.TrimSpace(common)
	if !filepath.IsAbs(common) {
		common = filepath.Join(root, common)
	}
	if abs, err := filepath.Abs(common); err == nil {
		common = abs
	}

	origin, _ := runGit(cwd, "config", "--get", "remote.origin.url")
	origin = strings.TrimSpace(origin)

	idSrc := origin
	if idSrc == "" {
		idSrc = common
	}
	sum := sha256.Sum256([]byte(idSrc))

	return &Repo{
		Root:      root,
		CommonDir: common,
		OriginURL: origin,
		ID:        hex.EncodeToString(sum[:])[:12],
	}, nil
}

// DisplayName returns a human-friendly repository name for pickers.
func (r *Repo) DisplayName() string {
	if r.OriginURL != "" {
		u := strings.TrimSuffix(r.OriginURL, ".git")
		if i := strings.LastIndexAny(u, "/:"); i >= 0 {
			u = u[i+1:]
		}
		if u != "" {
			return u
		}
	}
	return filepath.Base(r.Root)
}

// BranchExists reports whether the branch exists locally.
func (r *Repo) BranchExists(branch string) bool {
	cmd := gitCmd(r.Root, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return cmd.Run() == nil
}

// RemoteBranchExists reports whether a remote-tracking branch exists.
func (r *Repo) RemoteBranchExists(branch string) bool {
	cmd := gitCmd(r.Root, "show-ref", "--verify", "--quiet", "refs/remotes/origin/"+branch)
	return cmd.Run() == nil
}

// DefaultBranch returns the upstream default branch when it can be
// determined, otherwise "HEAD".
func (r *Repo) DefaultBranch() string {
	out, err := runGit(r.Root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
	if err == nil {
		name := strings.TrimSpace(out)
		name = strings.TrimPrefix(name, "origin/")
		if name != "" {
			return name
		}
	}
	return "HEAD"
}

// ListWorktrees parses `git worktree list --porcelain -z`.
func (r *Repo) ListWorktrees() ([]Worktree, error) {
	cmd := gitCmd(r.Root, "worktree", "list", "--porcelain", "-z")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}
	return parseWorktrees(out), nil
}

// parseWorktrees parses `git worktree list --porcelain -z` output.
func parseWorktrees(out []byte) []Worktree {
	var result []Worktree
	var cur *Worktree
	for _, rec := range strings.Split(string(out), "\x00") {
		if rec == "" {
			if cur != nil {
				result = append(result, *cur)
				cur = nil
			}
			continue
		}
		key, val, ok := strings.Cut(rec, " ")
		if !ok {
			key, val = rec, ""
		}
		switch key {
		case "worktree":
			cur = &Worktree{Path: val}
		case "branch":
			if cur != nil {
				cur.Branch = strings.TrimPrefix(val, "refs/heads/")
			}
		case "detached":
			if cur != nil {
				cur.Detached = true
			}
		case "bare":
			if cur != nil {
				cur.Bare = true
			}
		}
	}
	return result
}

// EnsureWorktree makes sure `branch` is checked out at path, creating the
// branch from baseRef when it does not exist yet.
func (r *Repo) EnsureWorktree(branch, path, baseRef string) error {
	existing, err := r.ListWorktrees()
	if err != nil {
		return err
	}
	for _, wt := range existing {
		if filepath.Clean(wt.Path) == filepath.Clean(path) {
			return nil // already registered
		}
	}

	if info, err := os.Stat(path); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("path exists and is not a directory: %s", path)
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return fmt.Errorf("cannot inspect existing path %s: %w", path, err)
		}
		if len(entries) > 0 {
			return fmt.Errorf("path exists but is not a managed worktree (refusing to overwrite): %s", path)
		}
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("cannot clear empty path %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("cannot stat %s: %w", path, err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("cannot create worktree parent dir: %w", err)
	}

	var args []string
	switch {
	case r.BranchExists(branch):
		args = []string{"worktree", "add", "--quiet", path, branch}
	case r.RemoteBranchExists(branch):
		// Local branch does not exist but origin/<branch> does: create the
		// local branch from the remote-tracking ref (like git checkout).
		args = []string{"worktree", "add", "--quiet", "-b", branch, path, "origin/" + branch}
	default:
		base := baseRef
		if base == "" {
			base = r.DefaultBranch()
		}
		args = []string{"worktree", "add", "--quiet", "-b", branch, path, base}
	}
	cmd := gitCmd(r.Root, args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("git worktree add failed: %s", msg)
	}
	return nil
}

// IsDirty reports whether the worktree has tracked modifications or
// untracked (non-ignored) files.
func (r *Repo) IsDirty(worktreePath string) (bool, error) {
	out, err := runGit(worktreePath, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("git status failed: %w", err)
	}
	return strings.TrimSpace(out) != "", nil
}

// RemoveWorktree removes the worktree. When force is true, `--force` is
// passed to git worktree remove.
func (r *Repo) RemoveWorktree(worktreePath string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, worktreePath)
	cmd := gitCmd(r.Root, args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("git worktree remove failed: %s", msg)
	}
	return nil
}

// Prune cleans up stale worktree administrative files.
func (r *Repo) Prune() error {
	cmd := gitCmd(r.Root, "worktree", "prune")
	return cmd.Run()
}

func gitCmd(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	return cmd
}

func runGit(dir string, args ...string) (string, error) {
	cmd := gitCmd(dir, args...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s", msg)
	}
	return stdout.String(), nil
}
