package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pkg/errors"
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

func (o *Options) globalConfigFile() string {
	return filepath.Join(o.ConfigDir, "gitconfig")
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
			CompletionOptions: cobra.CompletionOptions{
				DisableDefaultCmd: completions == "false",
			},
		}
	)
	c.AddCommand(
		NewLSCmd(&opts),
		NewAddCmd(&opts),
		NewRemoveCmd(&opts),
		NewUpdateCmd(&opts),
		NewSyncCmd(&opts),
		NewUndoCmd(&opts),

		NewCloneCmd(&opts),
		NewInitCmd(&opts),
		NewStatusCmd(&opts),
		NewInstallCmd(&opts),
		NewUninstallCmd(&opts),
		NewGitCmd(&opts),

		NewUtilCmd(&opts),
		NewVersionCmd(),
		newTestCmd(&opts),
	)
	f := c.PersistentFlags()
	f.StringVarP(&opts.ConfigDir, "config", "c", opts.ConfigDir, "configuration directory")
	f.StringVarP(&opts.Root, "dir", "d", opts.Root, "base of the git tree (where your configuration lives)")
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
			fmt.Fprintf(
				cmd.OutOrStdout(),
				"%s\n"+
					"commit:     %s\n"+
					"build date: %s\n"+
					"hash:       %s\n",
				Version, Commit, Date, Hash)
		},
	}
}

func NewAddCmd(opts *Options) *cobra.Command {
	var up bool // --update
	c := &cobra.Command{
		Use: "add <file...>", Short: "Add new files.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			g := opts.Git()
			if up {
				updated, err := getUpdated(g, opts, nil)
				if err != nil {
					return errors.Wrap(err, "could not list updated files")
				}
				args = append(args, updated...)
			}
			return add(opts, g, args)
		},
	}
	c.Flags().BoolVarP(&up, "update", "u", up, "update any changed files as well as add new ones")
	opts.addUserFlags(c.Flags())
	return c
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
		ValidArgsFunction: filesCompletionFunc(opts),
	}
	opts.addUserFlags(c.Flags())
	return c
}

func NewUpdateCmd(opts *Options) *cobra.Command {
	c := cobra.Command{
		Use:   "update [files...]",
		Short: "Update files in local git repo that have been modified",
		Long: "" +
			"Update is similar to 'add' in that it updates\n" +
			"the internal repository with new changes except that\n" +
			"it automatically updates files that have already\n" +
			"been added and have changed since the last update.",
		Example: "$ dots update\n" +
			"\t$ dots update ~/.bashrc",
		SuggestFor: []string{"add"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return update(opts, args)
		},
		ValidArgsFunction: modifiedCompletionFunc(opts),
	}
	opts.addUserFlags(c.Flags())
	return &c
}

func NewSyncCmd(r dotfiles.Repo) *cobra.Command {
	c := &cobra.Command{
		Use: "sync", Short: "Sync with the remote repository",
		RunE: func(*cobra.Command, []string) error {
			return sync(r.Git())
		},
	}
	return c
}

func NewStatusCmd(r dotfiles.Repo) *cobra.Command {
	c := &cobra.Command{
		Use:   "status",
		Short: "Show the status of files being tracked.",
		RunE: func(cmd *cobra.Command, args []string) error {
			g := r.Git()
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

func NewCloneCmd(opts *Options) *cobra.Command {
	var force bool
	c := &cobra.Command{
		Use:   "clone <uri>",
		Short: "Clone a remote repository",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			git := opts.Git()
			if git.Exists() {
				return errors.New("git repository already exists here")
			}
			if !force && exists(opts.repo()) {
				return fmt.Errorf("repository %q already exists", opts.repo())
			}
			return clone(opts, git, args[0])
		},
	}
	c.Flags().BoolVarP(&force, "force", "f", force, "overwrite the existing repo")
	return c
}

func clone(opts *Options, git *git.Git, repoSource string) error {
	// TODO after cloning run `git branch --set-upstream-to=origin/<branch> master`
	// to set the default branch so that we can have clean git pulls.
	//
	// Or better yet, do this by changing .git/config to have:
	//	[branch "master"]
	//		remote = origin
	//		merge = refs/heads/master
	//
	// This can also be made smarter by using the default branch name
	// that is used right after cloning the repo.
	//
	// Also add ~/.dots and ~/.config/dots to the repo's gitignore
	err := execute(git.Cmd("clone", "--bare", repoSource, opts.repo()))
	if err != nil {
		return err
	}
	// Configure git to ignore files that are not being tracked
	err = git.ConfigLocalSet("status.showUntrackedFiles", "no")
	if err != nil {
		return err
	}
	err = git.ConfigLocalSet("core.excludesFile", opts.excludesFile())
	if err != nil {
		return err
	}

	err = writeGitignore(opts)
	if err != nil {
		return err
	}
	err = writeGlobalConfig(opts)
	if err != nil {
		return err
	}
	return nil
}

func add(opts *Options, git *git.Git, files []string) (err error) {
	if !git.Exists() {
		err = git.InitBare()
		if err != nil {
			return err
		}
	}
	if err = cleanPaths(files); err != nil {
		return err
	}
	err = git.Add(files...)
	if err != nil {
		return err
	}
	opts.applyUserTo(git)
	return git.Commit(commitMessage("add", files))
}

func update(opts *Options, updated []string) (err error) {
	g := opts.git()
	updated, err = getUpdated(g, opts, updated)
	if err != nil {
		return err
	}
	err = g.Add(updated...)
	if err != nil {
		return err
	}
	opts.applyUserTo(g)
	g.SetOut(os.Stdout)
	return g.Commit(commitMessage("update", updated))
}

func getUpdated(g *git.Git, opts *Options, updated []string) ([]string, error) {
	if len(updated) > 0 {
		for i, f := range updated {
			updated[i] = filepath.Join(g.WorkingTree(), f)
		}
	}
	objects, err := g.Modifications()
	if err != nil {
		return nil, err
	}
	for _, o := range objects {
		updated = append(updated, filepath.Join(g.WorkingTree(), o.Name))
	}
	if opts.HasReadme() {
		updated = removeReadme(opts.Root, updated)
	}
	return updated, nil
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

func sync(g *git.Git) error {
	var (
		err error
	)
	if !g.HasRemote() {
		return errors.New("repo does not have a remote repo")
	}
	if err = execute(g.Cmd("pull")); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", err)
	}
	branch, err := g.CurrentBranch()
	if err != nil {
		return err
	}
	return execute(g.Cmd("push", "origin", branch))
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
	for i := 0; i < N; i++ {
		if i >= n {
			return true
		}
		if base[i] != test[i] {
			return false
		}
	}
	return false
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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

func writeGlobalConfig(opts *Options) error {
	filename := filepath.Join(opts.ConfigDir, "gitconfig")
	if !exists(filename) {
		f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			return err
		}
		if err = f.Close(); err != nil {
			return err
		}
	}
	return nil
}

type completeFunc func(
	_ *cobra.Command,
	_ []string,
	toComplete string,
) ([]string, cobra.ShellCompDirective)

func filesCompletionFunc(r dotfiles.Repo) completeFunc {
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

func modifiedCompletionFunc(r dotfiles.Repo) completeFunc {
	return func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		git := r.Git()
		files, err := git.ModifiedFiles()
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
			return writeGitignore(opts)
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
		return filepath.Join(dir, "."+name)
	}
	return ""
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
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
