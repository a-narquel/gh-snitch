package main

import (
	"fmt"

	"github.com/a-narquel/gh-snitch/internal/gitlog"
)

func main() {
	c := gitlog.Commit{Hash: "abc123", Author: "andre"}
	fmt.Printf("%+v\n", c)
}
