// Command zombie-scanner finds AWS resources that are dead but still billing.
// It is strictly read-only: no code path in this program creates, modifies, or
// deletes any AWS resource.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Sachinxmpl/zombie-scanner/cli"
)

// injected at build time via -ldflags -X
var (
	version = "dev"
	commit  = "none"
)

func main() {
	root := cli.NewRootCommand(version, commit)

	if err := root.ExecuteContext(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(exitCode(err))
	}
}

//  process exit status

// 0  scan completed (zombies may exist)
// 1  fatal - no credentials, bad flags, every region failed
// 2  Scan succeeded, but spend exceeded --fail-if-above
func exitCode(_ error) int {
	return 1
}
