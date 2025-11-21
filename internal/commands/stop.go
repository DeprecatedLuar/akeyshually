package commands

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/deprecatedluar/akeyshually/internal"
)

// Stop implements the stop command
// Stops both systemd service AND manual daemon instances
func Stop() {
	stopped := false

	// Stop systemd service if active
	hasService, err := internal.HasSystemdService()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to check systemd service: %v\n", err)
	} else if hasService {
		cmd := exec.Command("systemctl", "--user", "stop", "akeyshually")
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to stop systemd service: %v\n", err)
		} else {
			fmt.Println("Stopped systemd service")
			stopped = true
		}
	}

	// Also check for manual daemon and stop it
	pid, err := internal.GetRunningDaemonPid()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to check daemon status: %v\n", err)
	} else if pid > 0 {
		// Send SIGTERM to the process
		if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to stop daemon (PID: %d): %v\n", pid, err)
		} else {
			// Remove pidfile
			if err := internal.RemovePidFile(); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to remove pidfile: %v\n", err)
			}
			fmt.Printf("Stopped manual daemon (PID: %d)\n", pid)
			stopped = true
		}
	}

	// If nothing was stopped, exit with error
	if !stopped {
		fmt.Fprintf(os.Stderr, "akeyshually, there is nothing running\n")
		os.Exit(1)
	}
}
