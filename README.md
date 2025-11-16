<div align="center">
  <img src="other/assets/erm.webp" alt="akeyshually banner" width="400">
</div>

<p align="center">Errm... Akeyshually, this is an evdev-based userspace daemon configured in TOML that intercepts raw input events, performs stateful modifier tracking, and executes arbitrary shell commands through a fire-and-forget subprocess model regardless of session type or graphical environment manager</p>

<p align="center">
  <a href="https://github.com/DeprecatedLuar/akeyshually/stargazers">
    <img src="https://img.shields.io/github/stars/DeprecatedLuar/akeyshually?style=for-the-badge&logo=github&color=1f6feb&logoColor=white&labelColor=black"/>
  </a>
  <a href="https://github.com/DeprecatedLuar/akeyshually/blob/main/LICENSE">
    <img src="https://img.shields.io/github/license/DeprecatedLuar/akeyshually?style=for-the-badge&color=green&labelColor=black"/>
  </a>
</p>


---

Every shortcuts manager is coupled to your display server (X11 vs Wayland ), your window manager (sway vs Hyprland vs i3), and your current machine.

I made akeyshually to not only have my configs in a single git tracked file, but to work anywhere I want.

---

## The cool features you've never seen before

<img src="other/assets/ermactually.jpeg" alt="Actually..." align="right" width="200"/>

- Works on X11, Wayland, literally any WM/DE via evdev
- All settings declared in a single TOML config
- **Actually lightweight** takes about ~3MB binary, <3MB RAM, 0% CPU when idle
- Configs are hot-reloaded on edit
- Special modes like `.whileheld`, `.toggle`, `.switch`, `.onrelease`
- You can literally make an auto-clicker with a single line
- Works alongside remappers (keyd, kanata, kmonad, xremap...)

---

## Installation

```bash
curl -sSL https://raw.githubusercontent.com/DeprecatedLuar/akeyshually/main/install.sh | bash
```

<details>
<summary>Other Install Methods</summary>

<br>

**Manual Install**
```bash
# Build from source
git clone https://github.com/DeprecatedLuar/akeyshually.git
cd akeyshually
go build -ldflags="-s -w" -o akeyshually ./cmd

# Install to ~/.local/bin
./other/install-local.sh

# Or install system-wide
sudo cp akeyshually /usr/local/bin/
```

**Prerequisites:**
- Go 1.21+ (for building)
- User must be in `input` group:
  ```bash
  sudo usermod -aG input $USER
  # Logout and login for group change to take effect
  ```

</details>

<br>

First run auto-generates config files in `~/.config/akeyshually/`. Just run `akeyshually` and you're good.

---

<img src="other/assets/lovecowboy.webp" alt="Actually..." align="left" width="200"/>

## Commands

| Command | Description                                      |
|---------|--------------------------------------------------|
| start   | Daemonize in background                          |
| stop    | Stop daemon (via pidfile or systemctl)           |
| update  | Check for and install updates                    |
| version | Show version                                     |
| --help  | Show help                                        |

> [!NOTE]
> `akeyshually` with no args runs in foreground

---

## Configuration

Config lives at `~/.config/akeyshually/`:
- `config.toml` - All-in-one config (settings, shortcuts, command aliases)
- `akeyshually.service` - Systemd service file (with install instructions)

<details>
<summary>Configuration Example</summary>

<br>

