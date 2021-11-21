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
	"time"

	"github.com/harrybrwn/dots/git"
	"github.com/spf13/cobra"
)

const (
	name = "dots"
	repo = "repo"
)

// set at compile time with -ldflags
var completions string

type Options struct {
	Root      string // Root of user-added files
	ConfigDir string // Internal config folder
	NoColor   bool
	gitArgs   []string
}

func NewRootCmd() *cobra.Command {
	var (
		opts = Options{
			Root:      os.Getenv("HOME"),
			ConfigDir: configdir(),
		}
		c = &cobra.Command{
			Use:           name,
			Short:         "Manage your dot files.",
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
		NewSyncCmd(&opts),
		NewInstallCmd(&opts),
		NewGitCmd(&opts),
		NewUtilCmd(&opts),
		newTestCmd(&opts),
	)
	f := c.PersistentFlags()
	f.StringVarP(&opts.ConfigDir, "config", "c", opts.ConfigDir, "configuration directory")
	f.StringVarP(&opts.Root, "dir", "d", opts.Root, "base of the git tree (where your configuration lives)")
	f.StringVarP(&opts.Root, "root", "r", opts.Root, "root of the git tree (where your configuration lives)")
	f.BoolVar(&opts.NoColor, "no-color", opts.NoColor, "disable color output")
	f.StringSliceVar(&opts.gitArgs, "git-args", opts.gitArgs, "pass additional flags or arguments to the git command internally")
	c.SetUsageTemplate(IndentedCobraUsageTemplate)
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
		ValidArgsFunction: filesCompletionFunc(opts),
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
		Short: "Update files that have been modified",
		Long: "" +
			"Update is similar to 'add' in that it updates\n" +
			"the internal repository with new changes except that\n" +
			"it automatically updates files that have already\n" +
			"been added and have changed since the last update.",
		SuggestFor: []string{"add"},
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

func NewInstallCmd(opts *Options) *cobra.Command {
	c := &cobra.Command{
		Use:   "install [location]",
		Short: "Copy all of the tracked files to the current root (will overwrite existing files)",
		RunE: func(cmd *cobra.Command, args []string) error {
			loc := opts.Root
			if len(args) > 0 {
				loc = args[0]
			}
			git := git.New(opts.repo(), loc)
			return git.Cmd("checkout").Run()
		},
	}
	return c
}

func NewSyncCmd(opts *Options) *cobra.Command {
	c := &cobra.Command{
		Use:   "sync",
		Short: "Sync with the remote repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			var (
				err error
				c   *exec.Cmd
				git = opts.git()
			)
			git.SetErr(cmd.ErrOrStderr())
			git.SetOut(cmd.OutOrStdout())
			if !git.HasRemote() {
				return errors.New("repo does not have a remote repo")
			}
			c = git.Cmd("pull", "origin", "master")
			if err = c.Run(); err != nil {
				return err
			}
			c = git.Cmd("push", "origin", "master")
			if err = c.Run(); err != nil {
				return err
			}
			return nil
		},
	}
	return c
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
		NewCatCmd(opts),
		&cobra.Command{
			Use: "set-ssh-key <file>", Args: cobra.ExactArgs(1),
			Short: "Set an ssh identity file to be used on every remote operation.",
			RunE: func(cmd *cobra.Command, args []string) error {
				return opts.git().Cmd(
					"config", "--set", "core.sshCommand",
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
		&cobra.Command{
			Use: "modified",
			RunE: func(cmd *cobra.Command, args []string) error {
				git := opts.git()
				mods, err := git.Modifications()
				if err != nil {
					return err
				}
				for _, m := range mods {
					fmt.Printf("%c %+v\n", m.Type, m)
				}
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
			if !force && args[0] != "." && exists(filepath.Join(cwd, args[0])) {
				return fmt.Errorf(
					"file %q already exists",
					args[0],
				)
			}
			git := git.New(opts.repo(), cwd)
			if args[0] == "." {
				files, err := git.LsFiles()
				if err != nil {
					return err
				}
				command := []string{"checkout", "--"}
				command = append(command, files...)
				return git.Cmd(command...).Run()
			}
			return git.Cmd("checkout", "--", args[0]).Run()
		},
		ValidArgsFunction: filesCompletionFunc(opts),
	}
	c.Flags().BoolVarP(
		&force, "force", "f",
		force, "force git to overwrite the file if it already exists",
	)
	return c
}

func NewCatCmd(opts *Options) *cobra.Command {
	tmp := os.TempDir()
	c := &cobra.Command{
		Use:               "cat <filename>",
		Short:             "Print a file being tracked to standard out.",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: filesCompletionFunc(opts),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := filepath.Join(tmp, fmt.Sprintf("dots_cat_%v", time.Now().Unix()))
			defer func() {
				if err := os.RemoveAll(dir); err != nil {
					fmt.Fprintf(os.Stderr, "Error: could not delete temporary folder: %v", err)
				}
			}()
			err := os.Mkdir(dir, 0755)
			if err != nil {
				return err
			}
			git := git.New(opts.repo(), dir)
			err = git.Cmd("checkout", "--", args[0]).Run()
			if err != nil {
				return err
			}
			f, err := os.Open(filepath.Join(dir, args[0]))
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = io.Copy(os.Stdout, f)
			return err
		},
	}
	return c
}

func NewGitCmd(opts *Options) *cobra.Command {
	fn := func(_ *cobra.Command, a []string) error { return opts.git().Cmd(a...).Run() }
	return &cobra.Command{
		Use:                "git",
		Hidden:             true,
		DisableFlagParsing: true,
		RunE:               fn,
	}
}

func filesCompletionFunc(opts *Options) func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		git := opts.git()
		files, err := git.LsFiles()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		return files, cobra.ShellCompDirectiveDefault
	}
}

func newTestCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:    "test",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
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

func init() {
	cobra.AddTemplateFunc("indent", func(s string) string {
		parts := strings.Split(s, "\n")
		for i := range parts {
			parts[i] = "    " + parts[i]
		}
		return strings.Join(parts, "\n")
	})
}

// This is a template for cobra commands that more
// closely imitates the style of the go command help
// message.
var IndentedCobraUsageTemplate = `Usage:{{if .Runnable}}

	{{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
	{{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
	{{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
	{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

Available Commands:
{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
	{{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:

{{.LocalFlags.FlagUsagesWrapped 100 | trimTrailingWhitespaces | indent}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:

{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces | indent}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:
{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
	{{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`
