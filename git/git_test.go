package git

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func dirs(tmp string) (gitdir, tree string) {
	gitdir = filepath.Join(tmp, "repo")
	tree = filepath.Join(tmp, "tree")
	os.Mkdir(gitdir, 0755)
	os.Mkdir(tree, 0755)
	return
}

func ls(d string) {
	dirs, _ := os.ReadDir(d)
	fmt.Println(d)
	for _, e := range dirs {
		i, _ := e.Info()
		fmt.Printf("%v %s\n", i.Mode(), e.Name())
	}
}

func TestLsTree(t *testing.T) {
	tmp := t.TempDir()
	gd, tr := dirs(tmp)
	git := New(gd, tr)
	git.SetOut(io.Discard)
	git.SetErr(io.Discard)
	git.SetPersistentArgs([]string{"-c", "commit.gpgsign=false"})
	err := git.InitBare()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(git.WorkingTree(), "file.txt")
	if err = touch(path); err != nil {
		t.Fatal(err)
	}
	if err = git.Add(path); err != nil {
		t.Fatal(err)
	}
	err = git.Commit("first commit")
	if err != nil {
		t.Fatal(err)
	}
	files, err := git.LsFiles()
	if err != nil {
		t.Fatal(err)
	}
	if files[0] != "file.txt" {
		t.Error("did not add the right file name")
	}
}

func TestModifiedFiles(t *testing.T) {
	tmp := t.TempDir()
	gd, tr := dirs(tmp)
	git := New(gd, tr)
	git.SetOut(io.Discard)
	git.SetErr(io.Discard)
	git.SetPersistentArgs([]string{"-c", "commit.gpgsign=false"})
	err := setupTestRepo(
		git,
		newfile("test.txt", "this is a test"),
		newfile("x", "this is not an x"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err = git.Add("test.txt", "x"); err != nil {
		t.Fatal(err)
	}
	if err = git.Commit("first commit"); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(tr, "test.txt"), os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Write([]byte("hello")); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()
	files, err := git.ModifiedFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("there should be one modified file")
	}
	if files[0] != "test.txt" {
		t.Error("wrong file is being showed as modified")
	}
}

func TestPrintFile(t *testing.T) {
	// So, when running 'git checkout -- ./some/file' it marks that file as
	// modified when it was not. This is a test to figure out why this happens
	// and if I can stop it from happening.
	git := testgit(t)
	err := setupTestRepo(
		git,
		newfile("one", "this is the first"),
		newfile("two", "this is the second"),
		newfile("three", "this is the third"),
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, err = range []error{
		git.Add("."), git.Commit("x"),
		// appendfile(filepath.Join(git.WorkingTree(), "one"), "\nand this is on the second line"),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}
	git.SetOut(os.Stdout)
	git.SetErr(os.Stderr)

	tree := git.WorkingTree()
	tree2 := filepath.Join(filepath.Dir(git.WorkingTree()), "tree2")
	os.Mkdir(tree2, 0755)
	git.SetWorkingTree(tree2)
	// fmt.Println(git.Cmd().Args)
	err = git.Cmd("checkout", "--", "one").Run()
	if err != nil {
		t.Fatal(err)
	}

	git.SetWorkingTree(tree)
	// files, err := git.Modifications()
	// if err != nil {
	// 	t.Fatal(err)
	// }
	// for _, f := range files {
	// 	fmt.Println(f)
	// }
	// println()

	// cmd := git.Cmd("--no-pager", "diff-files")
	// cmd.Stderr = nil
	// cmd.Stdout = nil
	// cmd.Run()

	// git.Cmd("--no-pager", "diff-tree", "-r", "HEAD").Run()
	git.Cmd("--no-pager", "diff", "--name-only").Run()
	// git.Cmd("diff-tree", "HEAD", "-r", "--stat", "--full-index", "--text").Run()
	// git.Cmd("symbolic-ref", "--short", "--quiet", "HEAD").Run()
	// git.Cmd("rev-parse", "--short", "HEAD").Run()

	// println()
	files, err := git.Modifications()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Error("here should be no modified files")
		for _, f := range files {
			fmt.Println(f)
		}
	}
}

func testgit(t *testing.T) *Git {
	tmp := t.TempDir()
	gd, tr := dirs(tmp)
	git := New(gd, tr)
	git.SetOut(io.Discard)
	git.SetErr(io.Discard)
	git.SetPersistentArgs([]string{"-c", "commit.gpgsign=false"})
	return git
}

func touch(filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	return f.Close()
}

func setupTestRepo(g *Git, files ...fs.File) (err error) {
	if !g.Exists() {
		if err = g.InitBare(); err != nil {
			return err
		}
	}
	for _, f := range files {
		stat, err := f.Stat()
		if err != nil {
			return err
		}
		p := filepath.Join(g.WorkingTree(), stat.Name())
		if stat.IsDir() {
			err = os.Mkdir(p, 0755)
			if err != nil {
				return err
			}
		} else {
			file, err := os.Create(p)
			if err != nil {
				return err
			}
			_, err = io.Copy(file, f)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func newfile(name, content string) fs.File {
	return &file{
		name: name,
		b:    *bytes.NewBuffer([]byte(content)),
	}
}

func appendfile(name, content string) error {
	f, err := os.OpenFile(name, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(content)
	return err
}

type file struct {
	b    bytes.Buffer
	name string
}

func (f *file) Read(b []byte) (int, error) { return f.b.Read(b) }
func (f *file) Stat() (fs.FileInfo, error) { return f, nil }
func (f *file) Close() error               { return nil }
func (f *file) Name() string               { return f.name }
func (f *file) Size() int64                { return int64(f.b.Len()) }
func (f *file) Mode() fs.FileMode          { return fs.FileMode(0644) }
func (f *file) ModTime() time.Time         { return time.Now() }
func (f *file) IsDir() bool                { return false }
func (f *file) Sys() interface{}           { return nil }
