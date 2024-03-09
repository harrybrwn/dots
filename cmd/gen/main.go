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

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	cobradoc "github.com/spf13/cobra/doc"

	"github.com/harrybrwn/dots/cli"
)

type ShellType string

const (
	Bash       ShellType = "bash"
	Zsh        ShellType = "zsh"
	Fish       ShellType = "fish"
	Powershell ShellType = "powershell"
)

const (
	DefaultReleaseDir = "release"
)

type Flags struct {
	Name       string
	ReleaseDir string

	// Packaging flags
	deb         bool
	version     string
	packageDir  string
	description string
}

func (f *Flags) install(flag *flag.FlagSet) error {
	flag.StringVar(&f.ReleaseDir, "release", DefaultReleaseDir, "specify the release directory")
	flag.StringVar(&f.Name, "name", f.Name, "specify the program name (will effect completion scripts and man page file names)")
	flag.StringVar(&f.version, "version", cli.Version, "give the release a version")
	flag.StringVar(&f.packageDir, "package", f.packageDir, "directory that the debian package is being built from")
	flag.BoolVar(&f.deb, "deb", f.deb, "generate files for a debian package")
	flag.StringVar(&f.description, "description", f.description, "debian package description")
	return flag.Parse(os.Args[1:])
}

func (f *Flags) validate() error {
	if len(f.version) == 0 {
		return errors.New("no version given")
	} else if f.version[0] == 'v' {
		f.version = f.version[1:]
	}
	return nil
}

func (f *Flags) hasPackageDir() bool {
	return len(f.packageDir) != 0 && exists(f.packageDir)
}

func main() {
	var (
		// releasedir = "release"
		// name       = "dots"

		// // Debian packaging flags
		// deb         bool
		// version     = cli.Version
		// packageDir  string
		// description string
		flags Flags

		completionDir string
		manDir        string
	)
	err := flags.install(flag.CommandLine)
	if err != nil {
		log.Fatal(err)
	}
	if flags.Name == "" {
		fail("Error: no -name flag specified")
	}

	completionDir = filepath.Join(flags.ReleaseDir, "completion")
	manDir = filepath.Join(flags.ReleaseDir, "man")

	cmd := cli.NewRootCmd()
	cmd.DisableAutoGenTag = true

	if flags.deb {
		if err := flags.validate(); err != nil {
			log.Fatal(errors.Wrap(err, "flag validation failed"))
		}
		if !flags.hasPackageDir() {
			fail("use '-package' flag for the package directory")
		}
		maintainer, err := maintainer()
		if err != nil {
			log.Fatal(err)
		}
		_ = os.MkdirAll(filepath.Join(flags.packageDir, "DEBIAN"), 0755) // silent
		_ = os.MkdirAll(filepath.Join(flags.packageDir, "usr", "bin"), 0755)
		control := filepath.Join(flags.packageDir, "DEBIAN", "control")
		f, err := os.Create(control)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		for _, err := range []error{
			eat(fmt.Fprintf(f, "Package: %s\n", flags.Name)),
			eat(fmt.Fprintf(f, "Version: %s\n", flags.version)),
			eat(fmt.Fprintf(f, "Architecture: %s\n", runtime.GOARCH)),
			eat(f.WriteString("Depends: git\nPriority: optional\n")),
		} {
			if err != nil {
				log.Fatal(err)
			}
		}

		if flags.description != "" {
			_, err = fmt.Fprintf(f, "Description: %s\n", flags.description)
			if err != nil {
				log.Fatal(err)
			}
		}
		_, _ = fmt.Fprintf(f, "Maintainer: %s\n", maintainer)
		manDir = filepath.Join(flags.packageDir, "usr", "share", "man", "man1")
		cmd.CompletionOptions.DisableDefaultCmd = false
		for _, shell := range []ShellType{Bash, Zsh, Fish} {
			d := filepath.Join(
				flags.packageDir,
				findCompletionDir(shell),
			)
			err := genComp(cmd, d, shell, flags.Name)
			if err != nil {
				log.Fatal(err)
			}
		}
	} else {
		cmd.CompletionOptions.DisableDefaultCmd = false
		for _, shell := range []ShellType{Bash, Zsh, Fish, Powershell} {
			err := genComp(cmd, completionDir, shell, flags.Name)
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
	err = cobradoc.GenManTree(cmd, &cobradoc.GenManHeader{}, manDir)
	if err != nil {
		log.Fatal(err)
	}
}

func eat[T any](_ T, e error) error {
	return e
}

func fail(msg string) {
	fmt.Fprintf(os.Stderr, "%s\n", msg)
	os.Exit(1)
}

func genComp(cmd *cobra.Command, dir string, shell ShellType, prog string) error {
	var (
		name = completionScriptName(shell, prog)
		p    = filepath.Join(dir, string(shell))
	)
	if !exists(p) {
		err := os.MkdirAll(p, 0755)
		if err != nil {
			return errors.Wrapf(err, "could not create completion directory %q", p)
		}
	}
	f, err := os.OpenFile(filepath.Join(p, name), os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if err != nil {
		return errors.Wrap(err, "failed to open completion script file")
	}
	defer f.Close()
	gen := completionGenFunc(cmd, shell)
	return gen(f)
}

func completionGenFunc(cmd *cobra.Command, shell ShellType) func(io.Writer) error {
	switch shell {
	case Bash:
		return func(w io.Writer) error { return cmd.GenBashCompletionV2(w, true) }
	case Fish:
		return func(w io.Writer) error { return cmd.GenFishCompletion(w, true) }
	case Zsh:
		return cmd.GenZshCompletion
	case Powershell:
		return cmd.GenPowerShellCompletion
	default:
		panic("unknown shell type")
	}
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
