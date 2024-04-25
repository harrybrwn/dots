package tree

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
)

type NodeType uint8

const (
	LeafNode NodeType = iota
	TreeNode
)

type Node struct {
	Type     NodeType
	Name     string
	children map[string]*Node
	path     []string
}

func (n *Node) Path() string { return filepath.Join(n.path...) }

const rootName = "/"

// New will create a new tree from a list of files
func New(files []string) *Node {
	tree := &Node{
		Type:     TreeNode,
		Name:     rootName,
		children: make(map[string]*Node),
	}
	if len(files) > 0 {
		tree.addPaths(files)
	}
	return tree
}

func (n *Node) Add(paths ...string) {
	n.addPaths(paths)
}

func (n *Node) FilterBy(paths ...string) *Node {
	if len(paths) == 0 {
		return n
	}
	other := New(paths)
	return n.and(other).trimRoot(other)
}

// TrimSingleRoot will walk down the tree removing nodes as long as each node
// only has one child.
func (n *Node) TrimSingleRoot() *Node {
	for len(n.children) == 1 && n.Type != LeafNode {
		for _, child := range n.children {
			*n = *child
		}
	}
	return n
}

func (n *Node) TrimSingleRootUntil(base string) *Node {
	for len(n.children) == 1 && n.Type != LeafNode {
		for _, child := range n.children {
			*n = *child
			if child.Name == base {
				return n
			}
		}
	}
	return n
}

func (n *Node) ListPaths() []string {
	paths := make([]string, 0)
	traverse(n, func(node *Node) {
		if node.Type == LeafNode {
			p := append(node.path, node.Name)
			paths = append(paths, filepath.Join(p...))
		}
	})
	return paths
}

func (n *Node) addPaths(paths []string) {
	for _, p := range paths {
		n.addPath(p)
	}
}

func (n *Node) addPath(p string) {
	parts := fileSplit(p)
	expand(parts, n)
}

// Print will write a string representation of the tree to an io.Writer
func Print(w io.Writer, t *Node) error {
	p := printer{w: w, root: t, n: count(t), colorHook: NoColor}
	return p.walk(t, "")
}

func PrintColor(w io.Writer, t *Node, prehook func(*Node) string) error {
	p := printer{w: w, root: t, n: count(t), colorHook: prehook}
	return p.walk(t, "")
}

// PrintHeight will return the height of the output if Print is called.
func PrintHeight(tree *Node) int {
	var n = 1
	if len(tree.children) == 0 {
		return n
	}
	for _, ch := range tree.children {
		n += PrintHeight(ch)
	}
	return n
}

func ColorFolders(n *Node) string {
	if n.Type == TreeNode {
		return "\x1b[01;34m"
	}
	return ""
}

func NoColor(*Node) string { return "" }

// DirColor will return a terminal blue if the node is a tree node.
func DirColor(n *Node) string {
	if n.Type == TreeNode {
		return "\x1b[01;34m"
	}
	return ""
}

func expand(parts []string, tree *Node) {
	if tree == nil || len(parts) == 0 {
		return
	}
	const ix = 0
	if len(parts) == 1 {
		tree.insertChild(createNode(tree, parts[ix], LeafNode))
	}
	child, ok := tree.children[parts[ix]]
	if !ok {
		child = createNode(tree, parts[ix], TreeNode)
		tree.insertChild(child)
	}
	expand(parts[1:], child)
}

type printer struct {
	w         io.Writer
	root      *Node
	n         int // number of files
	colorHook func(*Node) string
}

func (p *printer) walk(t *Node, prefix string) error {
	var (
		i     int
		end   = len(t.children) - 1
		line  string
		color string
	)
	for _, node := range t.getChildren() {
		if i == end {
			line = "└──"
		} else {
			line = "├──"
		}
		color = p.colorHook(node)
		post := ""
		if strings.Contains(color, "\033[") {
			post = "\033[0m"
		}
		err := p.writef("%s%s %s%s%s\n", prefix, line, color, node.Name, post)
		if err != nil {
			return err
		}
		if i == end {
			err = p.walk(node, prefix+"   ")
		} else {
			err = p.walk(node, prefix+"│  ")
		}
		if err != nil {
			return err
		}
		i++
	}
	return nil
}

func (p *printer) writef(format string, v ...interface{}) error {
	_, err := fmt.Fprintf(p.w, format, v...)
	return err
}

func traverse(node *Node, fn func(node *Node)) {
	fn(node)
	for _, child := range node.getChildren() {
		traverse(child, fn)
	}
}

func count(root *Node) int {
	if len(root.children) == 0 {
		return 1
	}
	var n int
	for _, child := range root.children {
		n += count(child)
	}
	return n
}

func createNode(parent *Node, name string, tp NodeType) *Node {
	n := &Node{Name: name, Type: tp}
	if parent != nil {
		n.path = append(parent.path, parent.Name)
	}
	return n
}

func (n *Node) getChildren() []*Node {
	if n.children == nil {
		return nil
	}
	res := make([]*Node, len(n.children))
	i := 0
	for _, c := range n.children {
		res[i] = c
		i++
	}
	sort.Sort(nodelist(res))
	return res
}

func (n *Node) insertChild(child *Node) {
	if n.children == nil {
		n.children = make(map[string]*Node)
	}
	n.children[child.Name] = child
}

func (n *Node) and(other *Node) *Node {
	res := Node{
		Type:     n.Type,
		Name:     n.Name,
		children: make(map[string]*Node),
	}
	for name, child := range other.children {
		orig, ok := n.children[name]
		if !ok {
			continue
		}
		switch child.Type {
		case TreeNode:
			res.children[name] = orig.and(child)
		case LeafNode:
			var cp Node
			deepCopy(&cp, orig)
			res.children[name] = &cp
		}
	}
	return &res
}

func (n *Node) trimRoot(other *Node) *Node {
	if len(n.children) == 1 {
		for name, child := range other.children {
			orig, ok := n.children[name]
			if !ok {
				continue
			}
			n = orig.trimRoot(child)
			break
		}
	}
	return n
}

func deepCopy(dst, n *Node) {
	dst.Type = n.Type
	dst.Name = n.Name
	dst.path = append(dst.path, n.path...)
	if n.children != nil {
		dst.children = make(map[string]*Node, len(n.children))
		for k, v := range n.children {
			var c Node
			deepCopy(&c, v)
			dst.children[k] = &c
		}
	}
}

type nodelist []*Node

func (nl nodelist) Less(i, j int) bool {
	l, r := len(nl[i].children), len(nl[j].children)
	if l == r || (l > 0 && r > 0) {
		return strings.Compare(nl[i].Name, nl[j].Name) < 0
	}
	return l > r
}

func (nl nodelist) Len() int { return len(nl) }

func (nl nodelist) Swap(i, j int) {
	nl[i], nl[j] = nl[j], nl[i]
}

var _ sort.Interface = (*nodelist)(nil)

func fileSplit(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return r == filepath.Separator
	})
}
