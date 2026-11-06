// gh-snitch is a gh extension that shows who contributed what to a repository.
package main

import (
	"os"

	"github.com/a-narquel/gh-snitch/internal/cli"
)

func main() {
	// os.Args[0] is the program's own path; the rest are the user's arguments.
	os.Exit(cli.Run(os.Args[1:]))
}
