package tui

import (
	"errors"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/geraldc/workspace-cli/internal/workspace"
)

// PickerResult is returned by RunWorkspacePicker.
type PickerResult struct {
	Selected  workspace.Info
	CreateNew bool
}

type pickerModel struct {
	items     []workspace.Info
	cursor    int
	selected  workspace.Info
	createNew bool
	quitting  bool
	err       error
}

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	helpStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	rowStyle   = lipgloss.NewStyle().PaddingLeft(2)
	curStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
)

func RunWorkspacePicker(items []workspace.Info) (PickerResult, error) {
	m := pickerModel{items: items}
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return PickerResult{}, err
	}
	fm := finalModel.(pickerModel)
	if fm.err != nil {
		return PickerResult{}, fm.err
	}
	if fm.createNew {
		return PickerResult{CreateNew: true}, nil
	}
	if fm.selected.Name == "" {
		return PickerResult{}, errors.New("no workspace selected")
	}
	return PickerResult{Selected: fm.selected}, nil
}

func (m pickerModel) Init() tea.Cmd {
	return nil
}

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			m.err = errors.New("aborted")
			return m, tea.Quit
		case "n":
			m.createNew = true
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.items) > 0 {
				m.selected = m.items[m.cursor]
				m.quitting = true
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m pickerModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("Select workspace"))
	b.WriteString("\n\n")
	if len(m.items) == 0 {
		b.WriteString(helpStyle.Render("No workspaces yet."))
		b.WriteString("\n\n")
	}
	for i, item := range m.items {
		cursor := " "
		style := rowStyle
		if i == m.cursor {
			cursor = ">"
			style = curStyle
		}
		line := fmt.Sprintf("%s %s (%d repos)", cursor, item.Name, item.RepoCount)
		b.WriteString(style.Render(line))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/↓ or j/k: move • enter: select • n: new workspace • q: quit"))
	return b.String()
}
