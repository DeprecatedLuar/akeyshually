package commands

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"

	daemon "github.com/deprecatedluar/luar-daemonator"

	"github.com/deprecatedluar/akeyshually/internal/common"
	"github.com/deprecatedluar/akeyshually/internal/config"
)

// Emit validates a whitespace-separated remap token sequence, sends it to
// the running daemon over its IPC socket, and surfaces the daemon's reply.
// Tokens are validated locally with the same rules the config loader uses,
// so CLI and config reject bad tokens with identical messages.
func Emit(cmd string) {
	tokens := strings.Fields(cmd)
	if len(tokens) == 0 {
		fmt.Fprintf(os.Stderr, "akeyshually: emit requires at least one token\n")
		os.Exit(1)
	}

	for _, tok := range tokens {
		if err := config.ValidateRemapToken(tok); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	}

	d := daemon.New(common.AppName)
	conn, err := net.Dial("unix", d.RuntimePath(".sock"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "akeyshually: daemon not running: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	fmt.Fprintf(conn, "%s\n", strings.Join(tokens, " "))

	reply, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		fmt.Fprintf(os.Stderr, "akeyshually: no reply from daemon: %v\n", err)
		os.Exit(1)
	}

	reply = strings.TrimSpace(reply)
	if strings.HasPrefix(reply, "err:") {
		fmt.Fprintln(os.Stderr, reply)
		os.Exit(1)
	}
}
