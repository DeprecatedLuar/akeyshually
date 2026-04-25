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

// LoopState tracks active repeat loops and sustained processes across key events.
type LoopState struct {
	mu             sync.Mutex
	active         map[string]context.CancelFunc // repeat loops
	heldProcesses  map[string]*exec.Cmd          // sustained whileheld processes
	heldKeys       map[string][]uint16           // sustained remap hold keys
	persistentHeld map[string][]uint16           // >> persistent remap keys
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

	var combo string
	var shortcuts []*config.ParsedShortcut

	if matcher.IsModifierKey(code) {
		modifiers := m.GetCurrentModifiers()
		if !modifiers.Super && !modifiers.Ctrl && !modifiers.Alt && !modifiers.Shift {
			m.MarkTapCandidate(code)
		}
		m.UpdateModifierState(code, true)

		// Check for lone modifier shortcuts (super.doubletap, super.pressrelease, etc.)
		combo = matcher.GetKeyName(code) // "super", "ctrl", "alt", or "shift"
		shortcuts = m.GetShortcuts(combo)
		if len(shortcuts) == 0 {
			return false // No shortcuts, behave as normal modifier
		}
		// Fall through to ladder logic below
	} else {
		combo = m.GetCurrentCombo(code)
		m.UpdateModifierState(code, true)
		shortcuts = m.GetShortcuts(combo)

		if len(shortcuts) == 0 {
			return false
		}

		// Cancel any active modifier ladders (prevents super.doubletap from firing when super+t is pressed)
		modifiers := m.GetCurrentModifiers()
		if modifiers.Super {
			if state := stateMap.Get("super"); state != nil {
				state.Cancel()
				stateMap.Delete("super")
			}
		}
		if modifiers.Ctrl {
			if state := stateMap.Get("ctrl"); state != nil {
				state.Cancel()
				stateMap.Delete("ctrl")
			}
		}
		if modifiers.Alt {
			if state := stateMap.Get("alt"); state != nil {
				state.Cancel()
				stateMap.Delete("alt")
			}
		}
		if modifiers.Shift {
			if state := stateMap.Get("shift"); state != nil {
				state.Cancel()
				stateMap.Delete("shift")
			}
		}
	}

	// Forward second press to a goroutine waiting in the doubletap window
	if existing := stateMap.Get(combo); existing != nil {
		existing.SignalPress()
		return true
	}

	suppress := false

	// Switch always fires immediately, independent of any chain
	for _, s := range shortcuts {
		if s.Behavior == config.BehaviorSwitch {
			LogMatch(combo+".switch", m.GetComboCodes(code))
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
	stateMap.Set(combo, state)
	go runLadder(ctx, state, combo, candidates, cfg, loopState, virtual, modifiers, stateMap)
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

		// Signal release to active ladder if one exists
		combo := matcher.GetKeyName(code)
		if state := stateMap.Get(combo); state != nil {
			state.SignalRelease()
		}

		return false
	}

	combo := m.GetCurrentCombo(code)

	// Signal release to active goroutine if one exists
	if state := stateMap.Get(combo); state != nil {
		state.SignalRelease()
		return true
	}

	// No active goroutine - stop any sustained processes
	stopLoop(combo, loopState)
	stopHeldProcess(combo, loopState, virtual)
	return false
}

// runLadder is the timer ladder goroutine. Evaluates candidates against win conditions.
// State-driven: tracks count (presses), pressed (held), and steps through timer thresholds.
func runLadder(
	ctx context.Context,
	state *timers.ComboState,
	combo string,
	candidates []timers.Candidate,
	cfg *config.Config,
	loopState *LoopState,
	virtual *evdev.InputDevice,
	modifiers matcher.ModifierState,
	stateMap *timers.StateMap,
) {
	defer stateMap.Delete(combo)

	// State tracking
	count := 1      // First press already happened
	pressed := true // Key is down
	phase := 0      // Current phase boundary crossed

	// Build timer ladder: sorted unique thresholds
	ladder := buildTimerLadder(candidates, cfg.Settings.DefaultInterval)
	LogDebug("Ladder for %s: %v", combo, ladder)

	var timer *time.Timer
	var timerCh <-chan time.Time

	// Start first timer if ladder exists
	if len(ladder) > 0 {
		timer = time.NewTimer(ladder[0])
		timerCh = timer.C
		LogDebug("Doubletap timer started: %vms", ladder[0].Milliseconds())
	}

	for {
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return

		case <-state.PressCh:
			// Second press arrived
			count++
			pressed = true
			LogDebug("Second press: count=%d", count)

			// Prune dead candidates
			candidates = pruneCandidates(candidates, count, pressed, phase)
			if len(candidates) == 0 {
				LogDebug("no candidate won for %s (all pruned after second press)", combo)
				if timer != nil {
					timer.Stop()
				}
				return
			}

			// Check for immediate winner
			if winner := checkWinner(candidates, count, pressed, phase); winner != nil {
				if timer != nil {
					timer.Stop()
				}
				fireWinner(combo, winner, cfg, loopState, virtual, modifiers, ctx, state)
				return
			}

		case <-state.ReleaseCh:
			// Key released
			pressed = false
			LogDebug("Release: count=%d, pressed=false", count)

			// Prune dead candidates
			candidates = pruneCandidates(candidates, count, pressed, phase)
			if len(candidates) == 0 {
				LogDebug("no candidate won for %s (all pruned after release)", combo)
				if timer != nil {
					timer.Stop()
				}
				return
			}

			// Check for immediate winner
			if winner := checkWinner(candidates, count, pressed, phase); winner != nil {
				if timer != nil {
					timer.Stop()
				}
				fireWinner(combo, winner, cfg, loopState, virtual, modifiers, ctx, state)
				return
			}

		case <-timerCh:
			// Timer expired, advance phase
			phase++
			LogDebug("Timer expired: phase=%d", phase)

			// Prune dead candidates
			candidates = pruneCandidates(candidates, count, pressed, phase)

			// Check for winner
			if winner := checkWinner(candidates, count, pressed, phase); winner != nil {
				fireWinner(combo, winner, cfg, loopState, virtual, modifiers, ctx, state)
				return
			}

			// No winner yet, advance to next timer threshold
			if phase < len(ladder) {
				timer.Reset(ladder[phase])
				timerCh = timer.C
				LogDebug("Next timer: %vms", ladder[phase].Milliseconds())
			} else {
				// No more timers, resolve with current state
				LogDebug("no candidate won for %s (ladder exhausted)", combo)
				return
			}
		}
	}
}

