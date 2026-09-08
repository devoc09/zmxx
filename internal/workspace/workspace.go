// Package workspace maps git branches to managed worktree paths and to
// zmx session metadata (labels and session names).
package workspace

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/devoc09/zmxx/internal/gitx"
)

const (
	// DataDirName is the directory zmxx owns under XDG_DATA_HOME.
	DataDirName = "zmxx"
	// LabelMarker is the label that marks a session as managed by zmxx.
	LabelMarker = "zmxx"
)

// DataDir returns $XDG_DATA_HOME/zmxx or ~/.local/share/zmxx.
func DataDir() string {
	if x := os.Getenv("XDG_DATA_HOME"); x != "" {
		return filepath.Join(x, DataDirName)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), DataDirName)
	}
	return filepath.Join(home, ".local", "share", DataDirName)
}

// WorktreesDir is where all managed worktrees live.
func WorktreesDir() string { return filepath.Join(DataDir(), "worktrees") }

// RepoDir is the per-repository worktree directory.
func RepoDir(repoID string) string { return filepath.Join(WorktreesDir(), repoID) }

// BranchDir is the deterministic worktree path for a branch.
func BranchDir(repoID, branch string) string {
	return filepath.Join(RepoDir(repoID), branchSlug(branch)+"-"+hashHex(branch, 8))
}

// SessionName is the deterministic zmx session name for a branch.
// It stays short so the unix socket path (which is
// socket_dir + "/" + name) fits the platform limit.
func SessionName(repoID, branch string) string {
	return "zmxx-" + repoID + "-" + hashHex(branch, 12)
}

// IsManagedPath reports whether p lives under the zmxx worktrees dir.
// Both sides are canonicalized because git reports resolved paths
// (e.g. /private/var/... on macOS).
func IsManagedPath(p string) bool {
	base := gitx.CanonicalPath(WorktreesDir()) + string(filepath.Separator)
	return strings.HasPrefix(gitx.CanonicalPath(p), base)
}

// BuildLabels returns the labels applied to a zmx session so the picker can
// find every zmxx session and reconstruct its metadata.
func BuildLabels(repoID, reponame, branch, worktreePath string) map[string]string {
	return map[string]string{
		"zmxx":          "1",
		"zmxx.repo":     repoID,
		"zmxx.reponame": Encode(reponame),
		"zmxx.branch":   Encode(branch),
		"zmxx.worktree": Encode(worktreePath),
	}
}

// LabelString renders labels as the space-separated "key=value ..." form
// zmx's --labels / set commands expect. Keys are sorted for determinism.
func LabelString(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+labels[k])
	}
	return strings.Join(parts, " ")
}

// Encode encodes a value for a zmx label (labels only allow [A-Za-z0-9-_.]).
func Encode(s string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}

// Decode reverses Encode, returning s itself when it is not valid base64.
func Decode(s string) string {
	if b, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return string(b)
	}
	return s
}

func branchSlug(branch string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range branch {
		alnum := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if alnum || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	s := strings.Trim(b.String(), "-_.")
	if len(s) > 40 {
		s = s[:40]
	}
	if s == "" {
		s = "branch"
	}
	return s
}

func hashHex(s string, n int) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])[:n]
}
