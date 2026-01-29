package main

import (
	"fmt"
	"os"

	"github.com/harrybrwn/dots/cli"
)

//go:generate go run ./cmd/gen --name=dots

func main() {
	cmd := cli.NewRootCmd()
	err := cmd.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
