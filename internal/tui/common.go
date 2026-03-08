package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	stepSuccess = 99 // Common success step
)

type successModel struct {
	path   string
	name   string
	copied bool
}

func (m successModel) updateSuccess(msg tea.Msg) (successModel, string, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, "", nil
	}

	switch keyMsg.String() {
	case "enter":
		// Return path to let main.go print __SPACE_CD__
		return m, m.path, tea.Quit
	case "t", "T":
		if os.Getenv("TMUX") != "" {
			wsName := m.name
			// tmux new-session -d -s <name> -c <path> && tmux switch-client -t <name>
			cmd := exec.Command("tmux", "new-session", "-d", "-s", wsName, "-c", m.path)
			if err := cmd.Run(); err == nil {
				_ = exec.Command("tmux", "switch-client", "-t", wsName).Run()
				return m, "", tea.Quit
			}
		}
	case "c", "C":
		if err := copyToClipboard(m.path); err == nil {
			m.copied = true
		}
	case "e", "E":
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vim"
		}
		// Clear screen before launching editor
		fmt.Print("\033[H\033[2J")
		cmd := exec.Command(editor, m.path)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
		return m, "", tea.Quit
	case "q", "esc":
		return m, "", tea.Quit
	}
	return m, "", nil
}

func (m successModel) viewSuccess() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Workspace: %s\n", curStyle.Render(m.name)))
	b.WriteString(fmt.Sprintf("Path: %s\n\n", helpStyle.Render(m.path)))
	b.WriteString("What next?\n\n")
	b.WriteString(curStyle.Render("  [Enter] "))
	b.WriteString("Go to workspace (current shell)\n")
	if os.Getenv("TMUX") != "" {
		b.WriteString(curStyle.Render("  [T]     "))
		b.WriteString("Open in new Tmux session\n")
	}
	b.WriteString(curStyle.Render("  [C]     "))
	if m.copied {
		b.WriteString(helpStyle.Render("Path copied to clipboard!"))
	} else {
		b.WriteString("Copy path to clipboard")
	}
	b.WriteString("\n")
	b.WriteString(curStyle.Render("  [E]     "))
	b.WriteString("Open in $EDITOR\n")
	b.WriteString(curStyle.Render("  [Q/Esc] "))
	b.WriteString("Done (stay here)\n")
	return b.String()
}
