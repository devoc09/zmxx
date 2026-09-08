package gitx

import (
	"reflect"
	"testing"
)

func TestParseWorktreesPorcelain(t *testing.T) {
	in := "worktree /Users/me/code/repo\x00HEAD 1234abcd\x00bare\x00\x00" +
		"worktree /Users/me/code/repo\x00HEAD 1234abcd\x00branch refs/heads/main\x00\x00" +
		"worktree /Users/me/.local/share/zmxx/worktrees/abc123/feature-foo-1a2b\x00" +
		"HEAD 5678efab\x00branch refs/heads/feature/foo\x00\x00"
	got := parseWorktrees([]byte(in))
	want := []Worktree{
		{Path: "/Users/me/code/repo", Bare: true},
		{Path: "/Users/me/code/repo", Branch: "main"},
		{Path: "/Users/me/.local/share/zmxx/worktrees/abc123/feature-foo-1a2b", Branch: "feature/foo"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseWorktrees mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestParseWorktreesDetached(t *testing.T) {
	in := "worktree /x\x00HEAD 1234abcd\x00detached\x00\x00"
	got := parseWorktrees([]byte(in))
	if len(got) != 1 || !got[0].Detached || got[0].Branch != "" {
		t.Fatalf("unexpected: %#v", got)
	}
}

func TestParseWorktreesEmpty(t *testing.T) {
	if got := parseWorktrees(nil); len(got) != 0 {
		t.Fatalf("expected no worktrees, got %#v", got)
	}
}
