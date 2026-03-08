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
	"github.com/geraldc/workspace-cli/internal/gitops"
	"github.com/geraldc/workspace-cli/internal/workspace"
)

const (
	stepName int = iota
	stepRepoSearch
   // enter a zoxide search term
	stepRepoPick     // pick from multiple zoxide results
	stepAddAnother   // "add another repo?"
	stepBranchPick   // select branch for a repo
	stepNewBranch    // enter new branch name
	stepConfirm      // review and confirm
	stepCreating     // creation in progress
)

type creatorModel struct {
	cfg config.Config

	step    int
	prevErr string // inline error shown on current step

	nameInput string

	repoInput      string
	repoCandidates []string
	repoCursor     int

	selectedRepos []string // absolute paths confirmed by the user
	repoConfigs   []workspace.RepoConfig

	currentRepoIdx int
	branches       []gitops.BranchInfo
	branchCursor   int
	branchOffset   int
	newBranchName  string

	createdPath string // set on success
	success     successModel
	quitting    bool
	err         error
}

// messages from async commands
type zoxideResultMsg struct {
	paths []string
	err   error
}
type branchesFetchedMsg struct {
	branches []gitops.BranchInfo
	err      error
}
type workspaceCreatedMsg struct{ path string }
type workspaceErrMsg struct{ err error }

// RunCreateWorkspace launches the interactive workspace-creation wizard.
// On success it returns the path of the created workspace (to CD); on cancel or if no CD requested it returns "".
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
	case branchesFetchedMsg:
		if msg.err != nil {
			m.prevErr = msg.err.Error()
			return m, nil
		}
		m.branches = msg.branches
		m.branchCursor = 0
		m.branchOffset = 0
		return m, nil
	case workspaceCreatedMsg:
		m.createdPath = msg.path
		m.success = successModel{
			path: msg.path,
			name: strings.TrimSpace(m.nameInput),
		}
		m.step = stepSuccess
		return m, nil
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
		case stepBranchPick:
			return m.updateBranchPick(msg)
		case stepNewBranch:
			return m.updateNewBranch(msg)
		case stepConfirm:
			return m.updateConfirm(msg)
		case stepCreating:
			if msg.String() == "ctrl+c" {
				m.err = errors.New("aborted")
				m.quitting = true
				return m, tea.Quit
			}
		case stepSuccess:
			return m.updateSuccess(msg)
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
				m.currentRepoIdx = 0
				m.step = stepBranchPick
				return m, fetchBranchesCmd(m.selectedRepos[0])
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
		if len(m.selectedRepos) == 0 {
			m.step = stepConfirm
			return m, nil
		}
		m.currentRepoIdx = 0
		m.step = stepBranchPick
		return m, fetchBranchesCmd(m.selectedRepos[0])
	}
	return m, nil
}

func (m creatorModel) updateBranchPick(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.err = errors.New("aborted")
		m.quitting = true
		return m, tea.Quit
	case "up", "k":
		if m.branchCursor > 0 {
			m.branchCursor--
			if m.branchCursor < m.branchOffset {
				m.branchOffset = m.branchCursor
			}
		}
	case "down", "j":
		if m.branchCursor < len(m.branches)-1 {
			m.branchCursor++
			if m.branchCursor >= m.branchOffset+10 {
				m.branchOffset = m.branchCursor - 9
			}
		}
	case "left", "h":
		if m.branchOffset >= 10 {
			m.branchOffset -= 10
			m.branchCursor = m.branchOffset
		}
	case "right", "l":
		if m.branchOffset+10 < len(m.branches) {
			m.branchOffset += 10
			m.branchCursor = m.branchOffset
		}
	case "enter":
		if len(m.branches) == 0 {
			return m, nil
		}
		branch := m.branches[m.branchCursor]
		m.repoConfigs = append(m.repoConfigs, workspace.RepoConfig{
			RepoPath: m.selectedRepos[m.currentRepoIdx],
			Branch:   branch.Name,
		})
		return m.nextRepoOrConfirm()
	case "n", "N":
		m.step = stepNewBranch
		m.newBranchName = ""
		m.prevErr = ""
	case "esc":
		if m.currentRepoIdx > 0 {
			m.currentRepoIdx--
			m.repoConfigs = m.repoConfigs[:len(m.repoConfigs)-1]
			return m, fetchBranchesCmd(m.selectedRepos[m.currentRepoIdx])
		}
		m.step = stepAddAnother
	}
	return m, nil
}

func (m creatorModel) updateNewBranch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.err = errors.New("aborted")
		m.quitting = true
		return m, tea.Quit
	case "enter":
		name := strings.TrimSpace(m.newBranchName)
		if name == "" {
			m.prevErr = "branch name cannot be empty"
			return m, nil
		}
		baseBranch := ""
		if len(m.branches) > 0 {
			baseBranch = m.branches[m.branchCursor].Name
		}
		m.repoConfigs = append(m.repoConfigs, workspace.RepoConfig{
			RepoPath:    m.selectedRepos[m.currentRepoIdx],
			Branch:      name,
			IsNewBranch: true,
			BaseBranch:  baseBranch,
		})
		return m.nextRepoOrConfirm()
	case "esc":
		m.step = stepBranchPick
		m.prevErr = ""
	case "backspace":
		if len(m.newBranchName) > 0 {
			m.newBranchName = m.newBranchName[:len(m.newBranchName)-1]
		}
		m.prevErr = ""
	default:
		if isTypeable(msg) {
			m.newBranchName += msg.String()
			m.prevErr = ""
		}
	}
	return m, nil
}

