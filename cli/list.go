package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/harrybrwn/dots/cli/dotfiles"
	"github.com/harrybrwn/dots/git"
	"github.com/harrybrwn/dots/pkg/stdio"
	"github.com/harrybrwn/dots/tree"
	"github.com/harrybrwn/dots/tui"
)

type CLI interface {
	dotfiles.Repo
	stdio.ColorOption
}

type lsFlags struct {
	CLI
	flat, tree bool
	noPager    bool
	untracked  bool
	changed    bool
}

func NewLSCmd(cli *Options) *cobra.Command {
	flags := lsFlags{CLI: cli}
	test := false
	c := &cobra.Command{
		Use:   "ls",
		Short: "List the files being tracked",
		RunE: func(cmd *cobra.Command, args []string) error {
			g := cli.Git()
			if flags.untracked {
				dir := "."
				if len(args) > 0 {
					dir = args[0]
				}
				return untracked(
					cmd.OutOrStdout(),
					g,
					filepath.Join(cli.ConfigDir, "ignore"),
					&flags,
					dir,
				)
			}
			files, err := g.LsFiles()
			if err != nil {
				return err
			}
			tr := tree.New(files)

			if len(args) > 0 {
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}
				filter := make([]string, 0)
				for _, arg := range args {
					path := arg
					if !filepath.IsAbs(path) {
						path = filepath.Join(cwd, path)
					}
					rel, err := filepath.Rel(g.WorkingTree(), path)
					if err != nil {
						return err
					}
					if rel == "." {
						continue
					}
					filter = append(filter, rel)
				}
				tr = tr.FilterBy(filter...)
			}

			if flags.flat {
				return listFlat(cmd.OutOrStdout(), tr.ListPaths(), &flags)
			} else if flags.tree {
				mods, err := modifiedSet(g)
				if err != nil {
					return err
				}
				return listTree(cmd.OutOrStdout(), tr, mods, &flags)
			}
			mods, err := modifiedSet(g)
			if err != nil {
				return err
			}
			var tree *tui.TreeTree
			if flags.changed {
				tree = tui.NewModifiedTree(tr, mods)
			} else {
				tree = tui.NewTree(tr, mods)
			}
			return tui.Run(tree)
		},
		ValidArgsFunction: lsCompletionFunc(cli),
	}
	f := c.Flags()
	f.BoolVarP(&flags.flat, "flat", "f", flags.flat, "print as flat list")
	f.BoolVar(&flags.tree, "tree", flags.tree, "show files as a simple tree")
	f.BoolVarP(&flags.untracked, "untracked", "u", flags.untracked, "show only untracked files")
	f.BoolVarP(&flags.changed, "changed", "M", flags.changed, "display changed files")
	f.BoolVar(&flags.noPager, "no-pager", flags.noPager, "disable the automatic pager")
	f.BoolVar(&test, "test", test, "")
	_ = f.MarkHidden("test")
	return c
}

func listTree(out io.Writer, tr *tree.Node, mods modSet, flags *lsFlags) error {
	_, height, err := term.GetSize(0)
	if !flags.noPager && err != nil {
		fmt.Fprintf(os.Stderr, "Could not get terminal size: %v\n", err)
		return err
	}
	fn := mods.treeColor
	if flags.NoColor() {
		fn = mods.treeNoColor
	}
	pager := stdio.FindPager()
	if pager == "" {
		flags.noPager = true
	}
	if !flags.noPager && tree.PrintHeight(tr) > height {
		var buf bytes.Buffer
		if err = tree.PrintColor(&buf, tr, fn); err != nil {
			return err
		}
		return stdio.Page(pager, out, &buf)
	}
	return tree.PrintColor(out, tr, fn)
}

func listFlat(out io.Writer, files []string, flags *lsFlags) error {
	_, height, err := term.GetSize(0)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	for _, f := range files {
		if f[0] == '/' {
			f = f[1:]
		}
		fmt.Fprintf(&buf, "%s\n", f)
	}
	pager := stdio.FindPager()
	if pager == "" {
		flags.noPager = true
	}
	if !flags.noPager && len(files) > height {
		return stdio.Page(pager, out, &buf)
	}
	_, err = io.Copy(out, &buf)
	return err
}

