package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type PopupMsg string

func Popup(msg string) tea.Cmd {
	return func() tea.Msg { return PopupMsg(msg) }
}

type ClearPopupMsg struct{}

func ClearPopup() tea.Msg {
	return ClearPopupMsg{}
}

func ClearPopupIn(duration time.Duration) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(duration)
		return ClearPopupMsg{}
	}
}

type ErrorMsg struct {
	Error error
}

func SendError(err error) tea.Cmd {
	return func() tea.Msg {
		return ErrorMsg{Error: err}
	}
}

func SendErrorf(format string, args ...any) tea.Cmd {
	return func() tea.Msg {
		return ErrorMsg{Error: fmt.Errorf(format, args...)}
	}
}

type ClearErrorMsg struct {
	Lifetime time.Duration
}

func ClearErrorAfter(duration time.Duration) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(duration)
		return ClearErrorMsg{Lifetime: duration}
	}
}
