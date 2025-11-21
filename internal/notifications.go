package internal

import (
	"os/exec"
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
