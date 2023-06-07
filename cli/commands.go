package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func NewInitCmd(opts *Options) *cobra.Command {
	c := cobra.Command{
		Use:   "init",
		Short: "Initialize an empty project",
		RunE: func(cmd *cobra.Command, args []string) error {
			if exists(opts.repo()) && !yesOrNo(os.Stdin, os.Stdout, fmt.Sprintf("Would you like to overwrite %q", opts.repo())) {
				return errors.Errorf("%q already exists", opts.repo())
			}
			os.RemoveAll(opts.repo())
			os.RemoveAll(opts.ConfigDir)
			if err := os.Mkdir(opts.ConfigDir, os.FileMode(0775)); err != nil {
				return err
			}
			if err := os.Mkdir(opts.repo(), os.FileMode(0775)); err != nil {
				return err
			}
			g := opts.Git()
			g.SetErr(cmd.ErrOrStderr())
			conf, err := g.Config()
			if err != nil {
				return err
			}
			if !conf.Exists("init.defaultBranch") {
				if err = g.ConfigGlobalSet("init.defaultBranch", DefaultBranch); err != nil {
					return err
				}
			}
			c := exec.Command("git", "init", "--bare", opts.repo())
			c.Stderr = cmd.ErrOrStderr()
			err = c.Run()
			if err != nil {
				return err
			}
			err = g.ConfigLocalSet("status.showUntrackedFiles", "no")
			if err != nil {
				return err
			}
			err = g.ConfigLocalSet("core.excludesFile", opts.excludesFile())
			if err != nil {
				return err
			}
			err = writeGitignore(opts)
			if err != nil {
				return err
			}
			return nil
		},
	}
	return &c
}

func NewUndoCmd(opts *Options) *cobra.Command {
	c := cobra.Command{
		Use:   "undo",
		Short: "Undo the last add, rm, or update operation.",
		RunE: func(cmd *cobra.Command, args []string) error {
			g := opts.Git()
			err := g.RunCmd("reset", "--soft", "HEAD~1")
			if err != nil {
				return err
			}
			err = g.RunCmd("reset")
			if err != nil {
				return err
			}
			return nil
		},
	}
	return &c
}
