// Command taskship is a local-first task manager and coding-agent dispatcher.
package main

import (
	"os"

	"github.com/jhdenham/taskship/internal/cli"
)

// version is set at release build time via -ldflags "-X main.version=...".
var version = ""

func main() {
	if err := cli.NewRootCmd(version).Execute(); err != nil {
		os.Exit(1)
	}
}
