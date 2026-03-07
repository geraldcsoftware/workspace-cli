package tui

import (
"errors"
"fmt"
"os"
"os/exec"
"path/filepath"
"strings"

tea "github.com/charmbracelet/bubbletea"
"github.com/geraldc/workspace-cli/internal/config"
"github.com/geraldc/workspace-cli/internal/workspace"
)

type creatorStep int

const (
stepName       creatorStep = iota
stepRepoSearch             // enter a zoxide search term
stepRepoPick               // pick from multiple zoxide results
stepAddAnother             // "add another repo?"
stepConfirm                // review and confirm
stepCreating               // creation in progress
)

type creatorModel struct {
cfg config.Config

step    creatorStep
prevErr string // inline error shown on current step

nameInput string

repoInput      string
repoCandidates []string
repoCursor     int

selectedRepos []string // absolute paths confirmed by the user

createdPath string // set on success
quitting    bool
err         error
}

// messages from async commands
type zoxideResultMsg struct {
paths []string
err   error
}
type workspaceCreatedMsg struct{ path string }
type workspaceErrMsg struct{ err error }

// RunCreateWorkspace launches the interactive workspace-creation wizard.
// On success it returns the path of the created workspace; on cancel it returns "".
func RunCreateWorkspace(cfg config.Config) (string, error) {
m := creatorModel{cfg: cfg}
p := tea.NewProgram(m)
finalModel, runErr := p.Run()
if runErr != nil {
return "", runErr
}
fm := finalModel.(creatorModel)
if fm.err != nil {
return "", fm.err
}
return fm.createdPath, nil
}

func (m creatorModel) Init() tea.Cmd { return nil }

func (m creatorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
switch msg := msg.(type) {
case zoxideResultMsg:
return m.handleZoxideResult(msg)
case workspaceCreatedMsg:
m.createdPath = msg.path
m.quitting = true
return m, tea.Quit
case workspaceErrMsg:
// Stay on confirm step; show the error so the user can fix it.
m.prevErr = msg.err.Error()
m.step = stepConfirm
return m, nil
case tea.KeyMsg:
switch m.step {
case stepName:
return m.updateName(msg)
case stepRepoSearch:
return m.updateRepoSearch(msg)
case stepRepoPick:
return m.updateRepoPick(msg)
case stepAddAnother:
return m.updateAddAnother(msg)
case stepConfirm:
return m.updateConfirm(msg)
case stepCreating:
if msg.String() == "ctrl+c" {
m.err = errors.New("aborted")
m.quitting = true
return m, tea.Quit
}
}
}
return m, nil
}

// handleZoxideResult processes the async zoxide lookup response.
func (m creatorModel) handleZoxideResult(msg zoxideResultMsg) (tea.Model, tea.Cmd) {
if msg.err != nil {
m.prevErr = msg.err.Error()
m.step = stepRepoSearch
return m, nil
}
if len(msg.paths) == 1 {
m.appendRepo(msg.paths[0])
m.repoInput = ""
m.prevErr = ""
m.step = stepAddAnother
return m, nil
}
m.repoCandidates = msg.paths
m.repoCursor = 0
m.prevErr = ""
m.step = stepRepoPick
return m, nil
}

func (m *creatorModel) appendRepo(path string) {
for _, r := range m.selectedRepos {
if r == path {
return
}
}
m.selectedRepos = append(m.selectedRepos, path)
}

// ── step handlers ────────────────────────────────────────────────────────────

