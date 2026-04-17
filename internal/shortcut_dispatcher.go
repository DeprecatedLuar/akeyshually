package internal

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/deprecatedluar/akeyshually/internal/config"
	"github.com/deprecatedluar/akeyshually/internal/matcher"
	evdev "github.com/holoplot/go-evdev"
)

const (
	twhIdle       = 0 // no gesture in progress
	twhFirstPress = 1 // first press down, waiting for release
	twhPriming    = 2 // first tap done, timer1 running
	twhArmed      = 3 // second press down, timer2 running
	twhActive     = 4 // whileheld process running
)

type LoopState struct {
	mu            sync.Mutex
	active        map[string]context.CancelFunc // repeat loops: combo -> cancel
	heldProcesses map[string]*exec.Cmd          // whileheld: combo -> process
	heldKeys      map[string][]uint16           // whileheld remap: combo -> key codes held down
	holdTimers    map[string]context.CancelFunc // hold: combo -> cancel timer
	tapHoldTimers map[string]context.CancelFunc         // tap-vs-hold: combo -> cancel hold timer
	tapHoldNormal map[string]*config.ParsedShortcut     // tap-vs-hold: combo -> normal shortcut to fire on tap
	twhPhase      map[string]int                        // tapwhileheld: combo -> phase
	twhCancel     map[string]context.CancelFunc         // tapwhileheld: combo -> active timer cancel
}

func runTickerLoop(ctx context.Context, interval float64, fn func()) {
	ticker := time.NewTicker(time.Duration(interval) * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fn()
		}
	}
}

func NewLoopState() *LoopState {
	return &LoopState{
		active:        make(map[string]context.CancelFunc),
		heldProcesses: make(map[string]*exec.Cmd),
		heldKeys:      make(map[string][]uint16),
		holdTimers:    make(map[string]context.CancelFunc),
		tapHoldTimers: make(map[string]context.CancelFunc),
		tapHoldNormal: make(map[string]*config.ParsedShortcut),
		twhPhase:      make(map[string]int),
		twhCancel:     make(map[string]context.CancelFunc),
	}
}

