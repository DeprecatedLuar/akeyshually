package commands

import (
	"fmt"
	"os"
	"os/exec"
	"time"
)

// Restart implements the restart command
// Stops the daemon and then starts it again (silent operation)
func Restart() {
	// Stop (handles both systemd and manual mode) - suppress output
	stopCmd := exec.Command(os.Args[0], "stop")
	stopCmd.Run() // Ignore error and output - daemon might not be running

	// Wait briefly for process to fully exit
	time.Sleep(200 * time.Millisecond)

	// Start (handles both systemd and manual mode) - suppress output
	startCmd := exec.Command(os.Args[0], "start")
	if err := startCmd.Run(); err != nil {
		// Only show error if start fails
		fmt.Fprintf(os.Stderr, "Failed to restart daemon: %v\n", err)
		os.Exit(1)
	}
}