func untracked(
	out io.Writer,
	g *git.Git,
	excludeFile string,
	flags *lsFlags,
	dir string,
) (err error) {
	tr := tree.New(nil)
	cwd, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	var b bytes.Buffer
	cmd := g.Cmd(
		"-c", "status.showuntrackedfiles=yes",
		"ls-files", "-zo",
		"--exclude-standard",
		fmt.Sprintf("--exclude-from=%s", excludeFile),
		"--",
		cwd,
	)
	cmd.Stdout = &b
	cmd.Stderr = &b
	if err := cmd.Run(); err != nil {
		return err
	}
	for f := range strings.SplitSeq(b.String(), "\x00") {
		if len(f) == 0 {
			continue
		}
		tr.Add(f)
	}
	if flags.flat {
		return listFlat(out, tr.ListPaths(), flags)
	}
	var (
		buf   bytes.Buffer
		pager = stdio.FindPager()
	)
	_, height, err := term.GetSize(0)
	if err != nil {
		return err
	}
	if pager == "" {
		flags.noPager = true
	}
	if err = tree.PrintColor(&buf, tr, func(n *tree.Node) string {
		if n.Type == tree.TreeNode {
			return tree.DirColor(n)
		}
		return "\x1b[01;32mU\x1b[0m "
	}); err != nil {
		return err
	}
	if !flags.noPager && tree.PrintHeight(tr) > height {
		return stdio.Page(pager, out, &buf)
	}
	_, err = io.Copy(out, &buf)
	return err
}

func lsCompletionFunc(
	repo dotfiles.Repo,
) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(
		_ *cobra.Command,
		_ []string,
		toComplete string,
	) ([]string, cobra.ShellCompDirective) {
		files, err := repo.Git().LsFiles()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		set := make(map[string]struct{}, len(files))
		names := make([]string, 0, len(files))
		for _, f := range files {
			pathlist := strings.Split(f, string(filepath.Separator))
			if len(pathlist) == 0 {
				continue
			}
			_, ok := set[pathlist[0]]
			if !ok {
				set[pathlist[0]] = struct{}{}
				names = append(names, pathlist[0])
			}
		}
		return names, cobra.ShellCompDirectiveDefault
	}
}

func modifiedSet(g *git.Git) (modSet, error) {
	m := make(modSet)
	files, err := g.Modifications()
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		m[f.Name] = f.Type
		parts := strings.Split(f.Name, string(filepath.Separator))
		if len(parts) > 1 {
			for i := 1; i < len(parts); i++ {
				p := strings.Join(parts[:i], string(filepath.Separator))
				m[p] = tui.ModifiedInDirectory
			}
		}
	}
	return m, nil
}

type modSet map[string]git.ModType

func (ms modSet) treeColor(n *tree.Node) string {
	switch n.Type {
	case tree.LeafNode:
		p := filepath.Join(n.Path(), n.Name)
		t, ok := ms[p[1:]]
		if ok {
			col := 33
			switch t {
			case 'D':
				if n.Path() == "/" && n.Name == ReadMeName {
					return ""
				}
				col = 31
			case 'M':
				col = 33
			case git.ModAddition, git.ModRename:
				col = 32
			case git.ModUnmerged:
				col = 35
			}
			return fmt.Sprintf("\x1b[01;%dm%c \x1b[0m", col, t)
		}
		return ""
	case tree.TreeNode:
		return "\x1b[01;34m"
	default:
		return ""
	}
}

func (ms modSet) treeNoColor(n *tree.Node) string {
	if n.Type == tree.LeafNode {
		p := filepath.Join(n.Path(), n.Name)
		t, ok := ms[p[1:]]
		if ok {
			return fmt.Sprintf("%c ", t)
		}
	}
	return ""
}
