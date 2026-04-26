package commands

import (
	"fmt"
	"os"

	daemon "github.com/deprecatedluar/luar-daemonator"
)

// Start implements the start command
// Checks if daemon is already running, then daemonizes
func Start() {
	d := daemon.New("akeyshually")

	if d.IsRunning() {
		fmt.Fprintf(os.Stderr, "Errm... akeyshually, the daemon is already running\n")
		os.Exit(1)
	}

	if err := d.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start daemon: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Alright, the daemon started\n")
}
