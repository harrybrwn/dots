package git

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/matryer/is"
)

func dirs(tmp string) (gitdir, tree string) {
	gitdir = filepath.Join(tmp, "repo")
	tree = filepath.Join(tmp, "tree")
	os.Mkdir(gitdir, 0755)
	os.Mkdir(tree, 0755)
	return
}

func TestNew(t *testing.T) {
	is := is.New(t)
	tmp := t.TempDir()
	gitdir, worktree := dirs(tmp)
	g := New(gitdir, worktree)
	is.Equal(g.GitDir(), gitdir)
	is.Equal(g.WorkingTree(), worktree)
	is.NoErr(setupTestRepo(g))
	is.True(exists(g.WorkingTree()))
	is.True(exists(g.GitDir()))
}

func TestGit_Cmd(t *testing.T) {
	is := is.New(t)
	tmp := t.TempDir()
	gd, wt := dirs(tmp)
	g := New(gd, wt)
	cmd := g.Cmd()
	is.Equal(len(cmd.Args), 5)
	is.Equal(cmd.Args[0], "git")
	is.Equal(cmd.Args[1], "--git-dir")
	is.Equal(cmd.Args[2], g.GitDir())
	is.Equal(cmd.Args[3], "--work-tree")
	is.Equal(cmd.Args[4], g.WorkingTree())
	is.Equal(cmd.Args, []string{"git", "--git-dir", g.GitDir(), "--work-tree", g.WorkingTree()})
}

func TestGit_Remove(t *testing.T) {
	is := is.New(t)
	m := meta(t)
	g := m.Git()
	err := setupTestRepo(
		g,
		newfile("file", "this is a file"),
	)
	is.NoErr(err)
	is.NoErr(g.Add("."))
	is.NoErr(g.Commit("first commit"))
	is.NoErr(g.Remove("file"))
	mods, err := g.Modifications()
	is.NoErr(err)
	is.Equal(len(mods), 1)
	is.Equal(mods[0].Name, "file")
	is.Equal(mods[0].Type, ModDelete)
	is.True(exists(filepath.Join(g.WorkingTree(), "file")))

	err = g.Remove("not-here")
	is.True(err != nil) // should return error for non-existant file
	err = g.Add()
	is.True(err != nil) // should return error if no files given
}

func TestGit_HasRemote(t *testing.T) {
	is := is.New(t)
	m := meta(t)
	g := m.Git()
	is.NoErr(g.InitBare())
	is.True(!g.HasRemote())
	is.NoErr(g.RunCmd("remote", "add", "origin", "https://github.com/harrybrwn/dots"))
	is.True(g.HasRemote())
}

func TestGit_CurrentBranch(t *testing.T) {
	is := is.New(t)
	m := meta(t)
	_, err := m.git.CurrentBranch()
	is.True(err != nil) // should return an error if repo does not exist
	is.NoErr(setupTestRepo(m.git, newfile("test.txt", "")))
	is.NoErr(m.git.Add("."))
	is.NoErr(m.git.Commit("p"))
	branch := "testing-current-branch"
	is.NoErr(run(m.git.Cmd("branch", branch)))
	is.NoErr(run(m.git.Cmd("checkout", branch)))
	br, err := m.git.CurrentBranch()
	is.NoErr(err)
	is.Equal(br, branch)
}

func TestGit_Files(t *testing.T) {
	is := is.New(t)
	m := meta(t)
	files := []fs.File{
		newfile("test.txt", "empty..."),
		newfile("main.go", "package main\nfunc main() { println(\"hello\") }"),
		newfile("readme.md", "# test repo\n\nthis is a test"),
	}
	is.NoErr(setupTestRepo(m.git, files...))
	is.NoErr(m.git.Add("."))
	is.NoErr(m.git.Commit("c"))

	var (
		fmap   = make(map[string]struct{})
		objmap = make(map[string]struct{})
	)
	objects, err := m.git.Files()
	is.NoErr(err)

	// populate fmap
	for _, f := range files {
		stat, err := f.Stat()
		is.NoErr(err)
		fmap[stat.Name()] = struct{}{}
	}
	// populate objmap and check that all objects are in fmap
	for _, obj := range objects {
		_, ok := fmap[obj.Name]
		is.True(ok)
		objmap[obj.Name] = struct{}{}
	}
	// check that all files are in objmap
	for _, f := range files {
		stat, err := f.Stat()
		is.NoErr(err)
		_, ok := objmap[stat.Name()]
		is.True(ok)
	}
}

func TestGit_Config(t *testing.T) {
	is := is.New(t)
	m := meta(t)
	is.NoErr(setupTestRepo(m.git, newfile("test.txt", "empty...")))
	is.NoErr(m.git.Add("."))
	is.NoErr(m.git.Commit("c"))
	conf, err := m.git.Config()
	is.NoErr(err)
	is.True(len(conf) > 0)
	is.Equal(conf["core.filemode"], "true")
	is.Equal(conf["core.bare"], "true")
	for _, editor := range []string{
		"nano",
		"vim",
		"code",
	} {
		is.NoErr(m.git.ConfigLocalSet("core.editor", editor))
		conf, err = m.git.ConfigLocal()
		is.NoErr(err)
		is.Equal(conf["core.editor"], editor)
	}
}

