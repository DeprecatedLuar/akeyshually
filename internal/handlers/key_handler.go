package handlers

import (
	"context"
	"fmt"

	"github.com/deprecatedluar/akeyshually/internal/common"
	"github.com/deprecatedluar/akeyshually/internal/config"
	"github.com/deprecatedluar/akeyshually/internal/executor"
	"github.com/deprecatedluar/akeyshually/internal/keys"
	"github.com/deprecatedluar/akeyshually/internal/ladder"
	"github.com/deprecatedluar/akeyshually/internal/matcher"
	"github.com/deprecatedluar/akeyshually/internal/timers"
	evdev "github.com/holoplot/go-evdev"
)

func HandlePress(code uint16, value int32, m *matcher.Matcher, cfg *config.Config, loopState *executor.LoopState, injector *evdev.InputDevice, virtual *evdev.InputDevice, stateMap *timers.StateMap, emittedTracker *timers.EmittedModifierTracker) bool {
	common.LogKey(keys.GetKeyName(code), code)
	m.ClearTapCandidate()

	var combo string
	var shortcuts []*config.ParsedShortcut

	if matcher.IsModifierKey(code) {
		modifiers := m.GetCurrentModifiers()
		if !modifiers.Super && !modifiers.Ctrl && !modifiers.Alt && !modifiers.Shift {
			m.MarkTapCandidate(code)
		}
		m.UpdateModifierState(code, true)

		// Check for lone modifier shortcuts (super.doubletap, super.pressrelease, etc.)
		combo = keys.GetKeyName(code) // "super", "ctrl", "alt", or "shift"
		shortcuts = m.GetShortcuts(combo)
		if len(shortcuts) == 0 {
			return false // No shortcuts, behave as normal modifier
		}
		// Fall through to ladder logic below
	} else {
		combo = m.GetCurrentCombo(code)
		m.UpdateModifierState(code, true)

		// Check if modifiers have active ladders - escape hatch to combo or fallback to cancellation
		modifiers := m.GetCurrentModifiers()
		checkModifierEscape := func(modName string, isHeld bool) bool {
			if !isHeld {
				return false
			}
			if state := stateMap.Get(modName); state != nil {
				comboKey := modName + "+" + keys.GetKeyName(code)
				if len(cfg.ParsedShortcuts[comboKey]) > 0 {
					// Escape hatch: valid combo exists, migrate ladder to combo
					common.LogDebug("Escape hatch: %s ladder migrating to %s", modName, comboKey)
					select {
					case state.EscapeCh <- code:
					default:
					}
					// Ladder goroutine owns migration entirely - return immediately
					return true
				} else {
					// Fallback: no combo defined, cancel and emit modifier
					common.LogDebug("Cancelling %s ladder (combo detected), emitting %s keydown", modName, modName)
					state.Cancel()
					stateMap.Delete(modName)
					if virtual != nil {
						ladder.EmitModifierKey(virtual, keys.ResolveKeyCode, modName, true)
						emittedTracker.MarkEmitted(modName)
					}
				}
			}
			return false
		}

		if checkModifierEscape("super", modifiers.Super) {
			return true
		}
		if checkModifierEscape("ctrl", modifiers.Ctrl) {
			return true
		}
		if checkModifierEscape("alt", modifiers.Alt) {
			return true
		}
		if checkModifierEscape("shift", modifiers.Shift) {
			return true
		}

		shortcuts = m.GetShortcuts(combo)
		if len(shortcuts) == 0 {
			common.LogDebug("No shortcuts for %s, forwarding", combo)
			return false
		}

		common.LogDebug("Combo %s detected with modifiers: super=%v ctrl=%v alt=%v shift=%v",
			combo, modifiers.Super, modifiers.Ctrl, modifiers.Alt, modifiers.Shift)
	}

	// Forward second press to a goroutine waiting in the doubletap window
	if existing := stateMap.Get(combo); existing != nil {
		common.LogDebug(">>> FOUND EXISTING STATE for %s, calling SignalPress()", combo)
		existing.SignalPress()
		return true
	}
	common.LogDebug(">>> NO existing state for %s, will create new ladder", combo)

	suppress := false

	// Switch always fires immediately, independent of any chain
	for _, s := range shortcuts {
		if s.Behavior == config.BehaviorSwitch {
			common.LogMatch(combo+".switch", m.GetComboCodes(code))
			executeSwitchShortcut(combo, s, m, cfg)
			suppress = true
		}
	}

	// Build candidates for timer ladder
	candidates := timers.BuildCandidates(shortcuts)

	// No candidates means only switch/eager behaviors (already handled above)
	if len(candidates) == 0 {
		return suppress
	}

	// Launch timer ladder resolution goroutine
	modifiers := m.GetCurrentModifiers()
	ctx, cancel := context.WithCancel(context.Background())
	state := timers.NewComboState(cancel)
	common.LogDebug(">>> ADDING %s to stateMap, launching goroutine", combo)
	stateMap.Set(combo, state)
	go ladder.Run(ctx, state, combo, code, value, candidates, cfg, loopState, injector, virtual, modifiers, stateMap, emittedTracker, cfg.ParsedShortcuts)
	return true
}

