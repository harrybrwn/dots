package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/harrybrwn/dots/git"
	"github.com/spf13/cobra"
)

const (
	name = "dots"
	repo = "repo"
)

var completions string

type Options struct {
	Root      string // Root of user-added files
	ConfigDir string // Internal config folder
	NoColor   bool
}

func NewRootCmd() *cobra.Command {
	var (
		opts = Options{
			Root:      os.Getenv("HOME"),
			ConfigDir: configdir(),
		}
		c = &cobra.Command{
			Use:           name,
			SilenceErrors: true,
			SilenceUsage:  true,
			CompletionOptions: cobra.CompletionOptions{
				DisableDefaultCmd: completions == "false",
			},
		}
	)
	c.AddCommand(
		NewLSCmd(&opts),
		NewAddCmd(&opts),
		NewCloneCmd(&opts),
		NewRemoveCmd(&opts),
		NewStatusCmd(&opts),
		NewUpdateCmd(&opts),
		NewGitCmd(&opts),
		NewUtilCmd(&opts),
		newTestCmd(&opts),
	)
	f := c.PersistentFlags()
	f.StringVarP(&opts.ConfigDir, "config", "c", opts.ConfigDir, "configuration directory")
	f.StringVarP(&opts.Root, "base-dir", "d", opts.Root, "base of the git tree")
	f.BoolVar(&opts.NoColor, "no-color", opts.NoColor, "disable color output")
	return c
}

func NewAddCmd(opts *Options) *cobra.Command {
	c := &cobra.Command{
		Use:   "add",
		Short: "Add new files.",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			if len(args) == 0 {
				return errors.New("no files given")
			}
			git := git.New(opts.repo(), opts.Root)
			if !git.Exists() {
				err := git.InitBare()
				if err != nil {
					return err
				}
			}
			if err = cleanPaths(opts, args); err != nil {
				return err
			}
			err = git.Add(args...)
			if err != nil {
				return err
			}
			return git.Commit(commitMessage("add", args))
		},
	}
	return c
}

func NewRemoveCmd(opts *Options) *cobra.Command {
	c := &cobra.Command{
		Use:   "rm",
		Short: "Remove files from internal tracking",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cleanPaths(opts, args); err != nil {
				return err
			}
			return git.New(opts.repo(), opts.Root).Remove(args...)
		},
	}
	return c
}

func NewCloneCmd(opts *Options) *cobra.Command {
	var force bool
	c := &cobra.Command{
		Use:   "clone <uri>",
		Short: "Clone a remote repository",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			git := opts.git()
			if git.Exists() {
				return errors.New("git repository already exists here")
			}
			if !force && exists(opts.repo()) {
				return fmt.Errorf("repository %q already exists", opts.repo())
			}
			return git.Cmd("clone", "--bare", args[0], opts.repo()).Run()
		},
	}
	c.Flags().BoolVarP(&force, "force", "f", force, "overwrite the existing repo")
	return c
}

func NewUpdateCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Track the updates made to files already checked in to the repo",
		RunE: func(cmd *cobra.Command, args []string) error {
			git := opts.git()
			updated, err := git.ModifiedFiles()
			if err != nil {
				return err
			}
			for i, f := range updated {
				updated[i] = filepath.Join(opts.Root, f)
			}
			err = git.Add(updated...)
			if err != nil {
				return err
			}
			return git.Commit(commitMessage("update", updated))
		},
	}
}

func NewStatusCmd(opts *Options) *cobra.Command {
	c := &cobra.Command{
		Use:   "status",
		Short: "Show the status of files being tracked.",
		RunE: func(cmd *cobra.Command, args []string) error {
			g := git.New(opts.repo(), opts.Root)
			g.SetErr(cmd.ErrOrStderr())
			g.SetOut(cmd.OutOrStdout())
			err := g.Cmd(
				"--no-pager",
				"-c", "color.status=always",
				"diff", "--stat",
			).Run()
			if err != nil {
				return err
			}
			return g.Cmd(
				"-c", "color.status=always",
				"status",
			).Run()
		},
	}
	return c
}

func NewUtilCmd(opts *Options) *cobra.Command {
	c := &cobra.Command{
		Use:               "util",
		Short:             "A collection of utility commands",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
	}
	c.AddCommand(
		NewGetCmd(opts),
		&cobra.Command{
			Use:     "set-identity-file <file>",
			Short:   "Set an ssh identity file to be used on every remote operation.",
			Aliases: []string{"set-ssh-key"},
			Args:    cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				g := opts.git()
				return g.Cmd(
					"config", "--set",
					"core.sshCommand",
					fmt.Sprintf("ssh -i %s -o IdentitiesOnly=yes", args[0]),
				).Run()
			},
		},
		&cobra.Command{
			Use: "command", Short: "Print the internal git command being used",
			RunE: func(cmd *cobra.Command, args []string) error {
				g := opts.git()
				fmt.Println(strings.Join(g.Cmd().Args, " "))
				return nil
			},
		},
	)
	return c
}

