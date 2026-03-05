package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geraldc/workspace-cli/internal/config"
	"github.com/geraldc/workspace-cli/internal/discovery"
	"github.com/geraldc/workspace-cli/internal/gitops"
)

type Info struct {
	Name      string
	Path      string
	RepoCount int
}

type RepoStatus struct {
	Name      string
	Branch    string
	Modified  int
	Staged    int
	Untracked int
}

type StatusInfo struct {
	Name  string
	Path  string
	Repos []RepoStatus
}

func Create(cfg config.Config, wsName string, repoQueries []string, strategy string) ([]string, error) {
	wsDir := filepath.Join(cfg.WorkspaceBaseDir, wsName)
	if _, err := os.Stat(wsDir); err == nil {
		return nil, fmt.Errorf("workspace already exists: %s", wsDir)
	}
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		return nil, err
	}
	return addResolved(cfg, wsName, wsDir, repoQueries, strategy)
}

func Add(cfg config.Config, wsName string, repoQueries []string, strategy string) ([]string, error) {
	wsDir := filepath.Join(cfg.WorkspaceBaseDir, wsName)
	if st, err := os.Stat(wsDir); err != nil || !st.IsDir() {
		return nil, fmt.Errorf("workspace not found: %s", wsName)
	}
	return addResolved(cfg, wsName, wsDir, repoQueries, strategy)
}

func addResolved(cfg config.Config, wsName, wsDir string, repoQueries []string, strategy string) ([]string, error) {
	repos, err := discovery.FindRepos(cfg, false)
	if err != nil {
		return nil, err
	}
	resolved := make([]string, 0, len(repoQueries))
	for _, query := range repoQueries {
		repoPath, err := discovery.MatchQuery(repos, query)
		if err != nil {
			return nil, err
		}
		repoName := filepath.Base(repoPath)
		if _, err := os.Stat(filepath.Join(wsDir, repoName)); err == nil {
			continue
		}
		resolved = append(resolved, repoPath)
	}

	added := make([]string, 0, len(resolved))
	for _, repoPath := range resolved {
		repoName := filepath.Base(repoPath)
		wtPath := filepath.Join(wsDir, repoName)
		if err := gitops.AddWorktree(repoPath, wtPath, wsName, strategy); err != nil {
			return added, err
		}
		added = append(added, wtPath)
	}
	return added, nil
}

func List(cfg config.Config) ([]Info, error) {
	entries, err := os.ReadDir(cfg.WorkspaceBaseDir)
	if err != nil && os.IsNotExist(err) {
		return []Info{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]Info, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		p := filepath.Join(cfg.WorkspaceBaseDir, name)
		repoCount := countChildrenDirs(p)
		out = append(out, Info{Name: name, Path: p, RepoCount: repoCount})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func Status(cfg config.Config, wsName string) (StatusInfo, error) {
	wsDir := filepath.Join(cfg.WorkspaceBaseDir, wsName)
	if st, err := os.Stat(wsDir); err != nil || !st.IsDir() {
		return StatusInfo{}, fmt.Errorf("workspace not found: %s", wsName)
	}
	entries, err := os.ReadDir(wsDir)
	if err != nil {
		return StatusInfo{}, err
	}

	out := StatusInfo{Name: wsName, Path: wsDir}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		repoPath := filepath.Join(wsDir, entry.Name())
		porcelain, _ := gitops.StatusPorcelain(repoPath)
		modified, staged, untracked := parsePorcelain(porcelain)
		out.Repos = append(out.Repos, RepoStatus{
			Name:      entry.Name(),
			Branch:    gitops.CurrentBranch(repoPath),
			Modified:  modified,
			Staged:    staged,
			Untracked: untracked,
		})
	}
	sort.Slice(out.Repos, func(i, j int) bool { return out.Repos[i].Name < out.Repos[j].Name })
	return out, nil
}

func Remove(cfg config.Config, wsName string, force bool) error {
	wsDir := filepath.Join(cfg.WorkspaceBaseDir, wsName)
	if st, err := os.Stat(wsDir); err != nil || !st.IsDir() {
		return fmt.Errorf("workspace not found: %s", wsName)
	}
	entries, err := os.ReadDir(wsDir)
	if err != nil {
		return err
	}
	if !force {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			porcelain, _ := gitops.StatusPorcelain(filepath.Join(wsDir, entry.Name()))
			if strings.TrimSpace(porcelain) != "" {
				return fmt.Errorf("workspace has uncommitted changes (use --force): %s", wsName)
			}
		}
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		_ = gitops.RemoveWorktree(filepath.Join(wsDir, entry.Name()))
	}
	return os.RemoveAll(wsDir)
}

func parsePorcelain(text string) (modified, staged, untracked int) {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	for _, line := range lines {
		if len(line) < 2 {
			continue
		}
		if strings.HasPrefix(line, "??") {
			untracked++
			continue
		}
		if line[0] != ' ' {
			staged++
		}
		if line[1] != ' ' {
			modified++
		}
	}
	return
}

func countChildrenDirs(path string) int {
	entries, err := os.ReadDir(path)
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			count++
		}
	}
	return count
}
