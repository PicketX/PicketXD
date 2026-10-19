// SPDX-License-Identifier: GPL-3.0-or-later
package agentcli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PicketX/PicketXD/internal/hostagent"
)

func TestCleanCurrentPolicyRequiresRemoteAcceptanceNotJustLKG(t *testing.T) {
	st := hostagent.Status{EnforcementConfirmed: true}
	if !cleanCurrentPolicy(st) {
		t.Fatal("static readiness changed")
	}
	for _, state := range []string{"awaiting", "disconnected", "unknown"} {
		st.Desired.RemoteState = state
		if cleanCurrentPolicy(st) {
			t.Fatal("cached remote state is not clean", state)
		}
	}
	st.Desired.RemoteState = "current"
	if !cleanCurrentPolicy(st) {
		t.Fatal("current remote confirmation rejected")
	}
	st.Desired.LastError = "rejected"
	if cleanCurrentPolicy(st) {
		t.Fatal("rejected publication is not clean")
	}
}

func notifyTestPath(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "pxn-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return filepath.Join(dir, "notify")
}
func notifyTestListener(t *testing.T, path string) *net.UnixConn {
	t.Helper()
	c, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: path, Net: "unixgram"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}
func notifyTestReceive(t *testing.T, c *net.UnixConn) string {
	t.Helper()
	if err := c.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 4096)
	n, _, err := c.ReadFromUnix(b)
	if err != nil {
		t.Fatal(err)
	}
	return string(b[:n])
}
func notifyTestAbsent(t *testing.T, c *net.UnixConn) {
	t.Helper()
	if err := c.SetReadDeadline(time.Now().Add(40 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	_, _, err := c.ReadFromUnix(make([]byte, 4096))
	var e net.Error
	if !errors.As(err, &e) || !e.Timeout() {
		t.Fatal("unexpected notification or error", err)
	}
}
func TestNotifyEmitterFirstCleanConfirmationOnly(t *testing.T) {
	path := notifyTestPath(t)
	listener := notifyTestListener(t, path)
	var out bytes.Buffer
	emit := newEnforceEmitter(&out, path)
	clean := hostagent.Status{EnforcementConfirmed: true, Desired: hostagent.DesiredStatus{Generation: 7}}
	rejected := clean
	rejected.Desired.LastError = "bad\nREADY=1"
	failed := hostagent.Status{LastApplyError: "drift\nREADY=1"}
	awaiting, disconnected := clean, clean
	awaiting.Desired.RemoteState = "awaiting"
	disconnected.Desired.RemoteState = "disconnected"
	cases := []struct {
		status hostagent.Status
		want   string
	}{
		{hostagent.Status{}, "STATUS=Waiting for confirmed host policy"},
		{awaiting, "STATUS=Waiting for current Controller policy; accepted state retained"},
		{disconnected, "STATUS=Waiting for current Controller policy; accepted state retained"},
		{rejected, "STATUS=Publication rejected; accepted policy retained"},
		{clean, "READY=1\nSTATUS=Confirmed host policy generation 7"},
		{clean, "STATUS=Confirmed host policy generation 7"},
		{rejected, "STATUS=Publication rejected; accepted policy retained"},
		{failed, "STATUS=Host policy confirmation failed; installed rules retained"},
		{clean, "STATUS=Confirmed host policy generation 7"},
	}
	for i, tc := range cases {
		if err := emit(tc.status); err != nil {
			t.Fatal(i, err)
		}
		if got := notifyTestReceive(t, listener); got != tc.want {
			t.Fatalf("step%d: got %q want %q", i, got, tc.want)
		}
	}
	decoder := json.NewDecoder(&out)
	for i, tc := range cases {
		var got hostagent.Status
		if err := decoder.Decode(&got); err != nil {
			t.Fatal(i, err)
		}
		if got.EnforcementConfirmed != tc.status.EnforcementConfirmed || got.Desired.LastError != tc.status.Desired.LastError || got.LastApplyError != tc.status.LastApplyError {
			t.Fatal(i, got)
		}
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatal(extra, err)
	}
}
func TestNotifyEmitterDoesNotSignalWhenJSONWriteFails(t *testing.T) {
	path := notifyTestPath(t)
	listener := notifyTestListener(t, path)
	file, err := os.CreateTemp(t.TempDir(), "closed")
	if err != nil {
		t.Fatal(err)
	}
	file.Close()
	emit := newEnforceEmitter(file, path)
	if err := emit(hostagent.Status{EnforcementConfirmed: true}); err == nil {
		t.Fatal("JSON write failure ignored")
	}
	notifyTestAbsent(t, listener)
}
func TestNotifyEmitterFailureDoesNotConsumeReadiness(t *testing.T) {
	path := notifyTestPath(t)
	var out bytes.Buffer
	emit := newEnforceEmitter(&out, path)
	st := hostagent.Status{EnforcementConfirmed: true}
	if err := emit(st); err == nil {
		t.Fatal("missing notification socket ignored")
	}
	listener := notifyTestListener(t, path)
	if err := emit(st); err != nil {
		t.Fatal(err)
	}
	if got := notifyTestReceive(t, listener); !strings.HasPrefix(got, "READY=1\n") {
		t.Fatal(got)
	}
}
func TestNotifyEmitterOffIgnoresEnvironment(t *testing.T) {
	path := notifyTestPath(t)
	listener := notifyTestListener(t, path)
	t.Setenv("NOTIFY_SOCKET", path)
	var out bytes.Buffer
	if err := newEnforceEmitter(&out, "")(hostagent.Status{EnforcementConfirmed: true}); err != nil {
		t.Fatal(err)
	}
	notifyTestAbsent(t, listener)
	var st hostagent.Status
	if err := json.Unmarshal(out.Bytes(), &st); err != nil || !st.EnforcementConfirmed {
		t.Fatal(err, out.String())
	}
}
func TestNotifyEmitterAbstractSocket(t *testing.T) {
	path := "@picketx-" + filepath.Base(filepath.Dir(notifyTestPath(t)))
	listener := notifyTestListener(t, path)
	if err := newEnforceEmitter(io.Discard, path)(hostagent.Status{EnforcementConfirmed: true}); err != nil {
		t.Fatal(err)
	}
	if got := notifyTestReceive(t, listener); !strings.HasPrefix(got, "READY=1\n") {
		t.Fatal(got)
	}
}
func TestNotifySocketRequiresExplicitLongRunningMode(t *testing.T) {
	t.Setenv("NOTIFY_SOCKET", "relative")
	if got, err := enforceNotifySocket(false, true); err != nil || got != "" {
		t.Fatal(got, err)
	}
	for _, path := range []string{"/run/systemd/notify", "@picketx-notify"} {
		t.Setenv("NOTIFY_SOCKET", path)
		if got, err := enforceNotifySocket(true, false); err != nil || got != path {
			t.Fatal(got, err)
		}
	}
}