func CreateUnifiedHandler(m *matcher.Matcher, cfg *config.Config, loopState *LoopState, virtual *evdev.InputDevice) KeyHandler {
	bufferedKeys := make(map[uint16]bool)

	return func(code uint16, value int32) bool {
		// Forward media keys if disabled (let system handle them)
		if cfg.Settings.DisableMediaKeys && IsMediaKey(code) {
			return false // Forward to system
		}

		// Handle modifiers for tap detection
		if matcher.IsModifierKey(code) {
			if value == 1 {
				LogKey(matcher.GetKeyName(code), code)
				m.UpdateModifierState(code, true)
				// Check if pressed alone
				modifiers := m.GetCurrentModifiers()
				isAlone := checkModifierAlone(modifiers)
				if isAlone {
					m.MarkTapCandidate(code)
				}
			} else if value == 0 {
				// Modifier released

				// Check for double-tap (highest priority)
				if m.HasDoubleTapShortcut(code) {
					doubleTapState := m.GetDoubleTapState()
					if doubleTapState != nil {
						// Check if this is a second tap
						if doubleTapState.CheckSecondTap(code) {
							// Second tap within window - execute doubletap, cancel timer
							doubleTapState.CancelTimer()

							if shortcut := m.GetDoubleTapShortcut(code); shortcut != nil {
								resolvedCmd := cfg.ResolveCommand(shortcut.Commands[0])
								LogMatch(matcher.GetKeyName(code)+".doubletap", fmt.Sprintf("%d", code))
								LogTrigger(resolvedCmd)
								Execute(resolvedCmd, cfg)
							}
							m.UpdateModifierState(code, false)
							return false // Forward modifier
						} else {
							// First tap - start timer
							shortcut := m.GetDoubleTapShortcut(code)
							interval := shortcut.Interval
							if interval == 0 {
								interval = cfg.Settings.DefaultInterval
							}

							doubleTapState.StartTimer(code, interval, func() {
								// Timeout - check if .onrelease exists and execute
								if command, hasTap := m.CheckTap(code); hasTap {
									resolvedCmd := cfg.ResolveCommand(command)
									LogMatch(matcher.GetKeyName(code)+".tap", fmt.Sprintf("%d", code))
									LogTrigger(resolvedCmd)
									Execute(resolvedCmd, cfg)
								}
							})
							m.UpdateModifierState(code, false)
							return false // Forward modifier
						}
					}
				}

				// Check regular tap (only if no doubletap)
				if command, matched := m.CheckTap(code); matched {
					resolvedCmd := cfg.ResolveCommand(command)
					LogMatch(matcher.GetKeyName(code)+".tap", fmt.Sprintf("%d", code))
					LogTrigger(resolvedCmd)
					Execute(resolvedCmd, cfg)
				}
				m.UpdateModifierState(code, false)
			}
			return false // Forward modifiers
		}

		// KEY PRESS (value == 1)
		if value == 1 {
			LogKey(matcher.GetKeyName(code), code)
			m.ClearTapCandidate() // Any non-modifier clears tap

			// Compute combo early for tapwhileheld check (takes priority over doubletap)
			combo := m.GetCurrentCombo(code)

			// Handle tapwhileheld state machine
			if twh := m.CheckShortcut(combo, config.BehaviorTapWhileHeld, config.TimingPress); twh != nil {
				loopState.mu.Lock()
				phase := loopState.twhPhase[combo]
				loopState.mu.Unlock()

				if phase == twhPriming {
					// Second press within tap window — arm it, start hold timer
					loopState.mu.Lock()
					if cancel, ok := loopState.twhCancel[combo]; ok {
						cancel()
						delete(loopState.twhCancel, combo)
					}
					loopState.twhPhase[combo] = twhArmed
					loopState.mu.Unlock()
					startTwhHoldTimer(combo, twh, cfg, loopState, m.GetComboCodes(code), virtual, m.GetCurrentModifiers())
					bufferedKeys[code] = true
					return true
				}

				// First press — track it, but only suppress if no other behaviors exist
				loopState.mu.Lock()
				loopState.twhPhase[combo] = twhFirstPress
				loopState.mu.Unlock()

				hasOtherBehaviors := m.CheckShortcut(combo, config.BehaviorNormal, config.TimingPress) != nil ||
					m.CheckShortcut(combo, config.BehaviorWhileHeld, config.TimingPress) != nil ||
					m.CheckShortcut(combo, config.BehaviorHold, config.TimingPress) != nil
				if !hasOtherBehaviors {
					bufferedKeys[code] = true
					return true
				}
				// Fall through to normal/whileheld/hold handling
			}

			// Check if this key has a doubletap shortcut - suppress and wait for release
			if m.HasDoubleTapShortcut(code) {
				bufferedKeys[code] = true
				return true // Suppress, handle on release
			}

			hasRelease := m.HasReleaseShortcut(combo)
			pressMatched := false

			normalShortcut := m.CheckShortcut(combo, config.BehaviorNormal, config.TimingPress)
			holdShortcut := m.CheckShortcut(combo, config.BehaviorHold, config.TimingPress)
			whileheldShortcut := m.CheckShortcut(combo, config.BehaviorWhileHeld, config.TimingPress)

			if normalShortcut != nil && holdShortcut != nil {
				// Tap-vs-hold: defer decision until release or threshold
				startTapHoldTimer(combo, normalShortcut, holdShortcut, cfg, loopState, m.GetComboCodes(code), virtual, m.GetCurrentModifiers())
				pressMatched = true
			} else if normalShortcut != nil && whileheldShortcut != nil {
				// Tap-vs-whileheld: same timer logic, whileheld starts on threshold
				startTapHoldTimer(combo, normalShortcut, whileheldShortcut, cfg, loopState, m.GetComboCodes(code), virtual, m.GetCurrentModifiers())
				pressMatched = true
			} else if normalShortcut != nil {
				LogMatch(combo, m.GetComboCodes(code))
				executeShortcut(normalShortcut, cfg, virtual, m.GetCurrentModifiers())
				pressMatched = true
			} else if holdShortcut != nil {
				LogMatch(combo+".hold", m.GetComboCodes(code))
				startHoldTimer(combo, holdShortcut, cfg, loopState)
				pressMatched = true
			}

			// Check whileheld shortcuts (start process, kill on release)
			if !pressMatched {
				if whileheldShortcut != nil {
					LogMatch(combo+".whileheld", m.GetComboCodes(code))
					startHeldProcess(combo, whileheldShortcut, cfg, loopState, virtual, m.GetCurrentModifiers())
					pressMatched = true
				}
			}

			// Check repeat-whileheld shortcuts (repeat while held, stop on release)
			if !pressMatched {
				if shortcut := m.CheckShortcut(combo, config.BehaviorRepeatWhileHeld, config.TimingPress); shortcut != nil {
					LogMatch(combo+".repeat-whileheld", m.GetComboCodes(code))
					startLoop(combo, shortcut, cfg, loopState)
					pressMatched = true
				}
			}

			// Check repeat-toggle shortcuts (toggle repeat loop on/off)
			if !pressMatched {
				if shortcut := m.CheckShortcut(combo, config.BehaviorRepeatToggle, config.TimingPress); shortcut != nil {
					LogMatch(combo+".repeat-toggle", m.GetComboCodes(code))
					toggleLoop(combo, shortcut, cfg, m, loopState)
					pressMatched = true
				}
			}

			// Check switch shortcuts (cycle command)
			if !pressMatched {
				if shortcut := m.CheckShortcut(combo, config.BehaviorSwitch, config.TimingPress); shortcut != nil {
					LogMatch(combo+".switch", m.GetComboCodes(code))
					executeSwitchShortcut(combo, shortcut, m, cfg)
					pressMatched = true
				}
			}

			// Check pressrelease shortcuts (Commands[0] on press, Commands[1] on release)
			if !pressMatched {
				if shortcut := m.CheckShortcut(combo, config.BehaviorPressRelease, config.TimingPress); shortcut != nil {
					LogMatch(combo+".pressrelease", m.GetComboCodes(code))
					resolvedCmd := cfg.ResolveCommand(shortcut.Commands[0])
					LogTrigger(resolvedCmd)
					Execute(resolvedCmd, cfg)
					bufferedKeys[code] = true
					pressMatched = true
				}
			}

			// Buffer key if there's a release shortcut (regardless of press match)
			if hasRelease {
				bufferedKeys[code] = true
			}

			// Suppress if any shortcut matched (press or release)
			if pressMatched || hasRelease {
				return true
			}

			return false // Forward
		}

		// KEY RELEASE (value == 0)
		if value == 0 {
			combo := m.GetCurrentCombo(code)

			// Handle tapwhileheld state machine
			if twh := m.CheckShortcut(combo, config.BehaviorTapWhileHeld, config.TimingPress); twh != nil {
				loopState.mu.Lock()
				phase := loopState.twhPhase[combo]
				loopState.mu.Unlock()

				switch phase {
				case twhFirstPress:
					// Was it a tap or a hold? Only prime on tap.
					loopState.mu.Lock()
					_, wasHeld := loopState.heldProcesses[combo]
					_, wasHeldKey := loopState.heldKeys[combo]
					loopState.mu.Unlock()

					if !wasHeld && !wasHeldKey {
						loopState.mu.Lock()
						loopState.twhPhase[combo] = twhPriming
						loopState.mu.Unlock()
						startTwhPrimingTimer(combo, twh, cfg, loopState)
					} else {
						loopState.mu.Lock()
						loopState.twhPhase[combo] = twhIdle
						loopState.mu.Unlock()
					}
					// If we suppressed the first press (standalone tapwhileheld), suppress release too
					if bufferedKeys[code] {
						delete(bufferedKeys, code)
						return true
					}
					// Otherwise fall through — normal release handling proceeds

				case twhArmed:
					// Released before hold threshold — fire doubletap if defined
					loopState.mu.Lock()
					if cancel, ok := loopState.twhCancel[combo]; ok {
						cancel()
						delete(loopState.twhCancel, combo)
					}
					loopState.twhPhase[combo] = twhIdle
					loopState.mu.Unlock()
					delete(bufferedKeys, code)
					if dtShortcut := m.GetDoubleTapShortcut(code); dtShortcut != nil {
						resolvedCmd := cfg.ResolveCommand(dtShortcut.Commands[0])
						LogMatch(combo+".doubletap", m.GetComboCodes(code))
						LogTrigger(resolvedCmd)
						Execute(resolvedCmd, cfg)
					}
					return true

				case twhActive:
					// Stop whileheld process
					stopHeldProcess(combo, loopState, virtual)
					loopState.mu.Lock()
					loopState.twhPhase[combo] = twhIdle
					loopState.mu.Unlock()
					delete(bufferedKeys, code)
					return true
				}
			}

			// Fire tap if released before hold threshold
			if fireTapIfPending(combo, loopState, cfg, m.GetComboCodes(code), virtual, m.GetCurrentModifiers()) {
				return true
			}

			// Stop loops (if active)
			stopLoop(combo, loopState)

			// Stop held processes (if active)
			stopHeldProcess(combo, loopState, virtual)

			// Cancel hold timers (if active)
			cancelHoldTimer(combo, loopState)

			// Check for doubletap on non-modifier keys
			if m.HasDoubleTapShortcut(code) && bufferedKeys[code] {
				delete(bufferedKeys, code)
				doubleTapState := m.GetDoubleTapState()
				if doubleTapState != nil {
					if doubleTapState.CheckSecondTap(code) {
						// Second tap - execute doubletap
						doubleTapState.CancelTimer()
						if shortcut := m.GetDoubleTapShortcut(code); shortcut != nil {
							resolvedCmd := cfg.ResolveCommand(shortcut.Commands[0])
							LogMatch(combo+".doubletap", m.GetComboCodes(code))
							LogTrigger(resolvedCmd)
							Execute(resolvedCmd, cfg)
						}
						return true // Suppress
					} else {
						// First tap - start timer
						shortcut := m.GetDoubleTapShortcut(code)
						interval := shortcut.Interval
						if interval == 0 {
							interval = cfg.Settings.DefaultInterval
						}
						LogDebug("Doubletap timer started: %.0fms", interval)

						// Capture combo for fallback execution
						capturedCombo := combo
						capturedCodes := m.GetComboCodes(code)
						doubleTapState.StartTimer(code, interval, func() {
							// Timeout - execute single-tap shortcut if defined
							if s := m.CheckShortcut(capturedCombo, config.BehaviorNormal, config.TimingPress); s != nil {
								resolvedCmd := cfg.ResolveCommand(s.Commands[0])
								LogMatch(capturedCombo, capturedCodes)
								LogTrigger(resolvedCmd)
								Execute(resolvedCmd, cfg)
							}
						})
						return true // Suppress
					}
				}
			}

			// Check if this was a buffered key (for release shortcuts)
			if bufferedKeys[code] {
				delete(bufferedKeys, code)

				// Execute release shortcuts
				executeReleaseShortcuts(combo, m, cfg, code, virtual, m.GetCurrentModifiers())
				return true // Suppress release
			}
		}

		return false
	}
}

