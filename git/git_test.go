package git

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/matryer/is"
)

func dirs(tmp string) (gitdir, tree string) {
	gitdir = filepath.Join(tmp, "repo")
	tree = filepath.Join(tmp, "tree")
	_ = os.Mkdir(gitdir, 0755)
	_ = os.Mkdir(tree, 0755)
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
	g := m.Git()
	g.SetOut(io.Discard)
	g.SetErr(io.Discard)
	g.SetPersistentArgs([]string{"-c", "commit.gpgsign=false"})
	err := setupTestRepo(
		g,
		newfile("test.txt", "this is a test"),
		newfile("x", "this is not an x"),
	)
	is.NoErr(err)
	is.NoErr(g.Add("test.txt", "x"))
	is.NoErr(g.Commit("first commit"))
	is.NoErr(m.appendfile("test.txt", "hello"))
	files, err := g.ModifiedFiles()
	is.NoErr(err)
	is.Equal(len(files), 1)        // should only have one file modified
	is.Equal(files[0], "test.txt") // file should be shown as modified
	is.Equal(2, must(g.FileCount()))
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
	_ = os.Mkdir(tree2, 0755)
	git.SetWorkingTree(tree2)
	is.NoErr(git.Cmd("checkout", "--", "one").Run())
	git.SetWorkingTree(tree)
	is.NoErr(git.Cmd("--no-pager", "diff", "--name-only").Run())

	modfiles, err := git.Modifications()
	is.NoErr(err)
	is.Equal(len(modfiles), 0) // should not have any files marked as modified
	is.Equal(3, must(git.FileCount()))
	f := must(os.Open(git.indexFile()))
	index, err := readIndex(f)
	f.Close()
	is.NoErr(err)
	files, err := git.Files()
	is.NoErr(err)
	is.Equal(3, len(index.entries))
	is.Equal(3, len(files))
	for i := 0; i < 3; i++ {
		is.Equal(files[i].Name, index.entries[i].name)
		is.Equal(files[i].Size, int64(index.entries[i].statData.size))
		is.Equal(files[i].Hash, hex.EncodeToString(index.entries[i].oid))
		is.Equal(files[i].Mode, int(index.entries[i].mode))
	}
}

func TestGit_FileCount(t *testing.T) {
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
	n := must(git.FileCount())
	is.Equal(n, 3)
}

func TestObjectHash(t *testing.T) {
	is := is.New(t)
	git := testgit(t)
	is.NoErr(git.InitBare())
	const data = "hello there, this is a test"
	b := must(gitHashObject(git, data))
	oid := objectHashBytes(ObjBlob, uint64(len(data)), strings.NewReader(data))
	if !bytes.Equal(b, oid) {
		t.Errorf("wrong object id hash: got %v, want %v", oid, b)
	}
}

// Look for `do_read_index` and `read_index_from` in read-cache.c or
// `repo_read_index` in repository.c
func TestReadIndex(t *testing.T) {
	is := is.New(t)
	git := testgit(t)
	err := setupTestRepo(
		git,
		newfile("one", "this is the first file\n"),
		fileFrom("git_test.go"),
		fileFrom("git.go"),
		fileFrom("objects.go"),
		fileFrom("../Dockerfile"),
		fileFrom("../README.md"),
	)
	is.NoErr(err)
	is.NoErr(git.Add("."))
	is.NoErr(git.Commit("commit message"))
	git.SetOut(os.Stdout)
	git.SetErr(os.Stderr)
	filename := git.indexFile()
	f := must(os.Open(filename))
	index, err := readIndex(f)
	if err != nil {
		f.Close()
		t.Fatal(err)
	}
	is.Equal(uint32(cacheSignature), index.header.signature)
	is.NoErr(f.Close())
	is.Equal(index.header.entries, uint32(len(index.entries)))
	uid := os.Getuid()
	gid := os.Getgid()
	for _, e := range index.entries {
		is.True(e.nameLen > 0)
		is.Equal(e.nameLen, uint(len(e.name)))
		is.Equal(int(e.statData.uid), uid)
		is.Equal(int(e.statData.gid), gid)
		is.True(e.statData.dev != 0)
		is.True(e.statData.ino != 0)
		is.True(e.mode.Perm() == 0644 || e.mode.Perm() == 0755)
		is.True(e.statData.ctime.sec != 0)
		is.True(e.statData.ctime.nsec != 0)
		is.True(e.statData.mtime.sec != 0)
		is.True(e.statData.mtime.nsec != 0)
		d := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.Local)
		is.True(e.statData.ctime.Time().After(d))
		is.True(e.statData.mtime.Time().After(d))
	}
	f = must(os.OpenFile(filepath.Join(git.workTree, "one"), os.O_APPEND|os.O_WRONLY, 0644))
	_, err = f.Write([]byte("this is another test\n"))
	is.NoErr(err)
	is.NoErr(f.Close())
	modfiles := must(git.Modifications())
	is.Equal(len(modfiles), 1)
	is.Equal(modfiles[0].Name, "one")
	is.Equal(modfiles[0].Type, ModChanged)
}

