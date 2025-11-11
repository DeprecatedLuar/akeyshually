package commands

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/deprecatedluar/akeyshually/internal"
)

// Stop implements the stop command
// Checks if systemd service exists, otherwise stops manual daemon
func Stop() {
	// Check if systemd service is active
	hasService, err := internal.HasSystemdService()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to check systemd service: %v\n", err)
		os.Exit(1)
	}

	if hasService {
		// Use systemctl to stop the service
		cmd := exec.Command("systemctl", "--user", "stop", "akeyshually")
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to stop systemd service: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("akeyshually service stopped")
		return
	}

	// Manual daemon mode - use pidfile
	pid, err := internal.ReadPidFile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read pidfile: %v\n", err)
		os.Exit(1)
	}

	if pid <= 0 {
		fmt.Fprintf(os.Stderr, "akeyshually is not running (no pidfile found)\n")
		os.Exit(1)
	}

	if !internal.IsProcessRunning(pid) {
		fmt.Fprintf(os.Stderr, "akeyshually is not running (stale pidfile)\n")
		// Clean up stale pidfile
		internal.RemovePidFile()
		os.Exit(1)
	}

	// Send SIGTERM to the process
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to stop daemon (PID: %d): %v\n", pid, err)
		os.Exit(1)
	}

	// Remove pidfile
	if err := internal.RemovePidFile(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to remove pidfile: %v\n", err)
	}

	fmt.Printf("Errm... alright, the daemon stopped (PID: %d)\n", pid)
}
