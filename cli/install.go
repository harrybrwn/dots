package cli

import (
	"archive/tar"
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
		to  string
	)
	c := &cobra.Command{
		Use:   "install",
		Short: "Copy all of the tracked files to the current root (will overwrite existing files)",
		RunE: func(cmd *cobra.Command, args []string) error {
			var (
				git = opts.Git()
				c   = git.Cmd("archive", "--format=tar", "HEAD")
			)
			pipe, err := c.StdoutPipe()
			if err != nil {
				return err
			}
			defer pipe.Close()
			if err = c.Start(); err != nil {
				return err
			}
			dest := git.WorkingTree()
			if len(to) > 0 {
				dest = to
			}
			cmd.Printf("installing to %q\n", dest)
			err = install(dest, opts, git, tar.NewReader(pipe), yes)
			if err != nil {
				return err
			}
			err = c.Wait()
			if err != nil {
				return err
			}
			err = git.RunCmd("restore", "--staged", opts.Root)
			if err != nil {
				return err
			}

			if exists(filepath.Join(opts.ConfigDir, ReadMeName)) {
				err = restoreReadMe(git)
				if err != nil {
					return errors.Wrap(err, "failed to restore repo's base README.md")
				}
			}
			err = git.RunCmd("update-index", "--refresh")
			if err != nil {
				return errors.Wrap(err, "failed to refresh index")
			}
			return nil
		},
	}
	f := c.Flags()
	f.BoolVarP(&yes, "yes", "y", yes, "set all yes-or-no prompts to yes")
	f.StringVar(&to, "to", "", "install to an alternate location")
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
		if rel, err := filepath.Rel(opts.Root, p); err == nil && rel == ReadMeName {
			p = filepath.Join(opts.ConfigDir, ReadMeName)
		}
		log := func(string, ...interface{}) {}
		if opts.verbose {
			log = func(f string, v ...interface{}) { fmt.Printf(f+"\n", v...) }
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
			log("created directory %q", p)
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
			log("wrote file %q", p)
		case tar.TypeSymlink:
			err = os.Symlink(p, filepath.Join(dest, header.Linkname))
			if err != nil {
				return errors.Wrap(err, "could not create symbolic link")
			}
			log("created symlink %q", p)
		case tar.TypeLink:
			err = os.Link(p, filepath.Join(dest, header.Linkname))
			if err != nil {
				return err
			}
			log("created link %q", p)
		case tar.TypeBlock:
			return errors.New("cannot handle file type 'block'")
		case tar.TypeFifo:
			return errors.New("cannot handle file type 'fifo'")
		}
	}
}

func restoreReadMe(g *git.Git) error {
	mods, err := g.Modifications()
	if err != nil {
		return err
	}
	for _, mod := range mods {
		if mod.Type == git.ModDelete && mod.Name == ReadMeName {
			err = g.RunCmd("restore", mod.Name)
			if err != nil {
				return err
			}
			break
		}
	}
	return nil
}
