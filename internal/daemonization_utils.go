package internal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// NotifyError sends a desktop notification for critical errors
// Fails silently if notify-send is not available
func NotifyError(title, message string) {
	cmd := exec.Command("notify-send", "-u", "critical", "-a", "akeyshually", title, message)
	cmd.Run() // Fire and forget
}

// NotifyInfo sends a desktop notification for informational messages
func NotifyInfo(title, message string) {
	cmd := exec.Command("notify-send", "-u", "normal", "-a", "akeyshually", title, message)
	cmd.Run() // Fire and forget
}

// GetPidFilePath returns the path to the pidfile
func GetPidFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".config", "akeyshually", "akeyshually.pid"), nil
}

// WritePidFile writes the given PID to the pidfile
func WritePidFile(pid int) error {
	pidPath, err := GetPidFilePath()
	if err != nil {
		return err
	}

	pidStr := strconv.Itoa(pid)
	if err := os.WriteFile(pidPath, []byte(pidStr), 0644); err != nil {
		return fmt.Errorf("failed to write pidfile: %w", err)
	}

	return nil
}

// ReadPidFile reads the PID from the pidfile
func ReadPidFile() (int, error) {
	pidPath, err := GetPidFilePath()
	if err != nil {
		return 0, err
	}

	data, err := os.ReadFile(pidPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to read pidfile: %w", err)
	}

	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0, fmt.Errorf("invalid pid in pidfile: %w", err)
	}

	return pid, nil
}

// RemovePidFile deletes the pidfile
func RemovePidFile() error {
	pidPath, err := GetPidFilePath()
	if err != nil {
		return err
	}

	if err := os.Remove(pidPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove pidfile: %w", err)
	}

	return nil
}

// IsProcessRunning checks if a process with the given PID is running
func IsProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}

	// Send signal 0 to check if process exists
	// Returns nil if process exists, error otherwise
	err := syscall.Kill(pid, 0)
	return err == nil
}

// HasSystemdService checks if akeyshually systemd service is active
// Returns (isActive bool, error)
func HasSystemdService() (bool, error) {
	// Check if systemd is managing the daemon by checking service status
	// systemctl --user is-active returns 0 if active, non-zero otherwise
	cmd := exec.Command("systemctl", "--user", "is-active", "akeyshually")
	err := cmd.Run()

	// If command succeeds (exit code 0), service is active
	return err == nil, nil
}

// GetRunningDaemonPid checks for running akeyshually daemon
// Checks system first (ground truth via pgrep)
// Returns PID if running, 0 if not running
func GetRunningDaemonPid() (int, error) {
	cmd := exec.Command("pgrep", "-x", "akeyshually")
	output, err := cmd.Output()
	if err != nil {
		// pgrep returns exit code 1 when no processes found
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to search for running processes: %w", err)
	}

	pidStr := strings.TrimSpace(string(output))
	if pidStr == "" {
		return 0, nil
	}

	// pgrep can return multiple PIDs on multiple lines
	lines := strings.Split(pidStr, "\n")
	if len(lines) == 0 {
		return 0, nil
	}

	// Exclude current process
	currentPid := os.Getpid()
	for _, line := range lines {
		pid, err := strconv.Atoi(strings.TrimSpace(line))
		if err != nil {
			continue
		}
		if pid != currentPid {
			return pid, nil
		}
	}

	return 0, nil
}

// SpawnDaemon spawns a detached daemon process
// If replacingPid > 0, sets AKEYSHUALLY_REPLACING env to allow new daemon to start
// Returns the PID of the spawned process
// Used by: Daemonize(), Restart(), config watcher
func SpawnDaemon(replacingPid int) (int, error) {
	// Get current executable path and resolve symlinks
	executable, err := os.Executable()
	if err != nil {
		return 0, fmt.Errorf("failed to get executable path: %w", err)
	}

	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return 0, fmt.Errorf("failed to resolve executable path: %w", err)
	}

	// Open /dev/null for stdin, stdout, stderr redirection
	devNull, err := os.OpenFile("/dev/null", os.O_RDWR, 0)
	if err != nil {
		return 0, fmt.Errorf("failed to open /dev/null: %w", err)
	}
	defer devNull.Close()

	// Prepare environment - add AKEYSHUALLY_REPLACING if this is a restart
	env := os.Environ()
	if replacingPid > 0 {
		env = append(env, fmt.Sprintf("AKEYSHUALLY_REPLACING=%d", replacingPid))
	}

	// Set up process attributes for daemonization
	attr := &syscall.ProcAttr{
		Dir: "/",
		Env: env,
		Files: []uintptr{
			devNull.Fd(), // stdin -> /dev/null
			devNull.Fd(), // stdout -> /dev/null
			devNull.Fd(), // stderr -> /dev/null
		},
		Sys: &syscall.SysProcAttr{
			Setsid: true, // Create new session (detach from terminal)
		},
	}

	// Fork and exec in background
	pid, err := syscall.ForkExec(executable, []string{executable}, attr)
	if err != nil {
		return 0, fmt.Errorf("failed to spawn daemon: %w", err)
	}

	return pid, nil
}

// Daemonize forks the process to run in background
func Daemonize() error {
	pid, err := SpawnDaemon(0) // Not replacing anything
	if err != nil {
		return err
	}

	fmt.Printf("Errm... alright, the daemon started (PID: %d)\n", pid)

	// Write pidfile from parent process before exiting
	if err := WritePidFile(pid); err != nil {
		return fmt.Errorf("failed to write pidfile: %w", err)
	}

	return nil
}