func NewGetCmd(opts *Options) *cobra.Command {
	var force bool
	c := &cobra.Command{
		Use:   "get <file>",
		Short: "Pull a single file out and write it the to current working directory.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			if !force && exists(filepath.Join(cwd, args[0])) {
				return fmt.Errorf(
					"file %q already exists",
					args[0],
				)
			}
			git := git.New(filepath.Join(opts.ConfigDir, repo), cwd)
			return git.Cmd("checkout", "--", args[0]).Run()
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			git := opts.git()
			files, err := git.LsFiles()
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}
			return files, cobra.ShellCompDirectiveDefault
		},
	}
	c.Flags().BoolVarP(
		&force, "force", "f",
		force, "force git to overwrite the file if it already exists",
	)
	return c
}

func NewGitCmd(opts *Options) *cobra.Command {
	fn := func(c *cobra.Command, a []string) error { return opts.git().Cmd(a...).Run() }
	return &cobra.Command{
		Use:                "git",
		Hidden:             true,
		DisableFlagParsing: true,
		RunE:               fn,
	}
}

func newTestCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:    "test",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			git := git.New(opts.repo(), opts.Root)
			c := git.Cmd("ls-tree", "HEAD", "-r", "--full-tree", "--name-status")
			return c.Run()
		},
	}
}

// cleanPaths ensures that all the files given are absolute
// paths if they are relative
func cleanPaths(opts *Options, files []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	for i := range files {
		if !filepath.IsAbs(files[i]) {
			files[i] = filepath.Join(cwd, files[i])
		}
	}
	return nil
}

func commitMessage(op string, files []string) string {
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = filepath.Base(f)
	}
	return fmt.Sprintf("[%s] %s", op, strings.Join(names, ", "))
}

func (o *Options) repo() string {
	return filepath.Join(o.ConfigDir, repo)
}

func (o *Options) git() *git.Git {
	return git.New(o.repo(), o.Root)
}

func configdir() string {
	var (
		dir string
		ok  bool
	)
	dir, ok = os.LookupEnv("XDG_CONFIG_HOME")
	if ok {
		return filepath.Join(dir, name)
	}
	dir, ok = os.LookupEnv("HOME")
	if ok {
		return filepath.Join(dir, "."+name)
	}
	return ""
}

func page(stdout io.Writer, in io.Reader) error {
	pager, ok := os.LookupEnv("PAGER")
	if !ok {
		pager = "less"
	}
	cmd := exec.Command(pager)
	cmd.Stdout = stdout
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	go func() {
		defer stdin.Close()
		io.Copy(stdin, in)
	}()
	return cmd.Run()
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

type rankedFile struct {
	Name string
	Rank int
}

type RankedFiles []rankedFile

func NewRankedFiles(term string, files []string) RankedFiles {
	var rf = make(RankedFiles, len(files))
	for i, f := range files {
		rf[i] = rankedFile{Name: f, Rank: levenshtein(term, f)}
	}
	sort.Sort(rf)
	return rf
}

func (rf RankedFiles) Len() int           { return len(rf) }
func (rf RankedFiles) Swap(i, j int)      { rf[i], rf[j] = rf[j], rf[i] }
func (rf RankedFiles) Less(i, j int) bool { return rf[i].Rank < rf[j].Rank }

func (rf RankedFiles) AsStrings() []string {
	var list = make([]string, len(rf))
	for i, f := range rf {
		list[i] = f.Name
	}
	return list
}

var _ sort.Interface = (*RankedFiles)(nil)

func levenshtein(s, t string) int {
	var (
		i, j int
		m, n = len(s), len(t)
		d    = make([][]int, m+1) // d[0..m, 0..n]
	)
	for i = range d {
		d[i] = make([]int, n+1)
		d[i][0] = i
	}
	for j = range d[0] {
		d[0][j] = j
	}
	for j := 1; j <= n; j++ {
		for i := 1; i <= m; i++ {
			if s[i-1] == t[j-1] {
				d[i][j] = d[i-1][j-1]
			} else {
				min := d[i-1][j]
				if d[i][j-1] < min {
					min = d[i][j-1]
				}
				if d[i-1][j-1] < min {
					min = d[i-1][j-1]
				}
				d[i][j] = min + 1
			}
		}
	}
	return d[m][n]
}
