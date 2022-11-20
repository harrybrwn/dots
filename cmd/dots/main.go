package main

import (
	"fmt"
	"os"

	"github.com/harrybrwn/dots/cli"
)

func main() {
	cmd := cli.NewRootCmd()
	err := cmd.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
