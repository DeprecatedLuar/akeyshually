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
	pid, err := internal.GetRunningDaemonPid()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to check daemon status: %v\n", err)
		os.Exit(1)
	}

	if pid > 0 {
		fmt.Fprintf(os.Stderr, "Errm... akeyshually, the daemon is already running (PID: %d)\n", pid)
		os.Exit(1)
	}

	// Clean up stale pidfile if exists
	internal.RemovePidFile()

	// Daemonize
	if err := internal.Daemonize(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start daemon: %v\n", err)
		os.Exit(1)
	}
}
