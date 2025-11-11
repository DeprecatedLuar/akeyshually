# akeyshually

Portable keyboard shortcut daemon for Linux - a **command launcher** that maps keyboard combos to shell commands.

## What It Does

akeyshually is a command launcher, not an action executor. It doesn't have built-in "actions" - it simply executes any shell command you assign to a keyboard shortcut. Want to mute your microphone with `ctrl+mute`? Map it to `pactl set-source-mute @DEFAULT_SOURCE@ toggle`. Want to launch a browser? Map `super+b` to `brave-browser`.

**You provide the commands, akeyshually provides the keyboard shortcuts.**

## Features

- **Universal**: Works on X11, Wayland, any WM/DE via evdev
- **Lightweight**: ~3MB binary, <5MB RAM
- **Simple**: TOML config, fire-and-forget command execution
- **Portable**: Single static binary
- **Command-focused**: No built-in actions - compose with existing CLI tools

## Quick Start

```bash
# 1. Add user to input group (required for /dev/input/* access)
sudo usermod -aG input $USER
# Logout and login for group change to take effect

# 2. Build
go build -o akeyshually ./cmd

# 3. Install binary (optional)
sudo cp akeyshually /usr/local/bin/

# 4. Run once to generate config files
akeyshually
# Auto-creates ~/.config/akeyshually/ with default configs

# 5. Customize your shortcuts
nano ~/.config/akeyshually/shortcuts.toml

# 6. Install systemd service (optional)
systemctl --user link ~/.config/akeyshually/akeyshually.service
systemctl --user enable --now akeyshually
```

## Configuration

Config files are auto-generated in `~/.config/akeyshually/` on first run:
- `config.toml` - Settings (trigger mode, media keys)
- `shortcuts.toml` - Keyboard shortcuts
- `media-keys.toml` - Optional media key bindings
- `akeyshually.service` - Systemd service file

### Basic Example

```toml
[shortcuts]
"super+b" = "brave-browser"
"super+return" = "alacritty"
"ctrl+alt+t" = "alacritty"
"print" = "maim -s | xclip -selection clipboard -t image/png"
```

### With Command References

```toml
[shortcuts]
"super+b" = "browser"
"super+shift+b" = "browser_private"

[commands]
browser = "brave-browser --user-data-dir=/home/user/.config/BraveSoftware/default"
browser_private = "brave-browser --incognito"
```

### Modifier Tap Detection

Single modifier key taps trigger actions when pressed and released without other keys. Requires `trigger_on = "release"` mode.

**config.toml:**
```toml
[settings]
trigger_on = "release"
```

**shortcuts.toml:**
```toml
[shortcuts]
"super" = "rofi -show drun"      # Tap Super key alone → rofi
"super+t" = "alacritty"          # Super+T combo still works
```

How it works:
- Press Super alone → marked as tap candidate
- Press Super+T → combo executes, tap cancelled
- Release Super without other keys → tap action executes

### Key Names

**Modifiers:** `super`, `ctrl`, `alt`, `shift`

**Letters:** `a-z`

**Numbers:** `0-9`

**Special keys:** `return`/`enter`, `space`, `tab`, `esc`/`escape`, `backspace`, `print`/`printscreen`, `delete`, `insert`, `home`, `end`, `pageup`, `pagedown`

**Arrows:** `left`, `right`, `up`, `down`

**Function keys:** `f1`-`f12`

## Systemd User Service

The service file is auto-generated at `~/.config/akeyshually/akeyshually.service` on first run.

**Before installing**, edit it to add keyboard remapper dependencies if needed (keyd, kanata, etc.).

Install:

```bash
systemctl --user link ~/.config/akeyshually/akeyshually.service
systemctl --user enable --now akeyshually
```

Check status:

```bash
systemctl --user status akeyshually
```

## Architecture

```
User presses key
    ↓
evdev (/dev/input/event*)
    ↓
Keyboard detection (EV_KEY + EV_REP capabilities)
    ↓
Event listener (goroutine per keyboard)
    ↓
Key matcher (track modifier state, match combo)
    ↓
Executor (sh -c "command", fire-and-forget)
```

## Permissions

akeyshually requires read access to `/dev/input/event*` devices. Two options:

1. **User in input group** (recommended for MVP):
   ```bash
   sudo usermod -aG input $USER
   # Logout required
   ```

2. **Root service** (future enhancement):
   - Run as systemd system service
   - More complex but doesn't require logout

## Troubleshooting

**"Permission denied" error:**
- Verify you're in the input group: `groups | grep input`
- Remember to logout and login after adding to group

**"No keyboards detected":**
- Check devices: `ls -l /dev/input/by-id/*kbd*`
- Verify evdev access: `cat /dev/input/event* | head -c 1`

**Shortcut not triggering:**
- Check config syntax (key names lowercase, `+` separated)
- Verify command works: `sh -c "your-command"`
- Check logs if running as systemd service

## Development

See `vision.md` for architecture decisions and implementation details.

## License

MIT
