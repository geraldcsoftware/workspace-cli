package gitops

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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

func AddWorktree(repoPath, wtPath, workspaceName, strategy string) error {
	base := DetectBaseBranch(repoPath)
	_ = runGit(repoPath, "fetch", "--quiet", "origin")

	if strategy == "detach" {
		return runGit(repoPath, "worktree", "add", "--detach", wtPath, base)
	}

	if gitOK(repoPath, "rev-parse", "--verify", workspaceName) {
		return runGit(repoPath, "worktree", "add", wtPath, workspaceName)
	}
	if gitOK(repoPath, "rev-parse", "--verify", "origin/"+workspaceName) {
		return runGit(repoPath, "worktree", "add", "--track", "-b", workspaceName, wtPath, "origin/"+workspaceName)
	}
	return runGit(repoPath, "worktree", "add", "-b", workspaceName, wtPath, base)
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
	data, err := os.ReadFile(filepath.Join(wtPath, ".git"))
	if err != nil {
		return "", err
	}
	line := strings.TrimSpace(string(data))
	prefix := "gitdir: "
	if !strings.HasPrefix(line, prefix) {
		return "", fmt.Errorf("unexpected .git format in %s", wtPath)
	}
	gitDir := strings.TrimPrefix(line, prefix)
	idx := strings.Index(gitDir, "/.git/worktrees/")
	if idx == -1 {
		return "", fmt.Errorf("not a worktree gitdir: %s", gitDir)
	}
	return gitDir[:idx], nil
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
