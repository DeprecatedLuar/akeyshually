<div align="center">
  <img src="other/assets/erm.webp" alt="akeyshually banner" width="400">

<div align="center">

[Install](#installation) • [Commands](#commands) • [Configuration](#configuration) • [Behaviors](#behaviors) • [Key Names](#key-names)

</div>

</div>

<div align="center">

Errm... Akeyshually, this is NOT a remapper but an evdev-based userspace daemon configured in TOML that intercepts raw input events, performs stateful modifier tracking, and executes arbitrary shell commands through a fire-and-forget subprocess model regardless of session type or graphical environment manager

</div>

<div align="center">
  <a href="https://github.com/DeprecatedLuar/akeyshually/stargazers">
    <img src="https://img.shields.io/github/stars/DeprecatedLuar/akeyshually?style=for-the-badge&logo=github&color=1f6feb&logoColor=white&labelColor=black"/>
  </a>
  <a href="https://github.com/DeprecatedLuar/akeyshually/blob/main/LICENSE">
    <img src="https://img.shields.io/github/license/DeprecatedLuar/akeyshually?style=for-the-badge&color=green&labelColor=black"/>
  </a>
  <a href="https://github.com/DeprecatedLuar/akeyshually/releases">
    <img src="https://img.shields.io/github/v/release/DeprecatedLuar/akeyshually?style=for-the-badge&color=orange&labelColor=black"/>
  </a>
</div>

---

Every shortcuts manager is coupled to your display server (X11 vs Wayland), your window manager (sway vs Hyprland vs i3), and your current machine.

I made akeyshually to not only have my configs in a single git tracked file, but to work anywhere I want.

---

# Readme will be fully updated once I get some time, some sections were provisionally made with AI based on latest commits

## The cool features you've never seen before

<img src="other/assets/ermactually.jpeg" alt="Actually..." align="right" width="200"/>

- Works on X11, Wayland, literally any WM/DE via evdev
- All settings declared in a single (or not) TOML config
- **Actually lightweight** takes about ~3MB binary, <3MB RAM, 0% CPU when idle
- You can write literal drivers on steroids on a 10 line file.
- If a hardware has buttons akeyshually can bend them to your will
- Special modes like `.whileheld`, `.repeat-whileheld`, `.repeat-toggle`, `.switch`, `.doubletap`, `.onrelease`
- You can literally make an auto-clicker with a single line
- Works alongside remappers (keyd, kanata, kmonad, xremap...)
- It's illegal on 7+ countries and counting

---

## Installation

![Linux](https://img.shields.io/badge/Linux-FCC624?style=flat-square&logo=linux&logoColor=black) ![Go](https://img.shields.io/badge/Go-00ADD8?style=flat-square&logo=go&logoColor=white)

### Universal
```bash
curl -sSL https://raw.githubusercontent.com/DeprecatedLuar/the-satellite/main/satellite.sh | bash -s -- install DeprecatedLuar/akeyshually
```

### Go
```bash
go install github.com/DeprecatedLuar/akeyshually/cmd/akeyshually@latest
```

Make sure `$GOPATH/bin` (usually `~/go/bin`) is in your `PATH`.

<details>
<summary>Other Install Methods</summary>

<br>

**Manual Install**
1. Download binary for your OS from [releases](https://github.com/DeprecatedLuar/akeyshually/releases)
2. Make executable: `chmod +x akeyshually`
3. Move to PATH: `mv akeyshually ~/.local/bin/`

---

**From Source** (for try-harders)
```bash
git clone https://github.com/DeprecatedLuar/akeyshually.git
cd akeyshually
go build -ldflags="-s -w" -o akeyshually ./cmd/akeyshually
mv akeyshually ~/.local/bin/
```

---

>[!NOTE]
> User must be in `input` group: `sudo usermod -aG input $USER` (logout required)

</details>

First run auto-generates config files in `~/.config/akeyshually/`. Just run `akeyshually` and you're good.

---

<img src="other/assets/lovecowboy.webp" alt="Actually..." align="left" width="200"/>

## Commands

| Command | Description | Example |
|:--------|:------------|:--------|
| _(none)_ | Run in foreground | `akeyshually` |
| `start` | Daemonize in background | `akeyshually start` |
| `stop` | Stop daemon (pidfile or systemctl) | `akeyshually stop` |
| `restart` | Restart daemon | `akeyshually restart` |
| `enable FILE` | Enable a config overlay | `akeyshually enable gaming` |
| `disable FILE` | Disable a config overlay | `akeyshually disable gaming` |
| `list` | List all configs and overlay status | `akeyshually list` |
| `clear` | Disable all active overlays | `akeyshually clear` |
| `config [FILE]` | Edit a config file in `$EDITOR` | `akeyshually config` |
| `update` | Check for and install updates | `akeyshually update` |
| `version` | Show version | `akeyshually version` |
| `--help` | Show help | `akeyshually --help` |

---

## Configuration

Config lives at `~/.config/akeyshually/`:
- `config.toml` - All-in-one config (settings, shortcuts, command aliases)
- `akeyshually.service` - Systemd service file (with install instructions)

<h3 id="behaviors">Behaviors</h3>

**Triggers** — when the action fires (time-gated, race each other):

| Trigger | Syntax | Description |
|:--------|:-------|:------------|
| *(default)* / `.onpress` | `"key"` | Executes on key press |
| `.doubletap` / `.doubletap(ms)` | `"key.doubletap(200)"` | Executes on confirmed double-tap |
| `.hold` / `.hold(ms)` | `"key.hold(500)"` | Fire once after hold threshold (1 command) |
| `.pressrelease` | `"key.pressrelease" = ["cmd", "release_cmd"]` | Execute on press and release (either can be `""`) |
| `.taphold` / `.taphold(ms)` | `"key.taphold(200)"` | Tap once, then tap-and-hold on next press |
| `.longpress` / `.longpress(ms)` | `"key.longpress(500)"` | Fire once after threshold (one-shot, exits immediately) |
| `.holdrelease` / `.holdrelease(ms)` | `"key.holdrelease(500)" = ["hold_cmd", "release_cmd"]` | Execute at hold threshold and on release |

**Modifiers** — stack on top of triggers (never create timer ambiguity):

| Modifier | Syntax | Description |
|:---------|:-------|:------------|
| `.switch` | `"key.switch" = ["cmd1", "cmd2"]` | Cycles through a command array |
| `.repeat` | `"key.hold.repeat"` | Loops command while held |
| `.passthrough` | `"key.passthrough"` | Ignores modifiers when matching |

### Config Overlays

Basically you can overlay your main config with another config that overrides all conflicts so every file is modular and stack on each other.

```bash
akeyshually enable gaming    # activate overlay
akeyshually disable gaming   # deactivate
akeyshually list                  # see what's active
akeyshually clear                 # disable all overlays
```

An overlay file has the same format as `config.toml` — any shortcuts or command variables it defines override the base config ones. The `devices` setting is the exception: overlays **append** to the base device list rather than replacing it.

```toml
# ~/.config/akeyshually/gaming.toml

[settings]
devices = ["Xbox Controller"]

[shortcuts]
"super+f" = "steam"
```

Set `notify_on_overlay_change = true` in `[settings]` to get a desktop notification when overlays are toggled.

---

### This is what my personal config looks like so you can have an idea how I do it

```toml
[settings]
default_interval = 150
disable_media_keys = false  # Set to true to let system handle media keys (GNOME/KDE/etc.)
env_file = "~/.profile"

[shortcuts]
"super+k" = "edit_config"
"ctrl+shift+k" = "kill_switch"

#-[LAUNCHERS]--------------------------------

"super.pressrelease" = ["", "hotline"]
"super.doubletap(270)" = "$LAUNCHER"

"super+enter" = "$TERMINAL"
"super+b" = "$BROWSER"
"super+shift+b" = "brave"
"super+e" = "thunderbird"
"super+w" = "whatsapp"
"super+f" = "$FILEMANAGER"
"super+v" = "copyq toggle"
"shift+super+n" = "notetaker"

#-[WINDOW MANAGER]---------------------------

"super+x" = "kill_window"

#-[UTILS]------------------------------------

"print" = "grimblast -f -n copysave area ~/Media/Pictures/screenshots/latest.png"
"print.doubletap" = "last_screenshot"
"super+shift+p" = "last_screenshot"
"super+ctrl+p" = "grimblast -f -n copysave area"
"shift+print" = "grimblast -f -n -o save area"
"super+p" = "grimblast copy area"

"super+y" = "yap toggle & sleep 3 && tcpeek reconnect"

#-[MEDIA KEYS]-------------------------------

"volumeup" = "volume_up"
"volumedown" = "volume_down"
"mute" = "mute_toggle"
"brightnessup" = "brightness_up"
"brightnessdown" = "brightness_down"
"play" = "media_play_pause"
"nextsong" = "media_next"
"previoussong" = "media_previous"

"ctrl+mute" = "mute_mic"
"ctrl+volumeup" = "mic_up"
"ctrl+volumedown" = "mic_down"

[command_variables]#--------------------------

edit_config = "kitty micro ~/.config/akeyshually/config.toml"
kill_switch = "akeyshually stop && pkill -9 akeyshually"

#-[LAUNCHERS]--------------------------------

thunderbird = "thunderbird"
whatsapp = "flatpak run com.rtosta.zapzap"
notetaker = "bash -c \"source ~/.bashrc && notetaker\""

#-[UTILS]------------------------------------

mute_mic = "pactl set-source-mute @DEFAULT_SOURCE@ toggle || wpctl set-mute @DEFAULT_AUDIO_SOURCE@ toggle"
last_screenshot = "$IMAGE_VIEWER ~/Media/Pictures/screenshots/latest.png"

#-[MEDIA COMMANDS]---------------------------

volume_up = "wpctl set-volume -l 1.5 @DEFAULT_AUDIO_SINK@ 5%+"
volume_down = "wpctl set-volume @DEFAULT_AUDIO_SINK@ 5%-"
mute_toggle = "wpctl set-mute @DEFAULT_AUDIO_SINK@ toggle"
brightness_up = "sunset +5"
brightness_down = "sunset -5"
media_play_pause = "playerctl play-pause"
media_next = "playerctl next"
media_previous = "playerctl previous"

mic_up = "wpctl set-volume @DEFAULT_AUDIO_SOURCE@ 5%+"
mic_down = "wpctl set-volume @DEFAULT_AUDIO_SOURCE@ 5%-"
```

<details>
<summary>Shortcut Behaviors</summary>

<br>

**Normal (default):**
```toml
"super+t" = "kitty"  # Executes on key press
```

**Hold (fire once after threshold):**
```toml
"super+m.hold" = "mute"                 # Fire once after default threshold
"super+m.hold(500)" = "mute"            # Fire once after 500ms (no process management)
```

**Repeat while held:**
```toml
"f9.hold.repeat" = "xdotool click 1"         # Uses default_interval
"f9.hold.repeat(50)" = "xdotool click 1"     # Custom interval (50ms)
"f9.onpress.repeat" = "xdotool click 1"      # Toggle: start/stop on each press
```

**Switch (cycle through commands):**
```toml
"super+tab.switch" = ["cmd1", "cmd2", "cmd3"]  # Cycles on each press
```

**Double-tap (execute on quick double-tap):**
```toml
"super.doubletap(200)" = "$LAUNCHER"      # Double-tap within 200ms
"print.doubletap(300)" = "screen-record"  # Works on any single key
```

**Press/Release (dual commands):**
```toml
"super.pressrelease" = ["", "rofi"]            # Release only (modifier tap)
"super+m.pressrelease" = ["mic-on", "mic-off"] # Both press and release
```

**Tap-then-hold:**
```toml
"super+t.taphold" = "hold-cmd"                     # Tap once, then tap-and-hold
"super+t.taphold(200)" = "hold-cmd"                # Custom tap window (200ms)
"super+t.taphold(200, 500)" = "hold-cmd"           # Custom tap + hold thresholds
```

**Long press:**
```toml
"super+h.longpress(1000)" = "shutdown"  # Fire once after 1000ms (one-shot)
```

</details>

<details>
<summary id="key-names">Available Key Names</summary>

<br>

**Modifiers:**

| Modifier | Config Name | Notes |
|:---------|:------------|:------|
| Super / Win / Meta | `super` | L/R variants treated identically; supports lone tap via `.onrelease` |
| Control | `ctrl` | L/R variants treated identically |
| Alt | `alt` | L/R variants treated identically |
| Shift | `shift` | L/R variants treated identically |

**Letters:** `a-z`

**Numbers:** `0-9`

**Special keys:** `return`/`enter`, `space`, `tab`, `esc`/`escape`, `backspace`, `delete`, `insert`, `home`, `end`, `pageup`, `pagedown`, `semicolon`/`;`

**Arrows:** `left`, `right`, `up`, `down`

**Function keys:** `f1`-`f24`

**Print screen:** `print`/`printscreen`

**Numpad:** `kp0`-`kp9`, `kpplus`, `kpminus`, `kpasterisk`, `kpslash`, `kpenter`, `kpdot`

**Media keys:** `volumeup`, `volumedown`, `mute`, `brightnessup`, `brightnessdown`, `playpause`/`play`, `nextsong`/`next`, `previoussong`/`previous`, `calc`/`calculator`

**Gamepad:** `btn_south`, `btn_north`, `btn_east`, `btn_west`, `btn_tl`, `btn_tr`, `btn_tl2`, `btn_tr2`, `btn_start`, `btn_select`, `btn_mode`, `btn_thumbl`, `btn_thumbr`

**Tablet/generic:** `btn_0`-`btn_9`, `btn_tool_pen`, `btn_touch`, `btn_stylus`, `btn_stylus2`

**Other:** `102nd`, `ro`

</details>

---

<details>
<summary>Troubleshooting</summary>

<br>

**"Permission denied" error:**
```bash
groups | grep input  # Verify you're in input group
# If not there:
sudo usermod -aG input $USER
# Then logout and login
```

**"No keyboards detected":**
```bash
ls -l /dev/input/by-id/*kbd*  # Check devices exist
cat /dev/input/event* | head -c 1  # Test evdev access
```

**Shortcut not triggering:**
- Keys must be lowercase in config (`super+t` not `Super+T`)
- Verify command works: `sh -c "your-command"`
- Check logs if running as systemd service: `journalctl --user -u akeyshually`

**Enable debug logging:**
```bash
LOGGING=1 akeyshually
```

</details>

<div align="center">
  <img src="other/assets/nerdmoji.jpeg" alt="akeyshually banner" width="400">
</div>

---

<p align="center">
  <a href="https://github.com/DeprecatedLuar/akeyshually/issues">
    <img src="https://img.shields.io/badge/Found%20a%20bug%3F-Report%20it!-red?style=for-the-badge&logo=github&logoColor=white&labelColor=black"/>
  </a>
</p>
