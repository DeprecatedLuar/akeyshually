package commands

import (
	"fmt"
	"os"

	"github.com/deprecatedluar/akeyshually/internal"
)

// Start implements the start command
// Checks if daemon is already running, then daemonizes
func Start() {
	// Check if already running
	pid, err := internal.ReadPidFile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to check daemon status: %v\n", err)
		os.Exit(1)
	}

	if pid > 0 && internal.IsProcessRunning(pid) {
		fmt.Fprintf(os.Stderr, "Errm... akeyshually, the daemon is already running (PID: %d)\n", pid)
		os.Exit(1)
	}

	// Clean up stale pidfile if exists
	if pid > 0 {
		if err := internal.RemovePidFile(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to remove stale pidfile: %v\n", err)
		}
	}

	// Daemonize
	if err := internal.Daemonize(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start daemon: %v\n", err)
		os.Exit(1)
	}
}
