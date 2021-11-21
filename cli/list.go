package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/harrybrwn/dots/git"
	"github.com/harrybrwn/dots/tree"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type lsFlags struct {
	*Options
	flat    bool
	noPager bool
}

func NewLSCmd(opts *Options) *cobra.Command {
	flags := lsFlags{Options: opts}
	c := &cobra.Command{
		Use:   "ls",
		Short: "List the files being tracked",
		RunE: func(cmd *cobra.Command, args []string) error {
			g := git.New(opts.repo(), opts.Root)
			files, err := g.LsFiles()
			if err != nil {
				return err
			}
			if flags.flat {
				return listFlat(cmd.OutOrStdout(), files, &flags)
			}
			mods, err := modifiedSet(g)
			if err != nil {
				return err
			}
			return listTree(cmd.OutOrStdout(), files, mods, &flags)
		},
	}
	f := c.Flags()
	f.BoolVar(&flags.flat, "flat", flags.flat, "print as flat list")
	f.BoolVar(&flags.noPager, "no-pager", flags.noPager, "disable the automatic pager")
	return c
}

func listTree(out io.Writer, files []string, mods modSet, flags *lsFlags) error {
	var tr = tree.New(files)
	_, height, err := term.GetSize(0)
	if !flags.noPager && err != nil {
		fmt.Fprintf(os.Stderr, "Could not get terminal size: %v\n", err)
		return err
	}
	fn := mods.treeColor
	if flags.NoColor {
		fn = tree.NoColor
	}
	if !flags.noPager && tree.PrintHeight(tr) > height {
		var buf bytes.Buffer
		if err = tree.PrintColor(&buf, tr, fn); err != nil {
			return err
		}
		return page(out, &buf)
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
		_, err = buf.WriteString(fmt.Sprintf("%s\n", f))
		if err != nil {
			return err
		}
	}
	if !flags.noPager && len(files) > height {
		return page(out, &buf)
	}
	_, err = io.Copy(out, &buf)
	return err
}

func page(stdout io.Writer, in io.Reader) error {
	var (
		pager string
		args  = make([]string, 0)
	)
	p, ok := os.LookupEnv("GIT_PAGER")
	if ok {
		pager = p
	} else {
		p, ok = os.LookupEnv("PAGER")
		if !ok {
			pager = "less"
		}
		pager = p
	}
	if pager == "less" {
		args = append(args, "--raw-control-chars")
	}
	cmd := exec.Command(pager, args...)
	cmd.Stdout = stdout
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}
	go func() {
		defer stdin.Close()
		io.Copy(stdin, in)
	}()
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to run pager %q: %w", pager, err)
	}
	return nil
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
