package cli

import (
	"os"
	"path/filepath"
	"sort"
	"syscall"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func NewUninstallCmd(opts *Options) *cobra.Command {
	c := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove all managed files.",
		RunE: func(cmd *cobra.Command, args []string) error {
			g := opts.git()
			objects, err := g.Files()
			if err != nil {
				return err
			}
			dirs := make([]string, 0)
			for _, obj := range objects {
				f := filepath.Join(opts.Root, obj.Name)
				info, err := os.Stat(f)
				if err != nil {
					return err
				}
				if info.IsDir() {
					dirs = append(dirs, f)
					continue
				}
				err = os.Remove(f)
				if err != nil {
					return errors.Wrapf(err, "failed to uninstall file %q", f)
				}
			}
			directories := sort.StringSlice(dirs)
			sort.Sort(sort.Reverse(directories))
			for _, d := range dirs {
				err = os.Remove(d)
				if err != nil {
					if errors.Is(err, syscall.ENOTEMPTY) {
						cmd.Printf("%q is not empty, skipping\n", d)
						continue
					}
					return err
				}
			}
			cmd.Println("uninstall successful")
			return nil
		},
	}
	return c
}