func (m creatorModel) nextRepoOrConfirm() (tea.Model, tea.Cmd) {
	m.currentRepoIdx++
	if m.currentRepoIdx < len(m.selectedRepos) {
		m.step = stepBranchPick
		m.prevErr = ""
		return m, fetchBranchesCmd(m.selectedRepos[m.currentRepoIdx])
	}
	m.step = stepConfirm
	m.prevErr = ""
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
		if len(m.selectedRepos) > 0 {
			m.currentRepoIdx = len(m.selectedRepos) - 1
			m.repoConfigs = m.repoConfigs[:len(m.repoConfigs)-1]
			m.step = stepBranchPick
			return m, fetchBranchesCmd(m.selectedRepos[m.currentRepoIdx])
		}
		m.step = stepAddAnother
	case "enter":
		m.prevErr = ""
		m.step = stepCreating
		return m, m.createCmd()
	}
	return m, nil
}

func (m creatorModel) updateSuccess(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.success, m.createdPath, cmd = m.success.updateSuccess(msg)
	return m, cmd
}

// createCmd fires off workspace creation as a bubbletea command so the TUI
// remains responsive and can display the result (or error) inline.
func (m creatorModel) createCmd() tea.Cmd {
	cfg := m.cfg
	name := strings.TrimSpace(m.nameInput)
	configs := append([]workspace.RepoConfig(nil), m.repoConfigs...)
	return func() tea.Msg {
		_, err := workspace.CreateFromConfig(cfg, name, configs)
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

	case stepBranchPick:
		writeCreatorHeader(&b, m.nameInput, m.selectedRepos[:m.currentRepoIdx])
		repoPath := m.selectedRepos[m.currentRepoIdx]
		b.WriteString(fmt.Sprintf("Select branch for %s:\n\n", curStyle.Render(filepath.Base(repoPath))))

		if len(m.branches) == 0 {
			b.WriteString("  Fetching branches...\n")
		} else {
			start := m.branchOffset
			end := start + 10
			if end > len(m.branches) {
				end = len(m.branches)
			}

			for i := start; i < end; i++ {
				branch := m.branches[i]
				prefix := "  "
				if i == m.branchCursor {
					prefix = "▶ "
				}

				icon := " "
				if branch.IsLocal && branch.IsRemote {
					icon = "󰓦 "
				} else if branch.IsRemote {
					icon = "󰓅 "
				}

				line := fmt.Sprintf("%s%s%s", prefix, icon, branch.Name)
				if branch.Ahead > 0 || branch.Behind > 0 {
					line += fmt.Sprintf(" (↑%d ↓%d)", branch.Ahead, branch.Behind)
				}

				if i == m.branchCursor {
					b.WriteString(curStyle.Render(line))
				} else {
					b.WriteString(rowStyle.Render(line))
				}
				b.WriteString("\n")
			}
		}
		writeInlineErr(&b, m.prevErr)
		b.WriteString("\n")
		b.WriteString(helpStyle.Render(fmt.Sprintf("Page %d/%d • ↑/↓: move • ←/→: pages • enter: select • n: new branch • esc: back • ctrl+c: cancel", (m.branchOffset/10)+1, (len(m.branches)-1)/10+1)))

	case stepNewBranch:
		writeCreatorHeader(&b, m.nameInput, m.selectedRepos[:m.currentRepoIdx])
		repoPath := m.selectedRepos[m.currentRepoIdx]
		baseBranch := ""
		if len(m.branches) > 0 {
			baseBranch = m.branches[m.branchCursor].Name
		}
		b.WriteString(fmt.Sprintf("New branch name for %s (from %s): ", curStyle.Render(filepath.Base(repoPath)), baseBranch))
		b.WriteString(curStyle.Render(m.newBranchName))
		b.WriteString("█\n")
		writeInlineErr(&b, m.prevErr)
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("enter: confirm • esc: back • ctrl+c: cancel"))

	case stepConfirm:
		b.WriteString("Workspace: ")
		b.WriteString(curStyle.Render(m.nameInput))
		b.WriteString("\n\n")
		if len(m.repoConfigs) == 0 {
			b.WriteString(helpStyle.Render("(no repos — workspace will be created empty)"))
		} else {
			b.WriteString("Repos & Branches:\n")
			for _, rc := range m.repoConfigs {
				branchStr := rc.Branch
				if rc.IsNewBranch {
					branchStr += fmt.Sprintf(" (new from %s)", rc.BaseBranch)
				}
				b.WriteString(rowStyle.Render(fmt.Sprintf("  • %-20s @ %s", filepath.Base(rc.RepoPath), branchStr)))
				b.WriteString("\n")
			}
		}
		writeInlineErr(&b, m.prevErr)
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("enter: create • esc/b: back • ctrl+c: cancel"))

	case stepCreating:
		b.WriteString(helpStyle.Render(
			fmt.Sprintf("Creating workspace %q…", strings.TrimSpace(m.nameInput))))

	case stepSuccess:
		b.WriteString(m.success.viewSuccess())
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

func fetchBranchesCmd(path string) tea.Cmd {
	return func() tea.Msg {
		absPath, err := gitops.MainRepoFromWorktree(path)
		if err != nil {
			// If not a worktree, use as is
			absPath = path
		}
		_ = gitops.Fetch(absPath)
		branches, err := gitops.GetBranches(absPath)
		return branchesFetchedMsg{branches: branches, err: err}
	}
}
