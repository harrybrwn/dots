// Package tui holds the cli ui components.
package tui

import (
	"io"
	"log/slog"
	"os"

	"github.com/charmbracelet/bubbles/help"
	tea "github.com/charmbracelet/bubbletea"
)

func Run(tree Tree) error {
	f, err := os.OpenFile(
		os.ExpandEnv("$HOME/.cache/dots.log"),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0644,
	)
	if err != nil {
		return err
	}
	defer f.Close()
	l := logger(f)
	slog.SetDefault(l)
	m := Model{
		tree: treeModel{
			tree:   tree,
			logger: l,
			settings: Settings{
				Icons:  DefaultIcons(),
				Colors: DefaultColors(),
				Keys:   DefaultKeys(),
			},
		},
	}
	initialModel(&m.tree)
	p := tea.NewProgram(&m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

type Model struct {
	tree treeModel
}

func (m *Model) Init() tea.Cmd {
	return m.tree.Init()
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m.tree.Update(msg)
}

func (m *Model) View() string {
	return m.tree.View()
}

// initialModel sets up the root directory
func initialModel(m *treeModel) *treeModel {
	// width, height, err := term.GetSize(0)
	root, err := m.tree.Root()
	if err != nil {
		panic(err)
	}
	m.entries = make([]*treeEntry, 1, 16)
	m.entries[0] = &treeEntry{
		path: root.Path,
		name: root.Name, isDir: true, depth: 0,
		expanded: true,
	}
	leaves, err := m.tree.Expand(root.Path)
	if err == nil {
		for _, leaf := range leaves {
			m.entries = append(m.entries, &treeEntry{
				path:  leaf.Path,
				name:  leaf.Name,
				isDir: leaf.IsDir,
				depth: 1,
				style: leaf.Style,
			})
		}
	}
	return m
}

func logger(w io.Writer) *slog.Logger {
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: false,
	}))
}

type Help struct {
	model  help.Model
	cached string
}
