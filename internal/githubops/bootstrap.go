package githubops

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/geraldc/workspace-cli/internal/config"
	"github.com/geraldc/workspace-cli/internal/gitops"
)

func Bootstrap(cfg config.Config, repoName string, private, clone bool, workspaceName, strategy string) error {
	visibility := "--private"
	if !private {
		visibility = "--public"
	}
	if err := run("gh", "repo", "create", repoName, visibility, "--confirm"); err != nil {
		return err
	}
	fmt.Println("created github repository:", repoName)

	if !clone {
		return nil
	}
	if len(cfg.RepoRoots) == 0 {
		return fmt.Errorf("repo_roots is empty in config")
	}

	sshURL, err := output("gh", "repo", "view", repoName, "--json", "sshUrl", "--jq", ".sshUrl")
	if err != nil {
		return err
	}
	sshURL = strings.TrimSpace(sshURL)
	nameOnly := repoBaseName(repoName)

	canonicalPath := filepath.Join(cfg.RepoRoots[0], nameOnly)
	if _, err := os.Stat(canonicalPath); err == nil {
		return fmt.Errorf("canonical repo already exists: %s", canonicalPath)
	}
	if err := os.MkdirAll(cfg.RepoRoots[0], 0o755); err != nil {
		return err
	}
	if err := run("git", "clone", sshURL, canonicalPath); err != nil {
		return err
	}
	fmt.Println("cloned repository:", canonicalPath)

	if workspaceName == "" {
		return nil
	}
	wsDir := filepath.Join(cfg.WorkspaceBaseDir, workspaceName)
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		return err
	}
	wtPath := filepath.Join(wsDir, nameOnly)
	if err := gitops.AddWorktree(canonicalPath, wtPath, workspaceName, strategy); err != nil {
		return err
	}
	fmt.Println("added to workspace:", wtPath)
	return nil
}

func repoBaseName(repoName string) string {
	parts := strings.Split(strings.TrimSpace(repoName), "/")
	return parts[len(parts)-1]
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s failed: %w (%s)", name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func output(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s %s failed: %w", name, strings.Join(args, " "), err)
	}
	return string(out), nil
}
