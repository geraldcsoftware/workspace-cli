# space

`space` is a powerful CLI workspace manager for Go developers. It helps you organize your projects by creating isolated workspaces using Git worktrees, allowing you to work on multiple features or versions of the same repository simultaneously without the overhead of multiple clones.

## Features

- **TUI Workspace Picker:** A beautiful terminal user interface for quickly switching between or creating new workspaces.
- **Git Worktree Integration:** Automatically manages Git worktrees to keep your development environment clean.
- **Project Discovery:** Scans your specified repository roots to find and add projects easily.
- **Repository Bootstrapping:** Quickly create new GitHub repositories and add them to your workspaces.
- **Shell Integration:** Seamlessly change your shell's current directory to the selected workspace.

## Installation

### Homebrew (Recommended)

To install `space` using Homebrew, run the following commands:

```bash
brew install geraldcsoftware/tap/space
```

### Building from Source (Local Development)

The `Makefile` commands are intended for local development and testing.

1. Clone the repository:
   ```bash
   git clone https://github.com/geraldc/workspace-cli.git
   cd workspace-cli
   ```

2. Build and run locally:
   ```bash
   make build   # Builds the binary to bin/space
   make launch  # Builds and launches the application
   ```

### Shell Integration

To enable the `go` command to change your directory, add the following function to your shell profile (e.g., `~/.zshrc` or `~/.bashrc`):

```bash
space() {
  local out
  out=$(command space "$@")
  if [[ $out == __SPACE_CD__:* ]]; then
    cd "${out#__SPACE_CD__:}"
  else
    echo "$out"
  fi
}
```

## Configuration

`space` stores its configuration in `~/.config/space/config.json`. You can initialize a default configuration by running:

```bash
space config init
```

### Configuration Options

- `workspace_base_dir`: The root directory where workspaces will be created (default: `~/workspaces`).
- `repo_roots`: A list of directories to scan for Git repositories (default: `~/work`, `~/StudioProjects`).
- `max_depth`: Maximum recursion depth for finding repositories (default: 3).
- `cache_age_seconds`: How long to cache the list of discovered repositories (default: 3600).

## Usage

### Interactive Mode

Simply run `space` to open the interactive TUI:

```bash
space
```

Use the arrow keys to navigate your workspaces and press **Enter** to switch to the selected one.

### CLI Commands

- **Create a workspace:**
  ```bash
  space create <name> <query>... [--strategy branch|detach]
  ```
- **Add repositories to a workspace:**
  ```bash
  space add <name> <query>... [--strategy branch|detach]
  ```
- **List all workspaces:**
  ```bash
  space list
  ```
- **Check workspace status:**
  ```bash
  space status <name>
  ```
- **Remove a workspace:**
  ```bash
  space remove <name> [--force]
  ```
- **Search discovered repositories:**
  ```bash
  space repos [--refresh]
  ```
- **Bootstrap a new repository:**
  ```bash
  space bootstrap-repo <name> [--workspace name]
  ```
- **Show current configuration:**
  ```bash
  space config show
  ```

## License

[Add License Information Here]
