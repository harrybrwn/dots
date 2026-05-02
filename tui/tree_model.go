package tui

import (
	"cmp"
	"fmt"
	"log/slog"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Preview interface {
	View() string
	Open(*TreeEntry)
	IsOpen() bool
	Close()
}

type treeEntry struct {
	TreeEntry
	expanded bool
	depth    int
}

type treeModel struct {
	tree     Tree
	logger   *slog.Logger
	settings Settings
	// state
	entries  []*treeEntry
	selected int // currently selected tree entry index
	cursor   int // cursor's y-axis terminal cell position
	width    int
	height   int
	err      error
	closers  []func() error
	help     *Help

	previewStyle lipgloss.Style
	widestPath   int
	Preview      Preview
	previewBox   viewport.Model
}

func (m *treeModel) Init() tea.Cmd {
	m.settings.interpolate() // handle empty values
	m.help = NewHelp(&m.settings.Keys)
	m.previewStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder(), false, false, false, true).
		PaddingLeft(1).
		MarginLeft(1)
	return nil
}

func (m *treeModel) close() error {
	for _, fn := range m.closers {
		e := fn()
		if e != nil {
			return e
		}
	}
	return nil
}

func (m *treeModel) setHelp(on bool) {
	m.help.Set(on)
	helpHeight := m.help.Height()
	h := m.height - 1 - helpHeight
	m.cursor = clamp(m.cursor, 0, h)
	if m.selected >= len(m.entries)-1-helpHeight {
		// TODO: This is not exactly right but it works for now. If you expand
		// all, go to the second-to-last item and toggle the help message it
		// still gets messed up.
		m.cursor = h
	}
	m.log().Info("set help", "show", on)
}

func (m *treeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		borderHeight := m.settings.Styles.Screen.GetBorderTopSize() + m.settings.Styles.Screen.GetBorderBottomSize()
		m.width = msg.Width
		m.height = msg.Height - borderHeight
		m.previewBox = viewport.New(m.width, m.height-m.help.Height())
		m.previewBox.KeyMap = m.settings.Keys.viewportKeys()
		if m.Preview.IsOpen() {
			m.previewBox.SetContent(m.Preview.View())
		}

	case tea.KeyMsg:
		keys := m.settings.Keys
		switch {
		case key.Matches(msg, keys.Help):
			m.setHelp(!m.help.All())
		case key.Matches(msg, keys.Esc):
			m.setHelp(false) // disable full help message
			m.Preview.Close()
			// TODO: Alter state...
		case key.Matches(msg, keys.Quit):
			if err := m.close(); err != nil {
				panic(err)
			}
			return m, tea.Quit

		case key.Matches(msg, keys.GotoTop):
			m.cursor = 0
			m.selected = 0
		case key.Matches(msg, keys.GotoBottom):
			m.selected = len(m.entries) - 1
			m.cursor = m.height - 1 - m.help.Height()

		case key.Matches(msg, keys.Up):
			m.up(1)
		case key.Matches(msg, keys.Down):
			m.down(1)
		case key.Matches(msg, keys.HalfPageUp):
			m.up(m.height / 2)
		case key.Matches(msg, keys.HalfPageDown):
			m.down(m.height / 2)
		case key.Matches(msg, keys.ShiftUp):
			m.shiftUp(1)
		case key.Matches(msg, keys.ShiftDown):
			m.shiftDown(1)

		case key.Matches(msg, keys.ToggleDir):
			sel := m.entries[m.selected]
			if sel.IsDir {
				if sel.expanded {
					m.collapse(m.selected)
				} else {
					m.expand(m.selected)
				}
			} else {
				m.log().Info("selected", "path", sel.Path)
				m.Preview.Open(&sel.TreeEntry)
			}
		case key.Matches(msg, keys.ExpandDir):
			sel := m.entries[m.selected]
			if sel.IsDir {
				if !sel.expanded {
					m.expand(m.selected)
				}
			} else {
				m.Preview.Open(&sel.TreeEntry)
			}
		case key.Matches(msg, keys.CollapseDir):
			if m.Preview.IsOpen() {
				m.Preview.Close()
			}
			sel := m.entries[m.selected]
			if sel.IsDir && sel.expanded {
				if sel.expanded {
					m.collapse(m.selected)
				}
			} else {
				// find the first open parent above the current selected entry
				for i := m.selected; i >= 0; i-- {
					e := m.entries[i]
					if e.IsDir && e.depth < sel.depth && e.expanded {
						m.collapse(i)
						m.selected = i
						break
					}
				}
			}

		case key.Matches(msg, keys.ExpandAll):
			m.log().Debug("expand all")
		expandAll:
			for i := len(m.entries) - 1; i >= 0; i-- {
				if m.entries[i].IsDir && !m.entries[i].expanded {
					m.expand(i)
					goto expandAll
				}
			}
			h := m.height - 1 - m.help.Height()
			m.cursor = clamp(m.cursor, 0, h)
		case key.Matches(msg, keys.CollapseAll):
			m.log().Debug("collapse all")
		collapseAll:
			for i := range m.entries {
				if m.entries[i].IsDir && m.entries[i].expanded {
					m.collapse(i)
					goto collapseAll
				}
			}
			m.cursor = 0
			m.selected = 0
			m.expand(m.selected)
		}
	}
	return m, nil
}

func (m *treeModel) up(n int) {
	m.cursor = max(m.cursor-n, 0)
	m.selected = max(m.selected-n, 0)
}

func (m *treeModel) down(n int) {
	height := m.height - 1 - m.help.Height()
	m.cursor = min(m.cursor+n, height, len(m.entries)-1)
	m.selected = min(m.selected+n, len(m.entries)-1)
}

