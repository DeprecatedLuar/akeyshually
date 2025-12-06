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
		case "overlays", "overlay":
			HelpOverlays()
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
	gohelp.Item("akeyshually config [file]", "Edit config file in $EDITOR")
	gohelp.Item("akeyshually enable <file>", "Enable config overlay")
	gohelp.Item("akeyshually disable <file>", "Disable config overlay")
	gohelp.Item("akeyshually list", "List all config files and their status")
	gohelp.Item("akeyshually clear", "Disable all overlays")
	gohelp.Item("akeyshually help [topic]", "Show this help message")
	gohelp.Item("akeyshually version", "Show version information")

	fmt.Println("\nFlags:")
	gohelp.Item("--debug, -d", "Show device detection and verbose output")
	gohelp.Item("--logging, -l", "Show shortcut execution logging")

	fmt.Println("\nHelp Topics:")
	gohelp.Item("akeyshually help config", "Configuration file documentation")
	gohelp.Item("akeyshually help overlays", "Config overlay system documentation")

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

}

// HelpOverlays displays config overlay system documentation
func HelpOverlays() {
	gohelp.PrintHeader("Config Overlay System")

	gohelp.Paragraph("Overlay configs allow you to enable/disable groups of shortcuts dynamically without editing the main config.toml.")

	fmt.Println(gohelp.Header("Use Cases"))
	gohelp.Item("• Gaming mode", "Override window manager shortcuts while gaming")
	gohelp.Item("• Work profiles", "Different shortcuts for different projects")
	gohelp.Item("• Application sets", "Load shortcuts specific to certain apps")

	fmt.Println(gohelp.Header("How It Works"))
	gohelp.Item("1. Base config", "config.toml is always loaded first")
	gohelp.Item("2. Overlays merge", "Enabled overlays merge on top, overriding base shortcuts")
	gohelp.Item("3. Auto-reload", "Enabled overlays are watched for changes")

	fmt.Println(gohelp.Header("Commands"))
	gohelp.Item("akeyshually enable gaming.toml", "Enable overlay and restart daemon")
	gohelp.Item("akeyshually disable gaming.toml", "Disable overlay and restart daemon")
	gohelp.Item("akeyshually list", "Show all config files and their status")
	gohelp.Item("akeyshually clear", "Disable all overlays")
	gohelp.Item("akeyshually config gaming", "Edit gaming.toml overlay")

	fmt.Println(gohelp.Header("Creating Overlays"))
	gohelp.Paragraph("Create any .toml file in ~/.config/akeyshually/ with [shortcuts] and [command_variables] sections:")
	gohelp.Paragraph("Example gaming.toml:\n  [shortcuts]\n  \"super+w\" = \"echo 'disabled in gaming mode'\"\n  \"super+q\" = \"echo 'disabled in gaming mode'\"\n\n  [command_variables]\n  # Optional command aliases")

	fmt.Println(gohelp.Header("Settings"))
	gohelp.Item("notify_on_overlay_change", "Desktop notifications when overlays change (default: false)")
	gohelp.Paragraph("Enable in config.toml:\n  [settings]\n  notify_on_overlay_change = true")
}

