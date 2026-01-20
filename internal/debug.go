package internal

import (
	"fmt"
	"os"
	"time"
)

var (
	debugEnabled   bool
	loggingEnabled bool
)

func SetDebug(enabled bool) {
	debugEnabled = enabled
	loggingEnabled = enabled // debug implies logging
}

func SetLogging(enabled bool) {
	loggingEnabled = enabled
}

func IsDebugEnabled() bool {
	return debugEnabled
}

func IsLoggingEnabled() bool {
	return loggingEnabled
}

func timestamp() string {
	t := time.Now()
	return fmt.Sprintf("[%s:%02d]", t.Format("15:04:05"), t.Nanosecond()/10000000)
}

// LogKey logs every key press (muted/dim)
func LogKey(keyName string, code uint16) {
	if !loggingEnabled {
		return
	}
	fmt.Fprintf(os.Stderr, "\033[2m%s %-7s - %q, code: %d\033[0m\n", timestamp(), "Key", keyName, code)
}

// LogMatch logs when a shortcut combo matches
func LogMatch(combo string, codes string) {
	if !loggingEnabled {
		return
	}
	fmt.Fprintf(os.Stderr, "%s %-7s - %q, code: %s\n", timestamp(), "Combo", combo, codes)
}

// LogTrigger logs when a command is executed
func LogTrigger(command string) {
	if !loggingEnabled {
		return
	}
	fmt.Fprintf(os.Stderr, "%s %-7s - %q\n", timestamp(), "Trigger", command)
}

// LogDebug logs debug info (only when debug mode is on)
func LogDebug(format string, args ...any) {
	if !debugEnabled {
		return
	}
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}