func (m creatorModel) updateName(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
switch msg.String() {
case "ctrl+c":
m.err = errors.New("aborted")
m.quitting = true
return m, tea.Quit
case "enter":
name := strings.TrimSpace(m.nameInput)
if name == "" {
m.prevErr = "workspace name cannot be empty"
return m, nil
}
if strings.ContainsAny(name, "/\\:") {
m.prevErr = `workspace name cannot contain / \ :`
return m, nil
}
wsDir := filepath.Join(m.cfg.WorkspaceBaseDir, name)
if isDir(wsDir) {
m.prevErr = fmt.Sprintf("workspace %q already exists", name)
return m, nil
}
m.prevErr = ""
m.step = stepRepoSearch
m.repoInput = ""
case "backspace":
if len(m.nameInput) > 0 {
m.nameInput = m.nameInput[:len(m.nameInput)-1]
}
m.prevErr = ""
default:
if isTypeable(msg) {
m.nameInput += msg.String()
m.prevErr = ""
}
}
return m, nil
}

func (m creatorModel) updateRepoSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
switch msg.String() {
case "ctrl+c":
m.err = errors.New("aborted")
m.quitting = true
return m, tea.Quit
case "esc":
m.prevErr = ""
if len(m.selectedRepos) == 0 {
m.step = stepName
} else {
m.step = stepAddAnother
}
case "enter":
term := strings.TrimSpace(m.repoInput)
if term == "" {
if len(m.selectedRepos) > 0 {
m.prevErr = ""
m.step = stepConfirm
} else {
m.prevErr = "enter a search term (or esc to go back)"
}
return m, nil
}
m.prevErr = ""
return m, zoxideQuery(term)
case "backspace":
if len(m.repoInput) > 0 {
m.repoInput = m.repoInput[:len(m.repoInput)-1]
}
m.prevErr = ""
default:
if isTypeable(msg) {
m.repoInput += msg.String()
m.prevErr = ""
}
}
return m, nil
}

func (m creatorModel) updateRepoPick(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
switch msg.String() {
case "ctrl+c":
m.err = errors.New("aborted")
m.quitting = true
return m, tea.Quit
case "esc":
m.repoCandidates = nil
m.repoInput = ""
m.prevErr = ""
m.step = stepRepoSearch
case "up", "k":
if m.repoCursor > 0 {
m.repoCursor--
}
case "down", "j":
if m.repoCursor < len(m.repoCandidates)-1 {
m.repoCursor++
}
case "enter":
m.appendRepo(m.repoCandidates[m.repoCursor])
m.repoCandidates = nil
m.repoInput = ""
m.prevErr = ""
m.step = stepAddAnother
}
return m, nil
}

func (m creatorModel) updateAddAnother(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
switch msg.String() {
case "ctrl+c":
m.err = errors.New("aborted")
m.quitting = true
return m, tea.Quit
case "y", "Y":
m.step = stepRepoSearch
m.repoInput = ""
m.prevErr = ""
case "n", "N", "enter":
m.step = stepConfirm
m.prevErr = ""
}
return m, nil
}

func (m creatorModel) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
switch msg.String() {
case "ctrl+c":
m.err = errors.New("aborted")
m.quitting = true
return m, tea.Quit
case "esc", "b":
m.prevErr = ""
m.step = stepAddAnother
case "enter":
m.prevErr = ""
m.step = stepCreating
return m, m.createCmd()
}
return m, nil
}

// createCmd fires off workspace creation as a bubbletea command so the TUI
// remains responsive and can display the result (or error) inline.
func (m creatorModel) createCmd() tea.Cmd {
cfg := m.cfg
name := strings.TrimSpace(m.nameInput)
repos := append([]string(nil), m.selectedRepos...)
return func() tea.Msg {
_, err := workspace.CreateFromPaths(cfg, name, repos, "branch")
if err != nil {
return workspaceErrMsg{err: err}
}
return workspaceCreatedMsg{path: filepath.Join(cfg.WorkspaceBaseDir, name)}
}
}

// ── views ────────────────────────────────────────────────────────────────────

