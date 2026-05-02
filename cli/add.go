package cli

import (
	"github.com/harrybrwn/dots/git"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func NewAddCmd(opts *Options) *cobra.Command {
	var up bool // --update
	c := &cobra.Command{
		Use:   "add <file...>",
		Short: "Add new files",
		Args:  cobra.MinimumNArgs(1),
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
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return nil, cobra.ShellCompDirectiveDefault
		},
	}
	c.Flags().BoolVarP(&up, "update", "u", up, "update any changed files as well as add new ones")
	opts.addUserFlags(c.Flags())
	return c
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
