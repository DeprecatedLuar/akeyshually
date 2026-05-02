package ladder

import (
	"testing"

	"github.com/deprecatedluar/akeyshually/internal/config"
	"github.com/deprecatedluar/akeyshually/internal/timers"
)

// TestIsEliminated_Normal tests elimination rules for BehaviorNormal
func TestIsEliminated_Normal(t *testing.T) {
	tests := []struct {
		name    string
		count   int
		pressed bool
		phase   int
		hasHold bool
		want    bool
	}{
		// Eliminated by second press (count > 1 is doubletap territory)
		{"count=2", 2, true, 0, false, true},
		{"count=2 released", 2, false, 0, false, true},

		// Eliminated by holding past threshold IF hold is competing
		{"phase=1 pressed hasHold", 1, true, 1, true, true},
		{"phase=1 pressed no hold", 1, true, 1, false, false},
		{"phase=1 released hasHold", 1, false, 1, true, false},
		{"phase=0 pressed hasHold", 1, true, 0, true, false},

		// Normal scenarios where it survives
		{"count=1 pressed phase=0", 1, true, 0, false, false},
		{"count=1 released phase=0", 1, false, 0, false, false},
		{"count=1 released phase=1 no hold", 1, false, 1, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isEliminated(config.BehaviorNormal, tt.count, tt.pressed, tt.phase, tt.hasHold)
			if got != tt.want {
				t.Errorf("isEliminated(Normal, count=%d, pressed=%v, phase=%d, hasHold=%v) = %v, want %v",
					tt.count, tt.pressed, tt.phase, tt.hasHold, got, tt.want)
			}
		})
	}
}

