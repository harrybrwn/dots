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
			return listTree(cmd.OutOrStdout(), files, &flags)
		},
	}
	f := c.Flags()
	f.BoolVar(&flags.flat, "flat", flags.flat, "print as flat list")
	f.BoolVar(&flags.noPager, "no-pager", flags.noPager, "disable the automatic pager")
	return c
}

func listTree(out io.Writer, files []string, flags *lsFlags) error {
	var tr = tree.New(files)
	_, height, err := term.GetSize(0)
	if err != nil {
		return err
	}
	fn := tree.ColorFolders
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
