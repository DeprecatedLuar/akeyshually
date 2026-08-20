package ipc

import (
	"bufio"
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/deprecatedluar/akeyshually/internal/executor"
	evdev "github.com/holoplot/go-evdev"
)

type fakeWriter struct{}

func (fakeWriter) WriteOne(*evdev.InputEvent) error { return nil }

func startTestServer(t *testing.T) (sockPath string, cancel context.CancelFunc, done chan error) {
	t.Helper()
	sockPath = filepath.Join(t.TempDir(), "test.sock")

	outputs := executor.Outputs{
		Keyboard: executor.NewEventSink(fakeWriter{}),
		Pointer:  executor.NewEventSink(fakeWriter{}),
	}
	loopState := executor.NewLoopState()

	ctx, cancelFn := context.WithCancel(context.Background())
	done = make(chan error, 1)
	go func() { done <- Serve(ctx, sockPath, outputs, loopState) }()

	// Wait for the socket file to appear.
	for range 100 {
		if _, err := os.Stat(sockPath); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	return sockPath, cancelFn, done
}

func sendRequest(t *testing.T, sockPath, request string) string {
	t.Helper()
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte(request + "\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	reply, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf("read reply: %v", err)
	}
	return strings.TrimSpace(reply)
}

func TestServeOkReply(t *testing.T) {
	sockPath, cancel, done := startTestServer(t)
	defer cancel()

	reply := sendRequest(t, sockPath, ">a")
	if reply != "ok" {
		t.Fatalf("got reply %q, want ok", reply)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Serve returned error: %v", err)
	}
}

func TestServeErrReply(t *testing.T) {
	sockPath, cancel, _ := startTestServer(t)
	defer cancel()

	reply := sendRequest(t, sockPath, ">nosuchkey")
	if !strings.HasPrefix(reply, "err:") {
		t.Fatalf("got reply %q, want err: prefix", reply)
	}
}

func TestServeSocketPermissions(t *testing.T) {
	sockPath, cancel, _ := startTestServer(t)
	defer cancel()

	info, err := os.Stat(sockPath)
	if err != nil {
		t.Fatalf("stat socket: %v", err)
	}
	if perm := info.Mode().Perm(); perm != socketPerm {
		t.Fatalf("socket perm = %o, want %o", perm, socketPerm)
	}
}

func TestServeSocketRemovedOnShutdown(t *testing.T) {
	sockPath, cancel, done := startTestServer(t)

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Serve returned error: %v", err)
	}

	if _, err := os.Stat(sockPath); !os.IsNotExist(err) {
		t.Fatalf("socket file still exists after shutdown: %v", err)
	}
}
