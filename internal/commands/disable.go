package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/deprecatedluar/akeyshually/internal/config"
)

// Disable removes an overlay from the enabled list and restarts the daemon
func Disable(filename string) {
	if !strings.HasSuffix(filename, ".toml") {
		filename += ".toml"
	}

	// Check if overlay is actually enabled
	enabled, err := config.ReadEnabledState()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read enabled state: %v\n", err)
		os.Exit(1)
	}

	found := false
	for _, e := range enabled {
		if e == filename {
			found = true
			break
		}
	}

	if !found {
		fmt.Fprintf(os.Stderr, "Overlay not enabled: %s\n", filename)
		os.Exit(1)
	}

	if err := config.RemoveOverlay(filename); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to disable overlay: %v\n", err)
		os.Exit(1)
	}

	notifyOverlayChange(fmt.Sprintf("Disabled %s", filename))
	restartIfRunning()
}
