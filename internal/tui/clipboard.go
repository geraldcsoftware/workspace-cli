package tui

import (
	"os/exec"
	"runtime"
	"strings"
)

// copyToClipboard copies the given text to the system clipboard.
func copyToClipboard(text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		if _, err := exec.LookPath("wl-copy"); err == nil {
			cmd = exec.Command("wl-copy")
		} else {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		}
	default:
		return nil // Not supported on other platforms for now
	}

	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}