// buildTimerLadder extracts unique sorted timer thresholds from candidates
func buildTimerLadder(candidates []timers.Candidate, defaultInterval float64) []time.Duration {
	thresholds := make(map[int]time.Duration)

	for _, c := range candidates {
		interval := intervalOrDefault(c.Shortcut.Interval, defaultInterval)

		// Phase 1 threshold
		thresholds[1] = ms(interval)

		// Phase 2 threshold (for taphold/taplongpress)
		if c.Condition.Phase == 2 {
			holdInterval := intervalOrDefault(c.Shortcut.HoldInterval, defaultInterval)
			// Phase 2 timer starts after second press, so it's just the hold interval
			thresholds[2] = ms(holdInterval)
		}
	}

	// Convert to sorted slice
	var ladder []time.Duration
	for phase := 1; phase <= len(thresholds); phase++ {
		if duration, exists := thresholds[phase]; exists {
			ladder = append(ladder, duration)
		}
	}

	return ladder
}

// pruneCandidates removes candidates that can no longer win
func pruneCandidates(candidates []timers.Candidate, count int, pressed bool, phase int) []timers.Candidate {
	pruned := make([]timers.Candidate, 0, len(candidates))
	for _, c := range candidates {
		// Can this candidate still possibly win?
		// It's dead if:
		// - count already exceeds what it needs
		// - count is stuck below what it needs and we're past the phase where more presses can arrive
		// - pressed state is incompatible and we can't change it anymore

		if count > c.Condition.Count {
			// Too many presses
			continue
		}

		if count < c.Condition.Count && phase > c.Condition.Phase {
			// Not enough presses and we're past the phase boundary
			continue
		}

		// Candidate still alive
		pruned = append(pruned, c)
	}
	return pruned
}

