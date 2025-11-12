package main

import (
	"fmt"
	"os"

	"github.com/deprecatedluar/akeyshually/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Loaded %d shortcuts\n", len(cfg.ParsedShortcuts))
	fmt.Printf("\nParsed shortcuts:\n")
	for combo, shortcuts := range cfg.ParsedShortcuts {
		fmt.Printf("\nCombo: %q\n", combo)
		for _, s := range shortcuts {
			fmt.Printf("  - Behavior: %d, Timing: %d, Interval: %d, Commands: %v\n",
				s.Behavior, s.Timing, s.Interval, s.Commands)
		}
	}

	// Look for ctrl+k specifically
	if shortcuts, ok := cfg.ParsedShortcuts["ctrl+k"]; ok {
		fmt.Printf("\n✓ Found ctrl+k with %d variant(s)\n", len(shortcuts))
		for _, s := range shortcuts {
			fmt.Printf("  Behavior=%d (Loop=1), Timing=%d (Press=0)\n", s.Behavior, s.Timing)
		}
	} else {
		fmt.Printf("\n✗ ctrl+k NOT FOUND in parsed shortcuts\n")
		fmt.Printf("Available combos: ")
		for combo := range cfg.ParsedShortcuts {
			fmt.Printf("%q ", combo)
		}
		fmt.Println()
	}
}
