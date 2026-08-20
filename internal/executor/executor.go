package executor

import (
	"fmt"
	"os"
	"strings"

	"github.com/deprecatedluar/akeyshually/internal/config"
	"github.com/deprecatedluar/akeyshually/internal/matcher"
	evdev "github.com/holoplot/go-evdev"
)

type ExecContext struct {
	KeyCode   uint16
	Value     int32
	Virtual   *evdev.InputDevice
	Outputs   Outputs
	Modifiers matcher.ModifierState
	Config    *config.Config
	LoopState *LoopState
}

func Run(cmd string, ctx ExecContext) error {
	err := run(cmd, ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Execution error for %q: %v\n", cmd, err)
	}
	return err
}

func run(cmd string, ctx ExecContext) error {
	switch {
	case cmd == "":
		passthrough(ctx)
		return nil
	case IsRemap(cmd):
		return runRemap(cmd, ctx)
	default:
		runShell(cmd, ctx)
		return nil
	}
}

// IsRemap reports whether cmd is a key/pointer injection command (>, >>, <, <<).
func IsRemap(cmd string) bool {
	return strings.HasPrefix(cmd, ">") || strings.HasPrefix(cmd, "<")
}

func passthrough(ctx ExecContext) {
	if ctx.Virtual == nil {
		return
	}
	ctx.Virtual.WriteOne(&evdev.InputEvent{
		Type:  evdev.EV_KEY,
		Code:  evdev.EvCode(ctx.KeyCode),
		Value: ctx.Value,
	})
	ctx.Virtual.WriteOne(&evdev.InputEvent{Type: evdev.EV_SYN, Code: evdev.SYN_REPORT})
}