func checkModifierAlone(modifiers matcher.ModifierState) bool {
	count := 0
	if modifiers.Super {
		count++
	}
	if modifiers.Ctrl {
		count++
	}
	if modifiers.Alt {
		count++
	}
	if modifiers.Shift {
		count++
	}
	return count == 1
}

func executeShortcut(shortcut *config.ParsedShortcut, cfg *config.Config, virtual *evdev.InputDevice, heldModifiers matcher.ModifierState) {
	if shortcut.IsRemap {
		LogTrigger(">" + shortcut.RemapCombo)
		if virtual == nil {
			fmt.Fprintf(os.Stderr, "Remap error: no virtual device available\n")
			return
		}
		if err := EmitKeyCombo(virtual, shortcut.RemapCombo, heldModifiers); err != nil {
			fmt.Fprintf(os.Stderr, "Remap error: %v\n", err)
		}
		return
	}
	if len(shortcut.Commands) == 0 {
		return
	}
	resolvedCmd := cfg.ResolveCommand(shortcut.Commands[0])
	LogTrigger(resolvedCmd)
	Execute(resolvedCmd, cfg)
}

func startLoop(combo string, shortcut *config.ParsedShortcut, cfg *config.Config, state *LoopState) {
	state.mu.Lock()
	defer state.mu.Unlock()

	// Cancel existing loop if any
	if cancel, exists := state.active[combo]; exists {
		cancel()
	}

	interval := shortcut.Interval
	if interval == 0 {
		interval = cfg.Settings.DefaultInterval
	}

	ctx, cancel := context.WithCancel(context.Background())
	state.active[combo] = cancel

	resolvedCmd := cfg.ResolveCommand(shortcut.Commands[0])
	LogTrigger(resolvedCmd)

	go runTickerLoop(ctx, interval, func() { Execute(resolvedCmd, cfg) })
}

