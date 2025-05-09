package git

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	gitExec = "git"
)

func New(dir, tree string) *Git {
	return &Git{
		gitDir:   dir,
		workTree: tree,
	}
}

func Here() *Git {
	return New(".git", ".")
}

type Git struct {
	gitDir         string // --git-dir
	workTree       string // --work-tree
	args           []string
	stdout, stderr io.Writer
	stdin          io.Reader

	configGlobal string
	configSystem string
}

func (g *Git) Cmd(args ...string) *exec.Cmd {
	cmd := g.newCmd(args)
	if len(g.configGlobal) > 0 {
		cmd.Env = append(cmd.Env, fmt.Sprintf("GIT_CONFIG_GLOBAL=%s", g.configGlobal))
	}
	if len(g.configSystem) > 0 {
		cmd.Env = append(cmd.Env, fmt.Sprintf("GIT_CONFIG_SYSTEM=%s", g.configSystem))
	}
	g.setDefaultIO(cmd)
	return cmd
}

func (g *Git) CmdWithEnv(args []string, env map[string]string) *exec.Cmd {
	cmd := g.Cmd()
	for k, v := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	return cmd
}

func (g *Git) newCmd(args []string) *exec.Cmd {
	arguments := make([]string, 4, 4+len(args)+len(g.args))
	arguments[0] = "--git-dir"
	arguments[1] = g.gitDir
	arguments[2] = "--work-tree"
	arguments[3] = g.workTree
	arguments = append(arguments, g.args...)
	arguments = append(arguments, args...)
	return exec.Command(gitExec, arguments...)
}

func (g *Git) RunCmd(args ...string) error { return run(g.Cmd(args...)) }

func (g *Git) RunCmdWithEnv(env map[string]string, args ...string) error {
	c := g.Cmd(args...)
	for k, v := range env {
		c.Env = append(c.Env, fmt.Sprintf("%s=%s", k, v))
	}
	return run(c)
}

func (g *Git) Exists() bool {
	return exists(g.gitDir) && isGitDir(g.gitDir)
}

// InitBare will create a new bare repo. Equivalent to `git init --bare`.
func (g *Git) InitBare() error { return initBareRepo(g.gitDir) }

// WorkingTree will return the repositories working tree.
func (g *Git) WorkingTree() string { return g.workTree }

// GitDir will return the git directory.
func (g *Git) GitDir() string { return g.gitDir }

func (g *Git) SetWorkingTree(path string) { g.workTree = path }
func (g *Git) SetGitDir(path string)      { g.gitDir = path }

// SetPersistentArgs will set an array of arguments passed internally to the git
// command whenever the Cmd function is called.
func (g *Git) SetPersistentArgs(args []string) { g.args = args }

// AppendPersistentArgs will append to the array of arguments passed internally
// to the git command whenever the Cmd function is called.
func (g *Git) AppendPersistentArgs(args ...string) { g.args = append(g.args, args...) }

func (g *Git) Add(paths ...string) error {
	if len(paths) == 0 {
		return errors.New("no paths to add")
	}
	return run(g.Cmd(append([]string{"add"}, paths...)...))
}

func (g *Git) AddUpdate(paths ...string) error {
	return run(g.Cmd(append([]string{"add", "--update"}, paths...)...))
}

func (g *Git) Remove(files ...string) error {
	if len(files) == 0 {
		return errors.New("no files to remove")
	}
	args := []string{"rm", "--cached"}
	args = append(args, files...)
	return run(g.Cmd(args...))
}

func (g *Git) Commit(message string) error {
	return run(g.Cmd("commit", "-m", message))
}

func (g *Git) CommitAllowEmpty(message string) error {
	return run(g.Cmd("commit", "-m", message, "--allow-empty"))
}

func (g *Git) LsFiles() ([]string, error) {
	var (
		buf bytes.Buffer
		cmd = g.Cmd("ls-tree", "--full-tree", "-r", "--name-only", "HEAD")
	)
	cmd.Stdout = &buf
	err := run(cmd)
	if err != nil {
		return nil, err
	}
	return lines(buf.String()), nil
}

func (g *Git) ModifiedFiles() ([]string, error) {
	var (
		buf bytes.Buffer
		cmd = g.Cmd("diff-files", "--name-only")
	)
	cmd.Stdout = &buf
	err := run(cmd)
	if err != nil {
		return nil, err
	}
	return lines(buf.String()), nil
}

