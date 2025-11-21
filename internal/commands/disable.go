package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/deprecatedluar/akeyshually/internal"
	"github.com/deprecatedluar/akeyshually/internal/config"
)

// Disable removes an overlay from the enabled list and restarts the daemon
func Disable(filename string) {
	if !strings.HasSuffix(filename, ".toml") {
		filename += ".toml"
	}

	// Check if overlay is actually enabled
	enabled, err := internal.ReadEnabledState()
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

	if err := internal.RemoveOverlay(filename); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to disable overlay: %v\n", err)
		os.Exit(1)
	}

	// Notify if configured
	if cfg, err := config.Load(); err == nil && cfg.Settings.NotifyOnOverlayChange {
		internal.NotifyInfo("akeyshually", fmt.Sprintf("Disabled %s", filename))
	}

	// Restart daemon only if it's running
	pid, err := internal.GetRunningDaemonPid()
	if err == nil && pid > 0 {
		Restart()
	}
}
