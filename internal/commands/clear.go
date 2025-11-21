package commands

import (
	"fmt"
	"os"

	"github.com/deprecatedluar/akeyshually/internal"
)

// Clear disables all overlays and restarts the daemon
func Clear() {
	if err := internal.ClearAllOverlays(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to clear overlays: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("All overlays disabled")

	// Restart daemon only if it's running
	pid, err := internal.GetRunningDaemonPid()
	if err == nil && pid > 0 {
		Restart()
	}
}
