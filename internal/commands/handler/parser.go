package handler

import (
	"fmt"
	"os"

	"github.com/deprecatedluar/akeyshually/internal/commands"
)

// Parse handles all CLI argument parsing and command execution
// Returns true if foreground mode should run, false if command was handled
func Parse(args []string) bool {
	if len(args) == 0 {
		return true // Run in foreground
	}

	command := args[0]

	switch command {
	case "start":
		commands.Start()
		os.Exit(0)
	case "stop":
		commands.Stop()
		os.Exit(0)
	case "restart":
		commands.Restart()
		os.Exit(0)
	case "update":
		commands.Update()
		os.Exit(0)
	case "config", "conf":
		commands.Config()
		os.Exit(0)
	case "shortcuts":
		commands.Shortcuts()
		os.Exit(0)
	case "help", "-h", "--help":
		commands.Help(args[1:]...)
		os.Exit(0)
	case "version", "-v", "--version":
		commands.Version()
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		commands.Help()
		os.Exit(1)
	}

	return false
}
