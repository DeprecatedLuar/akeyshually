package executor

import (
	"context"
	"os/exec"
	"sync"
	"time"
)

// LoopState tracks active repeat loops and sustained processes across key events.
type LoopState struct {
	Mu             sync.Mutex
	Active         map[string]context.CancelFunc // repeat loops
	HeldProcesses  map[string]*exec.Cmd          // sustained whileheld processes
	HeldKeys       map[string][]uint16           // sustained remap hold keys
	PersistentHeld map[string][]uint16           // >> persistent remap keys
}

func NewLoopState() *LoopState {
	return &LoopState{
		Active:         make(map[string]context.CancelFunc),
		HeldProcesses:  make(map[string]*exec.Cmd),
		HeldKeys:       make(map[string][]uint16),
		PersistentHeld: make(map[string][]uint16),
	}
}

func runTickerLoop(ctx context.Context, interval float64, fn func()) {
	ticker := time.NewTicker(time.Duration(interval) * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fn()
		}
	}
}
