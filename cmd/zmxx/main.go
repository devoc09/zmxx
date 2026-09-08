// Command zmxx creates git worktree workspaces backed by zmx sessions and
// neovim.
package main

import (
	"os"

	"github.com/devoc09/zmxx/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
