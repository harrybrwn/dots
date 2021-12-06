package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/harrybrwn/dots/cli"
	"github.com/spf13/cobra"
	cobradoc "github.com/spf13/cobra/doc"
)

type ShellType string

const (
	Bash       ShellType = "bash"
	Zsh        ShellType = "zsh"
	Fish       ShellType = "fish"
	Powershell ShellType = "powershell"
)

func main() {
	var (
		releasedir    = "release"
		name          = "dots"
		completionDir = "release/completion"
		manDir        = "release/man"

		// Debian packaging flags
		deb         bool
		version     = cli.Version
		packageDir  string
		description string
	)
	flag.StringVar(&releasedir, "release", releasedir, "specify the release directory")
	flag.StringVar(&completionDir, "completion", completionDir, "specify the completion script output directory")
	flag.StringVar(&manDir, "man", manDir, "specify the man page output directory")
	flag.StringVar(&name, "name", name, "specify the program name (will effect completion scripts and man page file names)")
	flag.StringVar(&version, "version", version, "give the release a version")
	flag.StringVar(&packageDir, "package", packageDir, "directory that the debian package is being built from")
	flag.BoolVar(&deb, "deb", deb, "generate files for a debian package")
	flag.StringVar(&description, "description", description, "debian package description")
	flag.Parse()

	cmd := cli.NewRootCmd()
	cmd.DisableAutoGenTag = true

	if deb {
		if len(packageDir) == 0 || !exists(packageDir) {
			log.Fatal("use '-package' flag for the package directory")
		}
		maintainer, err := maintainer()
		if err != nil {
			log.Fatal(err)
		}
		if version == "" {
			log.Fatal("no version given")
		}
		os.MkdirAll(filepath.Join(packageDir, "DEBIAN"), 0755) // silent
		os.MkdirAll(filepath.Join(packageDir, "usr", "bin"), 0755)
		control := filepath.Join(packageDir, "DEBIAN", "control")
		f, err := os.Create(control)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		f.WriteString(fmt.Sprintf("Package: %s\n", name))
		f.WriteString(fmt.Sprintf("Version: %s\n", version))
		f.WriteString(fmt.Sprintf("Architecture: %s\n", runtime.GOARCH))
		f.WriteString("Depends: git\nPriority: optional\n")
		if description != "" {
			f.WriteString(fmt.Sprintf("Description: %s\n", description))
		}
		f.WriteString(fmt.Sprintf("Maintainer: %s\n", maintainer))
		manDir = filepath.Join(packageDir, "usr", "share", "man", "man1")
		cmd.CompletionOptions.DisableDefaultCmd = false
		for _, shell := range []ShellType{Bash, Zsh, Fish} {
			d := filepath.Join(
				packageDir,
				findCompletionDir(shell),
			)
			err := genComp(cmd, d, shell, name)
			if err != nil {
				log.Fatal(err)
			}
		}
	} else {
		cmd.CompletionOptions.DisableDefaultCmd = false
		for _, shell := range []ShellType{Bash, Zsh, Fish, Powershell} {
			err := genComp(cmd, completionDir, shell, name)
			if err != nil {
				log.Fatal(err)
			}
		}
	}

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
}

func genComp(cmd *cobra.Command, dir string, shell ShellType, prog string) error {
	var (
		name = completionScriptName(shell, prog)
		gen  func(io.Writer, bool) error
	)
	switch shell {
	case Bash:
		gen = cmd.GenBashCompletionV2
	case Fish:
		gen = cmd.GenFishCompletion
	case Zsh:
		gen = func(w io.Writer, _ bool) error { return cmd.GenZshCompletion(w) }
	case Powershell:
		gen = func(w io.Writer, _ bool) error { return cmd.GenPowerShellCompletion(w) }
	}
	p := filepath.Join(dir, string(shell))
	if !exists(p) {
		err := os.MkdirAll(p, 0755)
		if err != nil {
			return err
		}
	}
	f, err := os.OpenFile(filepath.Join(dir, string(shell), name), os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	return gen(f, true)
}

func findCompletionDir(shell ShellType) string {
	switch shell {
	case Bash:
		return "/usr/share/bash-completion/completions"
	case Zsh:
		return "/usr/share/zsh/vendor-completions"
	case Fish:
		return "/usr/share/fish/completions"
	default:
		return ""
	}
}

func findUserCompletionDir(shell ShellType, home string) string {
	switch shell {
	case Bash:
		return filepath.Join(home, "")
	case Zsh:
		zdot, ok := os.LookupEnv("ZDOTDIR")
		if !ok {
			return filepath.Join(home, "oh-my-zsh", "completions") // TODO this could be wrong
		}
		return filepath.Join(zdot, "oh-my-zsh", "completions")
	case Fish:
		return ""
	default:
		return ""
	}
}

func completionScriptName(shell ShellType, name string) string {
	switch shell {
	case Bash:
		return name
	case Zsh:
		return "_" + name
	case Fish:
		return name + ".fish"
	default:
		return name
	}
}

func maintainer() (string, error) {
	var (
		err error
		b   bytes.Buffer
		s   = [2]string{
			"user.name",
			"user.email",
		}
		cmd *exec.Cmd
	)
	for i := 0; i < 2; i++ {
		cmd = exec.Command("git", "config", "--global", "--get", s[i])
		cmd.Stdout = &b
		err = cmd.Run()
		if err != nil {
			return "", err
		}
		s[i] = strings.Trim(b.String(), "\n")
		b.Reset()
	}
	if len(s[0]) == 0 {
		return "", nil
	}
	if len(s[1]) == 0 {
		return s[0], nil
	}
	return fmt.Sprintf("%s <%s>", s[0], s[1]), nil
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return !os.IsNotExist(err)
}
