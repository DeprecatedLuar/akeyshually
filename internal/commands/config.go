package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// openInEditor opens a file in the user's preferred editor
func openInEditor(filePath string) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim" // Fallback
	}

	// Expand home directory
	if filePath[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error finding home directory: %v\n", err)
			os.Exit(1)
		}
		filePath = filepath.Join(home, filePath[1:])
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Config file not found: %s\n", filePath)
		fmt.Fprintf(os.Stderr, "Run akeyshually once to auto-generate config files.\n")
		os.Exit(1)
	}

	cmd := exec.Command(editor, filePath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error opening editor: %v\n", err)
		os.Exit(1)
	}
}

// Config opens the main config file in the editor
func Config() {
	openInEditor("~/.config/akeyshually/config.toml")
}

// Shortcuts opens the shortcuts config file in the editor
func Shortcuts() {
	openInEditor("~/.config/akeyshually/shortcuts.toml")
}
