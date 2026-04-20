package tui

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/charmbracelet/lipgloss"
	"github.com/harrybrwn/dots/git"
	"github.com/harrybrwn/dots/tree"
)

type TreeEntry struct {
	Path  string
	Name  string
	IsDir bool
	Style *lipgloss.Style
}

type Tree interface {
	Root() (*TreeEntry, error)
	Expand(dir string) ([]TreeEntry, error)
}

func NewOSTree() *osTree { return new(osTree) }

type osTree struct{}

func (ost *osTree) Root() (*TreeEntry, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return &TreeEntry{
		Path:  wd,
		Name:  filepath.Base(wd),
		IsDir: true,
	}, nil
}

func (ost *osTree) Expand(dir string) ([]TreeEntry, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	res := make([]TreeEntry, len(files))
	for i, f := range files {
		res[i] = TreeEntry{
			Path:  filepath.Join(dir, f.Name()),
			Name:  f.Name(),
			IsDir: f.IsDir(),
		}
	}
	slices.SortFunc(res, func(a, b TreeEntry) int {
		if a.IsDir {
			return -1
		} else if b.IsDir {
			return 1
		}
		return cmp.Compare(a.Name, b.Name)
	})
	return res, nil
}

func NewTree(root *tree.Node, mods map[string]git.ModType) *TreeTree {
	return &TreeTree{root: root, mods: mods}
}

func NewModifiedTree(root *tree.Node, mods map[string]git.ModType) *TreeTree {
	return &TreeTree{root: root, mods: mods, onlyModified: true}
}

type TreeTree struct {
	root         *tree.Node
	mods         modSet
	onlyModified bool
}

func (tt *TreeTree) Root() (*TreeEntry, error) {
	return &TreeEntry{
		Path:  tt.root.Name,
		Name:  tt.root.Name,
		IsDir: true,
	}, nil
}

// ModifiedInDirectory marks directories that contain modified files.
const ModifiedInDirectory = git.ModType(0x11)

func (tt *TreeTree) Expand(path string) ([]TreeEntry, error) {
	node, err := tt.root.Get(path)
	if err != nil {
		return nil, err
	}
	res := make([]TreeEntry, 0)
	for _, child := range node.GetChildren() {
		e := TreeEntry{
			Path:  filepath.Join(child.Path(), child.Name),
			Name:  child.Name,
			IsDir: child.Type == tree.TreeNode,
		}
		t, ok := tt.mods.get(child)
		if ok {
			style := lipgloss.NewStyle().Transform(func(s string) string {
				return fmt.Sprintf("%c %s", t, s)
			})
			var col string
			switch t {
			case git.ModDelete:
				col = "1"
			case git.ModChanged:
				// col = "214"
				col = "3"
			case git.ModUnmerged:
				col = "220"
			case git.ModAddition, git.ModRename:
				col = "32"
			case ModifiedInDirectory:
				style = style.UnsetTransform()
				col = "3"
			default:
				goto nostyle
			}
			style = style.
				Bold(false).
				Foreground(lipgloss.Color(col))
			if tt.onlyModified {
				if child.Type != tree.TreeNode {
					e.Style = &style
				}
			} else {
				e.Style = &style
			}
		nostyle:
		} else if tt.onlyModified {
			continue
		}
		res = append(res, e)
	}
	return res, nil
}

type modSet map[string]git.ModType

func (ms modSet) get(n *tree.Node) (git.ModType, bool) {
	p := filepath.Join(n.Path(), n.Name)
	if p[0] == '/' {
		p = p[1:]
	}
	t, ok := ms[p]
	return t, ok
}
