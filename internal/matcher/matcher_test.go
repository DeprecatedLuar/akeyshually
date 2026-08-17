package matcher

import (
	"testing"

	"github.com/deprecatedluar/akeyshually/internal/config"
)

func TestNewExcludesAxisShortcutsFromKeyboardMatching(t *testing.T) {
	axisShortcut := &config.ParsedShortcut{
		KeyCombo:  "abs_x",
		Direction: "+",
		Commands:  []string{"ydotool mousemove -x 12 -y 0"},
	}
	m := New(map[string][]*config.ParsedShortcut{
		"abs_x+": {axisShortcut},
	})

	if got := m.GetShortcuts("x"); len(got) != 0 {
		t.Fatalf("keyboard x matched axis shortcut: %v", got)
	}
	if got := m.GetShortcuts("abs_x"); len(got) != 0 {
		t.Fatalf("axis shortcut entered keyboard matcher: %v", got)
	}
}
