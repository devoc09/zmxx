// Package zmx wraps the zmx CLI for session management.
package zmx

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/devoc09/zmxx/internal/workspace"
)

// Session is one entry from `zmx list`, with zmxx metadata decoded.
type Session struct {
	Name     string
	PID      string
	Clients  string
	CWD      string // raw OSC7 value from zmx list
	CWDPath  string // decoded path, when the cwd is local
	Cmd      string
	Labels   map[string]string
	Current  bool // the session this process is attached to
	Repo     string
	RepoName string
	Branch   string
	Worktree string
}

// Zmxx reports whether the session carries the zmxx marker label.
func (s Session) Zmxx() bool { return s.Labels["zmxx"] == "1" }

func binary() (string, error) {
	return exec.LookPath("zmx")
}

// envPrefixCleared returns the current environment with ZMX_SESSION_PREFIX
// cleared so full session names are never re-prefixed.
func envPrefixCleared() []string {
	return setEnv(os.Environ(), "ZMX_SESSION_PREFIX", "")
}

// envWithoutSession returns the environment with ZMX_SESSION removed, used
// when creating sessions without stealing an attached terminal.
func envWithoutSession() []string {
	return setEnv(envPrefixCleared(), "ZMX_SESSION", "")
}

func setEnv(env []string, key, val string) []string {
	prefix := key + "="
	var out []string
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			out = append(out, e)
		}
	}
	if val != "" {
		out = append(out, key+"="+val)
	}
	return out
}

func run(dir string, env []string, stdin io.Reader, args ...string) ([]byte, []byte, error) {
	bin, err := binary()
	if err != nil {
		return nil, nil, fmt.Errorf("zmx not found in PATH: %w", err)
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = env
	if stdin != nil {
		cmd.Stdin = stdin
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.Bytes(), stderr.Bytes(), err
	}
	return stdout.Bytes(), stderr.Bytes(), nil
}

// List returns all sessions with their labels.
func List() ([]Session, error) {
	out, errOut, err := run("", envPrefixCleared(), nil, "list")
	if err != nil {
		if len(out) == 0 && len(errOut) > 0 {
			// zmx prints "no sessions found" to stderr; treat as empty.
			if strings.Contains(strings.ToLower(string(errOut)), "no sessions") {
				return nil, nil
			}
		}
		return nil, fmt.Errorf("zmx list: %s", strings.TrimSpace(string(errOut)+string(out)))
	}
	current := os.Getenv("ZMX_SESSION")
	var sessions []Session
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		s := parseSessionLine(line)
		s.Current = current != "" && s.Name == current
		sessions = append(sessions, s)
	}
	return sessions, nil
}

// ZmxxSessions returns every session managed by zmxx, across repositories.
func ZmxxSessions() ([]Session, error) {
	all, err := List()
	if err != nil {
		return nil, err
	}
	var out []Session
	for _, s := range all {
		if s.Zmxx() {
			out = append(out, s)
		}
	}
	return out, nil
}

// SessionExists reports whether a session with the exact name is running.
func SessionExists(name string) bool {
	out, _, err := run("", envPrefixCleared(), nil, "list", "--short")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == name {
			return true
		}
	}
	return false
}

// EnsureSession creates the session (with nvim running in the worktree) if
// it does not exist. It never steals the calling terminal: ZMX_SESSION is
// stripped and stdin is an immediate EOF, so the client detaches right away.
func EnsureSession(name, labelStr, worktreePath string, cmd []string) error {
	args := []string{"attach", "--labels", labelStr, name}
	args = append(args, cmd...)
	_, errOut, err := run(worktreePath, envWithoutSession(), strings.NewReader(""), args...)
	if err != nil {
		return fmt.Errorf("zmx attach (create): %s", strings.TrimSpace(string(errOut)))
	}
	return nil
}