// checkWinner returns the first candidate whose win condition is satisfied
func checkWinner(candidates []timers.Candidate, count int, pressed bool, phase int) *timers.Candidate {
	for i := range candidates {
		c := &candidates[i]
		if c.Condition.Count == count &&
		   c.Condition.Pressed == pressed &&
		   c.Condition.Phase <= phase {
			return c
		}
	}
	return nil
}

// fireWinner executes the winning candidate's shortcut
func fireWinner(
	combo string,
	winner *timers.Candidate,
	cfg *config.Config,
	loopState *LoopState,
	virtual *evdev.InputDevice,
	modifiers matcher.ModifierState,
	ctx context.Context,
	state *timers.ComboState,
) {
	s := winner.Shortcut

	switch s.Behavior {
	case config.BehaviorNormal:
		LogMatch(combo, combo)
		if s.Repeat {
			toggleLoopDirect(combo, s, cfg, loopState)
		} else {
			executeShortcutWithState(s, cfg, virtual, modifiers, loopState)
		}

	case config.BehaviorPressRelease:
		LogMatch(combo+".pressrelease", combo)
		if s.Commands[0] != "" {
			resolvedCmd := cfg.ResolveCommand(s.Commands[0])
			LogTrigger(resolvedCmd)
			Execute(resolvedCmd, cfg)
		}
		// 10ms gap between press and release commands
		time.AfterFunc(10*time.Millisecond, func() {
			if s.Commands[1] != "" {
				resolvedCmd := cfg.ResolveCommand(s.Commands[1])
				LogTrigger(resolvedCmd)
				Execute(resolvedCmd, cfg)
			}
		})

	case config.BehaviorHold, config.BehaviorLongPress:
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
			// For longpress, exit immediately
			if s.Behavior != config.BehaviorLongPress {
				select {
				case <-ctx.Done():
				case <-state.ReleaseCh:
				}
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

	case config.BehaviorDoubleTap:
		fire(combo+".doubletap", s.Commands[0], cfg)

	case config.BehaviorTapHold:
		LogMatch(combo+".taphold", combo)
		resolvedCmd := cfg.ResolveCommand(s.Commands[0])
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
		loopState.mu.Lock()
		if c, exists := loopState.heldProcesses[combo]; exists {
			StopProcess(c)
			delete(loopState.heldProcesses, combo)
		}
		loopState.mu.Unlock()

	case config.BehaviorTapLongPress:
		LogMatch(combo+".taplongpress", combo)
		resolvedCmd := cfg.ResolveCommand(s.Commands[1])
		LogTrigger(resolvedCmd)
		Execute(resolvedCmd, cfg)
	}
}

// fire is a helper for simple one-shot command execution with logging.
func fire(label, command string, cfg *config.Config) {
	resolvedCmd := cfg.ResolveCommand(command)
	LogMatch(label, label)
	LogTrigger(resolvedCmd)
	Execute(resolvedCmd, cfg)
}

// ms converts float64 milliseconds to time.Duration.
func ms(d float64) time.Duration {
	return time.Duration(d) * time.Millisecond
}

func intervalOrDefault(interval, def float64) float64 {
	if interval > 0 {
		return interval
	}
	return def
}

// --- Loop and process management ---

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
		codes := EmitKeysDown(virtual, shortcut.RemapCombo, heldModifiers)
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
		EmitKeysUp(virtual, codes)
		delete(state.heldKeys, combo)
		return
	}

	// Fallback: match by base key in case modifier state drifted
	baseKey := baseKeyFromCombo(combo)
	for storedCombo, codes := range state.heldKeys {
		if baseKeyFromCombo(storedCombo) == baseKey {
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
