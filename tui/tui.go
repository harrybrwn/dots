// Package tui holds the cli ui components.
package tui

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const LogFilename = "dots-tui.log"

func LogFilepath() string {
	cacheHome := os.Getenv("XDG_CACHE_HOME")
	if len(cacheHome) == 0 {
		home := os.Getenv("HOME")
		if len(home) == 0 {
			cacheHome = "/var/log"
		} else {
			cacheHome = filepath.Join(os.Getenv("HOME"), ".cache")
		}
	}
	return filepath.Join(cacheHome, LogFilename)
}

func Run(ctx context.Context, tree Tree, preview Preview) error {
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
	l.Info("starting tui")
	slog.SetDefault(l)
	keys := DefaultKeys(DefaultHelpIcons())
	if preview == nil {
		preview = NewStatPreview(keys)
	}
	settings := Settings{
		Icons:  DefaultIcons(),
		Colors: DefaultColors(),
		Keys:   keys,
		Styles: DefaultStyles(),
		Popups: DefaultPopupSettings(),
	}
	m := Model{
		logger:   l,
		settings: settings,
		tree: treeModel{
			Preview:  preview,
			tree:     tree,
			logger:   l,
			settings: settings,
		},
		errStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("1")),
	}
	initialModel(&m.tree)

	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()
	p := tea.NewProgram(&m, tea.WithContext(ctx), tea.WithAltScreen())
	_, err = p.Run()
	return err
}

type Model struct {
	logger        *slog.Logger
	settings      Settings
	tree          treeModel
	popup         string
	height, width int

	errors   []ErrorPopup
	errStyle lipgloss.Style
}

func (m *Model) Init() tea.Cmd {
	return m.tree.Init()
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	cmds := make([]tea.Cmd, 0, 1)
	switch msg := msg.(type) {
	case ErrorMsg:
		m.logger.Error("ErrorMsg", "error", msg.Error)
		m.errors = append(m.errors, ErrorPopup{msg: msg})
		duration := m.settings.Popups.ErrorDismissDuration
		cmds = append(cmds, ClearErrorAfter(duration))
	case ClearErrorMsg:
		if len(m.errors) > 0 {
			m.errors = m.errors[1:]
		}
	case PopupMsg:
		m.popup = string(msg)
	case ClearPopupMsg:
		m.popup = ""
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
		m.logger.Info("win size", "h", msg.Height, "w", msg.Width)
		_, cmd = m.tree.Update(msg)
		cmds = append(cmds, cmd)
	case tea.KeyMsg:
		_, cmd = m.tree.Update(msg)
		cmds = append(cmds, cmd)
	default:
		_, cmd = m.tree.Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m *Model) View() string {
	view := m.tree.View()
	treeWidth := lipgloss.Width(view)
	now := time.Now()
	if len(m.errors) > 0 {
		errorsHeight := 1
		var buf strings.Builder
		for i := range m.errors {
			if errorsHeight >= m.height {
				m.logger.Info("exceeded height limit", "errors", errorsHeight, "limit", m.height)
				break
			}
			if m.errors[i].shownAt == nil {
				m.errors[i].shownAt = &now
			}
			errStyleWidth := m.errStyle.GetBorderLeftSize() + m.errStyle.GetBorderRightSize()
			rawErrStr := strings.TrimRight(m.errors[i].msg.Error.Error(), "\n \t")
			errWidth := min(m.width-treeWidth-errStyleWidth, lipgloss.Width(rawErrStr))
			e := titledTopBorder(m.errStyle, lipgloss.Left, "Error", errWidth) + "\n" + m.errStyle.
				BorderTop(false).
				Width(errWidth).
				MaxWidth(m.width-treeWidth).
				Render(rawErrStr)
			errorsHeight += lipgloss.Height(e)
			errmsg := lipgloss.Place(
				m.width-treeWidth,
				lipgloss.Height(e),
				lipgloss.Right,
				lipgloss.Top,
				e,
			)
			buf.WriteString(errmsg)
			if i < len(m.errors)-1 {
				buf.WriteByte('\n')
			}
		}
		view = lipgloss.JoinHorizontal(lipgloss.Top, view, buf.String())
	}
	return view
}

type ErrorPopup struct {
	msg     ErrorMsg
	shownAt *time.Time
}

func titledTopBorder(style lipgloss.Style, position lipgloss.Position, title string, bodyWidth int) string {
	border := style.GetBorderStyle()
	var buf strings.Builder
	buf.WriteString(border.TopLeft)
	if len(title) > 0 {
		switch position {
		case lipgloss.Right:
			lineWidth := bodyWidth - len(title) - 3
			buf.WriteString(strings.Repeat(border.Top, lineWidth))
			buf.WriteByte(' ')
			buf.WriteString(title)
			buf.WriteByte(' ')
			buf.WriteString(border.Top)
		case lipgloss.Left:
			buf.WriteString(border.Top)
			buf.WriteByte(' ')
			buf.WriteString(title)
			buf.WriteByte(' ')
			lineWidth := bodyWidth - len(title) - 3
			buf.WriteString(strings.Repeat(border.Top, lineWidth))
		case lipgloss.Center:
			fallthrough
		default:
			lineWidth := max(0, (bodyWidth-len(title))/2)
			buf.WriteString(strings.Repeat(border.Top, lineWidth-1))
			buf.WriteByte(' ')
			buf.WriteString(title)
			buf.WriteByte(' ')
			if len(title)%2 == 0 {
				lineWidth -= 1
			}
			buf.WriteString(strings.Repeat(border.Top, lineWidth))
		}
	} else {
		buf.WriteString(strings.Repeat(border.Top, bodyWidth))
	}
	buf.WriteString(border.TopRight)
	return lipgloss.NewStyle().
		Foreground(style.GetBorderTopForeground()).
		Background(style.GetBorderTopBackground()).
		Render(buf.String())
}

// initialModel sets up the root directory
func initialModel(m *treeModel) *treeModel {
	root, err := m.tree.Root()
	if err != nil {
		panic(err)
	}
	m.entries = make([]*TreeEntry, 1, 16)
	m.entries[0] = &TreeEntry{
		Path:  root.Path,
		Name:  root.Name,
		IsDir: true,
		state: NodeStateExpanded,
		depth: 0,
	}
	leaves, err := m.tree.Expand(root.Path)
	if err == nil {
		for _, leaf := range leaves {
			m.entries = append(m.entries, &TreeEntry{
				Path:  leaf.Path,
				Name:  leaf.Name,
				IsDir: leaf.IsDir,
				Style: leaf.Style,
				state: NodeStateCollapsed,
				depth: 1,
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

// PartialModel is a [bubbletea.Model] that is missing an Init method.
type PartialModel interface {
	Update(tea.Msg) (tea.Model, tea.Cmd)
	View() string
}

// NoOpInitModel adds an Init method to a [PartialModel].
type NoOpInitModel struct{ PartialModel }

func (nim *NoOpInitModel) Init() tea.Cmd { return nil }

func Eprintf(format string, args ...any) tea.Cmd {
	return func() tea.Msg {
		fmt.Fprintf(os.Stderr, format, args...)
		return nil
	}
}
