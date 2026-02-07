package internal

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/deprecatedluar/akeyshually/internal/config"
)

type LoopState struct {
	mu            sync.Mutex
	active        map[string]context.CancelFunc // repeat loops: combo -> cancel
	heldProcesses map[string]*exec.Cmd          // whileheld: combo -> process
	holdTimers    map[string]context.CancelFunc // hold: combo -> cancel timer
}

func NewLoopState() *LoopState {
	return &LoopState{
		active:        make(map[string]context.CancelFunc),
		heldProcesses: make(map[string]*exec.Cmd),
		holdTimers:    make(map[string]context.CancelFunc),
	}
}

func CreateUnifiedHandler(m *Matcher, cfg *config.Config, loopState *LoopState) KeyHandler {
	bufferedKeys := make(map[uint16]bool)

	return func(code uint16, value int32) bool {
		// Forward media keys if disabled (let system handle them)
		if cfg.Settings.DisableMediaKeys && IsMediaKey(code) {
			return false // Forward to system
		}

		// Handle modifiers for tap detection
		if IsModifierKey(code) {
			if value == 1 {
				LogKey(GetKeyName(code), code)
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
								LogMatch(GetKeyName(code)+".doubletap", fmt.Sprintf("%d", code))
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
									LogMatch(GetKeyName(code)+".tap", fmt.Sprintf("%d", code))
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
					LogMatch(GetKeyName(code)+".tap", fmt.Sprintf("%d", code))
					LogTrigger(resolvedCmd)
					Execute(resolvedCmd, cfg)
				}
				m.UpdateModifierState(code, false)
			}
			return false // Forward modifiers
		}

		// KEY PRESS (value == 1)
		if value == 1 {
			LogKey(GetKeyName(code), code)
			m.ClearTapCandidate() // Any non-modifier clears tap

			// Check if this key has a doubletap shortcut - suppress and wait for release
			if m.HasDoubleTapShortcut(code) {
				bufferedKeys[code] = true
				return true // Suppress, handle on release
			}

			// Check for press-triggered shortcuts
			combo := m.GetCurrentCombo(code)

			hasRelease := m.HasReleaseShortcut(combo)
			pressMatched := false

			// Check normal press shortcuts
			if shortcut := m.CheckShortcut(combo, config.BehaviorNormal, config.TimingPress); shortcut != nil {
				LogMatch(combo, m.GetComboCodes(code))
				executeShortcut(shortcut, cfg)
				pressMatched = true
			}

			// Check whileheld shortcuts (start process, kill on release)
			if !pressMatched {
				if shortcut := m.CheckShortcut(combo, config.BehaviorWhileHeld, config.TimingPress); shortcut != nil {
					LogMatch(combo+".whileheld", m.GetComboCodes(code))
					startHeldProcess(combo, shortcut, cfg, loopState)
					pressMatched = true
				}
			}

			// Check hold shortcuts (fire after threshold)
			if !pressMatched {
				if shortcut := m.CheckShortcut(combo, config.BehaviorHold, config.TimingPress); shortcut != nil {
					LogMatch(combo+".hold", m.GetComboCodes(code))
					startHoldTimer(combo, shortcut, cfg, loopState)
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

			// Stop loops (if active)
			stopLoop(combo, loopState)

			// Stop held processes (if active)
			stopHeldProcess(combo, loopState)

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
				executeReleaseShortcuts(combo, m, cfg, code)
				return true // Suppress release
			}
		}

		return false
	}
}

func checkModifierAlone(modifiers ModifierState) bool {
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

func executeShortcut(shortcut *config.ParsedShortcut, cfg *config.Config) {
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

	go func() {
		ticker := time.NewTicker(time.Duration(interval * 1e6)) // Convert ms to nanoseconds
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				Execute(resolvedCmd, cfg)
			}
		}
	}()
}

func stopLoop(combo string, state *LoopState) {
	state.mu.Lock()
	defer state.mu.Unlock()

	if cancel, exists := state.active[combo]; exists {
		cancel()
		delete(state.active, combo)
	}
}

func startHeldProcess(combo string, shortcut *config.ParsedShortcut, cfg *config.Config, state *LoopState) {
	state.mu.Lock()
	defer state.mu.Unlock()

	// Kill existing if any
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

func stopHeldProcess(combo string, state *LoopState) {
	state.mu.Lock()
	defer state.mu.Unlock()

	if cmd, exists := state.heldProcesses[combo]; exists {
		StopProcess(cmd)
		delete(state.heldProcesses, combo)
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
		case <-time.After(time.Duration(interval * 1e6)):
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

func toggleLoop(combo string, shortcut *config.ParsedShortcut, cfg *config.Config, m *Matcher, state *LoopState) {
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

		go func() {
			ticker := time.NewTicker(time.Duration(interval * 1e6)) // Convert ms to nanoseconds
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					Execute(resolvedCmd, cfg)
				}
			}
		}()
	}
}

func executeSwitchShortcut(combo string, shortcut *config.ParsedShortcut, m *Matcher, cfg *config.Config) {
	key := fmt.Sprintf("%s.switch.%d", combo, shortcut.Timing)
	command := m.GetNextSwitchCommand(key, shortcut.Commands)
	resolvedCmd := cfg.ResolveCommand(command)

	LogTrigger(resolvedCmd)
	Execute(resolvedCmd, cfg)
}

func executeReleaseShortcuts(combo string, m *Matcher, cfg *config.Config, code uint16) {
	codes := m.GetComboCodes(code)

	// Check normal release
	if shortcut := m.CheckShortcut(combo, config.BehaviorNormal, config.TimingRelease); shortcut != nil {
		LogMatch(combo+".onrelease", codes)
		executeShortcut(shortcut, cfg)
	}

	// Check repeat-whileheld release (edge case)
	if shortcut := m.CheckShortcut(combo, config.BehaviorRepeatWhileHeld, config.TimingRelease); shortcut != nil {
		LogMatch(combo+".repeat-whileheld.onrelease", codes)
		executeShortcut(shortcut, cfg)
	}

	// Check repeat-toggle release
	if shortcut := m.CheckShortcut(combo, config.BehaviorRepeatToggle, config.TimingRelease); shortcut != nil {
		LogMatch(combo+".repeat-toggle.onrelease", codes)
		toggleLoop(combo, shortcut, cfg, m, nil)
	}

	// Check switch release
	if shortcut := m.CheckShortcut(combo, config.BehaviorSwitch, config.TimingRelease); shortcut != nil {
		LogMatch(combo+".switch.onrelease", codes)
		executeSwitchShortcut(combo, shortcut, m, cfg)
	}
}
