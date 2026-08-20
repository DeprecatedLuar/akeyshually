// Package ipc exposes the running daemon's remap engine over a local Unix
// socket so external processes (the CLI, scripts, aliases) can inject key
// and mouse events without a one-shot uinput device.
package ipc

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/deprecatedluar/akeyshually/internal/executor"
	"github.com/deprecatedluar/akeyshually/internal/matcher"
)

const socketPerm = 0600

// Serve accepts connections on sockPath until ctx is cancelled. Each
// connection carries one request: a single newline-terminated line of
// whitespace-separated remap tokens. Every token is run through
// executor.Run against outputs/loopState; the reply is "ok" or
// "err: <message>", then the connection closes.
func Serve(ctx context.Context, sockPath string, outputs executor.Outputs, loopState *executor.LoopState) error {
	os.Remove(sockPath) // stale socket left by an unclean previous exit

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", sockPath, err)
	}
	if err := os.Chmod(sockPath, socketPerm); err != nil {
		listener.Close()
		return fmt.Errorf("chmod %s: %w", sockPath, err)
	}

	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	// Serializes whole token sequences from concurrent CLI connections
	// against each other. Must not be loopState.Mu - runRemap's ">>"
	// branch already holds that lock while emitting, so reusing it here
	// would self-deadlock.
	var emitMu sync.Mutex

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				os.Remove(sockPath)
				return nil
			}
			continue
		}
		go handleConn(conn, outputs, loopState, &emitMu)
	}
}

func handleConn(conn net.Conn, outputs executor.Outputs, loopState *executor.LoopState, emitMu *sync.Mutex) {
	defer conn.Close()

	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil && line == "" {
		return
	}

	tokens := strings.Fields(line)
	if len(tokens) == 0 {
		fmt.Fprintln(conn, "err: empty request")
		return
	}

	execCtx := executor.ExecContext{
		Outputs:   outputs,
		LoopState: loopState,
		Modifiers: matcher.ModifierState{},
		Virtual:   nil,
	}

	emitMu.Lock()
	defer emitMu.Unlock()

	for _, tok := range tokens {
		if err := executor.Run(tok, execCtx); err != nil {
			fmt.Fprintf(conn, "err: %v\n", err)
			return
		}
	}
	fmt.Fprintln(conn, "ok")
}
