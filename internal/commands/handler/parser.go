package handler

import (
	"fmt"
	"os"

	"github.com/deprecatedluar/akeyshually/internal"
	"github.com/deprecatedluar/akeyshually/internal/commands"
)

// Parse handles all CLI argument parsing and command execution
// Returns true if foreground mode should run, false if command was handled
func Parse(args []string) bool {
	// Process flags first, collect remaining args
	var remaining []string
	for _, arg := range args {
		switch arg {
		case "--debug":
			internal.DebugEnabled = true
			internal.LoggingEnabled = true // Enable shortcut logging with debug
		default:
			remaining = append(remaining, arg)
		}
	}

	if len(remaining) == 0 {
		return true // Run in foreground
	}

	command := remaining[0]

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
	case "enable":
		if len(remaining) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: akeyshually enable <file.toml>\n")
			os.Exit(1)
		}
		commands.Enable(remaining[1])
		os.Exit(0)
	case "disable":
		if len(remaining) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: akeyshually disable <file.toml>\n")
			os.Exit(1)
		}
		commands.Disable(remaining[1])
		os.Exit(0)
	case "list", "ls":
		commands.List()
		os.Exit(0)
	case "clear":
		commands.Clear()
		os.Exit(0)
	case "config", "conf", "edit":
		filename := ""
		if len(remaining) > 1 {
			filename = remaining[1]
		}
		commands.Config(filename)
		os.Exit(0)
	case "-e":
		filename := ""
		if len(remaining) > 1 {
			filename = remaining[1]
		}
		commands.Config(filename)
		os.Exit(0)
	case "help", "-h", "--help":
		commands.Help(remaining[1:]...)
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
