package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/harrybrwn/dots/git"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

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