// SetLabels applies (or heals) labels on an existing session.
func SetLabels(name, labelStr string) error {
	_, errOut, err := run("", envPrefixCleared(), nil, "set", name, labelStr)
	if err != nil {
		return fmt.Errorf("zmx set: %s", strings.TrimSpace(string(errOut)))
	}
	return nil
}

// Kill terminates the session and all attached clients. A session that is
// already gone is treated as success.
func Kill(name string) error {
	_, errOut, err := run("", envPrefixCleared(), nil, "kill", name)
	if err != nil {
		msg := strings.TrimSpace(string(errOut))
		if strings.Contains(msg, "SessionNotFound") || strings.Contains(msg, "not found") {
			return nil
		}
		return fmt.Errorf("zmx kill: %s", msg)
	}
	return nil
}

// vtNoise matches ANSI CSI sequences other than SGR (color) codes, which
// zmx history --vt emits alongside the colors (cursor motion, mode resets).
var vtNoise = regexp.MustCompile("\x1b\\[[0-9;:?<]*[^0-9;:?<m]")

// cleanVT keeps SGR color sequences and drops other control sequences plus
// carriage returns, which would garble pagers such as the fzf preview pane.
func cleanVT(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	return vtNoise.ReplaceAllString(s, "")
}

// History returns the ANSI-colored scrollback tail of a session, rendered
// through zmx's virtual terminal (--vt) — the same 256-color SGR stream
// attached clients display.
func History(name string, maxLines int) (string, error) {
	out, errOut, err := run("", envPrefixCleared(), nil, "history", name, "--vt")
	if err != nil {
		return "", fmt.Errorf("zmx history: %s", strings.TrimSpace(string(errOut)))
	}
	lines := strings.Split(strings.TrimRight(cleanVT(string(out)), "\n"), "\n")
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return strings.Join(lines, "\n"), nil
}

// Attach attaches (or, when ZMX_SESSION is set, switches) the current
// terminal to the session. The child takes over stdin/stdout/stderr.
func Attach(name string) error {
	bin, err := binary()
	if err != nil {
		return fmt.Errorf("zmx not found in PATH: %w", err)
	}
	cmd := exec.Command(bin, "attach", name)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Keep ZMX_SESSION: zmx then switches the terminal's client instead of
	// nesting. Only clear the prefix so full names are used verbatim.
	cmd.Env = envPrefixCleared()
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 0 {
			return nil
		}
		return fmt.Errorf("zmx attach: %w", err)
	}
	return nil
}

func parseSessionLine(line string) Session {
	line = strings.TrimPrefix(line, "\u2192 ") // "→ "
	line = strings.TrimPrefix(line, "  ")
	s := Session{Labels: map[string]string{}}
	for _, field := range strings.Split(line, "\t") {
		key, val, ok := strings.Cut(field, "=")
		if !ok {
			continue
		}
		switch key {
		case "name":
			s.Name = val
		case "pid":
			s.PID = val
		case "clients":
			s.Clients = val
		case "cwd":
			s.CWD = val
			s.CWDPath = decodeCwd(val)
		case "cmd":
			s.Cmd = val
		default:
			s.Labels[key] = val
		}
	}
	if s.Zmxx() {
		s.Repo = s.Labels["zmxx.repo"]
		s.RepoName = workspace.Decode(s.Labels["zmxx.reponame"])
		s.Branch = workspace.Decode(s.Labels["zmxx.branch"])
		s.Worktree = workspace.Decode(s.Labels["zmxx.worktree"])
	}
	return s
}

// decodeCwd turns an OSC7 cwd (file://host/path or kitty-shell-cwd://...) into
// a plain path when it is a local file URL; otherwise it returns "".
func decodeCwd(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Path == "" {
		return ""
	}
	if u.Scheme != "file" && u.Scheme != "kitty-shell-cwd" {
		return ""
	}
	p, err := url.PathUnescape(u.Path)
	if err != nil {
		return u.Path
	}
	return p
}
