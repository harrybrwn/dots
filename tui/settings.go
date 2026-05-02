package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/harrybrwn/dots/pkg/nerdfonts"
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
	i := Icons{
		Cursor:       nerdfonts.FaArrowRight,
		Collapsed:    nerdfonts.CodChevronRight,
		Expanded:     nerdfonts.CodChevronDown,
		File:         nerdfonts.OctDot,
		SelectedFile: nerdfonts.OctDotFill,
	}
	return i
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
	Help,
	Screen,
	Selected lipgloss.Style
}

func DefaultStyles() Styles {
	s := Styles{
		Help:     lipgloss.NewStyle().AlignVertical(lipgloss.Bottom),
		Screen:   lipgloss.NewStyle(),
		Selected: lipgloss.NewStyle(),
	}
	return s
}

type Keys struct {
	Help,
	Esc,
	Quit,
	Up,
	Down,
	GotoTop,
	GotoBottom,
	PageUp,
	PageDown,
	HalfPageUp,
	HalfPageDown,
	ToggleDir,
	ExpandDir,
	CollapseDir,
	ExpandAll,
	CollapseAll,
	ShiftUp,
	ShiftDown,
	Select,
	Left,
	Right key.Binding
}

func DefaultKeys(icons HelpIcons) Keys {
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
			key.WithHelp(icons.Up+"/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp(icons.Down+"/j", "move down"),
		),
		GotoTop: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("g/home", "go to top"),
		),
		GotoBottom: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("G/end", "go to bottom"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", " ", "ctrl+d"),
			key.WithHelp("C-d/pgdn", "page down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "ctrl+u"),
			key.WithHelp("C-u/pgup", "page up"),
		),
		HalfPageUp: key.NewBinding(
			key.WithKeys("u", "ctrl+u"),
			key.WithHelp("u/ctrl+u", icons.Half+" page up"),
		),
		HalfPageDown: key.NewBinding(
			key.WithKeys("d", "ctrl+d"),
			key.WithHelp("d/ctrl+d", icons.Half+" page down"),
		),
		ToggleDir: key.NewBinding(
			key.WithKeys("tab", "o", "enter"),
			key.WithHelp(
				fmt.Sprintf("%s/%s/o", icons.Tab, icons.Return),
				"toggle dir",
			),
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
		Left: key.NewBinding(
			key.WithKeys("h", "left"),
			key.WithHelp("h/"+icons.Left, "move left"),
		),
		Right: key.NewBinding(
			key.WithKeys("l", "right"),
			key.WithHelp("l/"+icons.Right, "move right"),
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
		{k.Left, k.Right},
	}
}

func (k *Keys) viewportKeys() viewport.KeyMap {
	return viewport.KeyMap{
		PageDown:     k.PageDown,
		PageUp:       k.PageUp,
		HalfPageUp:   k.HalfPageUp,
		HalfPageDown: k.HalfPageDown,
		Down:         k.Down,
		Up:           k.Up,
		Left:         k.Left,
		Right:        k.Right,
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

func (s *Settings) interpolate() {
	i := s.Icons
	if len(s.Icons.SelectedCollapsed) == 0 {
		s.Icons.SelectedCollapsed = s.Icons.Collapsed
	}
	if len(i.SelectedExpanded) == 0 {
		s.Icons.SelectedExpanded = s.Icons.Expanded
	}
	if len(s.Icons.SelectedFile) == 0 {
		s.Icons.SelectedFile = s.Icons.File
	}
	if len(s.Icons.Cursor) == 0 {
		s.Icons.Cursor = ">"
	}
}

type HelpIcons struct {
	Up,
	Down,
	Left,
	Right,
	Return,
	Tab,
	Half string
}

func DefaultHelpIcons() HelpIcons {
	return HelpIcons{
		Up:     nerdfonts.FaArrowUp,
		Down:   nerdfonts.FaArrowDown,
		Left:   nerdfonts.MdArrowLeft,
		Right:  nerdfonts.MdArrowRight,
		Return: nerdfonts.MdKeyboardReturn,
		Tab:    nerdfonts.OctTab,
		Half:   "½", // U+00bd
	}
}