func (m creatorModel) View() string {
if m.quitting {
return ""
}
var b strings.Builder
b.WriteString(titleStyle.Render("New workspace"))
b.WriteString("\n\n")

switch m.step {
case stepName:
b.WriteString("Workspace name: ")
b.WriteString(curStyle.Render(m.nameInput))
b.WriteString("█\n")
writeInlineErr(&b, m.prevErr)
b.WriteString("\n")
b.WriteString(helpStyle.Render("enter: next • ctrl+c: cancel"))

case stepRepoSearch:
writeCreatorHeader(&b, m.nameInput, m.selectedRepos)
b.WriteString("Search repo: ")
b.WriteString(curStyle.Render(m.repoInput))
b.WriteString("█\n")
writeInlineErr(&b, m.prevErr)
b.WriteString("\n")
hint := "enter: search"
if len(m.selectedRepos) > 0 {
hint += " • enter (blank): done"
}
b.WriteString(helpStyle.Render(hint + " • esc: back • ctrl+c: cancel"))

case stepRepoPick:
writeCreatorHeader(&b, m.nameInput, m.selectedRepos)
b.WriteString("Pick a repo:\n\n")
for i, p := range m.repoCandidates {
if i == m.repoCursor {
b.WriteString(curStyle.Render("▶ " + p))
} else {
b.WriteString(rowStyle.Render("  " + p))
}
b.WriteString("\n")
}
b.WriteString("\n")
b.WriteString(helpStyle.Render("↑/↓ or j/k: move • enter: select • esc: search again • ctrl+c: cancel"))

case stepAddAnother:
writeCreatorHeader(&b, m.nameInput, m.selectedRepos)
b.WriteString(helpStyle.Render("Add another repo? [y/N]"))

case stepConfirm:
b.WriteString("Workspace: ")
b.WriteString(curStyle.Render(m.nameInput))
b.WriteString("\n\n")
if len(m.selectedRepos) == 0 {
b.WriteString(helpStyle.Render("(no repos — workspace will be created empty)"))
} else {
b.WriteString("Repos:\n")
for _, r := range m.selectedRepos {
b.WriteString(rowStyle.Render("  • " + r))
b.WriteString("\n")
}
}
writeInlineErr(&b, m.prevErr)
b.WriteString("\n")
b.WriteString(helpStyle.Render("enter: create • b/esc: back • ctrl+c: cancel"))

case stepCreating:
b.WriteString(helpStyle.Render(
fmt.Sprintf("Creating workspace %q…", strings.TrimSpace(m.nameInput))))
}

return b.String()
}

// ── helpers ──────────────────────────────────────────────────────────────────

func writeCreatorHeader(b *strings.Builder, name string, repos []string) {
b.WriteString("Workspace: ")
b.WriteString(curStyle.Render(name))
b.WriteString("\n")
for _, r := range repos {
b.WriteString(rowStyle.Render("✓ " + filepath.Base(r)))
b.WriteString("\n")
}
b.WriteString("\n")
}

func writeInlineErr(b *strings.Builder, msg string) {
if msg != "" {
b.WriteString("\n")
b.WriteString(errStyle.Render("✗ " + msg))
b.WriteString("\n")
}
}

func isTypeable(msg tea.KeyMsg) bool {
return len(msg.String()) == 1 || msg.String() == " "
}

func isDir(path string) bool {
st, err := os.Stat(path)
return err == nil && st.IsDir()
}

// zoxideQuery calls `zoxide query --list <term>` as a bubbletea command.
func zoxideQuery(term string) tea.Cmd {
return func() tea.Msg {
if _, err := exec.LookPath("zoxide"); err != nil {
return zoxideResultMsg{err: errors.New("zoxide not found — install with: brew install zoxide")}
}
out, err := exec.Command("zoxide", "query", "--list", term).Output()
if err != nil || strings.TrimSpace(string(out)) == "" {
return zoxideResultMsg{err: fmt.Errorf("no results found for %q", term)}
}
var paths []string
for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
if line = strings.TrimSpace(line); line != "" {
paths = append(paths, line)
}
}
if len(paths) == 0 {
return zoxideResultMsg{err: fmt.Errorf("no results found for %q", term)}
}
return zoxideResultMsg{paths: paths}
}
}
