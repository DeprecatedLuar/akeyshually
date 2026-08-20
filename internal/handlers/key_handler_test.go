package handlers

import (
	"testing"
	"time"

	"github.com/deprecatedluar/akeyshually/internal/config"
	"github.com/deprecatedluar/akeyshually/internal/executor"
	"github.com/deprecatedluar/akeyshually/internal/matcher"
	"github.com/deprecatedluar/akeyshually/internal/timers"
	evdev "github.com/holoplot/go-evdev"
)

// A modifier with no lone shortcut of its own must stay transparent even
// when it has configured child combos (e.g. shift has no lone shortcut but
// appears in "shift+print", "ctrl+shift+k"). This is the Krita regression:
// shift must reach the system on press, not be withheld pending a combo
// that may never come (e.g. a mouse drag, which generates no key event).
func TestModifierWithConfiguredChildIsForwarded(t *testing.T) {
	shortcut := &config.ParsedShortcut{
		KeyCombo: "ctrl+up",
		Behavior: config.BehaviorNormal,
		Commands: []string{">scrollup"},
	}
	cfg := &config.Config{
		ParsedShortcuts: map[string][]*config.ParsedShortcut{"ctrl+up": {shortcut}},
		EscapeMap:       map[string]bool{"ctrl": true},
	}
	m := matcher.New(cfg.ParsedShortcuts)
	stateMap := timers.NewStateMap()
	emittedTracker := timers.NewEmittedModifierTracker()

	suppressed := HandlePress(uint16(evdev.KEY_LEFTCTRL), 1, m, cfg,
		executor.NewLoopState(), executor.Outputs{}, nil, stateMap, emittedTracker)
	if suppressed {
		t.Fatal("ctrl press was withheld even though it has no lone shortcut of its own")
	}
	if state := stateMap.Get("ctrl"); state != nil {
		t.Fatal("ctrl should not have entered the ladder; it has no lone shortcuts")
	}
	if !emittedTracker.IsDown("ctrl") {
		t.Fatal("ctrl forwarded to system should be tracked as down")
	}
}

func TestModifierWithoutConfiguredChildIsForwarded(t *testing.T) {
	cfg := &config.Config{
		ParsedShortcuts: map[string][]*config.ParsedShortcut{},
		EscapeMap:       map[string]bool{},
	}
	m := matcher.New(cfg.ParsedShortcuts)

	suppressed := HandlePress(uint16(evdev.KEY_LEFTCTRL), 1, m, cfg,
		executor.NewLoopState(), executor.Outputs{}, nil,
		timers.NewStateMap(), timers.NewEmittedModifierTracker())
	if suppressed {
		t.Fatal("unconfigured ctrl press was unexpectedly suppressed")
	}
}

// Press then release with no intervening key: the modifier must be forwarded
// on press and its release must not be double-suppressed or dropped.
func TestModifierPressReleaseWithNoComboIsTransparent(t *testing.T) {
	shortcut := &config.ParsedShortcut{
		KeyCombo: "shift+print",
		Behavior: config.BehaviorNormal,
		Commands: []string{"screenshot"},
	}
	cfg := &config.Config{
		ParsedShortcuts: map[string][]*config.ParsedShortcut{"shift+print": {shortcut}},
		EscapeMap:       map[string]bool{"shift": true},
	}
	m := matcher.New(cfg.ParsedShortcuts)
	stateMap := timers.NewStateMap()
	emittedTracker := timers.NewEmittedModifierTracker()
	loopState := executor.NewLoopState()

	if suppressed := HandlePress(uint16(evdev.KEY_LEFTSHIFT), 1, m, cfg,
		loopState, executor.Outputs{}, nil, stateMap, emittedTracker); suppressed {
		t.Fatal("shift press was suppressed despite having no lone shortcut")
	}
	if !emittedTracker.IsDown("shift") {
		t.Fatal("shift should be tracked as down after being forwarded")
	}

	// Whether HandleRelease forwards the raw event or emits its own matching
	// keyup is an implementation detail — either way exactly one keyup
	// reaches the system. What matters is it always happens: never both
	// suppressed with nothing emitted (the original Krita bug) nor emitted twice.
	HandleRelease(uint16(evdev.KEY_LEFTSHIFT), 0, m, cfg,
		loopState, executor.Outputs{}, nil, stateMap, emittedTracker)
	if emittedTracker.IsDown("shift") {
		t.Fatal("shift should be tracked as up after release")
	}
}