func stopLoop(combo string, state *LoopState) {
	state.mu.Lock()
	defer state.mu.Unlock()

	if cancel, exists := state.active[combo]; exists {
		cancel()
		delete(state.active, combo)
	}
}

func startHeldProcess(combo string, shortcut *config.ParsedShortcut, cfg *config.Config, state *LoopState, virtual *evdev.InputDevice, heldModifiers matcher.ModifierState) {
	state.mu.Lock()
	defer state.mu.Unlock()

	if shortcut.IsRemap {
		// Release any previously held keys for this combo
		if codes, exists := state.heldKeys[combo]; exists {
			EmitKeysUp(virtual, codes)
		}
		LogDebug("whileheld remap: pressing down >%s", shortcut.RemapCombo)
		codes := EmitKeysDown(virtual, shortcut.RemapCombo, heldModifiers)
		LogDebug("whileheld remap: stored %d codes for %s", len(codes), combo)
		if len(codes) > 0 {
			state.heldKeys[combo] = codes
		}
		return
	}

	// Kill existing process if any
	if cmd, exists := state.heldProcesses[combo]; exists {
		StopProcess(cmd)
	}

	resolvedCmd := cfg.ResolveCommand(shortcut.Commands[0])
	LogTrigger(resolvedCmd)
	cmd := ExecuteTracked(resolvedCmd, cfg)
	if cmd != nil {
		state.heldProcesses[combo] = cmd
	}
}

