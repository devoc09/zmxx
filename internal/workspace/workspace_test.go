package workspace

import (
	"strings"
	"testing"
)

func TestSessionNameFormat(t *testing.T) {
	name := SessionName("abc123def456", "feature/foo")
	if !strings.HasPrefix(name, "zmxx-abc123def456-") {
		t.Fatalf("unexpected session name: %s", name)
	}
	// must be short enough for a unix socket path
	if len(name) > 40 {
		t.Fatalf("session name too long: %d", len(name))
	}
	// deterministic
	if name != SessionName("abc123def456", "feature/foo") {
		t.Fatal("session name is not deterministic")
	}
	// different branches get different names
	if name == SessionName("abc123def456", "feature/bar") {
		t.Fatal("distinct branches collided")
	}
}

func TestBranchDirSlug(t *testing.T) {
	dir := BranchDir("abc123def456", "feature/foo-bar")
	if !strings.Contains(dir, "/feature-foo-bar-") {
		t.Fatalf("unexpected dir: %s", dir)
	}
	// slashes become dashes, no double dashes
	dir2 := BranchDir("abc123def456", "a//b")
	if strings.Contains(dir2, "//") {
		t.Fatalf("unexpected dir: %s", dir2)
	}
	// long branch names are truncated but stay unique via hash
	long := strings.Repeat("x", 200)
	if len(BranchDir("abc123def456", long)) != len(BranchDir("abc123def456", long+"y")) {
		t.Fatal("truncated slugs should stay same length")
	}
}

func TestLabelRoundTrip(t *testing.T) {
	labels := BuildLabels("abc123def456", "my-repo", "feature/foo", "/Users/me/.local/share/zmxx/worktrees/abc123/feature-foo-1a2b")
	s := LabelString(labels)
	for _, want := range []string{"zmxx=1", "zmxx.repo=abc123def456"} {
		if !strings.Contains(s, want) {
			t.Fatalf("label string %q missing %q", s, want)
		}
	}
	if Decode(labels["zmxx.branch"]) != "feature/foo" {
		t.Fatalf("branch did not round-trip: %q", labels["zmxx.branch"])
	}
	if Decode(labels["zmxx.worktree"]) != "/Users/me/.local/share/zmxx/worktrees/abc123/feature-foo-1a2b" {
		t.Fatalf("worktree did not round-trip")
	}
	if Decode("not-base64!!") != "not-base64!!" {
		t.Fatal("Decode should fall back to input")
	}
}

func TestLabelCharacters(t *testing.T) {
	// values may contain any bytes; the encoded form must stay [A-Za-z0-9-_.]
	labels := BuildLabels("id", "名前", "branch/with space", "/tmp/a b")
	for k, v := range labels {
		if k == "zmxx" || k == "zmxx.repo" {
			continue
		}
		for _, r := range v {
			ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.'
			if !ok {
				t.Fatalf("label %s=%q contains disallowed char %q", k, v, r)
			}
		}
	}
}
