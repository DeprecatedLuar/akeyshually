package commands

import (
	gohelp "github.com/DeprecatedLuar/gohelp-luar"
)

var (
	helpRoot = gohelp.NewPage("akeyshually", "keyboard shortcut daemon").
		Usage("akeyshually [command] [flags]").
		Section("Commands",
			gohelp.Item("start", "Start daemon in background"),
			gohelp.Item("stop", "Stop running daemon"),
			gohelp.Item("restart", "Restart daemon"),
			gohelp.Item("update", "Check for and install updates"),
			gohelp.Item("config [file]", "Edit config file in $EDITOR"),
			gohelp.Item("enable <file>", "Enable config overlay"),
			gohelp.Item("disable <file>", "Disable config overlay"),
			gohelp.Item("list", "List all config files and their status"),
			gohelp.Item("clear", "Disable all overlays"),
			gohelp.Item("help [topic]", "Show this help message"),
			gohelp.Item("version", "Show version information"),
		).
		Section("Flags",
			gohelp.Item("-c, --config <file>", "Load custom config (name or path)"),
			gohelp.Item("--debug", "Show device detection and verbose output"),
		).
		Text("Config: ~/.config/akeyshually/")

	helpConfig = gohelp.NewPage("config", "configuration file documentation").
		Text("Config File: ~/.config/akeyshually/config.toml").
		Section("[settings]",
			gohelp.Item("default_interval", "Default interval for repeat behaviors (milliseconds, default: 100)"),
			gohelp.Item("disable_media_keys", "Forward media keys to system (default: false)"),
			gohelp.Item("shell", "Shell to use for commands (default: $SHELL, fallback: sh)"),
			gohelp.Item("env_file", "File to source before command execution (optional)"),
			gohelp.Item("devices", "List of device name substrings to grab (case-insensitive)", "devices = [\"Huion\", \"Xbox Controller\"]"),
		).
		Text("[settings]\ndefault_interval = 100\ndisable_media_keys = false\nshell = \"/bin/bash\"\nenv_file = \"~/.profile\"").
		Section("[shortcuts]",
			gohelp.Item("Key modifiers", "super, ctrl, alt, shift (lowercase, no left/right distinction)"),
			gohelp.Item("Keys", "lowercase letters, numbers, special keys (print, space, etc.)"),
			gohelp.Item("Syntax", "Use + to separate modifiers and key"),
			gohelp.Item("Triggers", ".onpress (default), .onrelease, .whileheld, .hold, .doubletap, .tapwhileheld, .pressrelease"),
			gohelp.Item("Modifiers", ".repeat-whileheld, .repeat-toggle, .switch, .passthrough"),
		).
		Text("[shortcuts]\n\"super.onrelease\" = \"rofi\"\n\"super+t\" = \"alacritty\"\n\"ctrl+alt+delete\" = \"systemctl reboot\"\n\"print\" = \"prtscr\"  # References [command_variables]").
		Section("[command_variables]",
			gohelp.Item("browser", "Reusable command alias", "browser = \"brave-browser --new-window\""),
			gohelp.Item("terminal", "Reusable command alias", "terminal = \"alacritty --working-directory ~\""),
		).
		Text("Auto-Reload: config file is automatically reloaded when modified (no restart needed)")

	helpOverlays = gohelp.NewPage("overlays", "config overlay system").
		Text("Overlay configs allow you to enable/disable groups of shortcuts dynamically without editing the main config.toml.").
		Section("Use Cases",
			gohelp.Item("Gaming mode", "Override window manager shortcuts while gaming"),
			gohelp.Item("Work profiles", "Different shortcuts for different projects"),
			gohelp.Item("Application sets", "Load shortcuts specific to certain apps"),
		).
		Section("How It Works",
			gohelp.Item("1. Base config", "config.toml is always loaded first"),
			gohelp.Item("2. Overlays merge", "Enabled overlays merge on top, overriding base shortcuts"),
			gohelp.Item("3. Auto-reload", "Enabled overlays are watched for changes"),
		).
		Section("Commands",
			gohelp.Item("enable gaming.toml", "Enable overlay and restart daemon", "akeyshually enable gaming.toml"),
			gohelp.Item("disable gaming.toml", "Disable overlay and restart daemon"),
			gohelp.Item("list", "Show all config files and their status"),
			gohelp.Item("clear", "Disable all overlays"),
			gohelp.Item("config gaming", "Edit gaming.toml overlay"),
		).
		Section("Settings",
			gohelp.Item("notify_on_overlay_change", "Desktop notifications when overlays change (default: false)"),
		).
		Text("Enable in config.toml:\n[settings]\nnotify_on_overlay_change = true")

	helpModifiers = gohelp.NewPage("modifiers", "triggers and modifiers syntax reference").
		Text("Triggers define when the action fires. Modifiers stack on top to change execution behavior.").
		Section("Triggers",
			gohelp.Item(".onpress", "Execute on key press (default, can be omitted)", "\"super+t\" = \"terminal\""),
			gohelp.Item(".onrelease", "Execute on key release — tap detection; cancelled by other keys or mouse clicks", "\"super.onrelease\" = \"rofi\""),
			gohelp.Item(".whileheld", "Start process on press, SIGTERM on release", "\"super+f.whileheld\" = \"$FILEMANAGER\""),
			gohelp.Item(".hold(ms)", "Trigger after held for duration; does not kill on release", "\"super+h.hold(500)\" = \"notify-send held\""),
			gohelp.Item(".doubletap(ms)", "Execute on double-tap; single keys only", "\"super.doubletap(300)\" = \"rofi -show drun\""),
			gohelp.Item(".tap(ms)whileheld(ms)", "Tap fires first command; tap-then-hold fires second", "\"super.tap(200)whileheld(500)\" = [\"rofi\", \"$FILEMANAGER\"]"),
			gohelp.Item(".pressrelease", "Different commands on press and release (requires 2-command array)", "\"mute.pressrelease\" = [\"mic-on\", \"mic-off\"]"),
		).
		Section("Modifiers",
			gohelp.Item(".repeat-whileheld(ms)", "Repeat command while key held; omit interval for default_interval", "\"super+up.repeat-whileheld(100)\" = \"volume_up\""),
			gohelp.Item(".repeat-toggle(ms)", "First press starts loop, second press stops it", "\"f1.repeat-toggle(50)\" = \"xdotool click 1\""),
			gohelp.Item(".switch", "Cycle through array of commands on each press", "\"f2.switch\" = [\"cmd1\", \"cmd2\", \"cmd3\"]"),
			gohelp.Item(".passthrough", "Match regardless of modifier state", "\"v.passthrough\" = \"copyq toggle\""),
		).
		Text("Restrictions:\n  • .doubletap and .tapwhileheld only work on single keys (no combos)\n  • .switch, .tapwhileheld, and .pressrelease require a command array")
)

// Help displays usage information or topic-specific help.
func Help(args ...string) {
	gohelp.Run(append([]string{"help"}, args...), helpRoot, helpConfig, helpOverlays, helpModifiers)
}
