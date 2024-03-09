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
)

type CLI interface {
	dotfiles.Repo
	stdio.ColorOption
}

type lsFlags struct {
	CLI
	flat    bool
	noPager bool
}

func NewLSCmd(cli CLI) *cobra.Command {
	flags := lsFlags{CLI: cli}
	c := &cobra.Command{
		Use:   "ls",
		Short: "List the files being tracked",
		RunE: func(cmd *cobra.Command, args []string) error {
			g := cli.Git()
			files, err := g.LsFiles()
			if err != nil {
				return err
			}
			// filenames := make([]string, 0, len(files))
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
			}
			mods, err := modifiedSet(g)
			if err != nil {
				return err
			}
			return listTree(cmd.OutOrStdout(), tr, mods, &flags)
		},
		ValidArgsFunction: lsCompletionFunc(cli),
	}
	f := c.Flags()
	f.BoolVar(&flags.flat, "flat", flags.flat, "print as flat list")
	f.BoolVar(&flags.noPager, "no-pager", flags.noPager, "disable the automatic pager")
	return c
}

func listTree(out io.Writer, tr *tree.Node, mods modSet, flags *lsFlags) error {
	// var tr = tree.New(files)
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
		_, err = buf.WriteString(fmt.Sprintf("%s\n", f))
		if err != nil {
			return err
		}
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

func lsCompletionFunc(repo dotfiles.Repo) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
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