func (g *Git) Files() ([]*FileObject, error) {
	var (
		buf bytes.Buffer
		c   = g.Cmd("ls-tree", "HEAD", "-r", "-t", "--long", "--full-tree")
	)
	c.Stdout = &buf
	err := run(c)
	if err != nil {
		return nil, err
	}
	var (
		i, j   int
		fields [4]string
		sc     = bufio.NewScanner(&buf)
		files  = make([]*FileObject, 0)
	)
	for sc.Scan() {
		var (
			line = sc.Text()
			f    FileObject
		)
		i = strings.IndexByte(line, '\t')
		f.Name = line[i+1:]
		parts := strings.Split(line[:i], " ")
		j = 0
		for _, s := range parts {
			if len(s) == 0 {
				continue
			}
			fields[j] = s
			j++
		}
		f.Mode, err = parseMode(fields[0])
		if err != nil {
			return nil, err
		}
		f.Type = objectType(fields[1])
		f.Hash = fields[2]
		if f.Type == ObjBlob {
			f.Size, err = strconv.ParseInt(fields[3], 10, 64)
			if err != nil {
				return nil, err
			}
		}
		files = append(files, &f)
	}
	return files, nil
}

// ModType is the type of modification that has been made to an object.
// See `git help diff-index`
type ModType byte

const (
	// Addition of a file
	ModAddition ModType = 'A'
	// Copy of a file into a new one
	ModCopy ModType = 'C'
	// Deletion of a file
	ModDelete ModType = 'D'
	// Modification of file contents of file mode
	ModChanged ModType = 'M'
	// File renamed
	ModRename ModType = 'R'
	// Change in the type of file
	ModFileType ModType = 'T'
	// File is unmerged (you must complete the merge before it can be committed)
	ModUnmerged ModType = 'U'
	// This should not happen, indicator of possible bug in git
	ModUnknown ModType = 'X'
)

func (mt ModType) String() string { return string(mt) }

// ModifiedFile is a file that has been modified
// and has a source and destination hash.
type ModifiedFile struct {
	Name     string
	Type     ModType
	Src, Dst ObjModification
}

type ObjModification struct {
	Mode int    // file mode
	Hash string // sha1 as hex
}

// Modifications will list all the file modifications that are being tracked by
// git.
func (g *Git) Modifications() ([]*ModifiedFile, error) {
	var buf bytes.Buffer
	c := g.Cmd("diff-index", "HEAD")
	c.Stdout = &buf
	err := run(c)
	if err != nil {
		return nil, err
	}
	var (
		i     int
		sc    = bufio.NewScanner(&buf)
		files = make([]*ModifiedFile, 0)
	)
	for sc.Scan() {
		var (
			f    ModifiedFile
			line = sc.Text()
		)
		if line[0] != ':' {
			return nil, errors.New("did not find a ':' at the front of each line")
		}
		i = strings.IndexByte(line, '\t')
		f.Name = line[i+1:]
		// Split the line but exclude the colon and the filename
		parts := strings.Split(line[1:i], " ")
		f.Src.Mode, err = parseMode(parts[0])
		if err != nil {
			return nil, err
		}
		f.Dst.Mode, err = parseMode(parts[1])
		if err != nil {
			return nil, err
		}
		f.Src.Hash = parts[2]
		f.Dst.Hash = parts[3]
		f.Type = ModType(parts[4][0])
		files = append(files, &f)
	}
	return files, nil
}

func (g *Git) ModifiedSet() (map[string]*ModifiedFile, error) {
	mods, err := g.Modifications()
	if err != nil {
		return nil, err
	}
	set := make(map[string]*ModifiedFile, len(mods))
	for _, m := range mods {
		set[m.Name] = m
	}
	return set, nil
}

func (g *Git) Head() (Ref, error) {
	return readRef(filepath.Join(g.gitDir, "HEAD"))
}

func (g *Git) HeadCommitHash() (Ref, error) {
	ref, err := g.Head()
	if err != nil {
		return "", err
	}
	return ref.fullFollow(g)
}

func (g *Git) FollowRef(ref Ref) (Ref, error) { return ref.Follow(g) }

func parseMode(s string) (int, error) {
	m, err := strconv.ParseUint(s, 8, 64)
	if err != nil {
		return 0, err
	}
	return int(m), nil
}

