// Package tui holds the cli ui components.
package tui

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const LogFilename = "dots-tui.log"

func LogFilepath() string {
	cacheHome := os.Getenv("XDG_CACHE_HOME")
	if len(cacheHome) == 0 {
		cacheHome = filepath.Join(os.Getenv("HOME"), ".cache")
	}
	return filepath.Join(cacheHome, LogFilename)
}

func Run(tree Tree) error {
	f, err := os.OpenFile(
		LogFilepath(),
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
			Preview: NewStatPreview(),
			tree:    tree,
			logger:  l,
			settings: Settings{
				Icons:  DefaultIcons(),
				Colors: DefaultColors(),
				Keys:   DefaultKeys(DefaultHelpIcons()),
				Styles: DefaultStyles(),
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
		TreeEntry: TreeEntry{
			Path:  root.Path,
			Name:  root.Name,
			IsDir: true,
		},
		depth:    0,
		expanded: true,
	}
	leaves, err := m.tree.Expand(root.Path)
	if err == nil {
		for _, leaf := range leaves {
			m.entries = append(m.entries, &treeEntry{
				TreeEntry: leaf,
				depth:     1,
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
	keys   help.KeyMap
	cached string
	height int
	width  int
}

func NewHelp(keys help.KeyMap) *Help {
	model := help.New()
	cached := model.View(keys)
	return &Help{
		model:  model,
		keys:   keys,
		cached: cached,
		height: lipgloss.Height(cached),
		width:  lipgloss.Width(cached),
	}
}

func (h *Help) Height() int { return h.height }
func (h *Help) Width() int  { return h.width }

func (h *Help) Toggle() {
	h.Set(!h.model.ShowAll)
}

func (h *Help) All() bool { return h.model.ShowAll }

func (h *Help) Set(on bool) {
	h.model.ShowAll = on
	h.cached = h.model.View(h.keys)
	h.height = lipgloss.Height(h.cached)
	h.width = lipgloss.Width(h.cached)
}

func (h *Help) View() string {
	return h.cached
}

func NewStatPreview() *StatPreview {
	return &StatPreview{
		root: os.Getenv("HOME"),
	}
}

type StatPreview struct {
	root    string
	current *TreeEntry
}

func (sv *StatPreview) View() string {
	path := filepath.Join(sv.root, sv.current.Path)
	stat, err := os.Stat(path)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "path:     %s\n", path)
	fmt.Fprintf(&b, "name:     %s\n", stat.Name())
	fmt.Fprintf(&b, "size:     %d\n", stat.Size())
	fmt.Fprintf(&b, "mode:     %s\n", stat.Mode())
	fmt.Fprintf(&b, "modified: %s\n", stat.ModTime().String())
	fmt.Fprintln(&b)
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	fmt.Fprintf(&b, "%s\n", content)
	return b.String()
}

func (sv *StatPreview) IsOpen() bool { return sv.current != nil }

func (sv *StatPreview) Open(e *TreeEntry) {
	sv.current = e
}

func (sv *StatPreview) Close() {
	sv.current = nil
}

type NoPreview struct{ path string }

func (np *NoPreview) View() string      { return np.path }
func (np *NoPreview) Open(e *TreeEntry) { np.path = e.Path }
func (np *NoPreview) Close()            { np.path = "" }
func (np *NoPreview) IsOpen() bool      { return len(np.path) > 0 }