```toml
# ~/.config/akeyshually/config.toml

[settings]
default_loop_interval = 100  # Milliseconds for .whileheld/.toggle behaviors
disable_media_keys = false   # Forward media keys to system (GNOME/KDE daemons)
#shell = "/bin/bash"         # Optional: override $SHELL
#env_file = "~/.profile"     # Optional: source before commands

[shortcuts]
"super+k" = "edit_config"

#-[LAUNCHERS]--------------------------------

"super.onrelease" = "rofi"  # Modifier tap (executes on release if pressed alone)
"super+b" = "browser"
"super+shift+b" = "browser2"
"super+return" = "kitty"
"super+f" = "dolphin"
"super+x" = "xkill"
"super+e" = "email"
"super+v" = "copyq toggle"
"super+w" = "whatsapp"
"shift+super+n" = "notetaker"

#-[UTILS]------------------------------------

"print" = "prtscr"
"super+p" = "prtscr"
"shift+print" = "/home/user/Workspace/tools/bin/screenshot-save"
"ctrl+print" = "bash -c \"xdg-open ~/Media/Pictures/temp.png\""
"ctrl+mute" = "mute_mic"

# Advanced behaviors
"f9.whileheld(50)" = "xdotool click 1"  # Auto-clicker: clicks every 50ms while held
"f10.toggle" = "xdotool click 1"        # Toggle: starts/stops loop on each press
"super+tab.switch" = ["window1", "window2", "window3"]  # Cycle through commands

# Media keys - uncomment to enable
#"volumeup" = "volume_up"
#"volumedown" = "volume_down"
#"mute" = "mute_toggle"

[command_variables]
edit_config = "kitty micro ~/.config/akeyshually/config.toml"

browser = "brave-browser --user-data-dir=/home/user/.config/BraveSoftware/1"
browser2 = "brave-browser --user-data-dir=/home/user/.config/BraveSoftware/2"
rofi = "~/.config/rofi/scripts/launcher_t7"
email = "flatpak run org.mozilla.Thunderbird"
mute_mic = "pactl set-source-mute @DEFAULT_SOURCE@ toggle"
whatsapp = "flatpak run com.rtosta.zapzap"
notetaker = "bash -c \"source ~/.bashrc && /home/user/.config/bash/bin/notetaker/notetaker\""
prtscr = "/home/user/Workspace/tools/bin/screenshot-save --temp"

# Media key commands (uncomment shortcuts above to use)
volume_up = "pactl set-sink-volume @DEFAULT_SINK@ +5%"
volume_down = "pactl set-sink-volume @DEFAULT_SINK@ -5%"
mute_toggle = "pactl set-sink-mute @DEFAULT_SINK@ toggle"
```

</details>

<details>
<summary>Shortcut Behaviors</summary>

<br>

**Normal (default):**
```toml
"super+t" = "kitty"  # Executes on key press
```

**While Held (repeat while key is held):**
```toml
"f9.whileheld" = "xdotool click 1"       # Uses default_loop_interval
"f9.whileheld(0.05)" = "xdotool click 1"   # Custom interval (50ms)
"f9.whileheld(0.015)" = "xdotool click"  # Sub-second intervals (15ms)
"f9.loop(50)" = "xdotool click 1"        # Alias for .whileheld
```

**Toggle (start/stop on each press):**
```toml
"f10.toggle" = "xdotool click 1"       # Loop continues after release
"f10.toggle(0.1)" = "xdotool click 1"  # Custom interval in seconds
```

**Switch (cycle through commands):**
```toml
"super+tab.switch" = ["cmd1", "cmd2", "cmd3"]  # Cycles on each press
```

**Release timing:**
```toml
"super.onrelease" = "rofi"  # Modifier tap: executes on release if pressed alone
"super+t.onrelease" = "cmd" # Executes when keys are released
```

</details>

<details>
<summary>Available Key Names</summary>

<br>

**Modifiers:** `super`, `ctrl`, `alt`, `shift`

**Letters:** `a-z`

**Numbers:** `0-9`

**Special keys:** `return`/`enter`, `space`, `tab`, `esc`/`escape`, `backspace`, `print`/`printscreen`, `delete`, `insert`, `home`, `end`, `pageup`, `pagedown`

**Arrows:** `left`, `right`, `up`, `down`

**Function keys:** `f1-f12`

**Media keys:** Enabled via `enable_media_keys = true` in config.toml (see `media-keys.toml` for defaults)

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
