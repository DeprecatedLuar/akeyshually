# Media Keys Issue

## Problem

Brightness keys (and potentially other media keys) don't work with modifier combinations like `ctrl+brightnessdown`.

## Root Cause

Brightness keys are NOT on the keyboard device. They come from a separate input device:
- **Device:** `/dev/input/event8` - "Video Bus"
- **Type:** ACPI/platform virtual device created by laptop firmware
- **Keys exposed:** `KEY_BRIGHTNESSDOWN` (224), `KEY_BRIGHTNESSUP` (225)

### Current Behavior

When pressing `Ctrl+BrightnessDown`:
1. `Ctrl` key event → comes from keyboard (event3 / kanata virtual)
2. `BrightnessDown` event → comes from Video Bus (event8)
3. akeyshually **only listens to keyboards**, so it never sees brightness keys
4. Even if we listen to Video Bus, modifier state is isolated per device

### Verification

```bash
# Test which device has brightness keys
sudo evtest /dev/input/event8

# Output shows:
# Event: type 1 (EV_KEY), code 224 (KEY_BRIGHTNESSDOWN), value 1
```

## Solution Required

### Phase 1: Multi-device listening
1. Detect devices with media key capabilities (Video Bus, Dell WMI hotkeys, etc.)
2. Add them to the listen list alongside keyboards
3. Grab + clone them like keyboards

### Phase 2: Shared modifier state
**Critical:** Modifiers and media keys come from different devices.

Current architecture:
```go
// Each keyboard gets its own matcher instance (isolated state)
for _, kbd := range keyboards {
    matcher := NewMatcher()  // Independent ModifierState
    go Listen(kbd, matcher)
}
```

Required architecture:
```go
// Shared modifier state across ALL devices
sharedState := NewSharedModifierState()

for _, device := range allInputDevices {
    matcher := NewMatcher(sharedState)  // Same state reference
    go Listen(device, matcher)
}
```

**Behavior:**
- Keyboard updates: `Ctrl` pressed → updates shared state
- Video Bus checks: `BrightnessDown` pressed → reads shared state → matches `"ctrl+brightnessdown"`

### Implementation Checklist

- [ ] Add device detection for non-keyboard media sources
  - [ ] Video Bus (brightness)
  - [ ] Dell WMI hotkeys
  - [ ] Intel HID button array
  - [ ] Platform-specific devices

- [ ] Refactor modifier state to be shared
  - [ ] Extract `ModifierState` from per-matcher to global
  - [ ] Thread-safe shared state (already using sync.RWMutex in TapState)
  - [ ] All matchers reference same state instance

- [ ] Update keyboard_listener.go
  - [ ] `FindMediaDevices()` - detect Video Bus and similar
  - [ ] Listen to media devices alongside keyboards
  - [ ] Forward media key events when not matched

- [ ] Handle edge cases
  - [ ] What if kanata starts forwarding brightness keys later?
  - [ ] Device hotplug (USB keyboards, docks)
  - [ ] Multiple Video Bus devices

### Alternative: Workaround via kanata

Map brightness keys to unused F-keys in kanata config:
```lisp
(defsrc
  brightnessdown brightnessup
)

(deflayer base
  f20 f21  ;; Map to akeyshually-friendly keys
)
```

**Pros:** Simple, no akeyshuality changes
**Cons:** Requires kanata config changes, less portable

## Notes

- TapState already uses shared state pattern (for mouse tap cancellation)
- LoopState is shared across handlers
- ModifierState is the only isolated state that needs refactoring
- This affects the architecture documented in CLAUDE.md section 4 (shortcut_matcher.go)

## Related Files

- `internal/keyboard_listener.go` - Device detection and listening
- `internal/shortcut_matcher.go` - ModifierState (currently per-matcher)
- `internal/event_handler.go` - CreateUnifiedHandler (uses matcher)
- `cmd/main.go` - startDaemon (creates matchers per keyboard)
