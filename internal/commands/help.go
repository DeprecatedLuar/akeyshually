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
		case "modifiers", "syntax", "behaviors":
			HelpModifiers()
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
	gohelp.Item("-c, --config <file>", "Load custom config (name or path)")
	gohelp.Item("--debug", "Show device detection and verbose output")

	fmt.Println("\nHelp Topics:")
	gohelp.Item("akeyshually help config", "Configuration file documentation")
	gohelp.Item("akeyshually help modifiers", "Shortcut modifiers and syntax reference")
	gohelp.Item("akeyshually help overlays", "Config overlay system documentation")

	gohelp.Paragraph("Config: ~/.config/akeyshually/")
}

// HelpConfig displays detailed configuration documentation
func HelpConfig() {
	gohelp.PrintHeader("Configuration Documentation")

	gohelp.Paragraph("Config File: ~/.config/akeyshually/config.toml")

	// Settings section
	fmt.Println(gohelp.Header("[settings]"))
	gohelp.Item("default_interval", "Default interval for repeat behaviors (milliseconds, default: 100)")
	gohelp.Item("disable_media_keys", "Forward media keys to system (default: false)")
	gohelp.Item("", "  • Set to true when using GNOME/KDE media key daemons")
	gohelp.Item("shell", "Shell to use for commands (default: $SHELL, fallback: sh)")
	gohelp.Item("env_file", "File to source before command execution (optional)")

	gohelp.Paragraph("Example:\n  [settings]\n  default_interval = 100\n  disable_media_keys = false\n  shell = \"/bin/bash\"\n  env_file = \"~/.profile\"")

	// Shortcuts section
	fmt.Println(gohelp.Header("[shortcuts]"))
	gohelp.Paragraph("Define keyboard shortcuts: \"modifier+modifier+key\" = \"command\"")
	gohelp.Item("Modifiers:", "super, ctrl, alt, shift (lowercase, no left/right distinction)")
	gohelp.Item("Keys:", "lowercase letters, numbers, special keys (print, space, etc.)")
	gohelp.Item("Syntax:", "Use + to separate modifiers and key")
	gohelp.Item("Behaviors:", ".whileheld, .repeat-whileheld, .repeat-toggle, .switch, .doubletap")
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

// HelpModifiers displays shortcut modifier and syntax documentation
func HelpModifiers() {
	gohelp.PrintHeader("Shortcut Modifiers and Syntax")

	gohelp.Paragraph("Modifiers control when and how shortcuts execute. Add them after the key combo using dot notation.")

	// Timing Modifiers
	fmt.Println(gohelp.Header("Timing Modifiers"))
	gohelp.Item(".onpress", "Execute on key press (default, can be omitted)")
	gohelp.Item("", "  Example: \"super+t\" = \"terminal\"")

	gohelp.Item(".onrelease", "Execute on key release (tap detection)")
	gohelp.Item("", "  Example: \"super.onrelease\" = \"rofi\"")
	gohelp.Item("", "  • Perfect for modifier-only shortcuts (tap Super to launch rofi)")
	gohelp.Item("", "  • Cancelled if other keys pressed while holding modifier")
	gohelp.Item("", "  • Cancelled if mouse clicked while holding modifier")

	// Behavior Modifiers
	fmt.Println(gohelp.Header("Behavior Modifiers"))
	gohelp.Item(".whileheld(interval)", "Run process while key held (kill on release)")
	gohelp.Item("", "  Example: \"super+f.whileheld\" = \"$FILEMANAGER\"")
	gohelp.Item("", "  • Starts process on press, sends SIGTERM on release")
	gohelp.Item("", "  • Good for apps that should only run while holding key")

	gohelp.Item(".repeat-whileheld(interval)", "Repeat command while key held")
	gohelp.Item("", "  Example: \"super+up.repeat-whileheld(100)\" = \"volume_up\"")
	gohelp.Item("", "  • Interval in milliseconds (omit for default_interval)")
	gohelp.Item("", "  • Executes continuously until key released")
	gohelp.Item("", "  • Float intervals supported: .repeat-whileheld(0.015) = 15ms")

	gohelp.Item(".repeat-toggle(interval)", "Start/stop repeating loop on each press")
	gohelp.Item("", "  Example: \"f1.repeat-toggle(50)\" = \"xdotool click 1\"")
	gohelp.Item("", "  • First press starts loop, second press stops it")
	gohelp.Item("", "  • Loop continues even after key released")
	gohelp.Item("", "  • Useful for auto-clickers or continuous actions")

	gohelp.Item(".switch", "Cycle through array of commands")
	gohelp.Item("", "  Example: \"f2.switch\" = [\"cmd1\", \"cmd2\", \"cmd3\"]")
	gohelp.Item("", "  • Each press executes next command in array")
	gohelp.Item("", "  • Wraps around to first command after last")
	gohelp.Item("", "  • Requires array of at least 2 commands")

	gohelp.Item(".doubletap(interval)", "Execute on double-tap of any single key")
	gohelp.Item("", "  Example: \"super.doubletap(300)\" = \"rofi -show drun\"")
	gohelp.Item("", "  Example: \"print.doubletap(300)\" = \"screen-record\"")
	gohelp.Item("", "  • Works on modifiers and regular keys (not key combos)")
	gohelp.Item("", "  • First tap starts timer, second tap within interval executes")
	gohelp.Item("", "  • If timeout expires, executes single-tap shortcut if defined")
	gohelp.Item("", "  • Mouse clicks cancel tap detection (modifiers only)")

	gohelp.Item(".passthrough", "Execute shortcut without consuming modifiers")
	gohelp.Item("", "  Example: \"v.passthrough\" = \"copyq toggle\"")
	gohelp.Item("", "  • Modifier state ignored when matching")
	gohelp.Item("", "  • Super+V and Ctrl+V both trigger \"v.passthrough\"")
	gohelp.Item("", "  • Useful for shortcuts that work with any modifier combo")

	// Combining Modifiers
	fmt.Println(gohelp.Header("Combining Modifiers"))
	gohelp.Paragraph("Behaviors and timing can be combined:\n  \"super+up.repeat-whileheld(100)\" = \"volume_up\"\n  \"f1.repeat-toggle(50)\" = \"xdotool click 1\"")
	gohelp.Paragraph("Restrictions:\n  • .doubletap only works on single keys (no key combos)\n  • .switch requires command array, others require single command")

	// Examples
	fmt.Println(gohelp.Header("Complete Examples"))
	gohelp.Paragraph("[shortcuts]\n# Basic shortcuts\n\"super+t\" = \"terminal\"                         # Execute on press\n\"super.onrelease\" = \"rofi\"                     # Tap Super for rofi\n\n# Double-tap detection\n\"super.onrelease\" = \"hotline\"                  # Single tap\n\"super.doubletap(200)\" = \"$LAUNCHER\"           # Double-tap within 200ms\n\n# Process lifecycle\n\"super+f.whileheld\" = \"$FILEMANAGER\"           # Open while held, close on release\n\n# Repeating commands\n\"super+up.repeat-whileheld(100)\" = \"volume_up\" # Hold to repeat\n\"f1.repeat-toggle(50)\" = \"xdotool click 1\"     # Toggle auto-clicker\n\n# Cycling commands\n\"f2.switch\" = [\"mode1\", \"mode2\", \"mode3\"]       # Cycle on each press\n\n# Passthrough\n\"v.passthrough\" = \"clipboard_manager\"          # Works with any modifiers")
}