func baseKeyFromCombo(combo string) string {
	parts := strings.Split(combo, "+")
	return parts[len(parts)-1]
}

func stopHeldProcess(combo string, state *LoopState, virtual *evdev.InputDevice) {
	state.mu.Lock()
	defer state.mu.Unlock()

	if codes, exists := state.heldKeys[combo]; exists {
		LogDebug("whileheld remap: releasing %d codes for %s", len(codes), combo)
		EmitKeysUp(virtual, codes)
		delete(state.heldKeys, combo)
		return
	}

	// Injected keys may be re-emitted by a remapper, changing modifier state between press and release.
	// Fall back to matching by base key if exact combo isn't found.
	baseKey := baseKeyFromCombo(combo)
	for storedCombo, codes := range state.heldKeys {
		if baseKeyFromCombo(storedCombo) == baseKey {
			LogDebug("whileheld remap: releasing %d codes for %s (combo drifted to %s)", len(codes), storedCombo, combo)
			EmitKeysUp(virtual, codes)
			delete(state.heldKeys, storedCombo)
			return
		}
	}

	if cmd, exists := state.heldProcesses[combo]; exists {
		StopProcess(cmd)
		delete(state.heldProcesses, combo)
	}

	// Same fallback for held processes
	for storedCombo, cmd := range state.heldProcesses {
		if baseKeyFromCombo(storedCombo) == baseKey {
			StopProcess(cmd)
			delete(state.heldProcesses, storedCombo)
			return
		}
	}
}

func startHoldTimer(combo string, shortcut *config.ParsedShortcut, cfg *config.Config, state *LoopState) {
	state.mu.Lock()

	// Cancel existing timer if any
	if cancel, exists := state.holdTimers[combo]; exists {
		cancel()
	}

	interval := shortcut.Interval
	if interval == 0 {
		interval = cfg.Settings.DefaultInterval
	}

	ctx, cancel := context.WithCancel(context.Background())
	state.holdTimers[combo] = cancel
	state.mu.Unlock()

	resolvedCmd := cfg.ResolveCommand(shortcut.Commands[0])

	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(interval) * time.Millisecond):
			state.mu.Lock()
			delete(state.holdTimers, combo)
			state.mu.Unlock()
			LogTrigger(resolvedCmd)
			Execute(resolvedCmd, cfg)
		}
	}()
}

