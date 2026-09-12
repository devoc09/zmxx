package zmx

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestCleanVT(t *testing.T) {
	in := "\x1b[0m\x1b[38;5;1mRED\x1b[0m\r\n\x1b[1m\x1b[48;5;4m BG \x1b[0m\r\x1b[5;1H\x1b[?25l"
	want := "\x1b[0m\x1b[38;5;1mRED\x1b[0m\n\x1b[1m\x1b[48;5;4m BG \x1b[0m"
	if got := cleanVT(in); got != want {
		t.Fatalf("cleanVT = %q, want %q", got, want)
	}
}

func TestHistoryVTColors(t *testing.T) {
	if _, err := exec.LookPath("zmx"); err != nil {
		t.Skip("zmx binary required")
	}
	zmxDir, err := os.MkdirTemp("/tmp", "zmxx-test-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(zmxDir) }) //nolint:gosec,errcheck // best-effort cleanup
	t.Setenv("ZMX_DIR", zmxDir)
	t.Setenv("ZMX_SESSION", "")
	t.Setenv("ZMX_SESSION_PREFIX", "")

	name := "vt-colors"
	if err := EnsureSession(name, "zmxx=1", "", []string{"bash", "-c", `printf '\033[31mRED\033[0m plain\n'; exec sleep 30`}); err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}
	t.Cleanup(func() { _ = Kill(name) })

	var got string
	deadline := time.Now().Add(5 * time.Second)
	for {
		got, err = History(name, 400)
		if err == nil && strings.Contains(got, "RED") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("history never showed RED (last err=%v, out=%q)", err, got)
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !strings.Contains(got, "\x1b[38;5;1mRED") {
		t.Fatalf("expected colored RED, got %q", got)
	}
	if strings.Contains(got, "\r") || vtNoise.MatchString(got) {
		t.Fatalf("expected control sequences stripped, got %q", got)
	}
}

func TestParseSessionLineWithLabels(t *testing.T) {
	line := "  name=zmxx-abc123def456-0123456789ab\tpid=1234\tclients=1\tcreated=1788887661\tcwd=file://freyja.local/Users/me/.local/share/zmxx/worktrees/abc123/feature-foo-1a2b\tcmd=nvim .\tzmxx=1\tzmxx.repo=abc123def456\tzmxx.reponame=bXktcmVwbw\tzmxx.branch=ZmVhdHVyZS9mb28\tzmxx.worktree=L1VzZXJzL21lL2xvY2FsL3NoYXJlL3pteHgvd29ya3RyZWVzL2FiYzEyMy9mZWF0dXJlLWZvby0xYTJi"
	s := parseSessionLine(line)
	if s.Name != "zmxx-abc123def456-0123456789ab" {
		t.Fatalf("name: %q", s.Name)
	}
	if s.Clients != "1" {
		t.Fatalf("clients: %q", s.Clients)
	}
	if !s.Zmxx() {
		t.Fatal("expected zmxx marker")
	}
	if s.Branch != "feature/foo" {
		t.Fatalf("branch: %q", s.Branch)
	}
	if s.Worktree != "/Users/me/local/share/zmxx/worktrees/abc123/feature-foo-1a2b" {
		t.Fatalf("worktree: %q", s.Worktree)
	}
	if s.CWDPath != "/Users/me/.local/share/zmxx/worktrees/abc123/feature-foo-1a2b" {
		t.Fatalf("cwd path: %q", s.CWDPath)
	}
}

func TestParseSessionLineCurrentArrow(t *testing.T) {
	line := "→ name=dev\tpid=1\tclients=2\tcreated=1"
	s := parseSessionLine(line)
	if s.Name != "dev" {
		t.Fatalf("name: %q", s.Name)
	}
	if s.Clients != "2" {
		t.Fatalf("clients: %q", s.Clients)
	}
}

func TestParseSessionLineNonZmxx(t *testing.T) {
	line := "  name=dev\tpid=1\tclients=0\tcreated=1"
	s := parseSessionLine(line)
	if s.Zmxx() {
		t.Fatal("plain session should not be zmxx")
	}
}

func TestDecodeCwd(t *testing.T) {
	cases := map[string]string{
		"file://freyja.local/Users/me/proj":  "/Users/me/proj",
		"file://host/Users/me/my%20project":  "/Users/me/my project",
		"kitty-shell-cwd://host/private/tmp": "/private/tmp",
		"http://example.com/not-a-cwd":       "",
		"":                                   "",
	}
	for in, want := range cases {
		if got := decodeCwd(in); got != want {
			t.Fatalf("decodeCwd(%q) = %q, want %q", in, got, want)
		}
	}
}
