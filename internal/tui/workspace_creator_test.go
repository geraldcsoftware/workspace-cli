package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/geraldc/workspace-cli/internal/config"
)

func TestCreatorModel_SuccessTransition(t *testing.T) {
	m := creatorModel{
		cfg: config.Config{WorkspaceBaseDir: "/tmp"},
		step: stepConfirm,
	}

	// Simulate workspace created message
	msg := workspaceCreatedMsg{path: "/tmp/my-ws"}
	newModel, cmd := m.Update(msg)
	m = newModel.(creatorModel)

	if m.step != stepSuccess {
		t.Errorf("expected stepSuccess, got %v", m.step)
	}
	if m.createdPath != "/tmp/my-ws" {
		t.Errorf("expected path /tmp/my-ws, got %s", m.createdPath)
	}
	if cmd != nil {
		t.Error("expected nil cmd after transition to success")
	}

	// Test Enter (Go to workspace)
	newModel, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(creatorModel)
	if !m.quitting {
		t.Error("expected quitting after Enter")
	}
	if m.createdPath != "/tmp/my-ws" {
		t.Error("expected path to be preserved for CD")
	}

	// Test Q (Done - No CD)
	m.quitting = false
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	m = newModel.(creatorModel)
	if !m.quitting {
		t.Error("expected quitting after Q")
	}
	if m.createdPath != "" {
		t.Errorf("expected empty path after Q, got %s", m.createdPath)
	}
}
