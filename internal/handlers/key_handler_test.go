package handlers

import (
	"testing"

	"github.com/deprecatedluar/akeyshually/internal/config"
	"github.com/deprecatedluar/akeyshually/internal/executor"
	"github.com/deprecatedluar/akeyshually/internal/matcher"
	"github.com/deprecatedluar/akeyshually/internal/timers"
	evdev "github.com/holoplot/go-evdev"
)

func TestModifierWithConfiguredChildIsDeferred(t *testing.T) {
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
	if !suppressed {
		t.Fatal("ctrl press was forwarded before configured child combo resolved")
	}
	state := stateMap.Get("ctrl")
	if state == nil {
		t.Fatal("ctrl escape-pending ladder was not created")
	}
	state.Cancel()
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
