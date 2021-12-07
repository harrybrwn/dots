package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	_ "unsafe"

	"github.com/harrybrwn/dots/git"
	"github.com/spf13/cobra"
)

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
		NewSetSSHKeyCmd(opts),
	)
	c.AddCommand(newUtilCommands(opts)...)
	return c
}

func NewSetSSHKeyCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use: "set-ssh-key <file>", Args: cobra.ExactArgs(1),
		Short: "Set an ssh identity file to be used on every remote operation.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.git().ConfigLocalSet(
				"core.sshcommand",
				fmt.Sprintf("ssh -i %s -o IdentitiesOnly=yes", args[0]),
			)
		},
	}
}

func NewGetCmd(opts *Options) *cobra.Command {
	var force bool
	c := &cobra.Command{
		Use:   "get <file>",
		Short: "Pull a single file out and write it the to current working directory.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			originTree := opts.Root
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
				return execute(git.Cmd(command...))
			}
			err = execute(git.Cmd("checkout", "--", args[0]))
			if err != nil {
				return err
			}
			// The "git checkout" command above will mark the file as modified,
			// This is a quick hack to undo that immediately after printing out
			// the file. This will ideally happen silently.
			git.SetWorkingTree(originTree)
			git.Cmd("--no-pager", "diff", "--name-only").Run()
			return nil
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
	c := &cobra.Command{
		Use:               "cat <filename>",
		Short:             "Print a file being tracked to standard out.",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: filesCompletionFunc(opts),
		RunE: func(cmd *cobra.Command, args []string) error {
			git := opts.git()
			name := args[0]
			if name[0] == '/' {
				name = name[1:]
			}
			c := git.Cmd("--no-pager", "show", fmt.Sprintf("HEAD:%s", name))
			c.Stdout = cmd.OutOrStdout()
			return execute(c)
		},
	}
	return c
}

func newUtilCommands(opts *Options) []*cobra.Command {
	return []*cobra.Command{
		{
			Use: "command", Short: "Print the internal git command being used",
			Aliases: []string{"cmd"},
			RunE: func(cmd *cobra.Command, args []string) error {
				g := opts.git()
				fmt.Fprintln(cmd.OutOrStdout(), strings.Join(g.Cmd().Args, " "))
				return nil
			},
		},
		{
			Use:   "modified",
			Short: "Print a table of info for modified files.",
			RunE: func(cmd *cobra.Command, args []string) error {
				git := opts.git()
				mods, err := git.Modifications()
				if err != nil {
					return err
				}
				if len(mods) == 0 {
					return nil
				}
				tab := tabwriter.NewWriter(cmd.OutOrStdout(), 3, 4, 1, ' ', 0)
				tab.Write([]byte("SOURCE\tDEST\tTYPE\tNAME\n"))
				for _, m := range mods {
					s := fmt.Sprintf("%s\t%s\t%c\t%s\n", m.Src.Hash, m.Dst.Hash, m.Type, m.Name)
					tab.Write([]byte(s))
				}
				return tab.Flush()
			},
		},
		{
			Use:   "objects",
			Short: "Print a table of info for git objects.",
			RunE: func(cmd *cobra.Command, args []string) error {
				git := opts.git()
				objects, err := git.Files()
				if err != nil {
					return err
				}
				tab := NewTable(cmd.OutOrStdout())
				tab.Head("HASH", "TYPE", "SIZE", "Name")
				for _, o := range objects {
					tab.Add(
						o.Hash,
						o.Type.String(),
						strconv.FormatInt(o.Size, 10),
						o.Name,
					)
				}
				return tab.Flush()
			},
		},
		{
			Use:   "graph",
			Short: "A fancy git log alias.",
			RunE: func(cmd *cobra.Command, args []string) error {
				git := opts.git()
				git.SetOut(cmd.OutOrStdout())
				c := git.Cmd(
					"logs", "--all", "--graph", "--abbrev-commit",
					"--decorate", "--oneline",
					"--date", "format:%a %b %d %l:%M:%S %P %Y",
				)
				return execute(c)
			},
		},
		{
			Use:   "add-readme",
			Short: "Add a README.md file to the git tree",
			RunE: func(cmd *cobra.Command, args []string) error {
				readme := filepath.Join(opts.ConfigDir, "README.md")
				if !exists(readme) {
					return fmt.Errorf("%q does not exist", readme)
				}
				git := git.New(opts.repo(), opts.ConfigDir)
				err := git.Add(readme)
				if err != nil {
					return err
				}
				return git.Commit("added readme")
			},
		},
	}
}

func NewTable(w io.Writer) *Table {
	return &Table{
		Header: make([]string, 0, 1),
		Body:   make([][]string, 0, 5),
		tab:    tabwriter.NewWriter(w, 2, 4, 1, ' ', 0)}
}

type Table struct {
	Header []string
	Body   [][]string
	tab    *tabwriter.Writer
}

func (t *Table) Head(header ...string) { t.Header = append(t.Header, header...) }
func (t *Table) Add(body ...string)    { t.Body = append(t.Body, body) }

func (t *Table) Flush() error {
	_, err := t.tab.Write([]byte(fmt.Sprintf("%s\n", strings.Join(t.Header, "\t"))))
	if err != nil {
		return err
	}
	for _, row := range t.Body {
		_, err = t.tab.Write([]byte(fmt.Sprintf("%s\n", strings.Join(row, "\t"))))
		if err != nil {
			return err
		}
	}
	return t.tab.Flush()
}

//go:linkname execute github.com/harrybrwn/dots/git.run
func execute(cmd *exec.Cmd) error

// func execute(cmd *exec.Cmd) error {
// 	var stderr bytes.Buffer
// 	cmd.Stderr = &stderr
// 	fmt.Println(cmd.Args)
// 	err := cmd.Run()
// 	if err != nil {
// 		msg := strings.Trim(stderr.String(), "\n")
// 		if len(msg) == 0 {
// 			return err
// 		}
// 		return fmt.Errorf("%s: %w", msg, err)
// 	}
// 	return nil
// }

func remove(index int, arr []string) []string {
	l := len(arr) - 1
	arr[index] = arr[l]
	return arr[:l]
}
