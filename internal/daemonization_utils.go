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

// Daemonize forks the process to run in background
func Daemonize() error {
	// Get current executable path
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Open /dev/null for stdin, stdout, stderr redirection
	devNull, err := os.OpenFile("/dev/null", os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("failed to open /dev/null: %w", err)
	}
	defer devNull.Close()

	// Fork and exec in background
	// Use syscall.ForkExec to create detached process
	args := []string{executable}

	// Set up process attributes for daemonization
	attr := &syscall.ProcAttr{
		Dir: "/",
		Env: os.Environ(),
		Files: []uintptr{
			devNull.Fd(), // stdin -> /dev/null
			devNull.Fd(), // stdout -> /dev/null
			devNull.Fd(), // stderr -> /dev/null
		},
		Sys: &syscall.SysProcAttr{
			Setsid: true, // Create new session (detach from terminal)
		},
	}

	pid, err := syscall.ForkExec(executable, args, attr)
	if err != nil {
		return fmt.Errorf("failed to daemonize: %w", err)
	}

	fmt.Printf("Errm... alright, the daemon started (PID: %d)\n", pid)

	// Write pidfile from parent process before exiting
	if err := WritePidFile(pid); err != nil {
		return fmt.Errorf("failed to write pidfile: %w", err)
	}

	return nil
}