func (g *Git) HasRemote() bool {
	var (
		b   bytes.Buffer
		cmd = g.Cmd("remote")
	)
	cmd.Stdout = &b
	err := run(cmd)
	if err != nil {
		return false
	}
	return b.Len() > 0
}

func (g *Git) objectFilename(hash string) string {
	return objectFilename(g.gitDir, hash)
}

func objectFilename(gitDir, hash string) string {
	return filepath.Join(
		gitDir,
		"objects",
		hash[:2],
		hash[2:],
	)
}

func (g *Git) OpenObject(ref Ref) (*Object, error) {
	ref, err := ref.fullFollow(g)
	if err != nil {
		return nil, err
	}
	r := string(ref)
	filename := g.objectFilename(r)
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	rc, err := zlib.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	var obj Object
	err = parseObject(rc, &obj)
	if err != nil {
		return nil, err
	}
	obj.Hash = r
	return &obj, nil
}

func (g *Git) WriteObject(o *Object) error {
	var (
		buf  bytes.Buffer
		hash string
	)
	if len(o.Hash) > 0 {
		hash = o.Hash
		_, err := o.writeTo(&buf)
		if err != nil {
			return err
		}
	} else {
		h := sha1.New()
		_, err := o.writeTo(&hashWriter{w: &buf, hash: h})
		if err != nil {
			return err
		}
		hash = hex.EncodeToString(h.Sum(nil))
	}
	f, err := os.OpenFile(
		g.objectFilename(hash),
		os.O_CREATE|os.O_WRONLY,
		0644,
	)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, &buf)
	if err != nil {
		return err
	}
	return nil
}

func (g *Git) HeadCommit() (*Commit, error) {
	ref, err := g.HeadCommitHash()
	if err != nil {
		return nil, err
	}
	obj, err := g.OpenObject(ref)
	if err != nil {
		return nil, err
	}
	if obj.Type != ObjCommit {
		return nil, errors.New("object at HEAD is not a commit object")
	}
	var cm Commit
	err = parseCommit(bufio.NewReader(bytes.NewReader(obj.Data)), &cm)
	if err != nil {
		return nil, err
	}
	return &cm, nil
}

func (g *Git) OpenCommit(ref Ref) (*Commit, error) {
	obj, err := g.OpenObject(ref)
	if err != nil {
		return nil, err
	}
	if obj.Type != ObjCommit {
		return nil, errors.New("object is not a commit object")
	}
	var cm Commit
	err = parseCommit(bufio.NewReader(bytes.NewReader(obj.Data)), &cm)
	if err != nil {
		return nil, err
	}
	return &cm, nil
}

func (g *Git) CommitTree(commit *Commit) ([]TreeEntry, error) {
	obj, err := g.OpenObject(NewHashRef(commit.Tree))
	if err != nil {
		return nil, err
	}
	if obj.Type != ObjTree {
		return nil, errors.New("commit tree is not a tree object")
	}
	return parseTree(obj.Data)
}

func (g Git) CommitParent(commit *Commit) (*Commit, error) {
	return g.OpenCommit(NewHashRef(commit.Parent))
}

type Config map[string]any

func (c Config) Exists(key string) bool {
	_, ok := c[key]
	return ok
}

func (g *Git) Config() (Config, error) {
	return g.config("--list")
}

func (g *Git) ConfigLocal() (Config, error) {
	return g.config("--local", "--list")
}

func (g *Git) ConfigGlobal() (Config, error) {
	return g.config("--global", "--list")
}

func (g *Git) ConfigSet(key, value string) error {
	return run(g.Cmd("config", key, value))
}

func (g *Git) ConfigLocalSet(key, value string) error {
	return run(g.Cmd("config", "--local", key, value))
}

func (g *Git) ConfigGlobalSet(key, value string) error {
	return run(g.Cmd("config", "--global", key, value))
}

func (g *Git) SetArgs(arguments ...string) { g.args = arguments }

func (g *Git) SetOut(out io.Writer) { g.stdout = out }
func (g *Git) SetErr(w io.Writer)   { g.stderr = w }

func (g *Git) SetGlobalConfig(filename string) *Git {
	g.configGlobal = filename
	return g
}

func (g *Git) SetSystemConfig(filename string) *Git {
	g.configSystem = filename
	return g
}

