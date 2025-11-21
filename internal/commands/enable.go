package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deprecatedluar/akeyshually/internal"
	"github.com/deprecatedluar/akeyshually/internal/config"
)

// Enable adds an overlay to the enabled list and restarts the daemon
func Enable(filename string) {
	// Validate filename ends with .toml
	if !strings.HasSuffix(filename, ".toml") {
		filename += ".toml"
	}

	// Check file exists in config dir
	configDir, err := config.GetConfigDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get config directory: %v\n", err)
		os.Exit(1)
	}

	overlayPath := filepath.Join(configDir, filename)
	if _, err := os.Stat(overlayPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Overlay not found: %s\n", filename)
		os.Exit(1)
	}

	// Add to enabled state
	if err := internal.AddOverlay(filename); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to enable overlay: %v\n", err)
		os.Exit(1)
	}

	// Notify if configured
	if cfg, err := config.Load(); err == nil && cfg.Settings.NotifyOnOverlayChange {
		internal.NotifyInfo("akeyshually", fmt.Sprintf("Enabled %s", filename))
	}

	// Restart daemon only if it's running
	pid, err := internal.GetRunningDaemonPid()
	if err == nil && pid > 0 {
		Restart()
	}
}
