package cli

import (
	"bytes"
	"fmt"
	"io"

	"github.com/harrybrwn/dots/git"
	"github.com/harrybrwn/dots/tree"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func NewLSCmd(opts *Options) *cobra.Command {
	var (
		flat bool
	)
	c := &cobra.Command{
		Use:   "ls",
		Short: "List the files being tracked",
		RunE: func(cmd *cobra.Command, args []string) error {
			g := git.New(opts.repo(), opts.Root)
			files, err := g.LsFiles()
			if err != nil {
				return err
			}
			if flat {
				return listFlat(cmd.OutOrStdout(), files)
			}
			return listTree(cmd.OutOrStdout(), files, opts.NoColor)
		},
	}
	flags := c.Flags()
	flags.BoolVar(&flat, "flat", flat, "print as flat list")
	return c
}

func listTree(out io.Writer, files []string, nocolor bool) error {
	var tr = tree.New(files)
	_, height, err := term.GetSize(0)
	if err != nil {
		return err
	}
	if tree.PrintHeight(tr) > height {
		var buf bytes.Buffer
		if err = tree.Print(&buf, tr); err != nil {
			return err
		}
		return page(out, &buf)
	}
	fn := tree.ColorFolders
	if nocolor {
		fn = tree.NoColor
	}
	return tree.PrintColor(out, tr, tree.ColorFolders)
}

func listFlat(out io.Writer, files []string) error {
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
	if len(files) > height {
		return page(out, &buf)
	}
	_, err = io.Copy(out, &buf)
	return err
}
