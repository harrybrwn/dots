package git

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	gitExec = "git"
)

func New(dir, tree string) *Git {
	return &Git{
		gitDir:   dir,
		workTree: tree,
	}
}

func Here() *Git {
	return New(".git", ".")
}

type Git struct {
	gitDir         string // --git-dir
	workTree       string // --work-tree
	args           []string
	stdout, stderr io.Writer
	stdin          io.Reader
}

func (g *Git) Cmd(args ...string) *exec.Cmd {
	arguments := make([]string, 4, 4+len(args))
	arguments[0] = "--git-dir"
	arguments[1] = g.gitDir
	arguments[2] = "--work-tree"
	arguments[3] = g.workTree
	arguments = append(arguments, args...)
	cmd := exec.Command(gitExec, arguments...)
	g.setDefaultIO(cmd)
	return cmd
}

func (g *Git) Exists() bool {
	return exists(g.gitDir) && isGitDir(g.gitDir)
}

// InitBare will create a new bare repo. Equivalent to `git init --bare`.
func (g *Git) InitBare() error { return initBareRepo(g.gitDir) }

// WorkingTree will return the repositories working tree.
func (g *Git) WorkingTree() string { return g.workTree }
func (g *Git) GitDir() string      { return g.gitDir }

func (g *Git) Add(paths ...string) error {
	if len(paths) == 0 {
		paths = append(paths, ".")
	}
	return g.Cmd(append([]string{"add"}, paths...)...).Run()
}

func (g *Git) Remove(files ...string) error {
	if len(files) == 0 {
		return errors.New("no files to remove")
	}
	args := []string{"rm", "--cached"}
	args = append(args, files...)
	return g.Cmd(args...).Run()
}

func (g *Git) Commit(message string) error {
	return g.Cmd("commit", "-m", message).Run()
}

func (g *Git) LsFiles() ([]string, error) {
	var (
		buf bytes.Buffer
		cmd = g.Cmd("ls-tree", "--full-tree", "-r", "--name-only", "HEAD")
	)
	cmd.Stdout = &buf
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	return lines(buf.String()), nil
}

func (g *Git) ModifiedFiles() ([]string, error) {
	var (
		buf bytes.Buffer
		cmd = g.Cmd("diff-files", "--name-only")
	)
	cmd.Stdout = &buf
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	return lines(buf.String()), nil
}

type Config map[string]interface{}

func (g *Git) Config() (Config, error) {
	return g.config("--list")
}

func (g *Git) ConfigLocal() (Config, error) {
	return g.config("--local", "--list")
}

func (g *Git) ConfigGlobal() (Config, error) {
	return g.config("--global", "--list")
}

func (g *Git) ConfigSet(key, value string) error {
	return g.Cmd("config", key, value).Run()
}

func (g *Git) ConfigLocalSet(key, value string) error {
	return g.Cmd("config", "--local", key, value).Run()
}

func (g *Git) SetArgs(arguments ...string) { g.args = arguments }

func (g *Git) SetOut(out io.Writer) { g.stdout = out }
func (g *Git) SetErr(w io.Writer)   { g.stderr = w }

func (g *Git) config(flags ...string) (Config, error) {
	var (
		buf  bytes.Buffer
		m    = make(Config)
		args = make([]string, 1+len(flags))
		cmd  = g.Cmd(args...)
	)
	args[0] = "config"
	for i := 0; i < len(flags); i++ {
		args[i+1] = flags[i]
	}
	cmd.Stdout = &buf
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(buf.String(), "\n")
	for _, l := range lines {
		parts := strings.SplitN(l, "=", 2)
		if len(parts) < 2 {
			continue
		}
		m[parts[0]] = parts[1]
	}
	return m, nil
}

func lines(s string) []string {
	sp := strings.Split(s, "\n")
	lines := make([]string, 0, len(sp))
	for _, f := range sp {
		f = strings.Trim(f, "\n")
		if len(f) == 0 {
			continue
		}
		lines = append(lines, f)
	}
	return lines
}

func initBareRepo(path string) error {
	const branch = "main"
	if err := os.MkdirAll(path, 0755); err != nil {
		return err
	}
	err := writeToFile(filepath.Join(path, "HEAD"), fmt.Sprintf("ref: refs/heads/%s\n", branch))
	if err != nil {
		return err
	}
	err = writeToFile(filepath.Join(path, "config"), `[core]
	repositoryformatversion = 0
	filemode = true
	bare = true
`)
	if err != nil {
		return err
	}
	for _, p := range []string{
		"branches",
		"hooks",
		"info",
		"objects",
		"objects/info",
		"objects/pack",
		"refs",
		"refs/heads",
		"refs/tags",
	} {
		err = os.Mkdir(filepath.Join(path, p), 0755)
		if err != nil {
			return err
		}
	}

	err = writeToFile(filepath.Join(path, "description"), "Unnamed repository; edit this file 'description' to name the repository.\n")
	if err != nil {
		return err
	}
	err = writeToFile(filepath.Join(path, "info", "exclude"), `# git ls-files --others --exclude-from=.git/info/exclude
# Lines that start with '#' are comments.
# For a project mostly in C, the following would be a good set of
# exclude patterns (uncomment them if you want to use them):
# *.[oa]
# *~
`)
	if err != nil {
		return err
	}
	return nil
}

func isGitDir(dir string) bool {
	return exists(filepath.Join(dir, "refs")) &&
		exists(filepath.Join(dir, "objects")) &&
		exists(filepath.Join(dir, "HEAD"))
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return !os.IsNotExist(err)
}

func writeToFile(filename string, data string) error {
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write([]byte(data))
	return err
}

func (g *Git) setDefaultIO(cmd *exec.Cmd) {
	if g.stdout == nil {
		g.stdout = os.Stdout
	}
	if g.stderr == nil {
		g.stderr = os.Stderr
	}
	if g.stdin == nil {
		g.stdin = os.Stdin
	}
	cmd.Stdout = g.stdout
	cmd.Stderr = g.stderr
	cmd.Stdin = g.stdin
}
