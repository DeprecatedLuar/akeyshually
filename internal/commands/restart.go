package commands

import (
	"os"
	"os/exec"
)

// Restart implements the restart command
// Stops the daemon and then starts it again
func Restart() {
	// Stop (handles both systemd and manual mode)
	stopCmd := exec.Command(os.Args[0], "stop")
	stopCmd.Stdout = os.Stdout
	stopCmd.Stderr = os.Stderr
	stopCmd.Run() // Ignore error - daemon might not be running

	// Start (handles both systemd and manual mode)
	startCmd := exec.Command(os.Args[0], "start")
	startCmd.Stdout = os.Stdout
	startCmd.Stderr = os.Stderr
	if err := startCmd.Run(); err != nil {
		os.Exit(1)
	}
}
