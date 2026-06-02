package tui

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/harrybrwn/dots/git"
)

type Preview interface {
	View() string
	Open(*TreeEntry)
	IsOpen() bool
	Close()
	Update(tea.Msg) (tea.Model, tea.Cmd)
}

// PreviewCloseMsg signals when the preview has been closed. Send with
// [PreviewClose].
type PreviewCloseMsg struct{}

// PreviewClose sends a [PreviewCloseMsg] message.
func PreviewClose() tea.Msg {
	return PreviewCloseMsg{}
}

func NewStatPreview(keys Keys) *StatPreview {
	return &StatPreview{
		root: os.Getenv("HOME"),
		keys: keys,
	}
}

type StatPreview struct {
	root    string
	current *TreeEntry
	keys    Keys
	err     error
}

func (sv *StatPreview) IsOpen() bool      { return sv.current != nil }
func (sv *StatPreview) Open(e *TreeEntry) { sv.current = e }
func (sv *StatPreview) Close()            { sv.current = nil }

func (sv *StatPreview) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := make([]tea.Cmd, 0, 1)
	m := NoOpInitModel{sv}
	if sv.err != nil {
		cmds = append(cmds, SendError(sv.err))
		sv.err = nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, sv.keys.Quit, sv.keys.Esc):
			cmds = append(cmds, PreviewClose)
		}
	}
	if len(cmds) > 0 {
		return &m, tea.Batch(cmds...)
	}
	return &m, nil
}

func (sv *StatPreview) View() string {
	path := filepath.Join(sv.root, sv.current.Path)
	stat, err := os.Stat(path)
	if err != nil {
		sv.err = err
		return fmt.Sprintf("Error: %v", err)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "path:     %s\n", path)
	fmt.Fprintf(&b, "name:     %s\n", stat.Name())
	fmt.Fprintf(&b, "size:     %d\n", stat.Size())
	fmt.Fprintf(&b, "mode:     %s\n", stat.Mode())
	fmt.Fprintf(&b, "modified: %s\n", stat.ModTime().String())
	return b.String()
}

type ExecPreview struct {
	proc  *exec.Cmd
	entry *TreeEntry
	mods  modSet
}

func NewExecPreview(proc *exec.Cmd, mods map[string]git.ModType) *ExecPreview {
	return &ExecPreview{
		proc: proc,
		mods: mods,
	}
}

func (ep *ExecPreview) View() string      { return "" }
func (ep *ExecPreview) Open(e *TreeEntry) { ep.entry = e }
func (ep *ExecPreview) Close()            { ep.entry = nil }
func (ep *ExecPreview) IsOpen() bool      { return ep.entry != nil }

func (ep *ExecPreview) Update(tea.Msg) (tea.Model, tea.Cmd) {
	m := NoOpInitModel{ep}
	if ep.entry == nil {
		return &m, PreviewClose
	}
	stem := ep.entry.Path
	if stem[0] == '/' {
		stem = stem[1:]
	}
	if _, ok := ep.mods[stem]; !ok {
		slog.Debug("path not in mod set", "path", stem, "keys", slices.Collect(maps.Keys(ep.mods)))
		return &m, PreviewClose
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return &m, SendErrorf("could not get home directory: %w", err)
	}
	args := make([]string, len(ep.proc.Args))
	copy(args, ep.proc.Args)
	args = append(args, filepath.Join(home, stem))

	proc := *ep.proc
	var stderr bytes.Buffer
	proc.Stderr = &stderr
	proc.Stdout = os.Stdout
	proc.Stdin = os.Stdin
	proc.Args = args
	slog.Info("exec preview", "cmd", args)
	return &m, tea.ExecProcess(&proc, func(err error) tea.Msg {
		ep.Close() // make sure we're auto closed
		// write the process's stderr buffer to our own stderr
		errmsg := stderr.String()
		_, _ = io.Copy(os.Stderr, &stderr)
		if err != nil {
			slog.Error("failed exec preview",
				"error", err,
				"stderr", errmsg,
				"cmd", args)
			return tea.Batch(
				SendError(fmt.Errorf("%w: %v", err, errmsg)),
				PreviewClose,
			)()
		}
		return PreviewCloseMsg{}
	})
}

type NoPreview struct{ path string }

func (np *NoPreview) View() string                        { return np.path }
func (np *NoPreview) Open(e *TreeEntry)                   { np.path = e.Path }
func (np *NoPreview) Close()                              { np.path = "" }
func (np *NoPreview) IsOpen() bool                        { return len(np.path) > 0 }
func (np *NoPreview) Update(tea.Msg) (tea.Model, tea.Cmd) { return &NoOpInitModel{np}, nil }
