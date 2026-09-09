// Package cli implements the zmxx command-line interface.
package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/devoc09/zmxx/internal/gitx"
	"github.com/devoc09/zmxx/internal/workspace"
	"github.com/devoc09/zmxx/internal/zmx"
)

// workspaceCmd builds the session command. nvim runs first; when it exits
// the session hands over to the user's shell so the workspace survives
// (a session whose command exits would otherwise be torn down by zmx).
func workspaceCmd() []string {
	return []string{"bash", "-c", `nvim .; exec "${SHELL:-/bin/sh}"`}
}

// Run dispatches a zmxx invocation. It returns the process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 1
	}
	switch args[0] {
	case "-h", "--help", "help":
		printUsage(stdout)
		return 0
	case "-v", "--version":
		fmt.Fprintln(stdout, "zmxx", version)
		return 0
	case "new":
		return cmdNew(args[1:], stdout, stderr)
	case "remove":
		return cmdRemove(args[1:], stdout, stderr)
	case "sessions":
		return cmdSessions(args[1:], stdout, stderr)
	case "preview":
		return cmdPreview(args[1:], stdout, stderr)
	case "switch":
		return cmdSwitch(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "zmxx: unknown command %q\n\n", args[0])
		printUsage(stderr)
		return 1
	}
}

const version = "0.1.0"

func printUsage(w io.Writer) {
	fmt.Fprint(w, `zmxx - git worktree workspaces for zmx + neovim

Usage:
  zmxx new <branch> [--base <ref>]   Create (or reuse) a worktree for a branch
                                     and open a persistent nvim session
  zmxx sessions [--json]             List all zmxx sessions
  zmxx remove <session> [--force]    Kill the session and remove its worktree
  zmxx preview <session>             Print scrollback tail (picker preview)
  zmxx switch <session>              Switch the terminal to another session

Options:
  -h, --help       Show this help
  -v, --version    Show version
`)
}

func requireRepo() (*gitx.Repo, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("cannot determine working directory: %w", err)
	}
	return gitx.FindRepo(cwd)
}