func TestRef(t *testing.T) {
	is := is.New(t)
	git := testgit(t)
	err := setupTestRepo(
		git,
		newfile("one", "this is the first file\n"),
		fileFrom("git_test.go"),
		fileFrom("git.go"),
		fileFrom("objects.go"),
		fileFrom("../Dockerfile"),
		fileFrom("../README.md"),
	)
	is.NoErr(err)
	is.NoErr(git.Add("."))
	is.NoErr(git.Commit("commit message"))
	is.Equal(
		must(must(git.Head()).Follow(git)),
		must(git.HeadCommitHash()),
	)
}

func TestOpenObject(t *testing.T) {
	is := is.New(t)
	git := testgit(t)
	err := setupTestRepoCommits(
		git,
		newfile("one", "this is the first file\n"),
		fileFrom("git_test.go"),
		fileFrom("git.go"),
		fileFrom("objects.go"),
		fileFrom("../Dockerfile"),
		fileFrom("../README.md"),
	)
	is.NoErr(err)
	files := must(git.Files())
	head, err := git.Head()
	is.NoErr(err)
	obj, err := git.OpenObject(head)
	is.NoErr(err)
	is.Equal(obj.Type, ObjCommit)
	obj, err = git.OpenObject(Ref(files[0].Hash))
	is.NoErr(err)
	is.Equal(obj.Type, ObjBlob)
	is.Equal(obj.Size, uint64(files[0].Size))

	obj, err = git.OpenObject(Ref("HEAD"))
	is.NoErr(err)
	var cm Commit
	err = parseCommit(bufio.NewReader(bytes.NewReader(obj.Data)), &cm)
	is.NoErr(err)
	is.True(!isZero(cm.Tree[:]))
	is.True(!isZero(cm.Parent[:]))
	is.True(len(cm.Author) > 0)
	is.True(len(cm.Commiter) > 0)
	is.True(len(cm.Message) > 0)
}

func TestParseTree(t *testing.T) {
	is := is.New(t)
	git := testgit(t)
	err := setupTestRepoCommits(
		git,
		newfile("one", "this is the first file\n"),
		fileFromTo("git_test.go", "git/git_test.go"),
		fileFromTo("git.go", "git/git.go"),
		fileFromTo("objects.go", "git/objects.go"),
		fileFrom("../Dockerfile"),
		fileFrom("../README.md"),
	)
	is.NoErr(err)
	files := must(git.Files())
	head, err := git.Head()
	is.NoErr(err)
	obj, err := git.OpenObject(head)
	is.NoErr(err)
	is.Equal(obj.Type, ObjCommit)
	obj, err = git.OpenObject(Ref(files[0].Hash))
	is.NoErr(err)
	is.Equal(obj.Type, ObjBlob)
	is.Equal(obj.Size, uint64(files[0].Size))
	cm, err := git.HeadCommit()
	is.NoErr(err)
	entries, err := git.CommitTree(cm)
	is.NoErr(err)
	for _, e := range entries {
		is.True(int(e.Mode) != 0)
		is.True(len(e.Name) > 0)
		is.True(!isZero(e.Hash[:]))
		if e.Name == "git" {
			is.Equal(e.Mode, TreeMode)
		}
	}
}

func TestParseLogs(t *testing.T) {
	is := is.New(t)
	git := testgit(t)
	err := setupTestRepoCommits(
		git,
		newfile("one", "this is the first file\n"),
		fileFromTo("git_test.go", "git/git_test.go"),
		fileFromTo("git.go", "git/git.go"),
		fileFromTo("objects.go", "git/objects.go"),
		fileFrom("../Dockerfile"),
		fileFrom("../README.md"),
	)
	is.NoErr(err)
	err = run(git.Cmd("commit", "-m", "empty emended commit", "--allow-empty", "--amend"))
	is.NoErr(err)
	f, err := os.Open(filepath.Join(git.gitDir, "logs/HEAD"))
	is.NoErr(err)
	defer f.Close()
	logs, err := parseLogs(f)
	is.NoErr(err)
	is.True(len(logs) > 0)
}

