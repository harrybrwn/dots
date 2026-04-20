package tui

import (
	"cmp"
	"fmt"
	"log/slog"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type treeEntry struct {
	path, name string
	isDir      bool
	expanded   bool
	depth      int
	style      *lipgloss.Style
}

type treeModel struct {
	tree     Tree
	logger   *slog.Logger
	settings Settings
	// state
	entries       []*treeEntry
	selected      int // currently selected tree entry index
	cursor        int // cursor's y-axis cell position
	width         int
	height        int
	err           error
	closers       []func() error
	help          help.Model
	cachedHelpMsg string
}

func (m *treeModel) Init() tea.Cmd {
	m.settings.Icons.fill() // extrapolate any empty values
	m.help = help.New()
	m.cachedHelpMsg = m.help.View(&m.settings.Keys)
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
	m.help.ShowAll = on
	m.cachedHelpMsg = m.help.View(&m.settings.Keys)
	helpHeight := lipgloss.Height(m.cachedHelpMsg)
	h := m.height - 1 - helpHeight
	m.cursor = clamp(m.cursor, 0, h)
	if m.selected >= len(m.entries)-1-helpHeight {
		// TODO: This is not exactly right but it works for now.
		m.cursor = h
	}
}

func (m *treeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		keys := m.settings.Keys
		switch {
		case key.Matches(msg, keys.Help):
			m.setHelp(!m.help.ShowAll)
		case key.Matches(msg, keys.Esc):
			m.setHelp(false) // disable full help message
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
			m.cursor = m.height - 1 - lipgloss.Height(m.cachedHelpMsg)

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
			if sel.isDir {
				if sel.expanded {
					m.collapse(m.selected)
				} else {
					m.expand(m.selected)
				}
			} else {
				m.log().Info("selected", "path", sel.path)
			}
		case key.Matches(msg, keys.ExpandDir):
			sel := m.entries[m.selected]
			if sel.isDir {
				if !sel.expanded {
					m.expand(m.selected)
				}
			}
		case key.Matches(msg, keys.CollapseDir):
			sel := m.entries[m.selected]
			if sel.isDir && sel.expanded {
				if sel.expanded {
					m.collapse(m.selected)
				}
			} else {
				// find the first open parent above the current selected entry
				for i := m.selected; i >= 0; i-- {
					e := m.entries[i]
					if e.isDir && e.depth < sel.depth && e.expanded {
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
				if m.entries[i].isDir && !m.entries[i].expanded {
					m.expand(i)
					goto expandAll
				}
			}
			h := m.height - 1 - lipgloss.Height(m.cachedHelpMsg)
			m.cursor = clamp(m.cursor, 0, h)
		case key.Matches(msg, keys.CollapseAll):
			m.log().Debug("collapse all")
		collapseAll:
			for i := range m.entries {
				if m.entries[i].isDir && m.entries[i].expanded {
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
	height := m.height - 1 - lipgloss.Height(m.cachedHelpMsg)
	m.cursor = min(m.cursor+n, height, len(m.entries)-1)
	m.selected = min(m.selected+n, len(m.entries)-1)
}

func (m *treeModel) shiftUp(n int) {
	if m.selected <= m.cursor {
		return
	}
	m.selected = max(m.selected-n, 0)
}

func (m *treeModel) shiftDown(n int) {
	start := max(m.selected-m.cursor, 0)
	h := m.height - lipgloss.Height(m.cachedHelpMsg)
	if start+h >= len(m.entries) {
		return
	}
	m.selected = min(m.selected+n, len(m.entries)-1)
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

		h     = m.height - lipgloss.Height(m.cachedHelpMsg)
		start = max(m.selected-m.cursor, 0)
		end   = min(start+h, len(m.entries))
		last  = end - 1 // final index
	)

	for i := start; i < end; i++ {
		e = m.entries[i]

		if m.selected == i {
			cursor = m.settings.Colors.Cursor.Render(m.settings.Icons.Cursor)
			if e.isDir {
				nameStyle = m.settings.Colors.SelectedFolder
			} else {
				nameStyle = m.settings.Colors.SelectedFile
			}
		} else {
			cursor = " "
			if e.isDir {
				nameStyle = m.settings.Colors.Folder
			} else {
				nameStyle = m.settings.Colors.File
			}
		}

		// Icon logic
		if e.isDir {
			if e.expanded {
				icon = m.settings.Icons.expanded(m.selected == i)
			} else {
				icon = m.settings.Icons.collapsed(m.selected == i)
			}
		} else {
			icon = m.settings.Icons.file(m.selected == i)
		}

		// custom styles
		if e.style != nil {
			nameStyle = *e.style
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
			nameStyle.Render(e.name),
		)
		s.WriteString(line)
		if i != last {
			s.WriteByte('\n')
		}
	}

	return lipgloss.JoinVertical(lipgloss.Top,
		s.String(),
		m.settings.Styles.Help.
			Height(m.height-len(m.entries)).
			AlignVertical(lipgloss.Bottom).
			Render(m.cachedHelpMsg),
	)
}

// expand reads a directory and inserts its children into the slice
func (m *treeModel) expand(index int) {
	root := m.entries[index]
	leaves, err := m.tree.Expand(root.path)
	if err != nil {
		fmt.Printf("Failed to expand: %v\n", err)
		m.log().Error("failed to expand", "error", err)
		m.err = err
		return
	}
	children := make([]*treeEntry, len(leaves))
	for i, leaf := range leaves {
		children[i] = &treeEntry{
			path:  leaf.Path,
			name:  leaf.Name,
			isDir: leaf.IsDir,
			style: leaf.Style,
			depth: root.depth + 1,
		}
	}
	newEntries := make([]*treeEntry, 0, len(m.entries)+len(children))
	// Insert children after the parent
	newEntries = append(newEntries, m.entries[:index+1]...)
	newEntries = append(newEntries, children...)
	newEntries = append(newEntries, m.entries[index+1:]...)
	m.entries = newEntries
	root.expanded = true
	m.log().Debug("expand", "path", root.path)
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
	m.log().Debug("collapse")
}

func (m *treeModel) log() *slog.Logger {
	return m.logger.With(
		"cur", m.cursor,
		"sel", m.selected,
		// "entries", len(m.entries),
		// "h", m.height,
	)
}

func clamp[T cmp.Ordered](v, smallest, largest T) T {
	return max(
		smallest,
		min(largest, v),
	)
}
