package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/matryer/is"
)

func TestClone(t *testing.T) {
	t.Parallel()
	is := is.New(t)
	tmp := t.TempDir()
	cmd := NewRootCmd()
	err := cmd.ParseFlags([]string{
		"-d", filepath.Join(tmp, "tree"),
		"-c", tmp,
	})
	is.NoErr(err)
	c, args, err := cmd.Find([]string{
		"clone",
		"https://github.com/harrybrwn/utest",
	})
	is.NoErr(err)
	is.NoErr(c.RunE(c, args))
	_, err = os.Stat(filepath.Join(tmp, repo))
	if os.IsNotExist(err) {
		t.Error("headless repo should exist")
	}
}

func TestRemoveReadme(t *testing.T) {
	is := is.New(t)
	files := []string{
		"/path/to/another",
		"/path/to/README.md",
		"/path/to/internal/README.md",
		"/home/user/.bashrc",
	}
	files = removeReadme("/path/to/", files)
	is.Equal(len(files), 3)
	is.Equal(files[0], "/path/to/another")
	is.Equal(files[1], "/home/user/.bashrc")
	is.Equal(files[2], "/path/to/internal/README.md")
}

func TestDirContainsPath(t *testing.T) {
	type table struct {
		dir  string
		path string
		exp  bool
	}
	for _, tt := range []table{
		{dir: "/", path: "/", exp: false},
		{dir: "/", path: "/x", exp: true},
		{dir: "/x", path: "/", exp: false},
		{dir: "/home/jim", path: "/home/jim/.config/dots/repo", exp: true},
		{dir: "/home/jim/.cache", path: "/home/jim/.config/dots/repo", exp: false},
	} {
		res := dirContainsPath(tt.dir, tt.path)
		if res != tt.exp && tt.exp {
			t.Errorf("path %q is expected to be a subpath of %q", tt.path, tt.dir)
			return
		}
		if res != tt.exp && !tt.exp {
			t.Errorf("path %q is not expected to be a subpath of %q", tt.path, tt.dir)
			return
		}
	}
}
