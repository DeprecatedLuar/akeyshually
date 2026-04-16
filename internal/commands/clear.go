package commands

import (
	"fmt"
	"os"

	"github.com/deprecatedluar/akeyshually/internal/config"
)

// Clear disables all overlays and restarts the daemon
func Clear() {
	if err := config.ClearAllOverlays(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to clear overlays: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("All overlays disabled")
	restartIfRunning()
}