func TestGit_LsTree(t *testing.T) {
	is := is.New(t)
	tmp := t.TempDir()
	gd, tr := dirs(tmp)
	git := New(gd, tr)
	git.SetOut(io.Discard)
	git.SetErr(io.Discard)
	git.SetPersistentArgs([]string{"-c", "commit.gpgsign=false"})
	is.NoErr(git.InitBare())
	path := filepath.Join(git.WorkingTree(), "file.txt")
	is.NoErr(touch(path))
	is.NoErr(git.Add(path))
	is.NoErr(git.Commit("first commit"))
	files, err := git.LsFiles()
	is.NoErr(err)
	is.Equal(files[0], "file.txt")
}

func TestGit_ModifiedFiles(t *testing.T) {
	is := is.New(t)
	m := meta(t)
	git := m.Git()
	git.SetOut(io.Discard)
	git.SetErr(io.Discard)
	git.SetPersistentArgs([]string{"-c", "commit.gpgsign=false"})
	err := setupTestRepo(
		git,
		newfile("test.txt", "this is a test"),
		newfile("x", "this is not an x"),
	)
	is.NoErr(err)
	is.NoErr(git.Add("test.txt", "x"))
	is.NoErr(git.Commit("first commit"))
	is.NoErr(m.appendfile("test.txt", "hello"))
	files, err := git.ModifiedFiles()
	is.NoErr(err)
	is.Equal(len(files), 1)        // should only have one file modified
	is.Equal(files[0], "test.txt") // file should be shown as modified
}

func TestNewObjectFromFile(t *testing.T) {
	is := is.New(t)
	m := meta(t)
	f1 := newfile("file.txt", "file contents") // inside actual git repo, control group
	f2 := newfile("file.txt", "file contents") // not tracked, test group
	is.NoErr(setupTestRepo(m.git, f1))
	is.NoErr(m.git.Add("."))
	is.NoErr(m.git.Commit("committed first file"))
	objs, err := m.git.Files()
	is.NoErr(err)
	obj, err := NewObjectFromFile(f2)
	is.NoErr(err)
	is.Equal(objs[0].Name, obj.Name) // should have same name
	//is.Equal(objs[0].Mode, obj.Mode) // should have same mode
	is.Equal(objs[0].Size, obj.Size) // should have same size
	is.Equal(objs[0].Type, obj.Type) // should have same type
	is.Equal(objs[0].Hash, obj.Hash) // should have same hash
	obj, err = NewObjectFromFile(newfile("file.txt", "different contents"))
	is.NoErr(err)
	is.Equal(objs[0].Name, obj.Name)  // should have same name
	is.True(objs[0].Hash != obj.Hash) // should have different hash for different contents
}

func TestObjects(t *testing.T) {
	for _, ot := range []ObjectType{
		ObjBlob, ObjTree, ObjCommit, ObjTag, ObjUnknown,
	} {
		otstr := ot.String()
		objtype := objectType(otstr)
		if ot != objtype {
			t.Errorf("object type converted to string and back does not match")
		}
	}
}

func TestGit_PrintFileModifications(t *testing.T) {
	// So, when running 'git checkout -- ./some/file' it marks that file as
	// modified when it was not. This is a test to figure out why this happens
	// and if I can stop it from happening.
	is := is.New(t)
	git := testgit(t)
	err := setupTestRepo(
		git,
		newfile("one", "this is the first"),
		newfile("two", "this is the second"),
		newfile("three", "this is the third"),
	)
	is.NoErr(err)
	is.NoErr(git.Add("."))
	is.NoErr(git.Commit("commit message"))
	git.SetOut(os.Stdout)
	git.SetErr(os.Stderr)

	tree := git.WorkingTree()
	tree2 := filepath.Join(filepath.Dir(git.WorkingTree()), "tree2")
	os.Mkdir(tree2, 0755)
	git.SetWorkingTree(tree2)
	is.NoErr(git.Cmd("checkout", "--", "one").Run())
	git.SetWorkingTree(tree)
	git.Cmd("--no-pager", "diff", "--name-only").Run()
	files, err := git.Modifications()
	is.NoErr(err)
	is.Equal(len(files), 0) // should not have any files marked as modified
}

type testmeta struct {
	tmp string
	git *Git
	t   *testing.T
}

func (tm *testmeta) Git() *Git { return tm.git }

func meta(t *testing.T) *testmeta {
	t.Helper()
	tmp := t.TempDir()
	gitdir, worktree := dirs(tmp)
	g := New(gitdir, worktree)
	g.SetOut(io.Discard)
	g.SetErr(io.Discard)
	g.SetPersistentArgs([]string{"-c", "commit.gpgsign=false"})
	return &testmeta{tmp: tmp, git: g, t: t}
}

func (tm *testmeta) appendfile(name, contents string) error {
	return appendfile(filepath.Join(tm.git.WorkingTree(), name), contents)
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

func newfile(name, contents string) fs.File {
	return &file{
		name: name,
		b:    bytes.NewBuffer([]byte(contents)),
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
	b    *bytes.Buffer
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

func isNoErr(is *is.I, errs ...error) {
	for _, e := range errs {
		is.NoErr(e)
	}
}

func debugCmd(cmd *exec.Cmd) {
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
}
