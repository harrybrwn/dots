package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/harrybrwn/dots/cli"
	"github.com/spf13/cobra"
	cobradoc "github.com/spf13/cobra/doc"
)

func main() {
	var (
		releasedir    = "release"
		name          = "dots"
		completionDir = "release/completion"
		manDir        = "release/man"
	)
	flag.StringVar(&releasedir, "release", releasedir, "specify the release directory")
	flag.StringVar(&completionDir, "completion", completionDir, "specify the completion script output directory")
	flag.StringVar(&manDir, "man", manDir, "specify the man page output directory")
	flag.StringVar(&name, "name", name, "specify the program name (will effect completion scripts and man page file names)")
	flag.Parse()

	cmd := cli.NewRootCmd()
	cmd.DisableAutoGenTag = true
	if !exists(manDir) {
		err := os.MkdirAll(manDir, 0755)
		if err != nil {
			log.Fatal(err)
		}
	}
	err := cobradoc.GenManTree(cmd, &cobradoc.GenManHeader{}, manDir)
	if err != nil {
		log.Fatal(err)
	}
	cmd.CompletionOptions.DisableDefaultCmd = false
	for _, shell := range []string{"bash", "zsh", "fish"} {
		err = genComp(cmd, completionDir, shell, name)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func genComp(cmd *cobra.Command, dir, shell, prog string) error {
	var (
		name string
		gen  func(io.Writer, bool) error
	)
	switch shell {
	case "bash":
		name = prog
		gen = cmd.GenBashCompletionV2
	case "fish":
		name = fmt.Sprintf("%s.fish", prog)
		gen = cmd.GenFishCompletion
	case "zsh":
		name = fmt.Sprintf("_%s", prog)
		gen = func(w io.Writer, _ bool) error { return cmd.GenZshCompletion(w) }
	case "powershell":
		name = prog
		gen = func(w io.Writer, _ bool) error { return cmd.GenPowerShellCompletion(w) }
	}
	p := filepath.Join(dir, shell)
	if !exists(p) {
		err := os.MkdirAll(p, 0755)
		if err != nil {
			return err
		}
	}
	f, err := os.OpenFile(filepath.Join(dir, shell, name), os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	return gen(f, true)
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return !os.IsNotExist(err)
}
