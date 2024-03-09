package tree

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func Test(t *testing.T) { t.Skip() }

func TestNewTree(t *testing.T) {
	files := []string{
		"home/user/.bashrc",
		"home/user/files/file.txt",
		"home/user/files/file2.txt",
		"config/file",
		"home/user/music",
		"groceries.txt",
	}
	tree := New(files)
	count := count(tree)
	if count != len(files) {
		t.Errorf("wrong file count: got %d, want %d", count, len(files))
	}
	if tree.Name != rootName {
		t.Error("base of the tree should be named '/'")
	}
	for _, k := range []string{"home", "config", "groceries.txt"} {
		if !contains(tree, k) {
			t.Fatalf("tree should have child named %q", k)
		}
	}
	home := tree.children["home"]
	if !contains(home, "user") {
		t.Fatal("home dir should have 'user' dir")
	}
	user := home.children["user"]
	for _, k := range []string{"files", "music", ".bashrc"} {
		if !contains(user, k) {
			t.Fatalf("user dir should have child named %q", k)
		}
	}
	filesdir := user.children["files"]
	for _, k := range []string{"file.txt", "file2.txt"} {
		if !contains(filesdir, k) {
			t.Fatalf("files dir should have child named %q", k)
		}
	}
	config := tree.children["config"]
	if !contains(config, "file") {
		t.Error("config dir should have a 'file' dir")
	}
	var buf bytes.Buffer
	err := Print(&buf, tree)
	if err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 {
		t.Error("did not write tree to buffer")
	}
	exp := Node{
		Name: rootName,
		Type: TreeNode,
		path: []string{},
		children: map[string]*Node{
			"groceries.txt": {Name: "groceries.txt", Type: LeafNode, path: []string{rootName}, children: map[string]*Node{}},
			"home": {
				Name: "home",
				Type: TreeNode,
				path: []string{rootName},
				children: map[string]*Node{"user": {
					Name: "user",
					Type: TreeNode,
					path: []string{rootName, "home"},
					children: map[string]*Node{
						".bashrc": {Name: ".bashrc", Type: LeafNode, path: []string{rootName, "home", "user"}, children: map[string]*Node{}},
						"music":   {Name: "music", Type: LeafNode, path: []string{rootName, "home", "user"}, children: map[string]*Node{}},
						"files": {
							Name: "files",
							Type: TreeNode,
							path: []string{rootName, "home", "user"},
							children: map[string]*Node{
								"file.txt":  {Name: "file.txt", Type: LeafNode, path: []string{rootName, "home", "user", "files"}, children: map[string]*Node{}},
								"file2.txt": {Name: "file2.txt", Type: LeafNode, path: []string{rootName, "home", "user", "files"}, children: map[string]*Node{}},
							},
						},
					},
				}},
			},
			"config": {
				Name: "config",
				Type: TreeNode,
				path: []string{rootName},
				children: map[string]*Node{"file": {
					Name:     "file",
					Type:     LeafNode,
					path:     []string{rootName, "config"},
					children: map[string]*Node{},
				}},
			},
		},
	}
	if err := nodeEq(tree, &exp); err != nil {
		t.Fatal(err)
	}
	tree = New(nil)
	err = nodeEq(tree, &Node{
		Type:     TreeNode,
		Name:     rootName,
		children: map[string]*Node{}},
	)
	if err != nil {
		t.Fatal(err)
	}
}

