// Command zombie-scanner finds AWS resources that are dead but still billing.
// It is strictly read-only: no code path in this program creates, modifies, or
// deletes any AWS resource.
package main

import (
	"context"
	"errors"
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

	err := root.ExecuteContext(context.Background())
	if err == nil {
		return
	}

	if !errors.Is(err, cli.ErrSpendAboveThreshold) {
		fmt.Fprintln(os.Stderr, "error:", err)
	}
	os.Exit(exitCode(err))
}

//  process exit status

// 0  scan completed (zombies may exist)
// 1  fatal - no credentials, bad flags, --strict with failures
// 2  Scan succeeded, but spend exceeded --fail-if-above
func exitCode(err error) int {
	if errors.Is(err, cli.ErrSpendAboveThreshold) {
		return 2
	}
	return 1
}
