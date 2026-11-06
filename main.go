package main

import (
	"fmt"
	"os"

	"github.com/a-narquel/gh-snitch/internal/gitlog"
)

func main() {
	commits, err := gitlog.Load(".")

	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	//number of commits
	fmt.Println(len(commits),"commits")
	


	
}