// When a combo matches, any modifier that was transparently forwarded and
// participates in that combo must be consumed (released to the system)
// before the fired command runs, so it does not leak into e.g. injected
// scroll events.
func TestMatchedComboConsumesHeldModifier(t *testing.T) {
	shortcut := &config.ParsedShortcut{
		KeyCombo: "ctrl+g",
		Behavior: config.BehaviorNormal,
		Commands: []string{"notify-send test"},
	}
	cfg := &config.Config{
		ParsedShortcuts: map[string][]*config.ParsedShortcut{"ctrl+g": {shortcut}},
		EscapeMap:       map[string]bool{},
	}
	m := matcher.New(cfg.ParsedShortcuts)
	stateMap := timers.NewStateMap()
	emittedTracker := timers.NewEmittedModifierTracker()
	loopState := executor.NewLoopState()

	// ctrl pressed alone first: forwarded transparently.
	HandlePress(uint16(evdev.KEY_LEFTCTRL), 1, m, cfg, loopState, executor.Outputs{}, nil, stateMap, emittedTracker)
	if !emittedTracker.IsDown("ctrl") {
		t.Fatal("ctrl should be marked down after being forwarded")
	}

	// 'g' completes ctrl+g: single candidate, no timers, so the ladder fires
	// immediately — but Run() still executes in its own goroutine, so wait
	// for it to finish (stateMap entry cleared) before asserting.
	HandlePress(uint16(evdev.KEY_G), 1, m, cfg, loopState, executor.Outputs{}, nil, stateMap, emittedTracker)
	waitForLadderDone(t, stateMap, "ctrl+g")

	if emittedTracker.IsDown("ctrl") {
		t.Fatal("ctrl should have been consumed (marked up) once ctrl+g matched")
	}
}

// waitForLadderDone polls until the ladder goroutine for combo has finished
// (stateMap entry cleared via its deferred Delete), since ladder.Run always
// executes asynchronously even when it resolves immediately.
func waitForLadderDone(t *testing.T, stateMap *timers.StateMap, combo string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if stateMap.Get(combo) == nil {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("ladder for %s did not finish within timeout", combo)
}

// A key that does not match any combo, arriving while a modifier was
// consumed by an earlier match but is still physically held, must have the
// modifier restored first so e.g. ctrl+c still works after ctrl+g fired.
func TestNonComboKeyRestoresConsumedModifier(t *testing.T) {
	comboShortcut := &config.ParsedShortcut{
		KeyCombo: "ctrl+g",
		Behavior: config.BehaviorNormal,
		Commands: []string{"notify-send test"},
	}
	cfg := &config.Config{
		ParsedShortcuts: map[string][]*config.ParsedShortcut{"ctrl+g": {comboShortcut}},
		EscapeMap:       map[string]bool{},
	}
	m := matcher.New(cfg.ParsedShortcuts)
	stateMap := timers.NewStateMap()
	emittedTracker := timers.NewEmittedModifierTracker()
	loopState := executor.NewLoopState()

	HandlePress(uint16(evdev.KEY_LEFTCTRL), 1, m, cfg, loopState, executor.Outputs{}, nil, stateMap, emittedTracker)
	HandlePress(uint16(evdev.KEY_G), 1, m, cfg, loopState, executor.Outputs{}, nil, stateMap, emittedTracker)
	waitForLadderDone(t, stateMap, "ctrl+g")
	if emittedTracker.IsDown("ctrl") {
		t.Fatal("setup: ctrl should have been consumed by ctrl+g")
	}

	// ctrl+c is not configured, so it should just forward "c" — but ctrl
	// must be re-asserted first since it's still physically held.
	if suppressed := HandlePress(uint16(evdev.KEY_C), 1, m, cfg,
		loopState, executor.Outputs{}, nil, stateMap, emittedTracker); suppressed {
		t.Fatal("unconfigured ctrl+c should be forwarded, not suppressed")
	}
	if !emittedTracker.IsDown("ctrl") {
		t.Fatal("ctrl should have been restored before forwarding the unmatched key")
	}
}