// TestIsEliminated_PressRelease tests elimination rules for BehaviorPressRelease
// (same rules as Normal)
func TestIsEliminated_PressRelease(t *testing.T) {
	tests := []struct {
		name    string
		count   int
		pressed bool
		phase   int
		hasHold bool
		want    bool
	}{
		{"count=2", 2, true, 0, false, true},
		{"phase=1 pressed hasHold", 1, true, 1, true, true},
		{"phase=1 pressed no hold", 1, true, 1, false, false},
		{"count=1 released", 1, false, 0, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isEliminated(config.BehaviorPressRelease, tt.count, tt.pressed, tt.phase, tt.hasHold)
			if got != tt.want {
				t.Errorf("isEliminated(PressRelease) = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsEliminated_Hold tests elimination rules for BehaviorHold family
func TestIsEliminated_Hold(t *testing.T) {
	behaviors := []config.BehaviorMode{
		config.BehaviorHold,
		config.BehaviorHoldRelease,
		config.BehaviorLongPress,
	}

	tests := []struct {
		name    string
		count   int
		pressed bool
		phase   int
		want    bool
	}{
		// Eliminated by releasing before threshold
		{"released before threshold", 1, false, 0, true},

		// NOT eliminated after threshold
		{"released after threshold", 1, false, 1, false},
		{"pressed before threshold", 1, true, 0, false},
		{"pressed after threshold", 1, true, 1, false},

		// NOT eliminated by count > 1 (can hold through second press)
		{"count=2 pressed phase=1", 2, true, 1, false},
	}

	for _, behavior := range behaviors {
		for _, tt := range tests {
			t.Run(behaviorName(behavior)+"_"+tt.name, func(t *testing.T) {
				got := isEliminated(behavior, tt.count, tt.pressed, tt.phase, false)
				if got != tt.want {
					t.Errorf("isEliminated(%s) = %v, want %v", behaviorName(behavior), got, tt.want)
				}
			})
		}
	}
}

// TestIsEliminated_DoubleTap tests elimination rules for BehaviorDoubleTap
func TestIsEliminated_DoubleTap(t *testing.T) {
	tests := []struct {
		name    string
		count   int
		pressed bool
		phase   int
		want    bool
	}{
		// Eliminated by window expiry with no second press
		{"window expired count=1", 1, false, 1, true},
		{"window expired count=1 pressed", 1, true, 1, true},

		// NOT eliminated within window
		{"within window released", 1, false, 0, false},
		{"within window pressed", 1, true, 0, false},

		// NOT eliminated when second press arrived (before phase 2)
		{"count=2 phase=0", 2, true, 0, false},
		{"count=2 phase=1", 2, true, 1, false},
		{"count=2 phase=1 released", 2, false, 1, false},

		// Eliminated by holding past phase 2 (taphold wins)
		{"count=2 phase=2", 2, true, 2, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isEliminated(config.BehaviorDoubleTap, tt.count, tt.pressed, tt.phase, false)
			if got != tt.want {
				t.Errorf("isEliminated(DoubleTap, count=%d, pressed=%v, phase=%d) = %v, want %v",
					tt.count, tt.pressed, tt.phase, got, tt.want)
			}
		})
	}
}

// TestIsEliminated_TapHold tests elimination rules for BehaviorTapHold
func TestIsEliminated_TapHold(t *testing.T) {
	tests := []struct {
		name    string
		count   int
		pressed bool
		phase   int
		want    bool
	}{
		// Eliminated by releasing after tap window without second press
		{"released phase=1 count=1", 1, false, 1, true},

		// NOT eliminated by releasing on first press (completing the "tap" part)
		{"released phase=0 count=1", 1, false, 0, false},

		// Eliminated by releasing on second press before hold threshold (doubletap wins)
		{"count=2 released phase=1", 2, false, 1, true},

		// Eliminated by holding past phase 2 on first press (hold wins, not taphold)
		{"count=1 pressed phase=2", 1, true, 2, true},

		// Survives when held on second press
		{"count=2 pressed phase=2", 2, true, 2, false},
		{"count=2 pressed phase=1", 2, true, 1, false},
		{"count=2 pressed phase=0", 2, true, 0, false},

		// Survives within tap window on first press
		{"count=1 pressed phase=0", 1, true, 0, false},
		{"count=1 pressed phase=1", 1, true, 1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isEliminated(config.BehaviorTapHold, tt.count, tt.pressed, tt.phase, false)
			if got != tt.want {
				t.Errorf("isEliminated(TapHold, count=%d, pressed=%v, phase=%d) = %v, want %v",
					tt.count, tt.pressed, tt.phase, got, tt.want)
			}
		})
	}
}

// TestIsEliminated_TapLongPress tests elimination rules for BehaviorTapLongPress
// (same rules as TapHold)
func TestIsEliminated_TapLongPress(t *testing.T) {
	tests := []struct {
		name    string
		count   int
		pressed bool
		phase   int
		want    bool
	}{
		{"released phase=1 count=1", 1, false, 1, true},
		{"released phase=0 count=1", 1, false, 0, false},
		{"count=2 released phase=1", 2, false, 1, true},
		{"count=1 pressed phase=2", 1, true, 2, true},
		{"count=2 pressed phase=2", 2, true, 2, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isEliminated(config.BehaviorTapLongPress, tt.count, tt.pressed, tt.phase, false)
			if got != tt.want {
				t.Errorf("isEliminated(TapLongPress) = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsEliminated_EscapePending tests elimination rules for BehaviorEscapePending
func TestIsEliminated_EscapePending(t *testing.T) {
	tests := []struct {
		name    string
		pressed bool
		want    bool
	}{
		{"pressed", true, false},
		{"released", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isEliminated(config.BehaviorEscapePending, 1, tt.pressed, 0, false)
			if got != tt.want {
				t.Errorf("isEliminated(EscapePending, pressed=%v) = %v, want %v",
					tt.pressed, got, tt.want)
			}
		})
	}
}

// TestPruning_NormalSolo tests solo normal behavior
func TestPruning_NormalSolo(t *testing.T) {
	candidates := []timers.Candidate{
		{Shortcut: &config.ParsedShortcut{Behavior: config.BehaviorNormal}},
	}

	// Solo normal should win immediately on press (no timer needed)
	pruned := pruneCandidates(candidates, 1, true, 0, false)
	if len(pruned) != 1 {
		t.Errorf("solo normal on press: got %d candidates, want 1", len(pruned))
	}

	// Solo normal should win on release too
	pruned = pruneCandidates(candidates, 1, false, 0, false)
	if len(pruned) != 1 {
		t.Errorf("solo normal on release: got %d candidates, want 1", len(pruned))
	}
}

// TestPruning_NormalPlusDoubleTap tests normal + doubletap competition
func TestPruning_NormalPlusDoubleTap(t *testing.T) {
	candidates := []timers.Candidate{
		{Shortcut: &config.ParsedShortcut{Behavior: config.BehaviorNormal}},
		{Shortcut: &config.ParsedShortcut{Behavior: config.BehaviorDoubleTap}},
	}

	tests := []struct {
		name       string
		count      int
		pressed    bool
		phase      int
		wantCount  int
		wantWinner config.BehaviorMode
	}{
		{
			name:      "first press - both survive",
			count:     1,
			pressed:   true,
			phase:     0,
			wantCount: 2,
		},
		{
			name:      "first release - both survive",
			count:     1,
			pressed:   false,
			phase:     0,
			wantCount: 2,
		},
		{
			name:       "window expires - normal wins",
			count:      1,
			pressed:    false,
			phase:      1,
			wantCount:  1,
			wantWinner: config.BehaviorNormal,
		},
		{
			name:       "second press - doubletap wins",
			count:      2,
			pressed:    true,
			phase:      0,
			wantCount:  1,
			wantWinner: config.BehaviorDoubleTap,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pruned := pruneCandidates(candidates, tt.count, tt.pressed, tt.phase, false)
			if len(pruned) != tt.wantCount {
				t.Errorf("got %d candidates, want %d", len(pruned), tt.wantCount)
			}
			if tt.wantCount == 1 && pruned[0].Shortcut.Behavior != tt.wantWinner {
				t.Errorf("winner = %s, want %s",
					behaviorName(pruned[0].Shortcut.Behavior),
					behaviorName(tt.wantWinner))
			}
		})
	}
}

// TestPruning_NormalPlusHold tests normal + hold competition
func TestPruning_NormalPlusHold(t *testing.T) {
	candidates := []timers.Candidate{
		{Shortcut: &config.ParsedShortcut{Behavior: config.BehaviorNormal}},
		{Shortcut: &config.ParsedShortcut{Behavior: config.BehaviorHold}},
	}

	tests := []struct {
		name       string
		count      int
		pressed    bool
		phase      int
		wantCount  int
		wantWinner config.BehaviorMode
	}{
		{
			name:       "release before threshold - normal wins",
			count:      1,
			pressed:    false,
			phase:      0,
			wantCount:  1,
			wantWinner: config.BehaviorNormal,
		},
		{
			name:       "held past threshold - hold wins",
			count:      1,
			pressed:    true,
			phase:      1,
			wantCount:  1,
			wantWinner: config.BehaviorHold,
		},
		{
			name:      "pressed before threshold - both survive",
			count:     1,
			pressed:   true,
			phase:     0,
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pruned := pruneCandidates(candidates, tt.count, tt.pressed, tt.phase, true)
			if len(pruned) != tt.wantCount {
				t.Errorf("got %d candidates, want %d", len(pruned), tt.wantCount)
			}
			if tt.wantCount == 1 && pruned[0].Shortcut.Behavior != tt.wantWinner {
				t.Errorf("winner = %s, want %s",
					behaviorName(pruned[0].Shortcut.Behavior),
					behaviorName(tt.wantWinner))
			}
		})
	}
}

// TestPruning_PressReleasePlusHold tests pressrelease + hold competition
func TestPruning_PressReleasePlusHold(t *testing.T) {
	candidates := []timers.Candidate{
		{Shortcut: &config.ParsedShortcut{Behavior: config.BehaviorPressRelease}},
		{Shortcut: &config.ParsedShortcut{Behavior: config.BehaviorHold}},
	}

	tests := []struct {
		name       string
		count      int
		pressed    bool
		phase      int
		wantCount  int
		wantWinner config.BehaviorMode
	}{
		{
			name:       "release before threshold - pressrelease wins",
			count:      1,
			pressed:    false,
			phase:      0,
			wantCount:  1,
			wantWinner: config.BehaviorPressRelease,
		},
		{
			name:       "held past threshold - hold wins",
			count:      1,
			pressed:    true,
			phase:      1,
			wantCount:  1,
			wantWinner: config.BehaviorHold,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pruned := pruneCandidates(candidates, tt.count, tt.pressed, tt.phase, true)
			if len(pruned) != tt.wantCount {
				t.Errorf("got %d candidates, want %d", len(pruned), tt.wantCount)
			}
			if tt.wantCount == 1 && pruned[0].Shortcut.Behavior != tt.wantWinner {
				t.Errorf("winner = %s, want %s",
					behaviorName(pruned[0].Shortcut.Behavior),
					behaviorName(tt.wantWinner))
			}
		})
	}
}

// TestPruning_DoubleTapSolo tests solo doubletap behavior
func TestPruning_DoubleTapSolo(t *testing.T) {
	candidates := []timers.Candidate{
		{Shortcut: &config.ParsedShortcut{Behavior: config.BehaviorDoubleTap}},
	}

	tests := []struct {
		name      string
		count     int
		pressed   bool
		phase     int
		wantCount int
	}{
		{
			name:      "second press within window - wins",
			count:     2,
			pressed:   true,
			phase:     0,
			wantCount: 1,
		},
		{
			name:      "window expires no second press - no winner",
			count:     1,
			pressed:   false,
			phase:     1,
			wantCount: 0,
		},
		{
			name:      "first press - survives",
			count:     1,
			pressed:   true,
			phase:     0,
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pruned := pruneCandidates(candidates, tt.count, tt.pressed, tt.phase, false)
			if len(pruned) != tt.wantCount {
				t.Errorf("got %d candidates, want %d", len(pruned), tt.wantCount)
			}
		})
	}
}

// TestPruning_HoldSolo tests solo hold behavior
func TestPruning_HoldSolo(t *testing.T) {
	candidates := []timers.Candidate{
		{Shortcut: &config.ParsedShortcut{Behavior: config.BehaviorHold}},
	}

	tests := []struct {
		name      string
		count     int
		pressed   bool
		phase     int
		wantCount int
	}{
		{
			name:      "held past threshold - wins",
			count:     1,
			pressed:   true,
			phase:     1,
			wantCount: 1,
		},
		{
			name:      "released before threshold - no winner",
			count:     1,
			pressed:   false,
			phase:     0,
			wantCount: 0,
		},
		{
			name:      "pressed before threshold - survives",
			count:     1,
			pressed:   true,
			phase:     0,
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pruned := pruneCandidates(candidates, tt.count, tt.pressed, tt.phase, false)
			if len(pruned) != tt.wantCount {
				t.Errorf("got %d candidates, want %d", len(pruned), tt.wantCount)
			}
		})
	}
}

// TestPruning_ThreeWay tests normal + doubletap + hold three-way competition
// These tests simulate progressive pruning as events occur
func TestPruning_ThreeWay(t *testing.T) {
	t.Run("initial press - all survive", func(t *testing.T) {
		candidates := []timers.Candidate{
			{Shortcut: &config.ParsedShortcut{Behavior: config.BehaviorNormal}},
			{Shortcut: &config.ParsedShortcut{Behavior: config.BehaviorDoubleTap}},
			{Shortcut: &config.ParsedShortcut{Behavior: config.BehaviorHold}},
		}
		pruned := pruneCandidates(candidates, 1, true, 0, true)
		if len(pruned) != 3 {
			t.Errorf("got %d candidates, want 3", len(pruned))
		}
	})

	t.Run("release before threshold - hold eliminated", func(t *testing.T) {
		candidates := []timers.Candidate{
			{Shortcut: &config.ParsedShortcut{Behavior: config.BehaviorNormal}},
			{Shortcut: &config.ParsedShortcut{Behavior: config.BehaviorDoubleTap}},
			{Shortcut: &config.ParsedShortcut{Behavior: config.BehaviorHold}},
		}
		pruned := pruneCandidates(candidates, 1, false, 0, true)
		if len(pruned) != 2 {
			t.Errorf("got %d candidates, want 2 (candidates: %s)", len(pruned), formatCandidates(pruned))
		}
	})

	t.Run("release before threshold then window expires - normal wins", func(t *testing.T) {
		// First prune: release at phase 0
		candidates := []timers.Candidate{
			{Shortcut: &config.ParsedShortcut{Behavior: config.BehaviorNormal}},
			{Shortcut: &config.ParsedShortcut{Behavior: config.BehaviorDoubleTap}},
			{Shortcut: &config.ParsedShortcut{Behavior: config.BehaviorHold}},
		}
		candidates = pruneCandidates(candidates, 1, false, 0, true)
		// Second prune: timer expires to phase 1
		candidates = pruneCandidates(candidates, 1, false, 1, true)
		if len(candidates) != 1 {
			t.Errorf("got %d candidates, want 1 (candidates: %s)", len(candidates), formatCandidates(candidates))
		}
		if candidates[0].Shortcut.Behavior != config.BehaviorNormal {
			t.Errorf("winner = %s, want normal", behaviorName(candidates[0].Shortcut.Behavior))
		}
	})

	t.Run("held past threshold - hold wins", func(t *testing.T) {
		// Timer expires while held continuously (no second press)
		// Normal eliminated: phase >= 1 AND pressed AND hasHold
		// DoubleTap eliminated: phase >= 1 AND count < 2 (window expired, no second press)
		// Hold survives: threshold reached while pressed
		candidates := []timers.Candidate{
			{Shortcut: &config.ParsedShortcut{Behavior: config.BehaviorNormal}},
			{Shortcut: &config.ParsedShortcut{Behavior: config.BehaviorDoubleTap}},
			{Shortcut: &config.ParsedShortcut{Behavior: config.BehaviorHold}},
		}
		candidates = pruneCandidates(candidates, 1, true, 1, true)
		if len(candidates) != 1 {
			t.Errorf("got %d candidates, want 1 (candidates: %s)", len(candidates), formatCandidates(candidates))
		}
		if candidates[0].Shortcut.Behavior != config.BehaviorHold {
			t.Errorf("winner = %s, want hold", behaviorName(candidates[0].Shortcut.Behavior))
		}
	})

	t.Run("second press before threshold - normal eliminated", func(t *testing.T) {
		candidates := []timers.Candidate{
			{Shortcut: &config.ParsedShortcut{Behavior: config.BehaviorNormal}},
			{Shortcut: &config.ParsedShortcut{Behavior: config.BehaviorDoubleTap}},
			{Shortcut: &config.ParsedShortcut{Behavior: config.BehaviorHold}},
		}
		candidates = pruneCandidates(candidates, 2, true, 0, true)
		if len(candidates) != 2 {
			t.Errorf("got %d candidates, want 2", len(candidates))
		}
		// Should have doubletap + hold
		hasHold := false
		hasDoubleTap := false
		for _, c := range candidates {
			if c.Shortcut.Behavior == config.BehaviorHold {
				hasHold = true
			}
			if c.Shortcut.Behavior == config.BehaviorDoubleTap {
				hasDoubleTap = true
			}
		}
		if !hasHold || !hasDoubleTap {
			t.Errorf("expected doubletap+hold, got %s", formatCandidates(candidates))
		}
	})

	t.Run("second press then released - doubletap wins", func(t *testing.T) {
		candidates := []timers.Candidate{
			{Shortcut: &config.ParsedShortcut{Behavior: config.BehaviorNormal}},
			{Shortcut: &config.ParsedShortcut{Behavior: config.BehaviorDoubleTap}},
			{Shortcut: &config.ParsedShortcut{Behavior: config.BehaviorHold}},
		}
		candidates = pruneCandidates(candidates, 2, false, 0, true)
		if len(candidates) != 1 {
			t.Errorf("got %d candidates, want 1", len(candidates))
		}
		if candidates[0].Shortcut.Behavior != config.BehaviorDoubleTap {
			t.Errorf("winner = %s, want doubletap", behaviorName(candidates[0].Shortcut.Behavior))
		}
	})
}

// TestPruning_LongPressPlusNormal tests longpress + normal (same as hold + normal)
func TestPruning_LongPressPlusNormal(t *testing.T) {
	candidates := []timers.Candidate{
		{Shortcut: &config.ParsedShortcut{Behavior: config.BehaviorNormal}},
		{Shortcut: &config.ParsedShortcut{Behavior: config.BehaviorLongPress}},
	}

	tests := []struct {
		name       string
		count      int
		pressed    bool
		phase      int
		wantCount  int
		wantWinner config.BehaviorMode
	}{
		{
			name:       "release before threshold - normal wins",
			count:      1,
			pressed:    false,
			phase:      0,
			wantCount:  1,
			wantWinner: config.BehaviorNormal,
		},
		{
			name:       "held past threshold - longpress wins",
			count:      1,
			pressed:    true,
			phase:      1,
			wantCount:  1,
			wantWinner: config.BehaviorLongPress,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pruned := pruneCandidates(candidates, tt.count, tt.pressed, tt.phase, true)
			if len(pruned) != tt.wantCount {
				t.Errorf("got %d candidates, want %d", len(pruned), tt.wantCount)
			}
			if tt.wantCount == 1 && pruned[0].Shortcut.Behavior != tt.wantWinner {
				t.Errorf("winner = %s, want %s",
					behaviorName(pruned[0].Shortcut.Behavior),
					behaviorName(tt.wantWinner))
			}
		})
	}
}

// TestPruning_PressReleaseSolo tests solo pressrelease behavior
func TestPruning_PressReleaseSolo(t *testing.T) {
	candidates := []timers.Candidate{
		{Shortcut: &config.ParsedShortcut{Behavior: config.BehaviorPressRelease}},
	}

	// Solo pressrelease should win on both press and release
	pruned := pruneCandidates(candidates, 1, true, 0, false)
	if len(pruned) != 1 {
		t.Errorf("solo pressrelease on press: got %d candidates, want 1", len(pruned))
	}

	pruned = pruneCandidates(candidates, 1, false, 0, false)
	if len(pruned) != 1 {
		t.Errorf("solo pressrelease on release: got %d candidates, want 1", len(pruned))
	}
}

// TestPruning_LongPressSolo tests solo longpress behavior
func TestPruning_LongPressSolo(t *testing.T) {
	candidates := []timers.Candidate{
		{Shortcut: &config.ParsedShortcut{Behavior: config.BehaviorLongPress}},
	}

	tests := []struct {
		name      string
		count     int
		pressed   bool
		phase     int
		wantCount int
	}{
		{
			name:      "held past threshold - wins",
			count:     1,
			pressed:   true,
			phase:     1,
			wantCount: 1,
		},
		{
			name:      "released before threshold - no winner",
			count:     1,
			pressed:   false,
			phase:     0,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pruned := pruneCandidates(candidates, tt.count, tt.pressed, tt.phase, false)
			if len(pruned) != tt.wantCount {
				t.Errorf("got %d candidates, want %d", len(pruned), tt.wantCount)
			}
		})
	}
}

