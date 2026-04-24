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
	"github.com/deprecatedluar/akeyshually/internal/timers"
	evdev "github.com/holoplot/go-evdev"
)

type LoopState struct {
	mu             sync.Mutex
	active         map[string]context.CancelFunc // repeat loops: combo -> cancel
	heldProcesses  map[string]*exec.Cmd          // hold remap: combo -> process
	heldKeys       map[string][]uint16           // hold remap: combo -> key codes held down
	persistentHeld map[string][]uint16           // >>: key name -> held key codes
}

func NewLoopState() *LoopState {
	return &LoopState{
		active:         make(map[string]context.CancelFunc),
		heldProcesses:  make(map[string]*exec.Cmd),
		heldKeys:       make(map[string][]uint16),
		persistentHeld: make(map[string][]uint16),
	}
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

func CreateUnifiedHandler(m *matcher.Matcher, cfg *config.Config, loopState *LoopState, virtual *evdev.InputDevice) KeyHandler {
	stateMap := timers.NewStateMap()

	return func(code uint16, value int32) bool {
		if cfg.Settings.DisableMediaKeys && IsMediaKey(code) {
			return false
		}
		if value == 1 {
			return handlePress(code, m, cfg, loopState, virtual, stateMap)
		}
		if value == 0 {
			return handleRelease(code, m, cfg, loopState, virtual, stateMap)
		}
		return false
	}
}

func handlePress(code uint16, m *matcher.Matcher, cfg *config.Config, loopState *LoopState, virtual *evdev.InputDevice, stateMap *timers.StateMap) bool {
	LogKey(matcher.GetKeyName(code), code)
	m.ClearTapCandidate()

	if matcher.IsModifierKey(code) {
		modifiers := m.GetCurrentModifiers()
		if !modifiers.Super && !modifiers.Ctrl && !modifiers.Alt && !modifiers.Shift {
			m.MarkTapCandidate(code)
		}
		m.UpdateModifierState(code, true)
		return false
	}

	combo := m.GetCurrentCombo(code)
	m.UpdateModifierState(code, true)
	shortcuts := m.GetShortcuts(combo)

	if len(shortcuts) == 0 {
		return false
	}

	// If a doubletap window is open, signal second press
	if existing := stateMap.Get(combo); existing != nil {
		existing.Lock()
		phase := existing.Phase
		existing.Unlock()
		if phase == timers.PhaseDoubleTapWindow {
			existing.SignalPress()
			return true
		}
		existing.Cancel()
		stateMap.Delete(combo)
	}

	// Switch fires immediately (modifier behavior, not a chain link)
	suppress := false
	for _, s := range shortcuts {
		if s.Behavior == config.BehaviorSwitch {
			LogMatch(combo+".switch", m.GetComboCodes(code))
			executeSwitchShortcut(combo, s, m, cfg)
			suppress = true
		}
	}

	// PressRelease Commands[0] fires immediately, independent of chain
	for _, s := range shortcuts {
		if s.Behavior == config.BehaviorPressRelease {
			LogMatch(combo+".pressrelease", m.GetComboCodes(code))
			if s.Commands[0] != "" {
				resolvedCmd := cfg.ResolveCommand(s.Commands[0])
				LogTrigger(resolvedCmd)
				Execute(resolvedCmd, cfg)
			}
			suppress = true
		}
	}

	// If no chain behaviors, handle onpress immediately
	if !hasChainBehaviors(shortcuts) {
		for _, s := range shortcuts {
			if s.Behavior == config.BehaviorNormal {
				if s.Repeat {
					LogMatch(combo+".repeat", m.GetComboCodes(code))
					toggleLoop(combo, s, cfg, m, loopState)
				} else {
					LogMatch(combo, m.GetComboCodes(code))
					executeShortcutWithState(s, cfg, virtual, m.GetCurrentModifiers(), loopState)
				}
				suppress = true
			}
		}
		return suppress
	}

	// Start chain goroutine
	modifiers := m.GetCurrentModifiers()
	ctx, cancel := context.WithCancel(context.Background())
	state := timers.NewComboState(cancel)
	stateMap.Set(combo, state)

	go runSequence(ctx, state, combo, shortcuts, cfg, loopState, virtual, modifiers, stateMap)

	return true
}

func handleRelease(code uint16, m *matcher.Matcher, cfg *config.Config, loopState *LoopState, virtual *evdev.InputDevice, stateMap *timers.StateMap) bool {
	if matcher.IsModifierKey(code) {
		if command, matched := m.CheckTap(code); matched {
			resolvedCmd := cfg.ResolveCommand(command)
			LogMatch(matcher.GetKeyName(code)+".tap", fmt.Sprintf("%d", code))
			LogTrigger(resolvedCmd)
			Execute(resolvedCmd, cfg)
		}
		m.UpdateModifierState(code, false)
		return false
	}

	combo := m.GetCurrentCombo(code)
	shortcuts := m.GetShortcuts(combo)

	state := stateMap.Get(combo)
	if state != nil {
		state.SignalRelease()
		return true
	}

	// No active chain — fire pressrelease Commands[1] if no chain behaviors defined
	// (when chain behaviors exist, the goroutine fires it at resolution)
	if !hasChainBehaviors(shortcuts) {
		for _, s := range shortcuts {
			if s.Behavior == config.BehaviorPressRelease && s.Commands[1] != "" {
				resolvedCmd := cfg.ResolveCommand(s.Commands[1])
				LogMatch(combo+".pressrelease.release", m.GetComboCodes(code))
				LogTrigger(resolvedCmd)
				Execute(resolvedCmd, cfg)
			}
		}
	}

	stopLoop(combo, loopState)
	stopHeldProcess(combo, loopState, virtual)
	return false
}

// runSequence is the chain goroutine: one per press, walks hold→doubletap→taphold links.
func runSequence(ctx context.Context, state *timers.ComboState, combo string, shortcuts []*config.ParsedShortcut, cfg *config.Config, loopState *LoopState, virtual *evdev.InputDevice, modifiers matcher.ModifierState, stateMap *timers.StateMap) {
	defer stateMap.Delete(combo)

	defaultInterval := cfg.Settings.DefaultInterval

	// Link 1: hold / holdrelease / longpress
	if s := getLink1(shortcuts); s != nil {
		state.Lock()
		state.Phase = timers.PhaseHoldWindow
		state.Unlock()

		interval := intervalOrDefault(s.Interval, defaultInterval)
		t1 := time.NewTimer(time.Duration(interval) * time.Millisecond)
		defer t1.Stop()

		select {
		case <-ctx.Done():
			return
		case <-t1.C:
			executeLink1(ctx, state, combo, s, cfg, loopState, virtual, modifiers)
			return
		case <-state.ReleaseCh:
			// Released before threshold — continue chain
		}
	} else {
		// No Link 1 — still need first release before doubletap window
		select {
		case <-ctx.Done():
			return
		case <-state.ReleaseCh:
		}
	}

	// After first release: check if chain continues
	if !hasLink2(shortcuts) {
		fireChainResolution(combo, shortcuts, cfg, loopState, virtual, modifiers)
		return
	}

	// Link 2: doubletap / taphold window — wait for second press
	state.Lock()
	state.Phase = timers.PhaseDoubleTapWindow
	state.Unlock()

	interval := getLink2Interval(shortcuts, defaultInterval)
	t2 := time.NewTimer(time.Duration(interval) * time.Millisecond)
	defer t2.Stop()

	select {
	case <-ctx.Done():
		return
	case <-t2.C:
		// Window expired — fire onpress fallback
		fireChainResolution(combo, shortcuts, cfg, loopState, virtual, modifiers)
		return
	case <-state.PressCh:
		// Second press within window — continue to Link 3
	}

	// Link 3: taphold threshold or wait for second release (doubletap only)
	if s := getTapHoldLink(shortcuts); s != nil {
		state.Lock()
		state.Phase = timers.PhaseTapHoldWindow
		state.Unlock()

		holdInterval := intervalOrDefault(s.HoldInterval, defaultInterval)
		t3 := time.NewTimer(time.Duration(holdInterval) * time.Millisecond)
		defer t3.Stop()

		select {
		case <-ctx.Done():
			return
		case <-t3.C:
			executeTapHold(ctx, state, combo, s, cfg, loopState, virtual)
		case <-state.ReleaseCh:
			// Released before threshold — tap wins (or doubletap if defined)
			if dt := getShortcutByBehavior(shortcuts, config.BehaviorDoubleTap); dt != nil {
				resolvedCmd := cfg.ResolveCommand(dt.Commands[0])
				LogMatch(combo+".doubletap", m_getComboCodes(combo))
				LogTrigger(resolvedCmd)
				Execute(resolvedCmd, cfg)
			} else {
				resolvedCmd := cfg.ResolveCommand(s.Commands[0])
				LogMatch(combo+".tap", m_getComboCodes(combo))
				LogTrigger(resolvedCmd)
				Execute(resolvedCmd, cfg)
			}
		}
	} else {
		// Only doubletap — wait for second release to confirm
		state.Lock()
		state.Phase = timers.PhaseTapHoldWindow
		state.Unlock()

		select {
		case <-ctx.Done():
			return
		case <-state.ReleaseCh:
		}

		if dt := getShortcutByBehavior(shortcuts, config.BehaviorDoubleTap); dt != nil {
			resolvedCmd := cfg.ResolveCommand(dt.Commands[0])
			LogMatch(combo+".doubletap", m_getComboCodes(combo))
			LogTrigger(resolvedCmd)
			Execute(resolvedCmd, cfg)
		}
	}
}

// executeLink1 runs the hold/holdrelease/longpress behavior after threshold fires.
func executeLink1(ctx context.Context, state *timers.ComboState, combo string, s *config.ParsedShortcut, cfg *config.Config, loopState *LoopState, virtual *evdev.InputDevice, modifiers matcher.ModifierState) {
	switch s.Behavior {
	case config.BehaviorHold:
		LogMatch(combo+".hold", combo)
		if s.Repeat {
			startLoop(combo, s, cfg, loopState)
			select {
			case <-ctx.Done():
			case <-state.ReleaseCh:
			}
			stopLoop(combo, loopState)
		} else if s.IsRemap {
			startHeldProcess(combo, s, cfg, loopState, virtual, modifiers)
			select {
			case <-ctx.Done():
			case <-state.ReleaseCh:
			}
			stopHeldProcess(combo, loopState, virtual)
		} else {
			resolvedCmd := cfg.ResolveCommand(s.Commands[0])
			LogTrigger(resolvedCmd)
			Execute(resolvedCmd, cfg)
			select {
			case <-ctx.Done():
			case <-state.ReleaseCh:
			}
		}

	case config.BehaviorHoldRelease:
		LogMatch(combo+".holdrelease", combo)
		if s.Commands[0] != "" {
			resolvedCmd := cfg.ResolveCommand(s.Commands[0])
			LogTrigger(resolvedCmd)
			Execute(resolvedCmd, cfg)
		}
		select {
		case <-ctx.Done():
		case <-state.ReleaseCh:
		}
		if s.Commands[1] != "" {
			resolvedCmd := cfg.ResolveCommand(s.Commands[1])
			LogMatch(combo+".holdrelease.release", combo)
			LogTrigger(resolvedCmd)
			Execute(resolvedCmd, cfg)
		}

	case config.BehaviorLongPress:
		resolvedCmd := cfg.ResolveCommand(s.Commands[0])
		LogMatch(combo+".longpress", combo)
		LogTrigger(resolvedCmd)
		Execute(resolvedCmd, cfg)
		// One-shot: no sustain
	}
}

// executeTapHold runs after taphold/taplongpress threshold fires at Link 3.
func executeTapHold(ctx context.Context, state *timers.ComboState, combo string, s *config.ParsedShortcut, cfg *config.Config, loopState *LoopState, virtual *evdev.InputDevice) {
	if s.Behavior == config.BehaviorTapLongPress {
		resolvedCmd := cfg.ResolveCommand(s.Commands[1])
		LogMatch(combo+".taplongpress", combo)
		LogTrigger(resolvedCmd)
		Execute(resolvedCmd, cfg)
		return
	}

	// taphold: sustain Commands[1] until release
	LogMatch(combo+".taphold", combo)
	resolvedCmd := cfg.ResolveCommand(s.Commands[1])
	LogTrigger(resolvedCmd)
	cmd := ExecuteTracked(resolvedCmd, cfg)
	if cmd != nil {
		loopState.mu.Lock()
		loopState.heldProcesses[combo] = cmd
		loopState.mu.Unlock()
	}
	select {
	case <-ctx.Done():
	case <-state.ReleaseCh:
	}
	stopHeldProcess(combo, loopState, virtual)
}

// fireChainResolution fires onpress fallback and pressrelease Commands[1] when the chain
// exhausts without any link triggering.
func fireChainResolution(combo string, shortcuts []*config.ParsedShortcut, cfg *config.Config, loopState *LoopState, virtual *evdev.InputDevice, modifiers matcher.ModifierState) {
	if s := getShortcutByBehavior(shortcuts, config.BehaviorNormal); s != nil {
		LogMatch(combo, combo)
		if s.Repeat {
			toggleLoopDirect(combo, s, cfg, loopState)
		} else {
			executeShortcutWithState(s, cfg, virtual, modifiers, loopState)
		}
	}
	if s := getShortcutByBehavior(shortcuts, config.BehaviorPressRelease); s != nil && s.Commands[1] != "" {
		resolvedCmd := cfg.ResolveCommand(s.Commands[1])
		LogMatch(combo+".pressrelease.release", combo)
		LogTrigger(resolvedCmd)
		Execute(resolvedCmd, cfg)
	}
}

// --- Chain link helpers ---

func hasChainBehaviors(shortcuts []*config.ParsedShortcut) bool {
	for _, s := range shortcuts {
		switch s.Behavior {
		case config.BehaviorHold, config.BehaviorHoldRelease, config.BehaviorLongPress,
			config.BehaviorDoubleTap, config.BehaviorTapHold, config.BehaviorTapLongPress:
			return true
		}
	}
	return false
}

func getLink1(shortcuts []*config.ParsedShortcut) *config.ParsedShortcut {
	for _, s := range shortcuts {
		switch s.Behavior {
		case config.BehaviorHold, config.BehaviorHoldRelease, config.BehaviorLongPress:
			return s
		}
	}
	return nil
}

func hasLink2(shortcuts []*config.ParsedShortcut) bool {
	for _, s := range shortcuts {
		switch s.Behavior {
		case config.BehaviorDoubleTap, config.BehaviorTapHold, config.BehaviorTapLongPress:
			return true
		}
	}
	return false
}

// getLink2Interval returns the doubletap/taphold window interval for Link 2.
func getLink2Interval(shortcuts []*config.ParsedShortcut, defaultInterval float64) float64 {
	for _, s := range shortcuts {
		switch s.Behavior {
		case config.BehaviorDoubleTap, config.BehaviorTapHold, config.BehaviorTapLongPress:
			return intervalOrDefault(s.Interval, defaultInterval)
		}
	}
	return defaultInterval
}

func getTapHoldLink(shortcuts []*config.ParsedShortcut) *config.ParsedShortcut {
	for _, s := range shortcuts {
		if s.Behavior == config.BehaviorTapHold || s.Behavior == config.BehaviorTapLongPress {
			return s
		}
	}
	return nil
}

func getShortcutByBehavior(shortcuts []*config.ParsedShortcut, behavior config.BehaviorMode) *config.ParsedShortcut {
	for _, s := range shortcuts {
		if s.Behavior == behavior {
			return s
		}
	}
	return nil
}

// m_getComboCodes is a logging helper — returns combo string directly since we don't have matcher here.
func m_getComboCodes(combo string) string {
	return combo
}

// --- Execution helpers ---

func intervalOrDefault(interval, defaultInterval float64) float64 {
	if interval > 0 {
		return interval
	}
	return defaultInterval
}

func executeShortcutWithState(shortcut *config.ParsedShortcut, cfg *config.Config, virtual *evdev.InputDevice, heldModifiers matcher.ModifierState, loopState *LoopState) {
	if shortcut.IsRemap {
		if virtual == nil {
			fmt.Fprintf(os.Stderr, "Remap error: no virtual device available\n")
			return
		}
		switch shortcut.RemapMode {
		case config.RemapTap:
			LogTrigger(">" + shortcut.RemapCombo)
			if err := EmitKeyCombo(virtual, shortcut.RemapCombo, heldModifiers); err != nil {
				fmt.Fprintf(os.Stderr, "Remap error: %v\n", err)
			}
		case config.RemapHoldForever:
			if loopState == nil {
				fmt.Fprintf(os.Stderr, "Remap error: >> requires loopState\n")
				return
			}
			LogTrigger(">>" + shortcut.RemapCombo)
			loopState.mu.Lock()
			codes := EmitKeysDown(virtual, shortcut.RemapCombo, heldModifiers)
			if len(codes) > 0 {
				loopState.persistentHeld[shortcut.RemapCombo] = codes
			}
			loopState.mu.Unlock()
		case config.RemapKeyUp:
			LogTrigger("<" + shortcut.RemapCombo)
			code, ok := matcher.ResolveKeyCode(shortcut.RemapCombo)
			if !ok {
				fmt.Fprintf(os.Stderr, "Remap error: unknown key %q\n", shortcut.RemapCombo)
				return
			}
			EmitKeysUp(virtual, []uint16{code})
		case config.RemapReleaseAll:
			if loopState == nil {
				return
			}
			LogTrigger("<<")
			loopState.mu.Lock()
			for key, codes := range loopState.persistentHeld {
				EmitKeysUp(virtual, codes)
				delete(loopState.persistentHeld, key)
			}
			loopState.mu.Unlock()
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

	if cancel, exists := state.active[combo]; exists {
		cancel()
	}

	interval := intervalOrDefault(shortcut.Interval, cfg.Settings.DefaultInterval)
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
		if codes, exists := state.heldKeys[combo]; exists {
			EmitKeysUp(virtual, codes)
		}
		LogDebug("hold remap: pressing down >%s", shortcut.RemapCombo)
		codes := EmitKeysDown(virtual, shortcut.RemapCombo, heldModifiers)
		LogDebug("hold remap: stored %d codes for %s", len(codes), combo)
		if len(codes) > 0 {
			state.heldKeys[combo] = codes
		}
		return
	}

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
		LogDebug("hold remap: releasing %d codes for %s", len(codes), combo)
		EmitKeysUp(virtual, codes)
		delete(state.heldKeys, combo)
		return
	}

	baseKey := baseKeyFromCombo(combo)
	for storedCombo, codes := range state.heldKeys {
		if baseKeyFromCombo(storedCombo) == baseKey {
			LogDebug("hold remap: releasing %d codes for %s (combo drifted to %s)", len(codes), storedCombo, combo)
			EmitKeysUp(virtual, codes)
			delete(state.heldKeys, storedCombo)
			return
		}
	}

	if cmd, exists := state.heldProcesses[combo]; exists {
		StopProcess(cmd)
		delete(state.heldProcesses, combo)
		return
	}

	for storedCombo, cmd := range state.heldProcesses {
		if baseKeyFromCombo(storedCombo) == baseKey {
			StopProcess(cmd)
			delete(state.heldProcesses, storedCombo)
			return
		}
	}
}

func toggleLoop(combo string, shortcut *config.ParsedShortcut, cfg *config.Config, m *matcher.Matcher, _ *LoopState) {
	if m.IsToggleActive(combo) {
		if cancel := m.StopToggleLoop(combo); cancel != nil {
			cancel()
		}
	} else {
		interval := intervalOrDefault(shortcut.Interval, cfg.Settings.DefaultInterval)
		ctx, cancel := context.WithCancel(context.Background())
		m.StartToggleLoop(combo, cancel)

		resolvedCmd := cfg.ResolveCommand(shortcut.Commands[0])
		LogTrigger(resolvedCmd)

		go runTickerLoop(ctx, interval, func() { Execute(resolvedCmd, cfg) })
	}
}

// toggleLoopDirect is used in chain fallback where we don't have a matcher reference.
// It duplicates the toggle logic using LoopState instead of matcher state.
func toggleLoopDirect(combo string, shortcut *config.ParsedShortcut, cfg *config.Config, loopState *LoopState) {
	loopState.mu.Lock()
	_, running := loopState.active[combo]
	loopState.mu.Unlock()

	if running {
		stopLoop(combo, loopState)
	} else {
		startLoop(combo, shortcut, cfg, loopState)
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