func HandleRelease(code uint16, value int32, m *matcher.Matcher, cfg *config.Config, loopState *executor.LoopState, injector *evdev.InputDevice, virtual *evdev.InputDevice, stateMap *timers.StateMap, emittedTracker *timers.EmittedModifierTracker) bool {
	if matcher.IsModifierKey(code) {
		combo := keys.GetKeyName(code)
		common.LogDebug("Modifier %s released", combo)

		if command, matched := m.CheckTap(code); matched {
			resolvedCmd := cfg.ResolveCommand(command)
			common.LogMatch(combo+".tap", fmt.Sprintf("%d", code))
			common.LogTrigger(resolvedCmd)
			ctx := executor.ExecContext{
				KeyCode:   code,
				Value:     value,
				Virtual:   virtual,
				Injector:  injector,
				Modifiers: m.GetCurrentModifiers(),
				Config:    cfg,
				LoopState: loopState,
			}
			executor.Run(resolvedCmd, ctx)
		}
		m.UpdateModifierState(code, false)

		// Signal release to active ladder if one exists
		if state := stateMap.Get(combo); state != nil {
			common.LogDebug("Signaling release to %s ladder", combo)
			state.SignalRelease()
		}

		// If we emitted this modifier to system, also emit the release
		if emittedTracker.WasEmitted(combo) {
			common.LogDebug("Emitting %s release (we emitted the press)", combo)
			ladder.EmitModifierKey(virtual, keys.ResolveKeyCode, combo, false)
			emittedTracker.ClearEmitted(combo)
			return true // Suppress original release since we emitted it
		}

		common.LogDebug("Forwarding %s release to system", combo)
		return false
	}

	combo := m.GetCurrentCombo(code)

	// Signal release to active goroutine if one exists
	if state := stateMap.Get(combo); state != nil {
		state.SignalRelease()
		return true
	}

	// No active goroutine - stop any sustained processes
	loopState.StopLoop(combo)
	loopState.StopHeldProcess(combo, injector)
	return false
}

func executeSwitchShortcut(combo string, shortcut *config.ParsedShortcut, m *matcher.Matcher, cfg *config.Config) {
	groupKey := combo
	if shortcut.AliasGroup != "" {
		groupKey = shortcut.AliasGroup
	}
	key := fmt.Sprintf("%s.switch.%d", groupKey, shortcut.Timing)
	command := m.GetNextSwitchCommand(key, shortcut.Commands)
	resolvedCmd := cfg.ResolveCommand(command)
	common.LogTrigger(resolvedCmd)
	executor.Execute(resolvedCmd, cfg)
}
