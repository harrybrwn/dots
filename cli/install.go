package cli

import (
	"archive/tar"
	"container/list"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/harrybrwn/dots/git"
)

func NewInstallCmd(opts *Options) *cobra.Command {
	var (
		yes    bool
		to     string
		dryRun bool
	)
	c := &cobra.Command{
		Use:   "install [source]",
		Short: "Copy all of the tracked files to the current root",
		Long: `Copy all of the tracked files to the current root (will overwrite existing
files). Also optionally clone from a remove source before installing.
`,
		Aliases: []string{"i"},
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			var (
				git = opts.Git()
				c   = git.Cmd("archive", "--format=tar", "HEAD")
			)
			if len(args) > 0 {
				if git.Exists() {
					return errors.New("git repository already exists here")
				}
				err := clone(opts, git, args[0])
				if err != nil {
					return err
				}
			}
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

			defer func() {
				e := c.Wait()
				if e != nil && err == nil {
					err = e
					return
				}
				env := map[string]string{
					"GIT_CONFIG_NOSYSTEM": "1", // skip the global config
				}
				e = git.RunCmdWithEnv(env, "restore", "--staged", opts.Root)
				if e != nil && err == nil {
					err = e
					return
				}
				if opts.HasReadme() {
					e = restoreReadMe(git)
					if e != nil && err == nil {
						err = errors.Wrap(e, "failed to restore repo's base README.md")
						return
					}
				}
				e = git.RunCmdWithEnv(env, "update-index", "--refresh")
				if e != nil && err == nil {
					err = errors.Wrap(err, "failed to refresh index")
				}
			}()
			cmd.Printf("installing to %q\n", dest)
			err = install(opts, dest, tar.NewReader(pipe), yes)
			if err != nil {
				return err
			}
			return
		},
	}
	f := c.Flags()
	f.BoolVarP(&yes, "yes", "y", yes, "set all yes-or-no prompts to yes")
	f.StringVar(&to, "to", "", "install to an alternate location")
	f.BoolVar(&dryRun, "dry-run", dryRun, "run the install without writing anything to disk")
	return c
}

type link struct {
	sym bool
	dst string
	src string
}

func install(opts *Options, dest string, archive *tar.Reader, yes bool) error {
	symlinks := list.New()
	log := opts.log()
	for {
		header, err := archive.Next()
		switch err {
		case nil:
		case io.EOF:
			goto finish
		default:
			return errors.Wrap(err, "could not get next tar header")
		}
		p := filepath.Join(dest, header.Name)
		if rel, err := filepath.Rel(opts.Root, p); err == nil && rel == ReadMeName {
			p = filepath.Join(opts.ConfigDir, ReadMeName)
		}
		if !yes && existsAndIsNotDir(p) {
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
			f, err := os.OpenFile(p, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, perm)
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
			l := link{
				sym: true,
				src: p,
				dst: header.Linkname,
			}
			symlinks.PushBack(l)
		case tar.TypeLink:
			l := link{
				src: p,
				dst: header.Linkname,
			}
			symlinks.PushBack(l)
		case tar.TypeBlock:
			return errors.New("cannot handle file type 'block'")
		case tar.TypeFifo:
			return errors.New("cannot handle file type 'fifo'")
		}
	}

finish:
	var err error
	for symlinks.Len() > 0 {
		l := symlinks.Remove(symlinks.Front()).(link)
		base, lnname := filepath.Split(l.src)
		ln, msg := os.Link, "link"
		if l.sym {
			ln = os.Symlink
			msg = "symlink"
		}
		e := changeDir(base, func() error {
			return ln(l.dst, lnname)
		})
		if e != nil {
			if err == nil {
				err = errors.Wrap(e, "could not create symbolic link")
			}
			fmt.Fprintf(os.Stderr, "error: failed to create %s %q -> %q\n", msg, l.src, l.dst)
			continue
		}
		log("created %s %q -> %q", msg, l.src, l.dst)
	}
	return err
}

func restoreReadMe(g *git.Git) error {
	mods, err := g.Modifications()
	if err != nil {
		return err
	}
	for _, mod := range mods {
		if mod.Type == git.ModDelete && mod.Name == ReadMeName {
			err = g.RunCmdWithEnv(map[string]string{
				"GIT_CONFIG_NOSYSTEM": "1", // skip reading the global config
			}, "restore", mod.Name)
			if err != nil {
				return err
			}
			break
		}
	}
	return nil
}

func changeDir(dst string, fn func() error) (err error) {
	var prev string
	prev, err = os.Getwd()
	if err != nil {
		return err
	}
	err = os.Chdir(dst)
	if err != nil {
		return err
	}
	defer func() {
		e := os.Chdir(prev)
		if e != nil && err == nil {
			err = e
		}
	}()
	err = fn()
	if err != nil {
		return err
	}
	return nil
}
