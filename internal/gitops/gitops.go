package gitops

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type BranchInfo struct {
	Name       string
	IsRemote   bool
	IsLocal    bool
	Upstream   string
	Ahead      int
	Behind     int
	LastCommit time.Time
}

func DetectBaseBranch(repoPath string) string {
	for _, branch := range []string{"main", "master", "develop"} {
		if gitOK(repoPath, "rev-parse", "--verify", branch) {
			return branch
		}
		if gitOK(repoPath, "rev-parse", "--verify", "origin/"+branch) {
			return branch
		}
	}
	out, err := gitOutput(repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "master"
	}
	return strings.TrimSpace(out)
}

func AddWorktree(repoPath, wtPath, branch string, isNew bool, base string) error {
	if isNew {
		if base != "" {
			return runGit(repoPath, "worktree", "add", "-b", branch, wtPath, base)
		}
		return runGit(repoPath, "worktree", "add", "-b", branch, wtPath)
	}

	// If the branch exists locally, use it
	if gitOK(repoPath, "rev-parse", "--verify", branch) {
		return runGit(repoPath, "worktree", "add", wtPath, branch)
	}

	// If it's a remote branch (e.g. from origin), track it
	remoteBranch := branch
	if !strings.HasPrefix(remoteBranch, "origin/") {
		remoteBranch = "origin/" + branch
	}

	if gitOK(repoPath, "rev-parse", "--verify", remoteBranch) {
		localName := strings.TrimPrefix(remoteBranch, "origin/")
		return runGit(repoPath, "worktree", "add", "--track", "-b", localName, wtPath, remoteBranch)
	}

	// Fallback: create a new branch from base (requested for legacy Add/Create)
	if base == "" {
		base = DetectBaseBranch(repoPath)
	}
	return runGit(repoPath, "worktree", "add", "-b", branch, wtPath, base)
}

func AddWorktreeDetach(repoPath, wtPath, base string) error {
	return runGit(repoPath, "worktree", "add", "--detach", wtPath, base)
}

func RemoveWorktree(wtPath string) error {
	mainRepo, err := MainRepoFromWorktree(wtPath)
	if err != nil {
		return os.RemoveAll(wtPath)
	}
	if err := runGit(mainRepo, "worktree", "remove", "--force", wtPath); err != nil {
		return os.RemoveAll(wtPath)
	}
	return nil
}

func MainRepoFromWorktree(wtPath string) (string, error) {
	// The requested command for reliability
	out, err := gitOutput(wtPath, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	commonDir := strings.TrimSpace(out)
	absCommonDir, err := filepath.Abs(filepath.Join(wtPath, commonDir))
	if err != nil {
		return "", err
	}
	// The parent of the .git directory (the common dir) is the repo root
	return filepath.Dir(absCommonDir), nil
}

func Fetch(path string) error {
	return runGit(path, "fetch", "--quiet", "--prune", "origin")
}

func GetBranches(path string) ([]BranchInfo, error) {
	// format: refname:short|upstream:short|committerdate:unix
	format := "%(refname:short)|%(upstream:short)|%(committerdate:unix)"
	out, err := gitOutput(path, "for-each-ref", "--sort=-committerdate", "--format="+format, "refs/heads", "refs/remotes/origin")
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	branchMap := make(map[string]*BranchInfo)

	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 3 {
			continue
		}
		refName := parts[0]
		upstream := parts[1]
		unixTime, _ := strconv.ParseInt(parts[2], 10, 64)
		commitTime := time.Unix(unixTime, 0)

		isRemote := strings.HasPrefix(refName, "origin/")
		name := strings.TrimPrefix(refName, "origin/")

		info, ok := branchMap[name]
		if !ok {
			info = &BranchInfo{Name: name, LastCommit: commitTime}
			branchMap[name] = info
		}

		if isRemote {
			info.IsRemote = true
		} else {
			info.IsLocal = true
			info.Upstream = upstream
			if info.LastCommit.Before(commitTime) {
				info.LastCommit = commitTime
			}
		}
	}

	var branches []BranchInfo
	for _, info := range branchMap {
		if info.IsLocal && info.Upstream != "" {
			// Calculate ahead/behind
			ahead, _ := gitCount(path, info.Upstream+".."+info.Name)
			behind, _ := gitCount(path, info.Name+".."+info.Upstream)
			info.Ahead = ahead
			info.Behind = behind
		}
		branches = append(branches, *info)
	}

	// Sort by date again because map iteration is random
	sort.Slice(branches, func(i, j int) bool {
		return branches[i].LastCommit.After(branches[j].LastCommit)
	})

	return branches, nil
}

func gitCount(path, revRange string) (int, error) {
	out, err := gitOutput(path, "rev-list", "--count", revRange)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(out))
}

func StatusPorcelain(path string) (string, error) {
	return gitOutput(path, "status", "--porcelain")
}

func CurrentBranch(path string) string {
	out, err := gitOutput(path, "branch", "--show-current")
	if err != nil {
		return "detached"
	}
	trimmed := strings.TrimSpace(out)
	if trimmed == "" {
		return "detached"
	}
	return trimmed
}

func runGit(path string, args ...string) error {
	cmd := exec.Command("git", append([]string{"-C", path}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s failed: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func gitOutput(path string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", path}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func gitOK(path string, args ...string) bool {
	cmd := exec.Command("git", append([]string{"-C", path}, args...)...)
	return cmd.Run() == nil
}
