package cli

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/harrybrwn/dots/git"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func NewInstallCmd(opts *Options) *cobra.Command {
	var (
		yes bool
	)
	c := &cobra.Command{
		Use:   "install [location]",
		Short: "Copy all of the tracked files to the current root (will overwrite existing files)",
		RunE: func(cmd *cobra.Command, args []string) error {
			var (
				git = opts.Git()
				c   = git.Cmd("archive", "--format=tar.gz", "HEAD")
			)
			pipe, err := c.StdoutPipe()
			if err != nil {
				return err
			}
			defer pipe.Close()
			if err = c.Start(); err != nil {
				return err
			}
			r, err := gzip.NewReader(pipe)
			if err != nil {
				return err
			}
			dest := git.WorkingTree()
			if len(args) > 0 {
				dest = args[0]
			}
			cmd.Printf("installing to %q\n", dest)
			err = install(dest, opts, git, tar.NewReader(r), yes)
			if err != nil {
				return err
			}
			return c.Wait()
		},
	}
	f := c.Flags()
	f.BoolVarP(&yes, "yes", "y", yes, "set all yes-or-no prompts to yes")
	return c
}

func install(dest string, opts *Options, git *git.Git, archive *tar.Reader, yes bool) error {
	for {
		header, err := archive.Next()
		switch err {
		case nil:
		case io.EOF:
			return nil
		default:
			return errors.Wrap(err, "could not get next tar header")
		}
		p := filepath.Join(dest, header.Name)
		if filepath.Base(header.Name) == ReadMeName {
			p = filepath.Join(opts.ConfigDir, ReadMeName)
		}
		if !yes && exists(p) {
			if !yesOrNo(
				os.Stdin, os.Stdout,
				fmt.Sprintf("would you like to overwrite %q", p),
			) {
				continue
			}
		}
		perm := header.FileInfo().Mode().Perm()
		switch header.Typeflag {
		case tar.TypeDir:
			err = os.MkdirAll(p, perm)
			if err != nil {
				if os.IsExist(err) {
					continue
				}
				return errors.Wrap(err, "could not create directory")
			}
		case tar.TypeReg:
			f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY, perm)
			if err != nil {
				return err
			}
			_, err = io.Copy(f, archive)
			if err != nil {
				f.Close()
				return errors.Wrap(err, "failed to copy file")
			}
			if err = f.Close(); err != nil {
				return errors.Wrap(err, "failed to close file")
			}
		case tar.TypeSymlink:
			err = os.Symlink(p, filepath.Join(dest, header.Linkname))
			if err != nil {
				return errors.Wrap(err, "could not create symbolic link")
			}
		case tar.TypeLink:
			err = os.Link(p, filepath.Join(dest, header.Linkname))
			if err != nil {
				return err
			}
		case tar.TypeBlock:
		case tar.TypeFifo:
		}
	}
}