func cmdNew(args []string, stdout, stderr io.Writer) int {
	branch := ""
	base := ""
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--base":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "zmxx: --base requires a value")
				return 1
			}
			i++
			base = args[i]
		case strings.HasPrefix(args[i], "--base="):
			base = strings.TrimPrefix(args[i], "--base=")
		case strings.HasPrefix(args[i], "-"):
			fmt.Fprintf(stderr, "zmxx: unknown flag %q\n", args[i])
			return 1
		case branch == "":
			branch = args[i]
		default:
			fmt.Fprintf(stderr, "zmxx: unexpected argument %q\n", args[i])
			return 1
		}
	}
	if branch == "" {
		fmt.Fprintln(stderr, "zmxx new: branch name required")
		return 1
	}

	repo, err := requireRepo()
	if err != nil {
		fmt.Fprintf(stderr, "zmxx: %v\n", err)
		return 1
	}

	worktreePath := workspace.BranchDir(repo.ID, branch)
	sessionName := workspace.SessionName(repo.ID, branch)

	if err := repo.EnsureWorktree(branch, worktreePath, base); err != nil {
		fmt.Fprintf(stderr, "zmxx: %v\n", err)
		return 1
	}

	labels := workspace.BuildLabels(repo.ID, repo.DisplayName(), branch, worktreePath)
	labelStr := workspace.LabelString(labels)

	// 1. Make sure the session exists (nvim running, labels applied) without
	//    stealing the calling terminal.
	if err := zmx.EnsureSession(sessionName, labelStr, worktreePath, workspaceCmd()); err != nil {
		fmt.Fprintf(stderr, "zmxx: %v\n", err)
		return 1
	}
	// 2. Heal labels on a pre-existing session that may lack them.
	if err := zmx.SetLabels(sessionName, labelStr); err != nil {
		fmt.Fprintf(stderr, "zmxx: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "worktree: %s\nsession:  %s\nbranch:   %s\n", worktreePath, sessionName, branch)
	fmt.Fprintln(stdout, "attaching...")

	// 3. Attach (or, when already inside a zmx session, switch) the terminal.
	if err := zmx.Attach(sessionName); err != nil {
		fmt.Fprintf(stderr, "zmxx: %v\n", err)
		return 1
	}
	return 0
}

// isRegisteredWorktree reports whether path is one of repo's worktrees.
func isRegisteredWorktree(repo *gitx.Repo, path string) (bool, error) {
	worktrees, err := repo.ListWorktrees()
	if err != nil {
		return false, err
	}
	for _, wt := range worktrees {
		if gitx.SamePath(wt.Path, path) {
			return true, nil
		}
	}
	return false, nil
}

// removeRepo resolves the repository a session's worktree belongs to. The
// worktree itself identifies it when it exists on disk; for a directory
// that is already gone, the current repository is used when the stale entry
// is registered there.
func removeRepo(worktreePath string) (*gitx.Repo, error) {
	if repo, err := gitx.FindRepo(worktreePath); err == nil {
		return repo, nil
	}
	if repo, err := requireRepo(); err == nil {
		if registered, regErr := isRegisteredWorktree(repo, worktreePath); regErr == nil && registered {
			return repo, nil
		}
	}
	return nil, fmt.Errorf("cannot resolve the repository of %s; run `zmxx remove` from inside it", worktreePath)
}

func cmdRemove(args []string, stdout, stderr io.Writer) int {
	name := ""
	force := false
	yes := false
	for _, a := range args {
		switch a {
		case "--force":
			force = true
		case "--yes":
			yes = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "usage: zmxx remove <session> [--force] [--yes]")
			return 0
		default:
			if strings.HasPrefix(a, "-") {
				fmt.Fprintf(stderr, "zmxx: unknown flag %q\n", a)
				return 1
			}
			if name != "" {
				fmt.Fprintf(stderr, "zmxx: unexpected argument %q\n", a)
				return 1
			}
			name = a
		}
	}
	if name == "" {
		fmt.Fprintln(stderr, "zmxx remove: session name required")
		return 1
	}
	sessions, err := zmx.ZmxxSessions()
	if err != nil {
		fmt.Fprintf(stderr, "zmxx: %v\n", err)
		return 1
	}
	var target *zmx.Session
	for i := range sessions {
		if sessions[i].Name == name {
			target = &sessions[i]
			break
		}
	}
	if target == nil {
		fmt.Fprintf(stderr, "zmxx: no zmxx session %q (see `zmxx sessions`)\n", name)
		return 1
	}
	worktreePath := target.Worktree
	sessionName := target.Name
	if worktreePath == "" {
		fmt.Fprintf(stderr, "zmxx: session %q has no %s label; remove its worktree by hand\n", sessionName, workspace.LabelMarker+".worktree")
		return 1
	}
	repo, err := removeRepo(worktreePath)
	if err != nil {
		fmt.Fprintf(stderr, "zmxx: %v\n", err)
		return 1
	}

	missing := false
	if _, statErr := os.Stat(worktreePath); os.IsNotExist(statErr) {
		// Registered in git but the directory is already gone.
		missing = true
	}

	if !missing && !force {
		dirty, err := repo.IsDirty(worktreePath)
		if err != nil {
			fmt.Fprintf(stderr, "zmxx: %v\n", err)
			return 1
		}
		if dirty {
			fmt.Fprintf(stderr, "zmxx: worktree %s has uncommitted changes; use --force to remove anyway\n", worktreePath)
			return 1
		}
	}

	if !yes && !force {
		fmt.Fprintf(stdout, "kill session %q and remove worktree %s? [y/N] ", sessionName, worktreePath)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.ToLower(strings.TrimSpace(answer))
		if answer != "y" && answer != "yes" {
			fmt.Fprintln(stdout, "aborted")
			return 0
		}
	}

	if err := zmx.Kill(sessionName); err != nil {
		fmt.Fprintf(stderr, "zmxx: %v\n", err)
		return 1
	}
	if missing {
		// The directory is already gone, so there is nothing to check out;
		// drop the stale registration instead of `git worktree remove`
		// (which fails with "is not a working tree").
		if err := repo.Prune(); err != nil {
			fmt.Fprintf(stderr, "zmxx: git worktree prune failed: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "removed session %s and stale worktree entry %s\n", sessionName, worktreePath)
		return 0
	}
	if err := repo.RemoveWorktree(worktreePath, force); err != nil {
		fmt.Fprintf(stderr, "zmxx: %v\n", err)
		return 1
	}
	_ = repo.Prune()
	fmt.Fprintf(stdout, "removed session %s and worktree %s\n", sessionName, worktreePath)
	return 0
}

type sessionJSON struct {
	Name     string `json:"name"`
	Repo     string `json:"repo"`
	RepoName string `json:"reponame"`
	Branch   string `json:"branch"`
	Worktree string `json:"worktree"`
	Clients  string `json:"clients"`
	CWD      string `json:"cwd,omitempty"`
	Current  bool   `json:"current"`
}

func cmdSessions(args []string, stdout, stderr io.Writer) int {
	asJSON := false
	for _, a := range args {
		switch a {
		case "--json":
			asJSON = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "usage: zmxx sessions [--json]")
			return 0
		default:
			fmt.Fprintf(stderr, "zmxx: unknown flag %q\n", a)
			return 1
		}
	}
	sessions, err := zmx.ZmxxSessions()
	if err != nil {
		fmt.Fprintf(stderr, "zmxx: %v\n", err)
		return 1
	}
	if asJSON {
		out := make([]sessionJSON, 0, len(sessions))
		for _, s := range sessions {
			out = append(out, sessionJSON{
				Name:     s.Name,
				Repo:     s.Repo,
				RepoName: s.RepoName,
				Branch:   s.Branch,
				Worktree: s.Worktree,
				Clients:  s.Clients,
				CWD:      s.CWD,
				Current:  s.Current,
			})
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			fmt.Fprintf(stderr, "zmxx: %v\n", err)
			return 1
		}
		return 0
	}
	for _, s := range sessions {
		mark := " "
		if s.Current {
			mark = "\u2192"
		}
		fmt.Fprintf(stdout, "%s %-38s %-8s clients  %s  %s\n", mark, s.Name, s.Clients, s.RepoName, s.Branch)
	}
	return 0
}

func cmdPreview(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: zmxx preview <session>")
		return 1
	}
	name := args[0]
	current := os.Getenv("ZMX_SESSION")
	if name == current {
		fmt.Fprintf(stdout, "\u2192 this terminal is currently attached to %s\n", name)
		return 0
	}
	history, err := zmx.History(name, 400)
	if err != nil {
		fmt.Fprintf(stderr, "zmxx: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, history)
	return 0
}

func cmdSwitch(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: zmxx switch <session>")
		return 1
	}
	name := args[0]
	current := os.Getenv("ZMX_SESSION")
	if name == current {
		fmt.Fprintf(stderr, "zmxx: already attached to session %s\n", name)
		return 1
	}
	if !zmx.SessionExists(name) {
		fmt.Fprintf(stderr, "zmxx: session %q does not exist (it may have ended); reopen its workspace with `zmxx new <branch>`\n", name)
		return 1
	}
	if err := zmx.Attach(name); err != nil {
		fmt.Fprintf(stderr, "zmxx: %v\n", err)
		return 1
	}
	return 0
}
