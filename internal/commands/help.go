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
			gohelp.Item("default_interval", "Default interval for repeat behaviors (milliseconds, default: 150)"),
			gohelp.Item("disable_media_keys", "Forward media keys to system (default: false)"),
			gohelp.Item("shell", "Shell to use for commands (default: $SHELL, fallback: sh)"),
			gohelp.Item("env_file", "File to source before command execution (optional)"),
			gohelp.Item("notify_on_overlay_change", "Desktop notifications when overlays change (default: false)"),
			gohelp.Item("devices", "List of device name substrings to grab (case-insensitive)", "devices = [\"Huion\", \"Xbox Controller\"]"),
		).
		Text("[settings]\ndefault_interval = 150\ndisable_media_keys = false\nshell = \"/bin/bash\"\nenv_file = \"~/.profile\"\nnotify_on_overlay_change = false").
		Section("[shortcuts]",
			gohelp.Item("Key modifiers", "super, ctrl, alt, shift (lowercase, no left/right distinction)"),
			gohelp.Item("Keys", "lowercase letters, numbers, special keys (print, space, etc.)"),
			gohelp.Item("Axis inputs", "x, y, z, rx, ry, rz, abs_x, abs_y, etc. with +/- direction (see 'help axis')"),
			gohelp.Item("Syntax", "Use + to separate modifiers and key"),
			gohelp.Item("Triggers", ".onpress (default), .hold, .doubletap, .taphold, .pressrelease, .longpress, etc."),
			gohelp.Item("Modifiers", ".switch, .repeat, .passthrough"),
		).
		Text("[shortcuts]\n\"super.pressrelease\" = [\"\", \"rofi\"]  # Tap modifier key\n\"super+t\" = \"alacritty\"\n\"f9.onpress.repeat\" = \"xdotool click 1\"  # Toggle auto-clicker\n\"rx+\" = \">scrollup\"  # Axis input (drawing tablet touchstrip)").
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
			gohelp.Item(".hold(ms)", "Fire once after held for duration (no process management)", "\"super+m.hold(500)\" = \"mute\""),
			gohelp.Item(".doubletap(ms)", "Execute on confirmed double-tap; single keys only", "\"super.doubletap(270)\" = \"rofi -show drun\""),
			gohelp.Item(".pressrelease", "Different commands on press and release (2-command array)", "\"mute.pressrelease\" = [\"mic-on\", \"mic-off\"]"),
			gohelp.Item(".taphold(tap_ms, hold_ms)", "Tap once, then tap-and-hold fires command on next press", "\"super+t.taphold(200, 500)\" = \"hold-cmd\""),
			gohelp.Item(".longpress(ms)", "Fire once after threshold (one-shot)", "\"super+h.longpress(1000)\" = \"shutdown\""),
			gohelp.Item(".holdrelease(ms)", "Execute at hold threshold AND on release (2-command array)", "\"mute.holdrelease(500)\" = [\"enable-mic\", \"disable-mic\"]"),
			gohelp.Item(".taplongpress(tap_ms, long_ms)", "Tap fires first, tap-then-longpress fires second (2-command array)", "\"super+space.taplongpress\" = [\"quick\", \"long\"]"),
		).
		Section("Modifiers",
			gohelp.Item(".switch", "Cycle through array of commands on each press", "\"f2.switch\" = [\"cmd1\", \"cmd2\", \"cmd3\"]"),
			gohelp.Item(".repeat", "Loop command: with .hold (while held) or .onpress (toggle)", "\"f9.onpress.repeat\" = \"xdotool click 1\""),
			gohelp.Item(".passthrough", "Match regardless of modifier state", "\"v.passthrough\" = \"copyq toggle\""),
		).
		Text("Restrictions:\n  • .doubletap and .taphold only work on single keys (no combos)\n  • .switch, .taphold, .pressrelease, .holdrelease, and .taplongpress require command arrays")

	helpAxis = gohelp.NewPage("axis", "absolute axis and peripheral support").
		Text("akeyshually supports absolute axis (ABS) events from evdev devices - bind any peripheral with axis input.").
		Section("Axis Names",
			gohelp.Item("Standard", "x, y, z, rx, ry, rz"),
			gohelp.Item("Absolute", "abs_x, abs_y, abs_z, abs_rx, abs_ry, abs_rz"),
		).
		Section("Syntax",
			gohelp.Item("Direction suffix", "Append + or - to axis name for direction", "\"rx+\" or \"abs_y-\""),
			gohelp.Item("Commands", "Any command works, including remap scroll output", "\">scrollup\", \">scrolldown\", \">scrollleft\", \">scrollright\""),
		).
		Text("[shortcuts]\n\"rx+\" = \">scrollup\"      # Touchstrip up → scroll up\n\"rx-\" = \">scrolldown\"    # Touchstrip down → scroll down\n\"abs_y+\" = \"volume_up\"   # Axis movement → volume control").
		Section("Device Detection",
			gohelp.Item("Auto-detection", "Most devices auto-detected by capability flags"),
			gohelp.Item("Explicit grab", "Add device name substring to [settings] devices array if not detected", "devices = [\"Tablet Monitor Touch Strip\"]"),
		).
		Text("Example: Huion Kamvas Pro 13 touchstrip scrolling\n\n[settings]\ndevices = [\"Tablet Monitor Touch Strip\"]\n\n[shortcuts]\n\"rx+\" = \">scrollup\"\n\"rx-\" = \">scrolldown\"")
)

// Help displays usage information or topic-specific help.
func Help(args ...string) {
	gohelp.Run(append([]string{"help"}, args...), helpRoot, helpConfig, helpOverlays, helpModifiers, helpAxis)
}
