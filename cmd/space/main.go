package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/geraldc/workspace-cli/internal/config"
	"github.com/geraldc/workspace-cli/internal/discovery"
	"github.com/geraldc/workspace-cli/internal/githubops"
	"github.com/geraldc/workspace-cli/internal/tui"
	"github.com/geraldc/workspace-cli/internal/workspace"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.EnsureAndLoad()
	if err != nil {
		return err
	}

	if len(os.Args) == 1 {
		return runWorkspacePicker(cfg)
	}

	switch os.Args[1] {
	case "config":
		return runConfig(cfg, os.Args[2:])
	case "create":
		return runCreate(cfg, os.Args[2:])
	case "add":
		return runAdd(cfg, os.Args[2:])
	case "list", "ls":
		return runList(cfg)
	case "status", "st":
		return runStatus(cfg, os.Args[2:])
	case "remove", "rm":
		return runRemove(cfg, os.Args[2:])
	case "repos":
		return runRepos(cfg, os.Args[2:])
	case "go":
		return runGo(cfg, os.Args[2:])
	case "bootstrap-repo":
		return runBootstrapRepo(cfg, os.Args[2:])
	case "help", "-h", "--help":
		printHelp()
		return nil
	default:
		return fmt.Errorf("unknown command: %s", os.Args[1])
	}
}

func runWorkspacePicker(cfg config.Config) error {
	items, err := workspace.List(cfg)
	if err != nil {
		return err
	}
	result, err := tui.RunWorkspacePicker(items)
	if err != nil {
		return err
	}
	if result.CreateNew {
		return runCreateInteractive(cfg)
	}
	if result.SelectedPath != "" {
		fmt.Printf("__SPACE_CD__:%s\n", result.SelectedPath)
	}
	return nil
}

func runCreateInteractive(cfg config.Config) error {
	wsPath, err := tui.RunCreateWorkspace(cfg)
	if err != nil {
		return err
	}
	if wsPath != "" {
		fmt.Fprintf(os.Stderr, "workspace created: %s\n", wsPath)
		fmt.Printf("__SPACE_CD__:%s\n", wsPath)
	}
	return nil
}

func runConfig(cfg config.Config, args []string) error {
	if len(args) == 0 || args[0] == "show" {
		fmt.Println("config file:", config.ConfigFilePath())
		fmt.Println("cache file:", config.CacheFilePath())
		fmt.Println("workspace_base_dir:", cfg.WorkspaceBaseDir)
		fmt.Println("repo_roots:", strings.Join(cfg.RepoRoots, ", "))
		fmt.Println("max_depth:", cfg.MaxDepth)
		fmt.Println("cache_age_seconds:", cfg.CacheAgeSeconds)
		return nil
	}
	if args[0] == "init" {
		return config.SaveDefaultsIfMissing()
	}
	return fmt.Errorf("unknown config subcommand: %s", args[0])
}

func runCreate(cfg config.Config, args []string) error {
	if len(args) < 2 {
		return errors.New("usage: create <workspace-name> <repo-query>... [--strategy branch|detach]")
	}
	strategy, filtered := extractStrategy(args)
	wsName := filtered[0]
	repos := filtered[1:]
	created, err := workspace.Create(cfg, wsName, repos, strategy)
	if err != nil {
		return err
	}
	fmt.Printf("created workspace %q with %d repo(s)\n", wsName, len(created))
	fmt.Printf("__SPACE_CD__:%s\n", filepath.Join(cfg.WorkspaceBaseDir, wsName))
	return nil
}

func runAdd(cfg config.Config, args []string) error {
	if len(args) < 2 {
		return errors.New("usage: add <workspace-name> <repo-query>... [--strategy branch|detach]")
	}
	strategy, filtered := extractStrategy(args)
	wsName := filtered[0]
	repos := filtered[1:]
	added, err := workspace.Add(cfg, wsName, repos, strategy)
	if err != nil {
		return err
	}
	fmt.Printf("added %d repo(s) to workspace %q\n", len(added), wsName)
	return nil
}

func runList(cfg config.Config) error {
	items, err := workspace.List(cfg)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		fmt.Println("no workspaces found")
		return nil
	}
	for _, item := range items {
		fmt.Printf("%s (%d repos)\n", item.Name, item.RepoCount)
	}
	return nil
}

func runStatus(cfg config.Config, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: status <workspace-name>")
	}
	info, err := workspace.Status(cfg, args[0])
	if err != nil {
		return err
	}
	fmt.Println("workspace:", info.Name)
	fmt.Println("path:", info.Path)
	for _, repo := range info.Repos {
		fmt.Printf("- %s  branch=%s  modified=%d staged=%d untracked=%d\n", repo.Name, repo.Branch, repo.Modified, repo.Staged, repo.Untracked)
	}
	return nil
}

func runRemove(cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("remove", flag.ContinueOnError)
	force := fs.Bool("force", false, "force remove with uncommitted changes")
	_ = fs.Bool("f", false, "shorthand for --force")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *force == false && fs.Lookup("f").Value.String() == "true" {
		*force = true
	}
	remaining := fs.Args()
	if len(remaining) != 1 {
		return errors.New("usage: remove <workspace-name> [--force]")
	}
	return workspace.Remove(cfg, remaining[0], *force)
}

func runRepos(cfg config.Config, args []string) error {
	refresh := len(args) > 0 && (args[0] == "--refresh" || args[0] == "-r")
	repos, err := discovery.FindRepos(cfg, refresh)
	if err != nil {
		return err
	}
	for _, repo := range repos {
		fmt.Println(repo)
	}
	return nil
}

func runGo(cfg config.Config, args []string) error {
	if len(args) == 0 {
		return runWorkspacePicker(cfg)
	}
	if len(args) != 1 {
		return errors.New("usage: go [workspace-name]")
	}
	path := filepath.Join(cfg.WorkspaceBaseDir, args[0])
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("workspace not found: %s", args[0])
	}
	fmt.Printf("__SPACE_CD__:%s\n", path)
	return nil
}

func runBootstrapRepo(cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("bootstrap-repo", flag.ContinueOnError)
	private := fs.Bool("private", true, "create as private repo")
	workspaceName := fs.String("workspace", "", "optionally add to this workspace")
	clone := fs.Bool("clone", true, "clone to repo root and optionally add as worktree")
	strategy := fs.String("strategy", "branch", "branch|detach")
	if err := fs.Parse(args); err != nil {
		return err
	}
	remaining := fs.Args()
	if len(remaining) != 1 {
		return errors.New("usage: bootstrap-repo <name> [--private=true|false] [--workspace name] [--clone=true|false]")
	}
	return githubops.Bootstrap(cfg, remaining[0], *private, *clone, *workspaceName, *strategy)
}

func extractStrategy(args []string) (string, []string) {
	strategy := "branch"
	filtered := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if args[i] == "--strategy" && i+1 < len(args) {
			strategy = args[i+1]
			i++
			continue
		}
		filtered = append(filtered, args[i])
	}
	return strategy, filtered
}

func printHelp() {
	fmt.Println("space - workspace manager")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  space                                # TUI workspace picker")
	fmt.Println("  space create <name> <query>... [--strategy branch|detach]")
	fmt.Println("  space add <name> <query>... [--strategy branch|detach]")
	fmt.Println("  space list")
	fmt.Println("  space status <name>")
	fmt.Println("  space remove <name> [--force]")
	fmt.Println("  space repos [--refresh]")
	fmt.Println("  space go [name]")
	fmt.Println("  space bootstrap-repo <name> [--workspace name]")
	fmt.Println("  space config [show|init]")
}
