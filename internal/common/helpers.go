package common

import (
	"os/exec"
)

const (
	AppName           = "akeyshually"
	notifySendBin     = "notify-send"
	notifyUrgencyCrit = "critical"
	notifyUrgencyNorm = "normal"
)

// NotifyError sends a desktop notification for critical errors
// Fails silently if notify-send is not available
func NotifyError(title, message string) {
	cmd := exec.Command(notifySendBin, "-u", notifyUrgencyCrit, "-a", AppName, title, message)
	cmd.Run() // Fire and forget
}

// NotifyInfo sends a desktop notification for informational messages
func NotifyInfo(title, message string) {
	cmd := exec.Command(notifySendBin, "-u", notifyUrgencyNorm, "-a", AppName, title, message)
	cmd.Run() // Fire and forget
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