func (g *Git) FileCount() (int, error) {
	f, err := os.Open(g.indexFile())
	if err != nil {
		return 0, err
	}
	defer f.Close()
	raw, err := io.ReadAll(f)
	if err != nil {
		return 0, err
	}
	var hdr indexCacheHeader
	err = hdr.UnmarshalBinary(raw[:12])
	if err != nil {
		return 0, err
	}
	return int(hdr.entries), nil
}

func (g *Git) config(flags ...string) (Config, error) {
	var (
		buf  bytes.Buffer
		m    = make(Config)
		args = make([]string, 1+len(flags))
	)
	args[0] = "config"
	for i := 0; i < len(flags); i++ {
		args[i+1] = flags[i]
	}
	cmd := g.Cmd(args...)
	cmd.Stdout = &buf
	err := run(cmd)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(buf.String(), "\n")
	for _, l := range lines {
		parts := strings.SplitN(l, "=", 2)
		if len(parts) < 2 {
			continue
		}
		m[parts[0]] = parts[1]
	}
	return m, nil
}

// CurrentBranch returns the name of the current branch.
func (g *Git) CurrentBranch() (string, error) {
	// TODO git symbolic-ref --quient HEAD
	f, err := os.OpenFile(filepath.Join(g.gitDir, "HEAD"), os.O_RDONLY, 0644)
	if err != nil {
		return "", err
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}
	if b[len(b)-1] == '\n' {
		b = b[:len(b)-1]
	}
	if bytes.HasPrefix(b, []byte("ref: ")) {
		b = bytes.ReplaceAll(b, []byte("ref: "), nil)
		return filepath.Base(string(b)), nil
	}
	return string(b), nil
}

func (g *Git) indexFile() string {
	return filepath.Join(g.gitDir, "index")
}

func lines(s string) []string {
	sp := strings.Split(s, "\n")
	lines := make([]string, 0, len(sp))
	for _, f := range sp {
		f = strings.Trim(f, "\n")
		if len(f) == 0 {
			continue
		}
		lines = append(lines, f)
	}
	return lines
}

func initBareRepo(path string) error {
	const branch = "main"
	if err := os.MkdirAll(path, 0755); err != nil {
		return err
	}
	err := writeToFile(filepath.Join(path, "HEAD"), fmt.Sprintf("ref: refs/heads/%s\n", branch))
	if err != nil {
		return err
	}
	err = writeToFile(filepath.Join(path, "config"), `[core]
	repositoryformatversion = 0
	filemode = true
	bare = true
`)
	if err != nil {
		return err
	}
	for _, p := range []string{
		"branches",
		"hooks",
		"info",
		"objects",
		"objects/info",
		"objects/pack",
		"refs",
		"refs/heads",
		"refs/tags",
	} {
		err = os.Mkdir(filepath.Join(path, p), 0755)
		if err != nil {
			return err
		}
	}
	err = writeToFile(
		filepath.Join(path, "description"),
		"Unnamed repository; edit this file 'description' to name the repository.\n",
	)
	if err != nil {
		return err
	}
	err = writeToFile(
		filepath.Join(path, "info", "exclude"),
		`# git ls-files --others --exclude-from=.git/info/exclude
# Lines that start with '#' are comments.
# For a project mostly in C, the following would be a good set of
# exclude patterns (uncomment them if you want to use them):
# *.[oa]
# *~
`,
	)
	if err != nil {
		return err
	}
	return nil
}

func isGitDir(dir string) bool {
	return exists(filepath.Join(dir, "refs")) &&
		exists(filepath.Join(dir, "objects")) &&
		exists(filepath.Join(dir, "HEAD"))
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return !os.IsNotExist(err)
}

func writeToFile(filename string, data string) error {
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write([]byte(data))
	return err
}

func (g *Git) setDefaultIO(cmd *exec.Cmd) {
	cmd.Stdout = g.stdout
	cmd.Stderr = g.stderr
	cmd.Stdin = g.stdin
}

func run(cmd *exec.Cmd) error {
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		msg := strings.Trim(stderr.String(), "\n")
		if len(msg) == 0 {
			return err
		}
		return fmt.Errorf("%s: %w", msg, err)
	}
	return nil
}

type hashWriter struct {
	w    io.Writer
	hash hash.Hash
}

func (hw *hashWriter) Write(b []byte) (int, error) {
	n, err := hw.w.Write(b)
	hashed, e := hw.hash.Write(b[:n])
	if err != nil {
		return n, e
	}
	if n != hashed {
		return n, io.ErrShortWrite
	}
	return n, err
}
