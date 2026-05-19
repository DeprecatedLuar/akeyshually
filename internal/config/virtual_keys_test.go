package config

import (
	"testing"
)

func TestExpandVirtualKeys(t *testing.T) {
	tests := []struct {
		name        string
		virtualKeys map[string]interface{}
		shortcuts   map[string]interface{}
		expected    map[string]interface{}
	}{
		{
			name: "simple virtual key expansion",
			virtualKeys: map[string]interface{}{
				"action": []interface{}{"playcd", "pausecd"},
			},
			shortcuts: map[string]interface{}{
				"action":      "cmd1",
				"action.hold": "cmd2",
			},
			expected: map[string]interface{}{
				"playcd":      "cmd1",
				"pausecd":     "cmd1",
				"playcd.hold": "cmd2",
				"pausecd.hold": "cmd2",
			},
		},
		{
			name: "virtual key with modifiers",
			virtualKeys: map[string]interface{}{
				"action": []interface{}{"playcd", "pausecd"},
			},
			shortcuts: map[string]interface{}{
				"super+action":      "cmd1",
				"super+action.hold": "cmd2",
			},
			expected: map[string]interface{}{
				"super+playcd":      "cmd1",
				"super+pausecd":     "cmd1",
				"super+playcd.hold": "cmd2",
				"super+pausecd.hold": "cmd2",
			},
		},
		{
			name: "virtual key with string value",
			virtualKeys: map[string]interface{}{
				"media": "playpause",
			},
			shortcuts: map[string]interface{}{
				"media": "cmd1",
			},
			expected: map[string]interface{}{
				"playpause": "cmd1",
			},
		},
		{
			name: "no virtual keys",
			virtualKeys: map[string]interface{}{},
			shortcuts: map[string]interface{}{
				"super+k": "cmd1",
			},
			expected: map[string]interface{}{
				"super+k": "cmd1",
			},
		},
		{
			name: "mixed virtual and regular keys",
			virtualKeys: map[string]interface{}{
				"action": []interface{}{"playcd", "pausecd"},
			},
			shortcuts: map[string]interface{}{
				"action":  "cmd1",
				"super+k": "cmd2",
			},
			expected: map[string]interface{}{
				"playcd":  "cmd1",
				"pausecd": "cmd1",
				"super+k": "cmd2",
			},
		},
		{
			name: "virtual key in alias",
			virtualKeys: map[string]interface{}{
				"action": []interface{}{"playcd", "pausecd"},
			},
			shortcuts: map[string]interface{}{
				"action/f1.hold": "cmd1",
			},
			expected: map[string]interface{}{
				"playcd.hold": "cmd1",
				"pausecd.hold": "cmd1",
				"f1.hold":     "cmd1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				VirtualKeys: tt.virtualKeys,
				Shortcuts:   tt.shortcuts,
			}

			err := expandVirtualKeys(cfg)
			if err != nil {
				t.Fatalf("expandVirtualKeys failed: %v", err)
			}

			// Check that all expected shortcuts are present
			for expectedKey, expectedValue := range tt.expected {
				actualValue, ok := cfg.Shortcuts[expectedKey]
				if !ok {
					t.Errorf("expected shortcut %q not found", expectedKey)
					continue
				}
				if actualValue != expectedValue {
					t.Errorf("shortcut %q: expected %v, got %v", expectedKey, expectedValue, actualValue)
				}
			}

			// Check that no extra shortcuts are present
			for actualKey := range cfg.Shortcuts {
				if _, ok := tt.expected[actualKey]; !ok {
					t.Errorf("unexpected shortcut %q found", actualKey)
				}
			}
		})
	}
}

func TestExpandVirtualKeysErrors(t *testing.T) {
	tests := []struct {
		name        string
		virtualKeys map[string]interface{}
		wantErr     bool
	}{
		{
			name: "invalid value type",
			virtualKeys: map[string]interface{}{
				"action": 123,
			},
			wantErr: true,
		},
		{
			name: "array with non-string element",
			virtualKeys: map[string]interface{}{
				"action": []interface{}{"playcd", 123},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				VirtualKeys: tt.virtualKeys,
				Shortcuts:   make(map[string]interface{}),
			}

			err := expandVirtualKeys(cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("expandVirtualKeys error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
