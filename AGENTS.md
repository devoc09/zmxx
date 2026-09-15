# Agent Development Guide

A file for [guiding coding agents](https://agents.md/).

## Commands

- **Toolchain:** `mise install` (installs `go` and `golangci-lint` pinned in
  `.mise.toml`; CI uses [mise-action](https://github.com/jdx/mise-action))
- **Build (CLI):** `go build -o ~/.local/bin/zmxx ./cmd/zmxx`
- **Test (Go):** `go test ./...`
- **Test filter (Go):** `go test ./internal/<pkg> -run <test name>`
- **Vet:** `go vet ./...`
- **Lint:** `golangci-lint run` (config: `.golangci.yml`; includes `gofmt`
  check — format with `go fmt ./...`)
- **Test (picker):** `nvim --headless -u NONE -l tests/picker.lua`
  - Requires the `zmx` binary. Runs real zmx sessions in a private socket
    directory (`ZMX_DIR`), stubs fzf-lua, and drives the picker callbacks
    directly.
- Go tests that shell out to `zmx` (e.g. `internal/cli`) skip themselves when
  the `zmx` binary is absent; the rest are pure unit tests.

## Directory Structure

- CLI entry point: `cmd/zmxx`
- CLI internals: `internal/`
  - `internal/cli`: arg parsing and output formatting; composes the packages below
  - `internal/gitx`: git plumbing only (`worktree list/add/remove/prune`, dirty checks)
  - `internal/workspace`: pure naming/metadata layer (paths, session names, labels)
  - `internal/zmx`: zmx CLI wrapper (`attach`, `list`, `set`, `kill`, `history`)
- Neovim plugin: `plugin/zmxx.lua`, `lua/zmxx/init.lua`
  - Talks to the CLI only via subprocesses: `zmxx ls --json` to list,
    `zmxx switch` to move, `zmxx preview` in the fzf preview
  - Picker UI requires fzf-lua

## Invariants

- **Deterministic naming** (`internal/workspace`): `repoID` is the first 12
  hex chars of the SHA-256 of the origin URL (fallback: git common dir);
  worktree path = slug(branch) + 8-char hash; session name =
  `zmxx-<repoID>` + 12-char hash. Session names must stay short because they
  end up in unix socket paths.
- **Labels are the discovery mechanism**: sessions are found via the `zmxx=1`
  marker label across all repositories, never by name-prefix matching. Label
  values are base64url (`workspace.Encode`) because zmx labels only allow
  `[A-Za-z0-9-_.]`.
- **Environment hygiene for every zmx call** (`internal/zmx`):
  `ZMX_SESSION_PREFIX` is always cleared so full names are used verbatim;
  `ZMX_SESSION` is stripped only when creating a session (so the caller's
  terminal is not stolen) and deliberately kept on attach so zmx switches the
  terminal's client instead of nesting.
- **Sessions are shell-based**: the session command is
  `bash -c 'nvim .; exec "${SHELL:-/bin/sh}"'`. Exiting nvim must not end the
  session; only `zmxx rm` kills it.
- **`zmxx switch` never creates sessions**: it verifies existence first,
  opting out of zmx attach's upsert behavior.

## Gotchas

- git reports resolved worktree paths (`/private/var/...` on macOS), while
  zmxx paths go through `$HOME`. Compare with `gitx.SamePath` /
  `gitx.CanonicalPath`, never plain string equality.
- Keep the `sessionJSON` struct in `internal/cli` in sync with what
  `lua/zmxx/init.lua` consumes.
