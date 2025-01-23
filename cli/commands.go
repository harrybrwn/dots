package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/harrybrwn/dots/cli/dotfiles"
)

func NewInitCmd(opts *Options) *cobra.Command {
	c := cobra.Command{
		Use:   "init",
		Short: "Initialize an empty project",
		RunE: func(cmd *cobra.Command, args []string) error {
			if exists(opts.repo()) &&
				!yesOrNo(
					os.Stdin,
					os.Stdout,
					fmt.Sprintf("Would you like to overwrite %q", opts.repo()),
				) {
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

func NewPullCmd(r dotfiles.Repo) *cobra.Command {
	c := cobra.Command{
		Use:   "pull",
		Short: "Download changes from the git repo",
		Long:  `Download changes from the git repo. Similar to 'git pull'.`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			g := r.Git()
			g.SetErr(cmd.ErrOrStderr())
			g.SetOut(cmd.OutOrStdout())
			return g.Cmd("pull").Run()
		},
	}
	return &c
}

func NewDiffCmd(r dotfiles.Repo) *cobra.Command {
	c := cobra.Command{
		Use:   "diff",
		Short: "Display a diff of the currently tracked files",
		Long: `Display a diff of the currently tracked files by running 'git diff' under the
hood.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			g := r.Git()
			g.SetErr(cmd.ErrOrStderr())
			g.SetOut(cmd.OutOrStdout())
			c := g.Cmd("diff")
			for _, arg := range args {
				if strings.HasPrefix("-", arg) {
					return fmt.Errorf("you're not allowed to pass flags like %q to 'git diff'", arg)
				}
				c.Args = append(c.Args, arg)
			}
			return c.Run()
		},
	}
	return &c
}

func NewStatusCmd(r dotfiles.Repo) *cobra.Command {
	c := &cobra.Command{
		Use:   "status",
		Short: "Show the status of files being tracked",
		RunE: func(cmd *cobra.Command, args []string) error {
			g := r.Git()
			g.SetErr(cmd.ErrOrStderr())
			g.SetOut(cmd.OutOrStdout())
			err := g.Cmd(
				"--no-pager",
				"-c", "color.status=always",
				"diff", "--stat",
			).Run()
			if err != nil {
				return err
			}
			return g.Cmd(
				"-c", "color.status=always",
				"status",
			).Run()
		},
	}
	return c
}