func TestExpand(t *testing.T) {
	var ok bool
	path := []string{"home", "user", "file.txt"}
	n := &Node{Name: rootName, Type: TreeNode}
	exp := Node{
		Type: TreeNode,
		Name: rootName,
		path: []string{},
		children: map[string]*Node{"home": {
			Type: TreeNode,
			Name: "home",
			path: []string{rootName},
			children: map[string]*Node{"user": {
				Type: TreeNode,
				Name: "user",
				path: []string{rootName, "home"},
				children: map[string]*Node{"file.txt": {
					Type: LeafNode,
					Name: "file.txt",
					path: []string{rootName, "home", "user"},
				}},
			}},
		}},
	}
	expand(path, n)
	var node = n
	for _, key := range path {
		if node, ok = node.children[key]; !ok {
			t.Error("child should have been found")
		}
	}
	if err := nodeEq(n, &exp); err != nil {
		t.Errorf("node was not equal to expected: %v", err)
	}

	var b bytes.Buffer
	err := PrintColor(&b, n, NoColor)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.String(), "\033[0m") {
		t.Error("no color print should not have control characters")
	}
	if PrintHeight(n) != 4 {
		t.Errorf("expected a print height of 4, got %d", PrintHeight(n))
	}
	b.Reset()
	err = PrintColor(&b, n, ColorFolders)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "\033[01;34m") {
		t.Error("did not find folder color control character")
	}
}

func TestAnd(t *testing.T) {
	tr := New(nil)
	tr.Add(
		"/path/to/file",
		"/path/to/another/file",
		"/path/in/a/place",
		"/others/over/here",
		"/others/over/there",
		"/this/is/the/third/file/tree",
		"/this/is/the/third/dir/tree",
	)
	other := New([]string{
		"/path/to",
		"/path/in/",
	})
	a := tr.and(other)
	if a.Name != "/" {
		t.Errorf("expected \"path\" got %q", a.Name)
	}
	if _, ok := a.children["path"]; !ok {
		t.Errorf("should have key %q", "path")
	}
	if len(a.children) != 1 {
		t.Error("expected only one child")
	}
	a = a.trimRoot(other)
	if len(a.children) != 2 {
		t.Error("expected 2 children")
	}
	if _, ok := a.children["in"]; !ok {
		t.Errorf("expected child %q to be present", "in")
	}
	if _, ok := a.children["to"]; !ok {
		t.Errorf("expected child %q to be present", "to")
	}
	paths := tr.ListPaths()
	exp := []string{
		"/others/over/here",
		"/others/over/there",
		"/path/in/a/place",
		"/path/to/another/file",
		"/path/to/file",
		"/this/is/the/third/dir/tree",
		"/this/is/the/third/file/tree",
	}
	if len(exp) != len(paths) {
		t.Error("unexpected length of paths")
	}
	for i, p := range paths {
		if p != exp[i] {
			t.Errorf("wrong path: got %q, expected %q", p, exp[i])
		}
	}
	printTree(nil, 0) // just for removing the "unused" linting error
}

func nodeEq(a, b *Node) error {
	if a.Type != b.Type || a.Name != b.Name {
		return fmt.Errorf("nodes have different types: %v, %v", a.Type, b.Type)
	}
	if len(a.path) != len(b.path) {
		return fmt.Errorf("path %v not equal to path %v", a.path, b.path)
	}
	if a.Path() != b.Path() {
		return fmt.Errorf("path %v not equal to path %v", a.Path(), b.Path())
	}
	for i := 0; i < len(a.path); i++ {
		if a.path[i] != b.path[i] {
			return fmt.Errorf("path %v not equal to path %v", a.path, b.path)
		}
	}
	for key, node := range a.children {
		bnode, ok := b.children[key]
		if !ok {
			return fmt.Errorf("key %q not found in node b", key)
		}
		if err := nodeEq(node, bnode); err != nil {
			return err
		}
	}
	return nil
}

func printTree(t *Node, n int) {
	if t == nil {
		return
	}
	fmt.Println(t.Name, t.Type)
	fmt.Printf("%s ", strings.Repeat(" ", n))
	if t.children == nil {
		return
	}
	for k, node := range t.children {
		fmt.Printf("%s\n", k)
		printTree(node, n+1)
	}
}

func contains(n *Node, key string) bool {
	if n.children == nil {
		return false
	}
	_, ok := n.children[key]
	return ok
}
