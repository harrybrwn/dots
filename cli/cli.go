// Package cli holds the dots cli.
package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/harrybrwn/dots/cli/dotfiles"
	"github.com/harrybrwn/dots/git"
)

const (
	ReadMeName    = dotfiles.ReadMeName
	name          = "dots"
	repo          = "repo"
	DefaultBranch = "main"
)

var (
	Version string // release version
	Commit  string // git commit of release
	Hash    string // sha256 of source code
	Date    string

	// set at compile time with -ldflags
	// "false" to disable completions command
	completions string
)

type Options struct {
	Root      string // Root of user-added files
	ConfigDir string // Internal config folder
	noColor   bool
	verbose   bool

	gitArgs []string

	user  string
	email string
}

func (o *Options) repo() string {
	return filepath.Join(o.ConfigDir, repo)
}

func (o *Options) Git() *git.Git { return o.git() }

func (o *Options) NoColor() bool { return o.noColor }

func (o *Options) git() *git.Git {
	return git.New(o.repo(), o.Root)
}

func (o *Options) HasReadme() bool {
	return exists(filepath.Join(o.ConfigDir, ReadMeName))
}

func (o *Options) excludesFile() string {
	return filepath.Join(o.ConfigDir, "ignore")
}

func (o *Options) applyUserTo(g interface{ AppendPersistentArgs(...string) }) {
	g.AppendPersistentArgs(
		"-c", fmt.Sprintf("user.name=%s", o.user),
		"-c", fmt.Sprintf("user.email=%s", o.email),
	)
}

func (o *Options) log() func(string, ...any) {
	if o.verbose {
		return func(f string, v ...any) { fmt.Printf(f+"\n", v...) }
	}
	return func(string, ...any) {}
}

type FlagSet interface {
	StringVar(ptr *string, name, value, description string)
	StringVarP(prt *string, name, shorthand, value, description string)
	BoolVarP(prt *bool, name, shorthand string, value bool, description string)
}

func (o *Options) addUserFlags(set FlagSet) {
	set.StringVarP(&o.user, "user", "U", o.user, "username used to make git commits")
	set.StringVarP(&o.email, "email", "e", o.email, "email used to make git commits")
}

func NewRootCmd() *cobra.Command {
	var (
		opts = Options{
			Root:      os.Getenv("HOME"),
			ConfigDir: configdir(),
			user:      "dots",
			email:     "dots@gopkgs.hrry.dev",
		}
		c = &cobra.Command{
			Use:   name,
			Short: "Manage your dot files.",
			Long: `
Manage your dots files without the hassle of working with git bare repos.
That statmement is to some degree untrue because that is all this tool
does under the hood. It handles all the crusty parts of managing a bare
git repo so that you don't have too.`,
			SilenceErrors: true,
			SilenceUsage:  true,
			Example: `  $ dots install github.com/harrybrwn/dotfiles
  $ dots ls
  $ dots add ~/.config/vlc/vlcrc
  $ dots sync # push local changes
  $ echo 'set number' >> ~/.vim/vimrc
  $ dots update # update all tracked files with local repo
  $ dots sync`,
			CompletionOptions: cobra.CompletionOptions{
				DisableDefaultCmd: completions == "false",
			},
		}
	)
	c.AddGroup(
		&cobra.Group{ID: "basic", Title: "Basic Commands:"},
	)

	cmds := []*cobra.Command{
		NewCloneCmd(&opts),
		NewInitCmd(&opts),
		NewInstallCmd(&opts),
		NewUninstallCmd(&opts),
		NewStatusCmd(&opts),
		NewPullCmd(&opts),
		NewDiffCmd(&opts),
		NewGitCmd(&opts),

		NewUtilCmd(&opts),
		NewVersionCmd(),
		newTestCmd(&opts),
	}

	cmds = append(cmds, asGroup("basic",
		NewLSCmd(&opts),
		NewSyncCmd(&opts),
		NewUndoCmd(&opts),
		NewAddCmd(&opts),
		NewRemoveCmd(&opts),
		NewUpdateCmd(&opts),
	)...)
	c.AddCommand(cmds...)

	f := c.PersistentFlags()
	f.StringVarP(&opts.ConfigDir, "config", "c", opts.ConfigDir, "configuration directory")
	f.StringVarP(
		&opts.Root,
		"dir",
		"d",
		opts.Root,
		"base of the git tree (where your configuration lives)",
	)
	// f.StringVarP(&opts.Root, "root", "r", opts.Root, "root of the git tree (where your configuration lives)")
	f.BoolVar(&opts.noColor, "no-color", opts.noColor, "disable color output")
	f.BoolVarP(&opts.verbose, "verbose", "v", opts.verbose, "run commands verbosely")
	f.StringSliceVar(&opts.gitArgs, "git-args", opts.gitArgs,
		"pass additional flags or arguments to the git command internally")
	c.SetUsageTemplate(IndentedCobraUsageTemplate)
	return c
}

func NewVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use: "version", Short: "Print the version and build info",
		Aliases: []string{"v"},
		Run: func(cmd *cobra.Command, args []string) {
			const format = "%s\n" +
				"commit:     %s\n" +
				"build date: %s\n" +
				"hash:       %s\n"
			fmt.Fprintf(
				cmd.OutOrStdout(),
				format,
				Version, Commit, Date, Hash)
		},
	}
}

func NewRemoveCmd(opts *Options) *cobra.Command {
	c := &cobra.Command{
		Use:   "rm <name...>",
		Short: "Remove files from internal tracking",
		Long:  "Remove files from the internal git repo. This will not remove any files on disk.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cleanPaths(args); err != nil {
				return err
			}
			g := opts.Git()
			err := g.Remove(args...)
			if err != nil {
				return err
			}
			if err = g.AddUpdate(args...); err != nil {
				return err
			}
			opts.applyUserTo(g)
			if err = g.Commit(commitMessage("remove", args)); err != nil {
				return err
			}
			return nil
		},
		ValidArgsFunction: gitFilesCompletionFunc(opts),
	}
	opts.addUserFlags(c.Flags())
	return c
}

func removeReadme(base string, files []string) []string {
	for i, f := range files {
		p, err := filepath.Rel(base, f)
		if err != nil {
			continue
		}
		if p == ReadMeName {
			return remove(i, files)
		}
	}
	return files
}

func NewGitCmd(r dotfiles.Repo) *cobra.Command {
	fn := func(cmd *cobra.Command, a []string) error {
		var (
			g = r.Git()
			c = g.Cmd(a...)
		)
		c.Stdout = cmd.OutOrStdout()
		return execute(c)
	}
	return &cobra.Command{
		Use:                "git",
		Hidden:             true,
		DisableFlagParsing: true,
		RunE:               fn,
	}
}

func writeGitignore(opts *Options) error {
	filename := opts.excludesFile()
	// Entries in the global gitignore have to be relative for some reason.
	ignored, err := filepath.Rel(opts.Root, opts.repo())
	if err != nil {
		return err
	}
	if !exists(filename) {
		f, err := os.Create(filename)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = fmt.Fprintf(f, "%s\n", ignored)
		if err != nil {
			return err
		}
	} else {
		f, err := os.OpenFile(filename, os.O_RDWR|os.O_APPEND, os.FileMode(0644))
		if err != nil {
			return err
		}
		defer f.Close()
		found := false
		buf := bufio.NewScanner(f)
		for buf.Scan() {
			if strings.Trim(buf.Text(), "\n\t ") == ignored {
				found = true
				break
			}
		}
		if err = buf.Err(); err != nil {
			return err
		}
		if !found {
			if _, err = f.WriteString(ignored); err != nil {
				return err
			}
			if _, err = f.Write([]byte{'\n'}); err != nil {
				return err
			}
		}
	}
	return nil
}

func dirContainsPath(dir, path string) bool {
	base := strings.Split(dir, string(filepath.Separator))
	test := strings.Split(path, string(filepath.Separator))
	if dir == "/" {
		base = []string{""}
	}
	if path == "/" {
		test = []string{""}
	}
	if len(base) == 0 || len(test) == 0 || len(base) > len(test) {
		return false
	}
	N := max(len(base), len(test))
	n := min(len(base), len(test))
	for i := range N {
		if i >= n {
			return true
		}
		if base[i] != test[i] {
			return false
		}
	}
	return false
}

type completeFunc func(
	_ *cobra.Command,
	_ []string,
	toComplete string,
) ([]string, cobra.ShellCompDirective)

func gitFilesCompletionFunc(r dotfiles.Repo) completeFunc {
	return func(
		_ *cobra.Command,
		_ []string,
		toComplete string,
	) ([]string, cobra.ShellCompDirective) {
		files, err := r.Git().LsFiles()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		return files, cobra.ShellCompDirectiveDefault
	}
}

func newTestCmd(*Options) *cobra.Command {
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
func cleanPaths(files []string) error {
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
		return filepath.Join(dir, ".config", name)
	}
	return ""
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func existsAndIsNotDir(path string) bool {
	stat, err := os.Stat(path)
	if err != nil || os.IsNotExist(err) {
		return false
	}
	return !stat.IsDir()
}

func yesOrNo(in io.Reader, out io.Writer, prompt string) bool {
	var res string
	fmt.Fprintf(out, "%s (y/n) ", prompt)
	_, err := fmt.Fscan(in, &res)
	if err != nil {
		return false
	}
	switch strings.ToLower(res) {
	case "y", "yes":
		return true
	}
	return false
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

// IndentedCobraUsageTemplate is a template for cobra commands that more closely
// imitates the style of the go command help message.
var IndentedCobraUsageTemplate = `Usage:{{if .Runnable}}

	{{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}

	{{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:

	{{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Available Commands:
{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
	{{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}
{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
	{{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

Additional Commands:
{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
	{{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:

{{.LocalFlags.FlagUsages | trimTrailingWhitespaces | indent}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:

{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces | indent}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
	{{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`