func cancelHoldTimer(combo string, state *LoopState) {
	state.mu.Lock()
	defer state.mu.Unlock()

	if cancel, exists := state.holdTimers[combo]; exists {
		cancel()
		delete(state.holdTimers, combo)
	}
}

func toggleLoop(combo string, shortcut *config.ParsedShortcut, cfg *config.Config, m *matcher.Matcher, state *LoopState) {
	if m.IsToggleActive(combo) {
		// Stop loop
		if cancel := m.StopToggleLoop(combo); cancel != nil {
			cancel()
		}
	} else {
		// Start loop (doesn't stop on key release)
		interval := shortcut.Interval
		if interval == 0 {
			interval = cfg.Settings.DefaultInterval
		}

		ctx, cancel := context.WithCancel(context.Background())
		m.StartToggleLoop(combo, cancel)

		resolvedCmd := cfg.ResolveCommand(shortcut.Commands[0])
		LogTrigger(resolvedCmd)

		go runTickerLoop(ctx, interval, func() { Execute(resolvedCmd, cfg) })
	}
}

func executeSwitchShortcut(combo string, shortcut *config.ParsedShortcut, m *matcher.Matcher, cfg *config.Config) {
	groupKey := combo
	if shortcut.AliasGroup != "" {
		groupKey = shortcut.AliasGroup
	}
	key := fmt.Sprintf("%s.switch.%d", groupKey, shortcut.Timing)
	command := m.GetNextSwitchCommand(key, shortcut.Commands)
	resolvedCmd := cfg.ResolveCommand(command)

	LogTrigger(resolvedCmd)
	Execute(resolvedCmd, cfg)
}

func executeReleaseShortcuts(combo string, m *matcher.Matcher, cfg *config.Config, code uint16, virtual *evdev.InputDevice, heldModifiers matcher.ModifierState) {
	codes := m.GetComboCodes(code)

	if shortcut := m.CheckShortcut(combo, config.BehaviorNormal, config.TimingRelease); shortcut != nil {
		LogMatch(combo+".onrelease", codes)
		executeShortcut(shortcut, cfg, virtual, heldModifiers)
	}

	if shortcut := m.CheckShortcut(combo, config.BehaviorRepeatWhileHeld, config.TimingRelease); shortcut != nil {
		LogMatch(combo+".repeat-whileheld.onrelease", codes)
		executeShortcut(shortcut, cfg, virtual, heldModifiers)
	}

	if shortcut := m.CheckShortcut(combo, config.BehaviorRepeatToggle, config.TimingRelease); shortcut != nil {
		LogMatch(combo+".repeat-toggle.onrelease", codes)
		toggleLoop(combo, shortcut, cfg, m, nil)
	}

	if shortcut := m.CheckShortcut(combo, config.BehaviorSwitch, config.TimingRelease); shortcut != nil {
		LogMatch(combo+".switch.onrelease", codes)
		executeSwitchShortcut(combo, shortcut, m, cfg)
	}

	if shortcut := m.CheckShortcut(combo, config.BehaviorPressRelease, config.TimingPress); shortcut != nil {
		LogMatch(combo+".pressrelease.release", codes)
		resolvedCmd := cfg.ResolveCommand(shortcut.Commands[1])
		LogTrigger(resolvedCmd)
		Execute(resolvedCmd, cfg)
	}
}

