package commands

import (
	"fmt"

	"github.com/DeprecatedLuar/gohelp"
)

// Help displays usage information
func Help() {
	gohelp.PrintHeader("akeyshually - keyboard shortcut daemon")

	fmt.Println("Usage:")
	gohelp.Item("akeyshually", "Run in foreground (current terminal)")
	gohelp.Item("akeyshually start", "Start daemon in background")
	gohelp.Item("akeyshually stop", "Stop running daemon")
	gohelp.Item("akeyshually restart", "Restart daemon")
	gohelp.Item("akeyshually help", "Show this help message")
	gohelp.Item("akeyshually version", "Show version information")

	gohelp.Paragraph("Config: ~/.config/akeyshually/shortcuts.toml")
}