// TestBuildTimerLadder tests timer phase generation
func TestBuildTimerLadder(t *testing.T) {
	tests := []struct {
		name         string
		candidates   []timers.Candidate
		defaultInterval float64
		wantPhases   int
		wantDurations []int // in milliseconds
	}{
		{
			name: "normal only - no timers",
			candidates: []timers.Candidate{
				{Shortcut: &config.ParsedShortcut{Behavior: config.BehaviorNormal}},
			},
			defaultInterval: 200,
			wantPhases:      0,
		},
		{
			name: "doubletap only - one phase",
			candidates: []timers.Candidate{
				{Shortcut: &config.ParsedShortcut{
					Behavior: config.BehaviorDoubleTap,
					Interval: 300,
				}},
			},
			defaultInterval: 200,
			wantPhases:      1,
			wantDurations:   []int{300},
		},
		{
			name: "hold only - one phase",
			candidates: []timers.Candidate{
				{Shortcut: &config.ParsedShortcut{
					Behavior: config.BehaviorHold,
					Interval: 400,
				}},
			},
			defaultInterval: 200,
			wantPhases:      1,
			wantDurations:   []int{400},
		},
		{
			name: "taphold - two phases",
			candidates: []timers.Candidate{
				{Shortcut: &config.ParsedShortcut{
					Behavior:     config.BehaviorTapHold,
					Interval:     250,
					HoldInterval: 500,
				}},
			},
			defaultInterval: 200,
			wantPhases:      2,
			wantDurations:   []int{250, 500},
		},
		{
			name: "normal + doubletap - one phase",
			candidates: []timers.Candidate{
				{Shortcut: &config.ParsedShortcut{Behavior: config.BehaviorNormal}},
				{Shortcut: &config.ParsedShortcut{
					Behavior: config.BehaviorDoubleTap,
					Interval: 0, // use default
				}},
			},
			defaultInterval: 200,
			wantPhases:      1,
			wantDurations:   []int{200},
		},
		{
			name: "normal + hold - one phase",
			candidates: []timers.Candidate{
				{Shortcut: &config.ParsedShortcut{Behavior: config.BehaviorNormal}},
				{Shortcut: &config.ParsedShortcut{
					Behavior: config.BehaviorHold,
					Interval: 350,
				}},
			},
			defaultInterval: 200,
			wantPhases:      1,
			wantDurations:   []int{350},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ladder := buildTimerLadder(tt.candidates, tt.defaultInterval)
			if len(ladder) != tt.wantPhases {
				t.Errorf("got %d phases, want %d", len(ladder), tt.wantPhases)
			}
			for i, want := range tt.wantDurations {
				if i >= len(ladder) {
					t.Errorf("missing phase %d", i)
					continue
				}
				gotMs := ladder[i].Milliseconds()
				if gotMs != int64(want) {
					t.Errorf("phase %d: got %dms, want %dms", i, gotMs, want)
				}
			}
		})
	}
}
