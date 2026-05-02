package cli

import (
	"os"
	"path/filepath"

	"github.com/harrybrwn/dots/cli/dotfiles"
	"github.com/harrybrwn/dots/git"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func NewUpdateCmd(opts *Options) *cobra.Command {
	c := cobra.Command{
		Use:   "update [files...]",
		Short: "Update files in local git repo that have been modified",
		Long: "" +
			"Update is similar to 'add' in that it updates\n" +
			"the internal repository with new changes except that\n" +
			"it automatically updates files that have already\n" +
			"been added and have changed since the last update.",
		Example: "  $ dots update\n" +
			"  $ dots update ~/.bashrc",
		SuggestFor: []string{"add"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return update(opts, args)
		},
		ValidArgsFunction: modifiedCompletionFunc(opts),
	}
	opts.addUserFlags(c.Flags())
	return &c
}

func update(opts *Options, updated []string) (err error) {
	g := opts.git()
	err = g.Cmd("pull").Run()
	if err != nil {
		return errors.Wrap(err, "failed to pull before updating")
	}
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
