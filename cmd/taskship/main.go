// Command taskship is a local-first task manager and coding-agent dispatcher.
package main

import (
	"os"

	"github.com/jhdenham/taskship/internal/cli"
)

// version is set at release build time via -ldflags "-X main.version=...".
var version = ""

func main() {
	os.Exit(cli.Execute(cli.NewRootCmd(version), os.Args[1:]))
}
