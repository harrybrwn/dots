package tree

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

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
	if tree.Name != "/" {
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
	Print(os.Stdout, tree)
}

func printTree(t *Node, n int) {
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
