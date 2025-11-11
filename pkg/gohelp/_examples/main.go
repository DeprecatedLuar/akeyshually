package main

import (
	"fmt"

	"github.com/DeprecatedLuar/gohelp"
)

func main() {
	gohelp.PrintHeader("Usage")
	fmt.Println("  myapp <command> [options]")
	fmt.Println()
	gohelp.Item("Location:", "~/.config/myapp/config.toml")
	gohelp.Item("Edit:", "myapp config")

	gohelp.PrintHeader("Commands")
	gohelp.Item("start [options]", "Start the service")
	gohelp.Item("stop", "Stop the service")
	gohelp.Item("restart", "Restart the service")

	gohelp.PrintHeader("Options")
	gohelp.Item("--config FILE", "Configuration file path")
	gohelp.Item("--verbose", "Enable verbose output")
	gohelp.Item("--help", "Show this help message")

	gohelp.Paragraph("All commands support --help for detailed information")

	gohelp.Separator()
}
