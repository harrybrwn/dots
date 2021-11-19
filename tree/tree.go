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
}

func New(files []string) *Node {
	if len(files) == 0 {
		return &Node{
			Type:     LeafNode,
			children: make(map[string]*Node),
		}
	}
	tree := &Node{Name: "/", Type: TreeNode, children: make(map[string]*Node)}
	for _, f := range files {
		parts := strings.Split(f, string([]rune{filepath.Separator}))
		expand(parts, tree)
	}
	return tree
}

func Print(w io.Writer, t *Node) error {
	p := printer{w: w, root: t, n: count(t), colorHook: NoColor}
	return p.walk(t, "")
}

func PrintColor(w io.Writer, t *Node, hook func(*Node) string) error {
	p := printer{w: w, root: t, n: count(t), colorHook: hook}
	return p.walk(t, "")
}

// PrintHeight will return the height of the output if Print is called.
func PrintHeight(tree *Node) int {
	if len(tree.children) == 0 {
		return 1
	}
	var n int
	for _, ch := range tree.children {
		n += PrintHeight(ch)
	}
	return n + 1
}

func ColorFolders(n *Node) string {
	switch n.Type {
	case TreeNode:
		return "\x1b[01;34m"
	case LeafNode:
		return ""
	default:
		return ""
	}
}

func NoColor(*Node) string { return "" }

func expand(parts []string, tree *Node) {
	if tree == nil {
		return
	}
	if len(parts) == 1 {
		tree.insertChild(createNode(parts[0], LeafNode))
		return
	}
	child, ok := tree.children[parts[0]]
	if !ok {
		child = createNode(parts[0], TreeNode)
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
	if t == nil {
		return nil
	}
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
		err := p.writef("%s%s %s%s\033[0m\n", prefix, line, color, node.Name)
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
	_, err := p.w.Write([]byte(fmt.Sprintf(format, v...)))
	return err
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

func createNode(name string, tp NodeType) *Node {
	return &Node{Name: name, Type: tp}
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
