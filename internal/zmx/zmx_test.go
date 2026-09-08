package zmx

import (
	"testing"
)

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
