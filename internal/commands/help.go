package commands

import (
	gohelp "github.com/DeprecatedLuar/gohelp-luar"
)

var (
	helpRoot = gohelp.NewPage("akeyshually", "keyboard shortcut daemon").
		Usage("akeyshually [command] [flags]").
		Section("Commands",
			gohelp.Cmd("start", "Start daemon in background"),
			gohelp.Cmd("stop", "Stop running daemon"),
			gohelp.Cmd("restart", "Restart daemon"),
			gohelp.Cmd("update", "Check for and install updates"),
			gohelp.Cmd("config [file]", "Edit config file in $EDITOR"),
			gohelp.Cmd("enable <file>", "Enable config overlay"),
			gohelp.Cmd("disable <file>", "Disable config overlay"),
			gohelp.Cmd("list", "List all config files and their status"),
			gohelp.Cmd("clear", "Disable all overlays"),
			gohelp.Cmd("help [topic]", "Show this help message"),
			gohelp.Cmd("version", "Show version information"),
		).
		Section("Flags",
			gohelp.Cmd("-c, --config <file>", "Load custom config (name or path)"),
			gohelp.Cmd("--debug", "Show device detection and verbose output"),
		).
		Text("Config: ~/.config/akeyshually/")

	helpConfig = gohelp.NewPage("config", "configuration file documentation").
		Text("Config File: ~/.config/akeyshually/config.toml").
		Section("[settings]",
			gohelp.Cmd("default_interval", "Default interval for repeat behaviors (milliseconds, default: 100)"),
			gohelp.Cmd("disable_media_keys", "Forward media keys to system (default: false)"),
			gohelp.Cmd("shell", "Shell to use for commands (default: $SHELL, fallback: sh)"),
			gohelp.Cmd("env_file", "File to source before command execution (optional)"),
		).
		Text("[settings]\ndefault_interval = 100\ndisable_media_keys = false\nshell = \"/bin/bash\"\nenv_file = \"~/.profile\"").
		Section("[shortcuts]",
			gohelp.Cmd("Modifiers", "super, ctrl, alt, shift (lowercase, no left/right distinction)"),
			gohelp.Cmd("Keys", "lowercase letters, numbers, special keys (print, space, etc.)"),
			gohelp.Cmd("Syntax", "Use + to separate modifiers and key"),
			gohelp.Cmd("Behaviors", ".whileheld, .repeat-whileheld, .repeat-toggle, .switch, .doubletap"),
			gohelp.Cmd("Timing", ".onpress (default), .onrelease"),
		).
		Text("[shortcuts]\n\"super.onrelease\" = \"rofi\"\n\"super+t\" = \"alacritty\"\n\"ctrl+alt+delete\" = \"systemctl reboot\"\n\"print\" = \"prtscr\"  # References [command_variables]").
		Section("[command_variables]",
			gohelp.Cmd("browser", "Reusable command alias").Example("browser = \"brave-browser --new-window\""),
			gohelp.Cmd("terminal", "Reusable command alias").Example("terminal = \"alacritty --working-directory ~\""),
		).
		Text("Auto-Reload: config file is automatically reloaded when modified (no restart needed)")

	helpOverlays = gohelp.NewPage("overlays", "config overlay system").
		Text("Overlay configs allow you to enable/disable groups of shortcuts dynamically without editing the main config.toml.").
		Section("Use Cases",
			gohelp.Cmd("Gaming mode", "Override window manager shortcuts while gaming"),
			gohelp.Cmd("Work profiles", "Different shortcuts for different projects"),
			gohelp.Cmd("Application sets", "Load shortcuts specific to certain apps"),
		).
		Section("How It Works",
			gohelp.Cmd("1. Base config", "config.toml is always loaded first"),
			gohelp.Cmd("2. Overlays merge", "Enabled overlays merge on top, overriding base shortcuts"),
			gohelp.Cmd("3. Auto-reload", "Enabled overlays are watched for changes"),
		).
		Section("Commands",
			gohelp.Cmd("enable gaming.toml", "Enable overlay and restart daemon").Example("akeyshually enable gaming.toml"),
			gohelp.Cmd("disable gaming.toml", "Disable overlay and restart daemon"),
			gohelp.Cmd("list", "Show all config files and their status"),
			gohelp.Cmd("clear", "Disable all overlays"),
			gohelp.Cmd("config gaming", "Edit gaming.toml overlay"),
		).
		Section("Settings",
			gohelp.Cmd("notify_on_overlay_change", "Desktop notifications when overlays change (default: false)"),
		).
		Text("Enable in config.toml:\n[settings]\nnotify_on_overlay_change = true")

	helpModifiers = gohelp.NewPage("modifiers", "shortcut modifiers and syntax reference").
		Text("Modifiers control when and how shortcuts execute. Add them after the key combo using dot notation.").
		Section("Timing",
			gohelp.Cmd(".onpress", "Execute on key press (default, can be omitted)").Example("\"super+t\" = \"terminal\""),
			gohelp.Cmd(".onrelease", "Execute on key release — tap detection; cancelled by other keys or mouse clicks").Example("\"super.onrelease\" = \"rofi\""),
		).
		Section("Behaviors",
			gohelp.Cmd(".whileheld", "Run process while key held, SIGTERM on release").Example("\"super+f.whileheld\" = \"$FILEMANAGER\""),
			gohelp.Cmd(".repeat-whileheld(ms)", "Repeat command while key held; omit interval for default_interval").Example("\"super+up.repeat-whileheld(100)\" = \"volume_up\""),
			gohelp.Cmd(".repeat-toggle(ms)", "First press starts loop, second press stops it").Example("\"f1.repeat-toggle(50)\" = \"xdotool click 1\""),
			gohelp.Cmd(".switch", "Cycle through array of commands on each press").Example("\"f2.switch\" = [\"cmd1\", \"cmd2\", \"cmd3\"]"),
			gohelp.Cmd(".doubletap(ms)", "Execute on double-tap; works on modifiers and single keys only").Example("\"super.doubletap(300)\" = \"rofi -show drun\""),
			gohelp.Cmd(".passthrough", "Match regardless of modifier state").Example("\"v.passthrough\" = \"copyq toggle\""),
		).
		Section("Combining",
			gohelp.Cmd(".repeat-whileheld + timing", "").Example("\"super+up.repeat-whileheld(100)\" = \"volume_up\""),
			gohelp.Cmd(".repeat-toggle + timing", "").Example("\"f1.repeat-toggle(50)\" = \"xdotool click 1\""),
		).
		Text("Restrictions:\n  • .doubletap only works on single keys (no key combos)\n  • .switch requires a command array, others require a single command")
)

// Help displays usage information or topic-specific help.
func Help(args ...string) {
	gohelp.Run(append([]string{"help"}, args...), helpRoot, helpConfig, helpOverlays, helpModifiers)
}
