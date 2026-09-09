# zmxx

`zmxx` is a workspace tool that combines [git worktree](https://git-scm.com/docs/git-worktree)
with [zmx](https://zmx.sh).

1. Navigate to the Git repository you want to work on in your terminal.
2. Run `zmxx new <branch>` to create a worktree and launch Neovim in a zmx session.
3. Press `Zl` in Neovim to open the zmx session picker and switch to another workspace.

## Requirements

- Go 1.24 or later (for building)
- [zmx](https://zmx.sh) 0.8.0 or later (required for labels and session switching)
- Neovim 0.10 or later (uses `vim.system`)
- [fzf-lua](https://github.com/ibhagwan/fzf-lua) (picker UI)

## Installation

### CLI

```sh
go install github.com/devoc09/zmxx/cmd/zmxx@latest
# Or build locally
go build -o ~/.local/bin/zmxx ./cmd/zmxx
```

### Neovim plugin (lazy.nvim)

```lua
{
  "devoc09/zmxx",
  dependencies = { "ibhagwan/fzf-lua" },
  event = "VeryLazy",
}
```

## Usage

```sh
# Create a workspace and launch Neovim (reuse an existing branch, or create one from HEAD)
zmxx new feature/foo

# Create a branch from a different starting point
zmxx new feature/bar --base main

# List all zmxx sessions
zmxx sessions

# Kill a session and remove its worktree (keep the branch)
zmxx remove zmxx-0123456789ab-0123456789ab

# Remove a workspace with work in progress (--force skips confirmation and discards uncommitted changes)
zmxx remove <session> --force
```

### Session lifetime

A workspace's zmx session is **shell-based**. `zmxx new` launches `nvim .` inside the session, but closing Neovim with `:q` keeps the session alive and returns you to a shell prompt in the workspace.

```sh
# Restart Neovim from the shell inside the session
nvim .
```

You can return to this workspace at any time using `zmxx switch` from another terminal or using the picker. Only `zmxx remove` explicitly terminates the session.

## Session picker

In Neovim:

| Action | Description |
| --- | --- |
| `Zl` | Open the picker for all zmxx sessions |
| `:ZmxxSessions` | Open the picker using a command |

In the picker:

- Filter by repository name, branch, worktree path, or connection count. Session names are hidden.
- Preview the selected session's scrollback with `zmx history`, using `follow` to keep the preview scrolled to the bottom.
- Press `Enter` to switch the terminal's zmx client to the selected session. Neovim itself continues running in the original session.
- When Neovim is running outside a zmx session, the picker opens a new terminal tab and attaches to the selected session. Detach with zmx's `Ctrl+\` to close the tab and return to the original Neovim instance.

## How it works

- Worktrees are placed at deterministic paths under `$XDG_DATA_HOME/zmxx/worktrees/<repo-id>/<branch-slug>-<hash>`. If `XDG_DATA_HOME` is unset, the base directory defaults to `~/.local/share/zmxx`.
- `<repo-id>` is the first 12 hexadecimal characters of the SHA-256 hash of the origin URL, or the Git common directory if no origin URL is available.
- zmx sessions are named `zmxx-<repo-id>-<branch-hash>`, with 12 characters for each hash, to fit within Unix socket path length limits.
- The session command is `bash -c 'nvim .; exec "${SHELL:-/bin/sh}"'`. After Neovim exits, the shell takes over, keeping the workspace alive until `zmxx remove`.
- Sessions are tagged with `zmxx=1` / `zmxx.repo` / `zmxx.reponame` / `zmxx.branch` / `zmxx.worktree` labels (metadata values use base64url encoding). The picker uses these labels to discover all zmxx sessions.
- Switching uses zmx's native Switch IPC: running `zmxx switch <name>` → `zmx attach <name>` from Neovim lets zmx use `ZMX_SESSION` to move **only the terminal's client** to the target session.
- If `ZMX_SESSION` is unset, `zmxx switch <name>` runs in a Neovim terminal tab, connecting as a client with a PTY for interactive use.
- `zmxx switch` only accepts existing sessions, preventing accidental session creation through upsert behavior.

## Limitations and notes

- Do not use `ZMX_SESSION_PREFIX` for sessions managed by zmxx. zmxx clears the prefix for every zmx call.
- `zmxx remove` kills the zmx session first. Removing the session you are currently attached to disconnects your terminal.
- Sessions share a single namespace across repositories. The picker displays all zmxx sessions.
- To run a command other than Neovim in a workspace, use `zmx attach <session-name> <command>` directly. Run `zmxx sessions` to find the session name.
