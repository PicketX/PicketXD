// SPDX-License-Identifier: GPL-3.0-or-later
package agentcli

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/PicketX/PicketXD/internal/hostagent"
)

func enforceNotifySocket(enabled, once bool) (string, error) {
	if !enabled {
		return "", nil
	}
	if once {
		return "", fmt.Errorf("--systemd-notify cannot be combined with --once")
	}
	path := os.Getenv("NOTIFY_SOCKET")
	if len(path) < 2 || len(path) > 107 || !(strings.HasPrefix(path, "/") || strings.HasPrefix(path, "@")) {
		return "", fmt.Errorf("--systemd-notify requires an absolute or abstract NOTIFY_SOCKET of 2..107 bytes")
	}
	return path, nil
}

// The driver's top-level confirmation already brackets current generation,
// ownership, application and strict cleanup. A rejected current publication
// can still confirm retained policy, but must not complete service startup.
// READY is a one-time startup event; STATUS never revokes that event.
func cleanCurrentPolicy(st hostagent.Status) bool {
	return st.EnforcementConfirmed && st.Desired.LastError == "" && st.LastApplyError == "" &&
		(st.Desired.RemoteState == "" || st.Desired.RemoteState == "current")
}

func newEnforceEmitter(out io.Writer, socket string) func(hostagent.Status) error {
	encoder := json.NewEncoder(out)
	ready := false
	return func(st hostagent.Status) error {
		if err := encoder.Encode(st); err != nil {
			return err
		}
		if socket == "" {
			return nil
		}
		clean := cleanCurrentPolicy(st)
		status := "Waiting for confirmed host policy"
		switch {
		case st.LastApplyError != "":
			status = "Host policy confirmation failed; installed rules retained"
		case st.Desired.LastError != "":
			status = "Publication rejected; accepted policy retained"
		case st.Desired.RemoteState != "" && st.Desired.RemoteState != "current":
			status = "Waiting for current Controller policy; accepted state retained"
		case clean:
			status = fmt.Sprintf("Confirmed host policy generation %d", st.Desired.Generation)
		}
		message := "STATUS=" + status
		if clean && !ready {
			message = "READY=1\n" + message
		}
		conn, err := net.DialTimeout("unixgram", socket, time.Second)
		if err != nil {
			return fmt.Errorf("systemd notification: %w", err)
		}
		defer conn.Close()
		if err = conn.SetWriteDeadline(time.Now().Add(time.Second)); err == nil {
			var n int
			n, err = io.WriteString(conn, message)
			if err == nil && n != len(message) {
				err = io.ErrShortWrite
			}
		}
		if err != nil {
			return fmt.Errorf("systemd notification: %w", err)
		}
		ready = ready || clean
		return nil
	}
}
