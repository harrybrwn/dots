package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)

type Settings struct {
	Icons  Icons
	Colors Colors
	Keys   Keys
	Styles Styles
}

type Icons struct {
	Cursor,
	Collapsed,
	Expanded,
	SelectedCollapsed,
	SelectedExpanded,
	File,
	SelectedFile string
}

func DefaultIcons() Icons {
	return Icons{
		Cursor: ">",
		// Collapsed: "",
		// Expanded:  "",
		Collapsed: "",
		Expanded:  "",
		// SelectedCollapsed: "",
		// SelectedExpanded: "",

		SelectedFile: "",
		File:         "",
	}
}

func CircleIcons() Icons {
	return Icons{
		Cursor:            ">",
		Collapsed:         "󰬪",
		Expanded:          "󰬦",
		SelectedCollapsed: "󰬫",
		SelectedExpanded:  "󰬧",
		File:              "",
		SelectedFile:      "",
	}
}

func FatIcons() Icons {
	return Icons{
		Cursor:    ">",
		Collapsed: "▶",
		Expanded:  "▼",
		File:      "○",
	}
}

type Colors struct {
	Cursor,
	SelectedFile,
	SelectedFolder,
	File,
	Folder lipgloss.Style
}

func DefaultColors() Colors {
	folder := "31"
	// folder := "4"

	c := Colors{
		Cursor: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212")),
		File: lipgloss.NewStyle().Foreground(lipgloss.Color("255")),
		SelectedFile: lipgloss.NewStyle().Foreground(lipgloss.Color("255")).
			Bold(true),
		Folder: lipgloss.NewStyle().Foreground(lipgloss.Color(folder)),
	}
	c.SelectedFolder = c.Folder.Bold(true)
	return c
}

type Styles struct {
	Help lipgloss.Style
}

func DefaultStyles() Styles {
	return Styles{
		Help: lipgloss.NewStyle().AlignVertical(lipgloss.Bottom),
	}
}

type Keys struct {
	Help,
	Esc,
	Quit,
	Up,
	Down,
	GotoTop,
	GotoBottom,
	// PageUp,
	// PageDown,
	HalfPageUp,
	HalfPageDown,
	ToggleDir,
	ExpandDir,
	CollapseDir,
	ExpandAll,
	CollapseAll,
	ShiftUp,
	ShiftDown,
	Select key.Binding
}

func DefaultKeys() Keys {
	return Keys{
		Esc: key.NewBinding(
			key.WithKeys("esc"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "view help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c", "q"),
			key.WithHelp("q/C-c", "quit"),
		),
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("↓/j", "move down"),
		),
		GotoTop: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("g/home", "go to top"),
		),
		GotoBottom: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("G/end", "go to bottom"),
		),
		// PageUp: key.NewBinding(
		// 	key.WithKeys("pgup"),
		// 	key.WithHelp("b/pgup", "page up"),
		// ),
		// PageDown: key.NewBinding(
		// 	key.WithKeys("pgdown", " "),
		// 	key.WithHelp("f/pgdn", "page down"),
		// ),
		HalfPageUp: key.NewBinding(
			key.WithKeys("u", "ctrl+u"),
			key.WithHelp("C-u", "½ page up"),
		),
		HalfPageDown: key.NewBinding(
			key.WithKeys("d", "ctrl+d"),
			key.WithHelp("C-d", "½ page down"),
		),
		ToggleDir: key.NewBinding(
			key.WithKeys("tab", "o", "enter"),
			key.WithHelp("󰌑//o", "toggle dir"),
		),
		ExpandDir: key.NewBinding(
			key.WithKeys("l"),
		),
		CollapseDir: key.NewBinding(
			key.WithKeys("h"),
		),
		ExpandAll: key.NewBinding(
			key.WithKeys("E"),
			key.WithHelp("E", "expand all"),
		),
		CollapseAll: key.NewBinding(
			key.WithKeys("W"),
			key.WithHelp("W", "collapse all"),
		),
		ShiftUp: key.NewBinding(
			key.WithKeys("ctrl+y"),
			key.WithHelp("C-y", "shift up"),
		),
		ShiftDown: key.NewBinding(
			key.WithKeys("ctrl+e"),
			key.WithHelp("C-e", "shift down"),
		),
	}
}

func (k *Keys) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Help,
		k.Quit,
		k.Up,
		k.Down,
		k.ToggleDir,
	}
}

func (k *Keys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Help, k.Quit, k.Up, k.Down},
		{k.HalfPageUp, k.HalfPageDown, k.GotoTop, k.GotoBottom},
		{k.ToggleDir, k.ExpandDir, k.CollapseDir},
		{k.ExpandAll, k.CollapseAll},
	}
}

func (i *Icons) collapsed(selected bool) string {
	if selected {
		return i.SelectedCollapsed
	}
	return i.Collapsed
}

func (i *Icons) expanded(selected bool) string {
	if selected {
		return i.SelectedExpanded
	}
	return i.Expanded
}

func (i *Icons) file(selected bool) string {
	if selected {
		return i.SelectedFile
	}
	return i.File
}

func (i *Icons) fill() {
	if len(i.SelectedCollapsed) == 0 {
		i.SelectedCollapsed = i.Collapsed
	}
	if len(i.SelectedExpanded) == 0 {
		i.SelectedExpanded = i.Expanded
	}
	if len(i.SelectedFile) == 0 {
		i.SelectedFile = i.File
	}
	if len(i.Cursor) == 0 {
		i.Cursor = ">"
	}
}