func (m *treeModel) shiftUp(n int) {
	start := m.selected - m.cursor
	height := m.height - m.help.Height()
	if start > 0 {
		if m.cursor < height-1 {
			// Shift the entire screen.
			m.cursor = min(m.cursor+n, height)
		} else {
			m.selected = m.selected - n
		}
	}
	m.log().Info("shift up", "screen-h", height)
}

func (m *treeModel) shiftDown(n int) {
	var (
		h     = m.height - m.help.Height()
		start = max(m.selected-m.cursor, 0)
		end   = start + h
	)
	if end < len(m.entries) {
		if m.cursor > 0 {
			// Shift the entire screen. This is the most common case.
			m.cursor = max(m.cursor-n, 0)
		} else {
			// cursor is at the top and can't be shifted
			m.selected = m.selected + n
		}
	}
	m.log().Info("shift down")
}

func (m *treeModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress q to quit", m.err)
	}

	var (
		s         strings.Builder
		nameStyle lipgloss.Style
		cursor    string
		icon      string
		e         *treeEntry

		h     = m.height - m.help.Height()
		start = max(m.selected-m.cursor, 0)
		end   = min(start+h, len(m.entries))
		last  = end - 1 // final index
	)

	for i := start; i < end; i++ {
		e = m.entries[i]

		if m.selected == i {
			cursor = m.settings.Colors.Cursor.Render(m.settings.Icons.Cursor)
			if e.IsDir {
				nameStyle = m.settings.Colors.SelectedFolder
			} else {
				nameStyle = m.settings.Colors.SelectedFile
			}
		} else {
			cursor = " "
			if e.IsDir {
				nameStyle = m.settings.Colors.Folder
			} else {
				nameStyle = m.settings.Colors.File
			}
		}

		// Icon logic
		if e.IsDir {
			if e.expanded {
				icon = m.settings.Icons.expanded(m.selected == i)
			} else {
				icon = m.settings.Icons.collapsed(m.selected == i)
			}
		} else {
			icon = m.settings.Icons.file(m.selected == i)
		}

		// custom styles
		if e.Style != nil {
			nameStyle = *e.Style
			if m.selected == i {
				nameStyle = nameStyle.Bold(true)
			}
		}

		// Indentation based on depth
		indent := strings.Repeat("  ", e.depth)

		line := fmt.Sprintf(
			" %s %s%s %s",
			cursor,
			indent,
			icon,
			nameStyle.Render(e.Name),
		)
		s.WriteString(line)
		if i != last {
			s.WriteByte('\n')
		}
	}

	helpView := m.settings.Styles.Help.
		Height(m.height - len(m.entries)).
		AlignVertical(lipgloss.Bottom).
		Render(m.help.View())
	treeView := s.String()
	screen := treeView

	if m.Preview.IsOpen() {
		screen = lipgloss.JoinHorizontal(lipgloss.Top,
			treeView,
			m.preview(lipgloss.Width(treeView)))
	}
	return lipgloss.JoinVertical(lipgloss.Top,
		m.settings.Styles.Screen.Render(screen),
		helpView)
}

func (m *treeModel) preview(treeWidth int) string {
	previewStyle := m.previewStyle.
		Height(min(m.height-m.help.Height(), len(m.entries))).
		MarginLeft(m.widestPath + 1 - treeWidth)
	view := m.Preview.View()
	return previewStyle.Render(view)
}

// expand reads a directory and inserts its children into the slice
func (m *treeModel) expand(index int) {
	root := m.entries[index]
	leaves, err := m.tree.Expand(root.Path)
	if err != nil {
		fmt.Printf("Failed to expand: %v\n", err)
		m.log().Error("failed to expand", "error", err)
		m.err = err
		return
	}
	children := make([]*treeEntry, len(leaves))
	for i, leaf := range leaves {
		children[i] = &treeEntry{
			TreeEntry: leaf,
			depth:     root.depth + 1,
			expanded:  false,
		}
	}
	newEntries := make([]*treeEntry, 0, len(m.entries)+len(children))
	// Insert children after the parent
	newEntries = append(newEntries, m.entries[:index+1]...)
	newEntries = append(newEntries, children...)
	newEntries = append(newEntries, m.entries[index+1:]...)
	m.entries = newEntries
	root.expanded = true
	m.widestPath = 0
	for _, e := range m.entries {
		m.widestPath = max(m.widestPath, m.lineWidth(e))
	}
	m.log().Debug("expand", "path", root.Path)
}

// collapse removes all nested children from the slice
func (m *treeModel) collapse(index int) {
	e := m.entries[index]
	e.expanded = false
	start := index + 1
	end := start
	for end < len(m.entries) && m.entries[end].depth > e.depth {
		end++
	}
	m.entries = append(m.entries[:start], m.entries[end:]...)
	m.widestPath = 0
	for _, e := range m.entries {
		m.widestPath = max(m.widestPath, m.lineWidth(e))
	}
	m.log().Debug("collapse")
}

func (m *treeModel) lineWidth(entry *treeEntry) int {
	w := len(entry.Name)
	w += len(m.settings.Icons.Expanded)
	w += entry.depth * 2                  // indent
	w += 2 + len(m.settings.Icons.Cursor) // cursor and cursor spacing
	return w
}

func (m *treeModel) log() *slog.Logger {
	return m.logger.With(
		"cur", m.cursor,
		"sel", m.selected,
		// "entries", len(m.entries),
		"h", m.height, "w", m.width,
	)
}

func clamp[T cmp.Ordered](v, smallest, largest T) T {
	return max(
		smallest,
		min(largest, v),
	)
}
