//go:build integration

// SPDX-License-Identifier: GPL-3.0-or-later
package hostagent

import (
	vnet "github.com/vishvananda/netlink"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

func strictExample(t *testing.T) string {
	t.Helper()
	return strings.ReplaceAll(example(t), "handling: graceful", "handling: strict")
}
func strictConfirmed(st Status) bool {
	return st.EnforcementConfirmed && st.Strict != nil && st.Strict.CleanupConfirmed
}
func strictProcessConnections(t *testing.T) []net.Conn {
	t.Helper()
	var out []net.Conn
	for _, host := range []string{"127.0.0.1", "::1"} {
		for _, protocol := range []string{"tcp", "udp"} {
			port := "18443"
			if protocol == "udp" {
				port = "18444"
			}
			c, e := net.DialTimeout(protocol, net.JoinHostPort(host, port), time.Second)
			if e != nil {
				t.Fatal(e)
			}
			t.Cleanup(func() { c.Close() })
			strictConnectionEcho(t, c, true)
			out = append(out, c)
		}
	}
	return out
}
func strictConnectionEcho(t *testing.T, c net.Conn, allowed bool) {
	t.Helper()
	c.SetDeadline(time.Now().Add(200 * time.Millisecond))
	_, e := c.Write([]byte("s"))
	var b [1]byte
	if e == nil {
		_, e = io.ReadFull(c, b[:])
	}
	if allowed && (e != nil || b[0] != 's') || !allowed && e == nil {
		t.Fatal("strict packet echo", allowed, e)
	}
	c.SetDeadline(time.Time{})
}
func TestEnforceProcessStrictRevocation(t *testing.T) {
	isolatedProcess(t, func() {
		processServers(t)
		raw := strictExample(t)
		args := changedArg(processArgs(t, raw), "--handling", "strict")
		p := startCommand(t, args)
		p.until(t, strictConfirmed)
		clients := strictProcessConnections(t)
		publish(t, argument(args, "--policy"), "broken")
		p.until(t, func(st Status) bool { return strictConfirmed(st) && st.Desired.LastError != "" })
		for _, c := range clients {
			strictConnectionEcho(t, c, true)
		}
		publish(t, argument(args, "--policy"), noGrants(raw))
		st := p.until(t, func(st Status) bool { return strictConfirmed(st) && st.Desired.Generation == 2 })
		if st.Strict.Result.Selected != 4 || st.Strict.Result.Deleted != 4 || st.Strict.Result.Remaining != 0 {
			t.Fatal(st)
		}
		for _, c := range clients {
			strictConnectionEcho(t, c, false)
		}
		publish(t, argument(args, "--policy"), strings.ReplaceAll(raw, "grant-loopback", "grant-regrant"))
		p.until(t, func(st Status) bool { return strictConfirmed(st) && st.Desired.Generation == 3 })
		for _, c := range clients {
			if _, ok := c.LocalAddr().(*net.TCPAddr); ok {
				strictConnectionEcho(t, c, false)
			}
		}
		fresh := strictProcessConnections(t)
		p.stop(t, syscall.SIGTERM, false)
		dir := argument(args, "--state-dir")
		ownerBefore, _ := os.ReadFile(filepath.Join(dir, "owner.json"))
		desiredBefore, _ := os.ReadFile(filepath.Join(dir, "desired.json"))
		if _, e, log := onceCommand(t, changedArg(args, "--handling", "graceful")); e == nil || !strings.Contains(log, "handling migration") {
			t.Fatal("mode migration accepted", e, log)
		}
		ownerAfter, _ := os.ReadFile(filepath.Join(dir, "owner.json"))
		desiredAfter, _ := os.ReadFile(filepath.Join(dir, "desired.json"))
		if !reflect.DeepEqual(ownerBefore, ownerAfter) || !reflect.DeepEqual(desiredBefore, desiredAfter) {
			t.Fatal("mode rejection mutated state")
		}
		for _, c := range fresh {
			strictConnectionEcho(t, c, true)
		}
		t.Log("actual strict binary retained invalid-publication permission, then revoked/deleted four dual-stack TCP/UDP flows; regrant admitted fresh flows while cleaned old TCP stayed blocked; handling migration refused without journal/cache mutation")
	})
}
func TestEnforceProcessStrictExpiryWithoutProcess(t *testing.T) {
	isolatedProcess(t, func() {
		processServers(t)
		expiry := time.Now().Add(3 * time.Second).UTC()
		raw := strings.ReplaceAll(strictExample(t), "validity: {mode: permanent}", "validity: {mode: bounded, expiresAt: '"+expiry.Format(time.RFC3339Nano)+"'}")
		args := changedArg(processArgs(t, raw), "--handling", "strict")
		p := startCommand(t, args)
		first := p.until(t, strictConfirmed)
		clients := strictProcessConnections(t)
		if !first.Strict.ValidUntil.Equal(expiry) {
			t.Fatal(first)
		}
		p.stop(t, syscall.SIGKILL, true)
		time.Sleep(max(0, time.Until(expiry.Add(75*time.Millisecond))))
		for _, c := range clients {
			strictConnectionEcho(t, c, false)
		}
		ownerPath := filepath.Join(argument(args, "--state-dir"), "owner.json")
		before, _ := os.ReadFile(ownerPath)
		st, e, log := onceCommand(t, args)
		if e != nil || !strictConfirmed(st) || st.Strict.PreviousCleanup == nil || !st.Strict.PreviousCleanup.Confirmed || st.Strict.PreviousCleanup.ImageDigest != first.Application.ImageDigest || st.Strict.PreviousCleanup.Result.Selected != 4 || st.Strict.PreviousCleanup.Result.Deleted != 4 || st.Strict.PreviousCleanup.Result.Remaining != 0 || st.Strict.Result.Selected != 0 || st.Strict.Result.Remaining != 0 || !st.Desired.Restored || st.Application.Attempt != first.Application.Attempt || st.Application.ImageDigest != first.Application.ImageDigest {
			t.Fatal(st, e, log)
		}
		after, _ := os.ReadFile(ownerPath)
		if !reflect.DeepEqual(before, after) {
			t.Fatal("restart cleanup changed owner journal")
		}
		publish(t, argument(args, "--policy"), strings.ReplaceAll(strictExample(t), "grant-loopback", "grant-regrant"))
		restarted := startCommand(t, args)
		restarted.until(t, func(st Status) bool { return strictConfirmed(st) && st.Desired.Generation == 2 })
		for _, c := range clients {
			if _, ok := c.LocalAddr().(*net.TCPAddr); ok {
				strictConnectionEcho(t, c, false)
			}
		}
		strictProcessConnections(t)
		restarted.stop(t, syscall.SIGTERM, false)
		t.Logf("SIGKILL before %s: old TCP/UDP IPv4/IPv6 independently blocked at kernel deadline; once restart deleted4 remaining0 without changing image/attempt/journal; later regrant did not revive cleaned TCP", expiry.Format(time.RFC3339Nano))
	})
}

func strictKeepGrants() string {
	return `
---
apiVersion: picketx.io/v1alpha1
kind: Grant
metadata: {name: keep4, uid: grant-keep4-v1}
spec:
  subject: {type: source, cidr: 127.0.0.2/32}
  scopes:
    - resourceRef: {name: demo, uid: resource-demo-v1}
  validity: {mode: permanent}
---
apiVersion: picketx.io/v1alpha1
kind: Grant
metadata: {name: keep6, uid: grant-keep6-v1}
spec:
  subject: {type: source, cidr: '::2/128'}
  scopes:
    - resourceRef: {name: demo, uid: resource-demo-v1}
  validity: {mode: permanent}
`
}

func strictKeptConnections(t *testing.T) []net.Conn {
	t.Helper()
	lo, e := vnet.LinkByName("lo")
	if e != nil {
		t.Fatal(e)
	}
	if e = vnet.AddrAdd(lo, &vnet.Addr{IPNet: &net.IPNet{IP: net.ParseIP("::2"), Mask: net.CIDRMask(128, 128)}}); e != nil {
		t.Fatal(e)
	}
	var out []net.Conn
	for i, source := range []string{"127.0.0.2", "::2"} {
		dest := []string{"127.0.0.1", "::1"}[i]
		for _, protocol := range []string{"tcp", "udp"} {
			d := net.Dialer{Timeout: time.Second}
			port := "18443"
			if protocol == "tcp" {
				d.LocalAddr = &net.TCPAddr{IP: net.ParseIP(source)}
			} else {
				d.LocalAddr = &net.UDPAddr{IP: net.ParseIP(source)}
				port = "18444"
			}
			c, e := d.Dial(protocol, net.JoinHostPort(dest, port))
			if e != nil {
				t.Fatal(e)
			}
			t.Cleanup(func() { c.Close() })
			strictConnectionEcho(t, c, true)
			out = append(out, c)
		}
	}
	return out
}

func TestEnforceProcessStrictRegrantAfterOfflineExpiry(t *testing.T) {
	isolatedProcess(t, func() {
		processServers(t)
		expiry := time.Now().Add(5 * time.Second).UTC()
		raw := strings.ReplaceAll(strictExample(t), "validity: {mode: permanent}", "validity: {mode: bounded, expiresAt: '"+expiry.Format(time.RFC3339Nano)+"'}") + strictKeepGrants()
		args := changedArg(processArgs(t, raw), "--handling", "strict")
		p := startCommand(t, args)
		p.until(t, strictConfirmed)
		old := strictProcessConnections(t)
		kept := strictKeptConnections(t)
		if !time.Now().Before(expiry) {
			t.Fatal("fixture did not kill before expiry")
		}
		p.stop(t, syscall.SIGKILL, true)
		time.Sleep(max(0, time.Until(expiry.Add(75*time.Millisecond))))
		for _, c := range old {
			strictConnectionEcho(t, c, false)
		}
		for _, c := range kept {
			strictConnectionEcho(t, c, true)
		}
		publish(t, argument(args, "--policy"), strings.ReplaceAll(strictExample(t), "grant-loopback", "grant-fresh")+strictKeepGrants())
		st, e, log := onceCommand(t, args)
		if e != nil || !strictConfirmed(st) || st.Desired.Generation != 2 {
			t.Fatal(st, e, log)
		}
		if st.Strict.PreviousCleanup == nil || !st.Strict.PreviousCleanup.Confirmed || st.Strict.PreviousCleanup.Result.Selected != 4 || st.Strict.PreviousCleanup.Result.Deleted != 4 || st.Strict.PreviousCleanup.Result.Remaining != 0 || st.Strict.Result.Selected != 0 || st.Strict.PreviousCleanup.ImageDigest == st.Application.ImageDigest {
			t.Fatal("missing separate old-image cleanup proof", st)
		}
		// Restart directly into new permission, without an intermediate expired once run.
		for _, c := range old {
			if _, ok := c.LocalAddr().(*net.TCPAddr); ok {
				strictConnectionEcho(t, c, false)
			}
		}
		for _, c := range kept {
			strictConnectionEcho(t, c, true)
		}
		strictProcessConnections(t)
		t.Log("direct fresh-Grant restart after SIGKILL/offline expiry blocked both old TCP clients, preserved four independently allowed TCP/UDP flows, and admitted four fresh dual-stack clients")
	})
}
