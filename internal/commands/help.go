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
	gohelp.Item("akeyshually config", "Edit main config file in $EDITOR")
	gohelp.Item("akeyshually shortcuts", "Edit shortcuts file in $EDITOR")
	gohelp.Item("akeyshually help [topic]", "Show this help message")
	gohelp.Item("akeyshually version", "Show version information")

	fmt.Println("\nHelp Topics:")
	gohelp.Item("akeyshually help config", "Configuration file documentation")

	gohelp.Paragraph("Config: ~/.config/akeyshually/")
}

// HelpConfig displays detailed configuration documentation
func HelpConfig() {
	gohelp.PrintHeader("Configuration Documentation")

	gohelp.Paragraph("Config Directory: ~/.config/akeyshually/")

	// config.toml
	fmt.Println(gohelp.Header("config.toml (optional)"))
	gohelp.Paragraph("[settings]")
	gohelp.Item("trigger_on", "When to execute shortcuts: \"press\" (default) or \"release\"")
	gohelp.Item("", "  • press: Execute on key down")
	gohelp.Item("", "  • release: Execute on key up (required for modifier taps)")
	gohelp.Item("enable_media_keys", "Load media-keys.toml: true or false (default: false)")
	gohelp.Item("shell", "Shell to use for commands (default: $SHELL, fallback: sh)")
	gohelp.Item("env_file", "File to source before command execution (optional)")

	gohelp.Paragraph("Example:\n  [settings]\n  trigger_on = \"release\"\n  enable_media_keys = false\n  shell = \"/bin/bash\"\n  env_file = \"~/.profile\"")

	// shortcuts.toml
	fmt.Println(gohelp.Header("shortcuts.toml (required)"))
	gohelp.Paragraph("[shortcuts]\nDefine keyboard shortcuts: \"modifier+modifier+key\" = \"command\"")
	gohelp.Item("Modifiers:", "super, ctrl, alt, shift (lowercase, no left/right distinction)")
	gohelp.Item("Keys:", "lowercase letters, numbers, special keys (print, space, etc.)")
	gohelp.Item("Syntax:", "Use + to separate modifiers and key")

	gohelp.Paragraph("Examples:\n  [shortcuts]\n  \"super+t\" = \"alacritty\"\n  \"ctrl+alt+delete\" = \"systemctl reboot\"\n  \"print\" = \"maim -s | xclip -selection clipboard -t image/png\"\n  \"super+b\" = \"browser\"  # References [commands] section")

	gohelp.Paragraph("Modifier Tap Detection (release mode only):\n  \"super\" = \"rofi -show drun\"  # Execute on lone Super tap")

	// Commands section
	fmt.Println(gohelp.Header("commands (optional - for DRY)"))
	gohelp.Paragraph("Define reusable command aliases. Referenced shortcuts look up here first.")
	gohelp.Paragraph("Example:\n  [commands]\n  browser = \"brave-browser --new-window\"\n  terminal = \"alacritty --working-directory ~\"")

	// media-keys.toml
	fmt.Println(gohelp.Header("media-keys.toml (optional)"))
	gohelp.Paragraph("Loaded only if enable_media_keys=true in config.toml\nSame format as shortcuts.toml for media key bindings")

	// Auto-reload
	fmt.Println(gohelp.Header("Auto-Reload"))
	gohelp.Paragraph("Config files are automatically reloaded when modified (no restart needed)")

	// Environment
	fmt.Println(gohelp.Header("Environment Variables"))
	gohelp.Item("LOGGING=1", "Enable shortcut execution logging to stderr")
}
