package internal

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/deprecatedluar/akeyshually/internal/config"
)

type LoopState struct {
	mu     sync.Mutex
	active map[string]context.CancelFunc // "super+k" (combo only) -> cancel
}

func NewLoopState() *LoopState {
	return &LoopState{
		active: make(map[string]context.CancelFunc),
	}
}

func CreateUnifiedHandler(m *Matcher, cfg *config.Config, loopState *LoopState) KeyHandler {
	logging := isLoggingEnabled()
	bufferedKeys := make(map[uint16]bool)

	return func(code uint16, value int32) bool {
		// Forward media keys if disabled (let system handle them)
		if cfg.Settings.DisableMediaKeys && IsMediaKey(code) {
			return false // Forward to system
		}

		// Handle modifiers for tap detection
		if IsModifierKey(code) {
			if value == 1 {
				m.UpdateModifierState(code, true)
				// Check if pressed alone
				modifiers := m.GetCurrentModifiers()
				isAlone := checkModifierAlone(modifiers)
				if isAlone {
					m.MarkTapCandidate(code)
				}
			} else if value == 0 {
				// Modifier released - check tap
				if command, matched := m.CheckTap(code); matched {
					resolvedCmd := cfg.ResolveCommand(command)
					if logging {
						fmt.Fprintf(os.Stderr, "[SHORTCUT TAP] %s\n", resolvedCmd)
					}
					Execute(resolvedCmd, cfg)
				}
				m.UpdateModifierState(code, false)
			}
			return false // Forward modifiers
		}

		// KEY PRESS (value == 1)
		if value == 1 {
			m.ClearTapCandidate() // Any non-modifier clears tap

			// Check for press-triggered shortcuts
			combo := m.GetCurrentCombo(code)

			// Check normal press shortcuts
			if shortcut := m.CheckShortcut(combo, config.BehaviorNormal, config.TimingPress); shortcut != nil {
				executeShortcut(shortcut, cfg, logging)
				return true
			}

			// Check loop shortcuts (start loop)
			if shortcut := m.CheckShortcut(combo, config.BehaviorLoop, config.TimingPress); shortcut != nil {
				startLoop(combo, shortcut, cfg, loopState, logging)
				return true
			}

			// Check toggle shortcuts (start/stop loop)
			if shortcut := m.CheckShortcut(combo, config.BehaviorToggle, config.TimingPress); shortcut != nil {
				toggleLoop(combo, shortcut, cfg, m, loopState, logging)
				return true
			}

			// Check switch shortcuts (cycle command)
			if shortcut := m.CheckShortcut(combo, config.BehaviorSwitch, config.TimingPress); shortcut != nil {
				executeSwitchShortcut(combo, shortcut, m, cfg, logging)
				return true
			}

			// Check if any release shortcuts exist (buffer key)
			if m.HasReleaseShortcut(combo) {
				bufferedKeys[code] = true
				return true // Suppress
			}

			return false // Forward
		}

		// KEY RELEASE (value == 0)
		if value == 0 {
			combo := m.GetCurrentCombo(code)

			// Stop loops (if active)
			stopLoop(combo, loopState)

			// Check if this was a buffered key
			if bufferedKeys[code] {
				delete(bufferedKeys, code)

				// Execute release shortcuts
				executeReleaseShortcuts(combo, m, cfg, logging)
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

func executeShortcut(shortcut *config.ParsedShortcut, cfg *config.Config, logging bool) {
	if len(shortcut.Commands) == 0 {
		return
	}
	resolvedCmd := cfg.ResolveCommand(shortcut.Commands[0])
	if logging {
		fmt.Fprintf(os.Stderr, "[SHORTCUT] %s\n", resolvedCmd)
	}
	Execute(resolvedCmd, cfg)
}

func startLoop(combo string, shortcut *config.ParsedShortcut, cfg *config.Config, state *LoopState, logging bool) {
	state.mu.Lock()
	defer state.mu.Unlock()

	// Cancel existing loop if any
	if cancel, exists := state.active[combo]; exists {
		cancel()
	}

	interval := shortcut.Interval
	if interval == 0 {
		interval = cfg.Settings.DefaultLoopInterval
	}

	ctx, cancel := context.WithCancel(context.Background())
	state.active[combo] = cancel

	go func() {
		ticker := time.NewTicker(time.Duration(interval) * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				resolvedCmd := cfg.ResolveCommand(shortcut.Commands[0])
				if logging {
					fmt.Fprintf(os.Stderr, "[SHORTCUT LOOP] %s\n", resolvedCmd)
				}
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

func toggleLoop(combo string, shortcut *config.ParsedShortcut, cfg *config.Config, m *Matcher, state *LoopState, logging bool) {
	if m.IsToggleActive(combo) {
		// Stop loop
		if cancel := m.StopToggleLoop(combo); cancel != nil {
			cancel()
		}
		if logging {
			fmt.Fprintf(os.Stderr, "[SHORTCUT TOGGLE] Stopped: %s\n", combo)
		}
	} else {
		// Start loop (doesn't stop on key release)
		interval := shortcut.Interval
		if interval == 0 {
			interval = cfg.Settings.DefaultLoopInterval
		}

		ctx, cancel := context.WithCancel(context.Background())
		m.StartToggleLoop(combo, cancel)

		go func() {
			ticker := time.NewTicker(time.Duration(interval) * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					resolvedCmd := cfg.ResolveCommand(shortcut.Commands[0])
					if logging {
						fmt.Fprintf(os.Stderr, "[SHORTCUT TOGGLE LOOP] %s\n", resolvedCmd)
					}
					Execute(resolvedCmd, cfg)
				}
			}
		}()

		if logging {
			fmt.Fprintf(os.Stderr, "[SHORTCUT TOGGLE] Started: %s\n", combo)
		}
	}
}

func executeSwitchShortcut(combo string, shortcut *config.ParsedShortcut, m *Matcher, cfg *config.Config, logging bool) {
	key := fmt.Sprintf("%s.switch.%d", combo, shortcut.Timing)
	command := m.GetNextSwitchCommand(key, shortcut.Commands)
	resolvedCmd := cfg.ResolveCommand(command)

	if logging {
		fmt.Fprintf(os.Stderr, "[SHORTCUT SWITCH] %s\n", resolvedCmd)
	}
	Execute(resolvedCmd, cfg)
}

func executeReleaseShortcuts(combo string, m *Matcher, cfg *config.Config, logging bool) {
	// Check normal release
	if shortcut := m.CheckShortcut(combo, config.BehaviorNormal, config.TimingRelease); shortcut != nil {
		executeShortcut(shortcut, cfg, logging)
	}

	// Check loop release (shouldn't happen, but for completeness)
	if shortcut := m.CheckShortcut(combo, config.BehaviorLoop, config.TimingRelease); shortcut != nil {
		executeShortcut(shortcut, cfg, logging)
	}

	// Check toggle release
	if shortcut := m.CheckShortcut(combo, config.BehaviorToggle, config.TimingRelease); shortcut != nil {
		toggleLoop(combo, shortcut, cfg, m, nil, logging)
	}

	// Check switch release
	if shortcut := m.CheckShortcut(combo, config.BehaviorSwitch, config.TimingRelease); shortcut != nil {
		executeSwitchShortcut(combo, shortcut, m, cfg, logging)
	}
}

func isLoggingEnabled() bool {
	val := os.Getenv("LOGGING")
	return val == "1" || val == "true" || val == "yes"
}