func TestGatherCommits(t *testing.T) {
	is := is.New(t)
	git := testgit(t)
	start := time.Now()
	err := setupTestRepoCommits(
		git,
		newfile("one", "this is the first file\n"),
		fileFromTo("git_test.go", "git/git_test.go"),
		fileFromTo("git.go", "git/git.go"),
		fileFromTo("objects.go", "git/objects.go"),
		fileFrom("../Dockerfile"),
		fileFrom("../README.md"),
	)
	is.NoErr(err)
	err = setupTestRepo(git, newfile("help.txt", "another file"))
	is.NoErr(err)
	is.NoErr(git.Add("help.txt"))
	err = run(git.Cmd("commit", "-m", "empty emended commit"))
	is.NoErr(err)
	c, err := git.HeadCommit()
	is.NoErr(err)
	commits := []*Commit{c}
	for !c.IsRoot() {
		c, err = git.CommitParent(c)
		is.NoErr(err)
		commits = append(commits, c)
	}
	for _, c := range commits {
		is.True(len(c.Message) > 0)
		is.Equal(c.AuthorTime.Unix(), start.Unix())
		is.Equal(c.CommiterTime.Unix(), start.Unix())
	}
}

func objectHashBytes(typ ObjectType, size uint64, r io.Reader) []byte {
	raw := objectHash(typ, size, r)
	enc := make([]byte, hex.EncodedLen(len(raw)))
	hex.Encode(enc, raw)
	return enc
}

func gitHashObject(g *Git, data string) ([]byte, error) {
	cmd := g.newCmd([]string{"hash-object", "--stdin", "-t", "blob"})
	pipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	go func() {
		defer pipe.Close()
		_, err := pipe.Write([]byte(data))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error while writing to pipe: %v", err)
		}
	}()
	var buf bytes.Buffer
	cmd.Stdout = &buf
	err = cmd.Run()
	if err != nil {
		return nil, err
	}
	b := buf.Bytes()
	if b[len(b)-1] == 10 {
		b = b[:len(b)-1]
	}
	return b, nil
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

func addFileToTree(g *Git, f fs.File) error {
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
		file, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, stat.Mode())
		if err != nil {
			return err
		}
		_, err = io.Copy(file, f)
		if err != nil {
			return err
		}
	}
	return nil
}

func setupTestRepo(g *Git, files ...fs.File) (err error) {
	if !g.Exists() {
		if err = g.InitBare(); err != nil {
			return err
		}
	}
	for _, f := range files {
		if err = addFileToTree(g, f); err != nil {
			return err
		}
	}
	return nil
}

func setupTestRepoCommits(g *Git, files ...fs.File) (err error) {
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
		dir := filepath.Dir(p)
		if !exists(dir) {
			err = os.MkdirAll(dir, 0755)
			if err != nil {
				return err
			}
		}
		if stat.IsDir() {
			err = os.Mkdir(p, 0755)
			if err != nil {
				return err
			}
		} else {
			file, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, stat.Mode())
			if err != nil {
				return err
			}
			_, err = io.Copy(file, f)
			if err != nil {
				return err
			}
			err = g.Add(p)
			if err != nil {
				return err
			}
			err = g.Commit(fmt.Sprintf("adding %q", p))
			if err != nil {
				return err
			}
		}
	}
	if err = g.CommitAllowEmpty("final setup commit"); err != nil {
		return err
	}
	return nil
}

func newfile(name, contents string) fs.File {
	return &file{
		name: name,
		b:    bytes.NewBuffer([]byte(contents)),
		stat: &stat{size: int64(len(contents)), name: name},
	}
}

func fileFrom(filename string) fs.File {
	_, name := filepath.Split(filename)
	return fileFromTo(filename, name)
}

func fileFromTo(filename, dest string) fs.File {
	stat, err := os.Stat(filename)
	if err != nil {
		panic(err)
	}
	f, err := os.Open(filename) // read file
	if err != nil {
		panic(err)
	}
	defer f.Close()
	var buf bytes.Buffer
	_, err = io.Copy(&buf, f)
	if err != nil {
		panic(err)
	}
	return &file{
		name: dest,
		b:    &buf,
		stat: stat,
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
	stat fs.FileInfo
}

func (f *file) Read(b []byte) (int, error) { return f.b.Read(b) }
func (f *file) Close() error               { return nil }
func (f *file) Stat() (fs.FileInfo, error) {
	if f.stat != nil {
		return f.stat, nil
	}
	return &stat{
		size: int64(f.b.Len()),
		name: f.name,
	}, nil
}

type stat struct {
	size int64
	name string
}

func (s *stat) Size() int64        { return s.size }
func (s *stat) Name() string       { return s.name }
func (s *stat) Mode() fs.FileMode  { return fs.FileMode(0664) }
func (s *stat) Sys() any           { return nil }
func (s *stat) IsDir() bool        { return false }
func (s *stat) ModTime() time.Time { return time.Now() }

func must[T any](v T, e error) T {
	if e != nil {
		panic(e)
	}
	return v
}

func isZero(b []byte) bool {
	cum := uint64(0)
	for _, c := range b {
		cum += uint64(c)
	}
	return cum == 0
}