// startTapHoldTimer starts the hold threshold timer when both normal and hold shortcuts exist
// for the same combo. On timeout, fires hold. On early release, fireTapIfPending fires tap.
func startTapHoldTimer(combo string, normalShortcut *config.ParsedShortcut, holdShortcut *config.ParsedShortcut, cfg *config.Config, state *LoopState, codes string, virtual *evdev.InputDevice, heldModifiers matcher.ModifierState) {
	interval := holdShortcut.Interval
	if interval == 0 {
		interval = cfg.Settings.DefaultInterval
	}

	state.mu.Lock()
	if cancel, exists := state.tapHoldTimers[combo]; exists {
		cancel()
	}
	state.tapHoldNormal[combo] = normalShortcut
	ctx, cancel := context.WithCancel(context.Background())
	state.tapHoldTimers[combo] = cancel
	state.mu.Unlock()

	go func() {
		select {
		case <-ctx.Done():
			return // Key released before threshold — tap handled by fireTapIfPending
		case <-time.After(time.Duration(interval) * time.Millisecond):
			state.mu.Lock()
			if _, stillPending := state.tapHoldTimers[combo]; stillPending {
				delete(state.tapHoldTimers, combo)
				delete(state.tapHoldNormal, combo)
				state.mu.Unlock()
				if holdShortcut.Behavior == config.BehaviorWhileHeld {
					LogMatch(combo+".whileheld", codes)
					startHeldProcess(combo, holdShortcut, cfg, state, virtual, heldModifiers)
				} else {
					LogMatch(combo+".hold", codes)
					executeShortcut(holdShortcut, cfg, virtual, heldModifiers)
				}
			} else {
				state.mu.Unlock()
			}
		}
	}()
}

// fireTapIfPending fires the tap (normal) shortcut if the key was released before the hold
// threshold. Returns true if tap-vs-hold was handled (caller should suppress the event).
func fireTapIfPending(combo string, state *LoopState, cfg *config.Config, codes string, virtual *evdev.InputDevice, heldModifiers matcher.ModifierState) bool {
	state.mu.Lock()
	cancel, exists := state.tapHoldTimers[combo]
	if !exists {
		state.mu.Unlock()
		return false
	}
	cancel()
	normalShortcut := state.tapHoldNormal[combo]
	delete(state.tapHoldTimers, combo)
	delete(state.tapHoldNormal, combo)
	state.mu.Unlock()

	if normalShortcut != nil {
		LogMatch(combo, codes)
		executeShortcut(normalShortcut, cfg, virtual, heldModifiers)
	}
	return true
}

// startTwhPrimingTimer starts timer1: the window within which a second press must happen.
// On timeout, resets to idle (gesture abandoned).
func startTwhPrimingTimer(combo string, shortcut *config.ParsedShortcut, cfg *config.Config, state *LoopState) {
	interval := shortcut.Interval
	if interval == 0 {
		interval = cfg.Settings.DefaultInterval
	}

	ctx, cancel := context.WithCancel(context.Background())
	state.mu.Lock()
	state.twhCancel[combo] = cancel
	state.mu.Unlock()

	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(interval) * time.Millisecond):
			state.mu.Lock()
			if state.twhPhase[combo] == twhPriming {
				state.twhPhase[combo] = twhIdle
				delete(state.twhCancel, combo)
			}
			state.mu.Unlock()
		}
	}()
}

// startTwhHoldTimer starts timer2: the hold threshold on the second press.
// On timeout (still held), starts the whileheld process.
func startTwhHoldTimer(combo string, shortcut *config.ParsedShortcut, cfg *config.Config, state *LoopState, codes string, virtual *evdev.InputDevice, heldModifiers matcher.ModifierState) {
	interval := shortcut.HoldInterval
	if interval == 0 {
		interval = cfg.Settings.DefaultInterval
	}

	ctx, cancel := context.WithCancel(context.Background())
	state.mu.Lock()
	state.twhCancel[combo] = cancel
	state.mu.Unlock()

	go func() {
		select {
		case <-ctx.Done():
			return // Released before threshold — doubletap handled by release path
		case <-time.After(time.Duration(interval) * time.Millisecond):
			state.mu.Lock()
			if state.twhPhase[combo] == twhArmed {
				state.twhPhase[combo] = twhActive
				delete(state.twhCancel, combo)
				state.mu.Unlock()
				LogMatch(combo+".tapwhileheld", codes)
				startHeldProcess(combo, shortcut, cfg, state, virtual, heldModifiers)
			} else {
				state.mu.Unlock()
			}
		}
	}()
}
