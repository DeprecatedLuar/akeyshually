package handler

import (
	"fmt"
	"os"

	"github.com/deprecatedluar/akeyshually/internal"
	"github.com/deprecatedluar/akeyshually/internal/commands"
)

// ParseResult contains the result of CLI argument parsing
type ParseResult struct {
	RunForeground bool
	ConfigPath    string // Custom config path (empty = default)
}

// Parse handles all CLI argument parsing and command execution
// Returns ParseResult with foreground flag and optional config path
func Parse(args []string) ParseResult {
	// Process flags first, collect remaining args
	var remaining []string
	var configPath string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--debug":
			internal.SetDebug(true)
		case "-c", "--config":
			if i+1 < len(args) {
				configPath = args[i+1]
				i++ // Skip next arg
			} else {
				fmt.Fprintf(os.Stderr, "Error: %s requires a config path\n", arg)
				os.Exit(1)
			}
		default:
			remaining = append(remaining, arg)
		}
	}

	if len(remaining) == 0 {
		return ParseResult{RunForeground: true, ConfigPath: configPath}
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

	return ParseResult{} // Never reached, commands exit
}
