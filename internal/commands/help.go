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
			gohelp.Item("default_interval", "Default interval for repeat behaviors (milliseconds)", "default_interval = 150"),
			gohelp.Item("disable_media_keys", "Forward media keys to system", "disable_media_keys = false"),
			gohelp.Item("shell", "Shell to use for commands (default: $SHELL, fallback: sh)", "shell = \"/bin/bash\""),
			gohelp.Item("env_file", "File to source before command execution", "env_file = \"~/.profile\""),
			gohelp.Item("notify_on_overlay_change", "Desktop notifications when overlays change", "notify_on_overlay_change = true"),
			gohelp.Item("devices", "List of device name substrings to grab (case-insensitive)", "devices = [\"Huion\", \"Xbox Controller\"]"),
		).
		Section("[virtual_keys]",
			gohelp.Item("Virtual keys", "Unify multiple physical keys into a single virtual key name"),
			gohelp.Item("Use case", "Hardware that alternates between sending different key codes (e.g., Bluetooth headphones)"),
			gohelp.Item("Syntax (array)", "virtualname = [\"key1\", \"key2\"]", "media = [\"playcd\", \"pausecd\"]"),
			gohelp.Item("Syntax (string)", "virtualname = \"singlekey\"", "media = \"playpause\""),
			gohelp.Item("Expansion", "Shortcuts using virtual keys expand at config load time", "\"media.hold\" = \"cmd\" → \"playcd.hold\" + \"pausecd.hold\""),
			gohelp.Item("File-scoped", "Virtual keys only expand within the config/overlay where they're defined"),
		).
		Section("[shortcuts]",
			gohelp.Item("Key modifiers", "super, ctrl, alt, shift (lowercase, no left/right distinction)"),
			gohelp.Item("Keys", "lowercase letters, numbers, function keys, navigation, etc. (see 'help keys')"),
			gohelp.Item("Axis inputs", "x, y, z, rx, ry, rz, abs_x, abs_y, etc. with +/- direction (see 'help axis')"),
			gohelp.Item("Remap output", ">key, >lclick, >scrollup - inject key/mouse/scroll events (see 'help remap')"),
			gohelp.Item("Syntax", "Use + to separate modifiers and key", "\"super+t\" = \"alacritty\""),
			gohelp.Item("Triggers", ".onpress (default), .hold, .doubletap, .taphold, .pressrelease, .longpress, etc."),
			gohelp.Item("Modifiers", ".switch, .repeat, .passthrough"),
		).
		Section("Examples",
			gohelp.Item("Tap modifier key", "Execute command on modifier release", "\"super.pressrelease\" = [\"\", \"rofi\"]"),
			gohelp.Item("Launch terminal", "Simple key combo", "\"super+t\" = \"alacritty\""),
			gohelp.Item("Toggle auto-clicker", "Remap key to mouse click, repeat on press", "\"f9.onpress.repeat\" = \">lclick\""),
			gohelp.Item("Axis scrolling", "Touchstrip or peripheral axis input", "\"rx+\" = \">scrollup\""),
			gohelp.Item("Key to mouse button", "Remap any key to mouse click", "\"f1\" = \">lclick\""),
			gohelp.Item("Virtual keys", "Unify alternating hardware keys", "[virtual_keys]\nmedia = [\"playcd\", \"pausecd\"]\n\n[shortcuts]\n\"media.hold\" = \"playerctl next\""),
		).
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
			gohelp.Item("disable gaming.toml", "Disable overlay and restart daemon", "akeyshually disable gaming.toml"),
			gohelp.Item("list", "Show all config files and their status", "akeyshually list"),
			gohelp.Item("clear", "Disable all overlays", "akeyshually clear"),
			gohelp.Item("config gaming", "Edit gaming.toml overlay", "akeyshually config gaming"),
		).
		Section("Settings",
			gohelp.Item("notify_on_overlay_change", "Desktop notifications when overlays change", "notify_on_overlay_change = true"),
		)

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
			gohelp.Item(".tappressrelease(tap_ms)", "Tap then press fires first, release fires second (2-command array)", "\"mute.tappressrelease(200)\" = [\"start\", \"stop\"]"),
			gohelp.Item(".tapholdrelease(tap_ms, hold_ms)", "Tap then hold fires first, release fires second (2-command array)", "\"f1.tapholdrelease\" = [\"hold-start\", \"hold-end\"]"),
		).
		Section("Modifiers",
			gohelp.Item(".switch", "Cycle through array of commands on each press", "\"f2.switch\" = [\"cmd1\", \"cmd2\", \"cmd3\"]"),
			gohelp.Item(".repeat", "Loop command: with .hold (while held) or .onpress (toggle)", "\"f9.onpress.repeat\" = \"xdotool click 1\""),
			gohelp.Item(".passthrough", "Match regardless of modifier state", "\"v.passthrough\" = \"copyq toggle\""),
		).
		Section("Restrictions",
			gohelp.Item("Single keys only", ".doubletap and .taphold only work on single keys (no combos)"),
			gohelp.Item("Array commands", ".switch, .pressrelease, .holdrelease, .taplongpress, .tappressrelease, and .tapholdrelease require 2+ commands"),
		)

	helpAxis = gohelp.NewPage("axis", "absolute axis and peripheral support").
		Text("akeyshually supports absolute axis (ABS) events from evdev devices - bind any peripheral with axis input.").
		Section("Axis Names",
			gohelp.Item("Standard", "x, y, z, rx, ry, rz"),
			gohelp.Item("Absolute", "abs_x, abs_y, abs_z, abs_rx, abs_ry, abs_rz"),
		).
		Section("Syntax",
			gohelp.Item("Direction suffix", "Append + or - to axis name for direction", "\"rx+\" or \"abs_y-\""),
			gohelp.Item("Scroll output", "Remap to scroll events", "\">scrollup\", \">scrolldown\", \">scrollleft\", \">scrollright\""),
			gohelp.Item("Shell command", "Any shell command works", "\"volume_up\", \"brightness-control +10\""),
		).
		Section("Examples",
			gohelp.Item("Touchstrip scroll up", "Positive direction triggers scroll up", "\"rx+\" = \">scrollup\""),
			gohelp.Item("Touchstrip scroll down", "Negative direction triggers scroll down", "\"rx-\" = \">scrolldown\""),
			gohelp.Item("Axis volume control", "Map axis movement to shell commands", "\"abs_y+\" = \"volume_up\""),
		).
		Section("Device Detection",
			gohelp.Item("Auto-detection", "Most devices auto-detected by capability flags"),
			gohelp.Item("Explicit grab", "Add device name substring to [settings] devices array", "devices = [\"Tablet Monitor Touch Strip\"]"),
		)

	helpRemap = gohelp.NewPage("remap", "key and mouse button injection").
		Text("Remap syntax (> prefix) injects keyboard keys, mouse buttons, and scroll events via evdev.").
		Section("Syntax",
			gohelp.Item(">key", "Tap key combo (press and release)", "\">return\", \">ctrl+c\", \">super+t\""),
			gohelp.Item(">>key", "Hold key forever (until << releases)", "\">>shift\""),
			gohelp.Item("<key", "Release single key", "\"<shift\""),
			gohelp.Item("<<", "Release all persistent held keys", "\"<<\""),
		).
		Section("Mouse Buttons",
			gohelp.Item("Left click", "lclick, leftclick, lbutton, leftbutton, mouse1, btn_left", "\">lclick\""),
			gohelp.Item("Right click", "rclick, rightclick, rbutton, rightbutton, mouse2, btn_right", "\">rclick\""),
			gohelp.Item("Middle click", "mclick, middleclick, mbutton, middlebutton, mouse3, btn_middle", "\">mclick\""),
			gohelp.Item("Forward/Back", "forward/mouse4, back/mouse5, btn_side, btn_extra", "\">forward\""),
		).
		Section("Scroll/Wheel",
			gohelp.Item("Vertical", "scrollup/wheelup, scrolldown/wheeldown", "\">scrollup\""),
			gohelp.Item("Horizontal", "scrollleft/wheelleft, scrollright/wheelright", "\">scrollleft\""),
		).
		Section("Examples",
			gohelp.Item("Auto-clicker", "Toggle mouse click on/off", "\"f9.onpress.repeat\" = \">lclick\""),
			gohelp.Item("Remap key to click", "F1 triggers left click", "\"f1\" = \">lclick\""),
			gohelp.Item("Key to scroll", "F2/F3 scroll up/down", "\"f2\" = \">scrollup\""),
			gohelp.Item("Sticky modifier", "Hold shift, release later", "\"f5\" = \">>shift\"\n\"f6\" = \"<<\""),
		)

	helpKeys = gohelp.NewPage("keys", "complete list of available key names").
		Text("All key names are case-insensitive. Aliases listed in parentheses.").
		Section("Modifiers",
			gohelp.Item("super", "Meta/Windows/Command key"),
			gohelp.Item("ctrl", "Control key (alias: ctl)"),
			gohelp.Item("alt", "Alt key"),
			gohelp.Item("shift", "Shift key"),
		).
		Section("Letters & Numbers",
			gohelp.Item("a-z", "All lowercase letters"),
			gohelp.Item("0-9", "Number row keys"),
		).
		Section("Function Keys",
			gohelp.Item("f1-f12", "Standard function keys"),
			gohelp.Item("f13-f24", "Extended function keys"),
		).
		Section("Navigation",
			gohelp.Item("left, right, up, down", "Arrow keys"),
			gohelp.Item("home, end", "Home and End keys"),
			gohelp.Item("pageup, pagedown", "Page navigation"),
		).
		Section("Editing",
			gohelp.Item("return", "Enter/Return key (alias: enter)"),
			gohelp.Item("space", "Space bar"),
			gohelp.Item("tab", "Tab key"),
			gohelp.Item("backspace", "Backspace key"),
			gohelp.Item("delete", "Delete key"),
			gohelp.Item("insert", "Insert key"),
			gohelp.Item("esc", "Escape key (alias: escape)"),
		).
		Section("Punctuation",
			gohelp.Item("semicolon", "Semicolon key (alias: ;)"),
		).
		Section("Media Keys",
			gohelp.Item("volumeup, volumedown", "Volume control"),
			gohelp.Item("mute", "Mute toggle"),
			gohelp.Item("brightnessup, brightnessdown", "Screen brightness"),
			gohelp.Item("playpause", "Play/pause media (alias: play)"),
			gohelp.Item("nextsong", "Next track (alias: next)"),
			gohelp.Item("previoussong", "Previous track (alias: previous)"),
			gohelp.Item("playcd, pausecd, stopcd", "CD player controls"),
			gohelp.Item("ejectcd", "Eject CD (aliases: closecd, ejectclosecd)"),
			gohelp.Item("calc", "Calculator key (alias: calculator)"),
		).
		Section("Numpad",
			gohelp.Item("kp0-kp9", "Numpad number keys"),
			gohelp.Item("kpplus, kpminus", "Numpad plus and minus"),
			gohelp.Item("kpasterisk, kpslash", "Numpad multiply and divide"),
			gohelp.Item("kpenter", "Numpad Enter"),
			gohelp.Item("kpdot", "Numpad decimal point"),
		).
		Section("Special Keys",
			gohelp.Item("print", "Print Screen (alias: printscreen)"),
			gohelp.Item("ro", "RO key (Japanese keyboard)"),
			gohelp.Item("102nd", "102nd key (extra key on some keyboards)"),
		).
		Section("Mouse Buttons",
			gohelp.Item("btn_left", "Left mouse button (aliases: lclick, leftclick, lbutton, leftbutton, mouse1)"),
			gohelp.Item("btn_right", "Right mouse button (aliases: rclick, rightclick, rbutton, rightbutton, mouse2)"),
			gohelp.Item("btn_middle", "Middle mouse button (aliases: mclick, middleclick, mbutton, middlebutton, mouse3)"),
			gohelp.Item("btn_forward", "Forward mouse button (aliases: forward, mouse4)"),
			gohelp.Item("btn_back", "Back mouse button (aliases: back, mouse5)"),
			gohelp.Item("btn_side, btn_extra", "Additional mouse buttons"),
		).
		Section("Gamepad Buttons (Generic)",
			gohelp.Item("btn_south, btn_north", "Face buttons (bottom/top)"),
			gohelp.Item("btn_east, btn_west", "Face buttons (right/left)"),
			gohelp.Item("btn_tl, btn_tr", "Top shoulder buttons (left/right)"),
			gohelp.Item("btn_tl2, btn_tr2", "Bottom shoulder buttons/triggers (left/right)"),
			gohelp.Item("btn_start, btn_select", "Start and Select buttons"),
			gohelp.Item("btn_mode", "Mode/Guide/Home button"),
			gohelp.Item("btn_thumbl, btn_thumbr", "Analog stick click buttons (left/right)"),
		).
		Section("Gamepad Buttons (Xbox Layout)",
			gohelp.Item("gp_a, gp_b, gp_x, gp_y", "Face buttons (A/B/X/Y)"),
			gohelp.Item("gp_lb, gp_rb", "Bumpers (left/right)"),
			gohelp.Item("gp_lt, gp_rt", "Triggers (left/right)"),
			gohelp.Item("gp_ls, gp_rs", "Stick buttons (left/right)"),
			gohelp.Item("gp_start, gp_select, gp_guide", "System buttons"),
		).
		Section("Gamepad Buttons (PlayStation Layout)",
			gohelp.Item("gp_cross, gp_circle", "Face buttons (Cross/Circle)"),
			gohelp.Item("gp_square, gp_triangle", "Face buttons (Square/Triangle)"),
			gohelp.Item("gp_l1, gp_r1", "Shoulder buttons (L1/R1)"),
			gohelp.Item("gp_l2, gp_r2", "Triggers (L2/R2)"),
			gohelp.Item("gp_l3, gp_r3", "Stick buttons (L3/R3)"),
		).
		Section("Tablet/Pen Buttons",
			gohelp.Item("btn_0-btn_9", "Generic button codes (tablets, peripherals)"),
			gohelp.Item("btn_tool_pen", "Pen tool indicator"),
			gohelp.Item("btn_touch", "Touch contact"),
			gohelp.Item("btn_stylus, btn_stylus2", "Stylus buttons (primary/secondary)"),
		).
		Section("Scroll/Wheel (Remap Output Only)",
			gohelp.Item("scrollup", "Scroll up (aliases: wheelup)"),
			gohelp.Item("scrolldown", "Scroll down (aliases: wheeldown)"),
			gohelp.Item("scrollleft", "Scroll left (aliases: wheelleft)"),
			gohelp.Item("scrollright", "Scroll right (aliases: wheelright)"),
		)
)

// Help displays usage information or topic-specific help.
func Help(args ...string) {
	gohelp.Run(append([]string{"help"}, args...), helpRoot, helpConfig, helpOverlays, helpModifiers, helpAxis, helpRemap, helpKeys)
}
