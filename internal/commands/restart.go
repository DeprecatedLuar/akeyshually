package commands

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/deprecatedluar/akeyshually/internal"
)

// Restart implements the restart command
// Stops the daemon and then starts it again (silent operation)
func Restart() {
	// Check if daemon is running via systemd
	hasService, err := internal.HasSystemdService()
	if err == nil && hasService {
		// Let systemd handle the restart
		cmd := exec.Command("systemctl", "--user", "restart", "akeyshually")
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to restart systemd service: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Manual mode: kill old daemon and spawn new one directly
	// This is the same approach used by config_watcher.go:restartSelf()
	pid, err := internal.GetRunningDaemonPid()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to check daemon status: %v\n", err)
		os.Exit(1)
	}

	oldPid := pid
	if pid > 0 {
		// Kill the old daemon
		if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to stop daemon: %v\n", err)
			os.Exit(1)
		}
		// Remove stale pidfile
		internal.RemovePidFile()
	}

	// Spawn new daemon - pass old PID so it knows to replace it
	// This tells startDaemon() to ignore "already running" check for that PID
	newPid, err := internal.SpawnDaemon(oldPid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to spawn daemon: %v\n", err)
		os.Exit(1)
	}

	// Write new pidfile
	if err := internal.WritePidFile(newPid); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to write pidfile: %v\n", err)
	}
}
