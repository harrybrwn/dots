package cli

import (
	"fmt"
	"os"

	"github.com/harrybrwn/dots/cli/dotfiles"
	"github.com/harrybrwn/dots/git"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func NewSyncCmd(r dotfiles.Repo) *cobra.Command {
	c := &cobra.Command{
		Use:   "sync",
		Short: "Sync with the remote repository",
		Long:  "Download updates in the remote repo and push local updates to the remote repo.",
		RunE: func(*cobra.Command, []string) error {
			return sync(r.Git())
		},
	}
	return c
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
	err = execute(g.Cmd("push", "origin", branch))
	if err != nil {
		return err
	}
	head, err := g.Head()
	if err != nil {
		return err
	}
	err = g.CreateRemoteRef("origin", branch, head)
	if err != nil {
		return err
	}
	return nil
}
