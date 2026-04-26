package executor

import (
	"strings"

	"github.com/deprecatedluar/akeyshually/internal/config"
	"github.com/deprecatedluar/akeyshually/internal/matcher"
	evdev "github.com/holoplot/go-evdev"
)

type ExecContext struct {
	KeyCode   uint16
	Value     int32
	Virtual   *evdev.InputDevice
	Injector  *evdev.InputDevice
	Modifiers matcher.ModifierState
	Config    *config.Config
	LoopState *LoopState
}

func Run(cmd string, ctx ExecContext) {
	switch {
	case cmd == "":
		passthrough(ctx)
	case isRemap(cmd):
		runRemap(cmd, ctx)
	default:
		runShell(cmd, ctx)
	}
}

func isRemap(cmd string) bool {
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
