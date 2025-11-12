package commands

import (
	"fmt"
	"os"

	"github.com/DeprecatedLuar/gohelp"
)

// Help displays usage information or topic-specific help
func Help(args ...string) {
	// Check for topic-specific help
	if len(args) > 0 {
		switch args[0] {
		case "config":
			HelpConfig()
			return
		default:
			fmt.Fprintf(os.Stderr, "Unknown help topic: %s\n\n", args[0])
		}
	}

	// General help
	gohelp.PrintHeader("akeyshually - keyboard shortcut daemon")

	fmt.Println("Usage:")
	gohelp.Item("akeyshually", "Run in foreground (current terminal)")
	gohelp.Item("akeyshually start", "Start daemon in background")
	gohelp.Item("akeyshually stop", "Stop running daemon")
	gohelp.Item("akeyshually restart", "Restart daemon")
	gohelp.Item("akeyshually update", "Check for and install updates")
	gohelp.Item("akeyshually config", "Edit config file in $EDITOR")
	gohelp.Item("akeyshually help [topic]", "Show this help message")
	gohelp.Item("akeyshually version", "Show version information")

	fmt.Println("\nHelp Topics:")
	gohelp.Item("akeyshually help config", "Configuration file documentation")

	gohelp.Paragraph("Config: ~/.config/akeyshually/")
}

// HelpConfig displays detailed configuration documentation
func HelpConfig() {
	gohelp.PrintHeader("Configuration Documentation")

	gohelp.Paragraph("Config File: ~/.config/akeyshually/config.toml")

	// Settings section
	fmt.Println(gohelp.Header("[settings]"))
	gohelp.Item("default_loop_interval", "Default interval for .loop/.toggle behaviors (milliseconds, default: 100)")
	gohelp.Item("disable_media_keys", "Forward media keys to system (default: false)")
	gohelp.Item("", "  • Set to true when using GNOME/KDE media key daemons")
	gohelp.Item("shell", "Shell to use for commands (default: $SHELL, fallback: sh)")
	gohelp.Item("env_file", "File to source before command execution (optional)")

	gohelp.Paragraph("Example:\n  [settings]\n  default_loop_interval = 100\n  disable_media_keys = false\n  shell = \"/bin/bash\"\n  env_file = \"~/.profile\"")

	// Shortcuts section
	fmt.Println(gohelp.Header("[shortcuts]"))
	gohelp.Paragraph("Define keyboard shortcuts: \"modifier+modifier+key\" = \"command\"")
	gohelp.Item("Modifiers:", "super, ctrl, alt, shift (lowercase, no left/right distinction)")
	gohelp.Item("Keys:", "lowercase letters, numbers, special keys (print, space, etc.)")
	gohelp.Item("Syntax:", "Use + to separate modifiers and key")
	gohelp.Item("Behaviors:", ".loop, .toggle, .switch, .whileheld (optional interval: .loop(50))")
	gohelp.Item("Timing:", ".onpress (default), .onrelease")

	gohelp.Paragraph("Examples:\n  [shortcuts]\n  \"super.onrelease\" = \"rofi\"\n  \"super+t\" = \"alacritty\"\n  \"ctrl+alt+delete\" = \"systemctl reboot\"\n  \"print\" = \"prtscr\"  # References [command_variables] section")

	gohelp.Paragraph("Media Keys:\n  Media key shortcuts are included as comments. Uncomment to enable:\n  # \"volumeup\" = \"volume_up\"\n  # \"volumedown\" = \"volume_down\"")

	// Command variables section
	fmt.Println(gohelp.Header("[command_variables]"))
	gohelp.Paragraph("Define reusable command aliases. Shortcuts reference these first, then use literal strings.")
	gohelp.Paragraph("Example:\n  [command_variables]\n  browser = \"brave-browser --new-window\"\n  terminal = \"alacritty --working-directory ~\"")

	// Auto-reload
	fmt.Println(gohelp.Header("Auto-Reload"))
	gohelp.Paragraph("Config file is automatically reloaded when modified (no restart needed)")

	// Environment
	fmt.Println(gohelp.Header("Environment Variables"))
	gohelp.Item("LOGGING=1", "Enable shortcut execution logging to stderr")
}
