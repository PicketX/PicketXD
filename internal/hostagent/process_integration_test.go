//go:build integration

// SPDX-License-Identifier: GPL-3.0-or-later
package hostagent

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	vnet "github.com/vishvananda/netlink"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func isolatedProcess(t *testing.T, run func()) {
	t.Helper()
	if os.Getenv("PICKETX_KERNEL_TESTS") != "1" {
		t.Skip("explicit kernel opt-in required")
	}
	if os.Getenv("PICKETX_ENFORCE_CASE") == t.Name() {
		run()
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "unshare", "--net", os.Args[0], "-test.run", "^"+t.Name()+"$", "-test.v")
	cmd.Env = append(os.Environ(), "PICKETX_ENFORCE_CASE="+t.Name())
	data, e := cmd.CombinedOutput()
	t.Logf("independent process namespace:\n%s", data)
	if e != nil {
		t.Fatal("process acceptance failed", e)
	}
}
func processServers(t *testing.T) {
	t.Helper()
	lo, e := vnet.LinkByName("lo")
	if e != nil {
		t.Fatal(e)
	}
	if e = vnet.LinkSetUp(lo); e != nil {
		t.Fatal(e)
	}
	for _, host := range []string{"127.0.0.1", "::1"} {
		listener, e := net.Listen("tcp", net.JoinHostPort(host, "18443"))
		if e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() { listener.Close() })
		go func() {
			for {
				c, e := listener.Accept()
				if e != nil {
					return
				}
				go func() { defer c.Close(); io.Copy(c, c) }()
			}
		}()
		packet, e := net.ListenPacket("udp", net.JoinHostPort(host, "18444"))
		if e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() { packet.Close() })
		go func() {
			buf := make([]byte, 128)
			for {
				n, peer, e := packet.ReadFrom(buf)
				if e != nil {
					return
				}
				packet.WriteTo(buf[:n], peer)
			}
		}()
	}
}
func processProbe(t *testing.T, allowed bool) {
	t.Helper()
	for _, host := range []string{"127.0.0.1", "::1"} {
		for _, protocol := range []string{"tcp", "udp"} {
			port := "18443"
			if protocol == "udp" {
				port = "18444"
			}
			address := net.JoinHostPort(host, port)
			c, e := net.DialTimeout(protocol, address, 180*time.Millisecond)
			if e == nil {
				c.SetDeadline(time.Now().Add(180 * time.Millisecond))
				_, e = c.Write([]byte("p"))
				var b [1]byte
				if e == nil {
					_, e = io.ReadFull(c, b[:])
				}
				c.Close()
				if e == nil && b[0] != 'p' {
					e = fmt.Errorf("bad echo")
				}
			}
			if allowed && e != nil || !allowed && e == nil {
				t.Fatalf("%s %s allowed=%v: %v", protocol, address, allowed, e)
			}
		}
	}
}
func processConnections(t *testing.T) []net.Conn {
	t.Helper()
	var out []net.Conn
	for _, host := range []string{"127.0.0.1", "::1"} {
		c, e := net.DialTimeout("tcp", net.JoinHostPort(host, "18443"), time.Second)
		if e != nil {
			t.Fatal(e)
		}
		out = append(out, c)
		t.Cleanup(func() { c.Close() })
	}
	return out
}
func processEcho(t *testing.T, connections []net.Conn) {
	t.Helper()
	for _, c := range connections {
		c.SetDeadline(time.Now().Add(time.Second))
		if _, e := c.Write([]byte("x")); e != nil {
			t.Fatal(e)
		}
		var b [1]byte
		if _, e := io.ReadFull(c, b[:]); e != nil || b[0] != 'x' {
			t.Fatal("established graceful connection lost", e)
		}
		c.SetDeadline(time.Time{})
	}
}

//go:embed testdata/policy.yaml
var processPolicy string

func example(t *testing.T) string {
	t.Helper()
	return processPolicy
}

func noGrants(raw string) string {
	parts := strings.Split(raw, "---\n")
	return parts[0] + "---\n" + parts[len(parts)-1]
}
func emptyPublication() string {
	return "apiVersion: picketx.io/v1alpha1\nkind: Configuration\nmetadata: {name: empty, uid: empty-v1}\nspec: {empty: true}\n"
}
func publish(t *testing.T, path, raw string) {
	t.Helper()
	temp := path + ".new"
	if e := os.WriteFile(temp, []byte(raw), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.Rename(temp, path); e != nil {
		t.Fatal(e)
	}
}
func processArgs(t *testing.T, raw string) []string {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "policy.yaml")
	publish(t, path, raw)
	return []string{"enforce", "--policy", path, "--state-dir", filepath.Join(root, "state"), "--provider", "static-test", "--agent", "node", "--destination", "127.0.0.1", "--destination", "::1", "--mark-first", "0x504a0000", "--mark-last", "0x504a003f", "--poll", "30ms"}
}
func argument(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}
func changedArg(args []string, flag, value string) []string {
	out := append([]string{}, args...)
	for i, a := range out {
		if a == flag {
			out[i+1] = value
			return out
		}
	}
	return append(out, flag, value)
}

type lockedLog struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}
func (l *lockedLog) String() string { l.mu.Lock(); defer l.mu.Unlock(); return l.b.String() }

type commandProcess struct {
	cmd        *exec.Cmd
	statuses   chan Status
	wait       chan error
	stderr     lockedLog
	stopped    bool
	stopReader chan struct{}
	readDone   chan struct{}
}

func agentBinary(t *testing.T) string {
	t.Helper()
	path := os.Getenv("PICKETX_AGENT_BINARY")
	if !filepath.IsAbs(path) {
		t.Fatal("explicit prebuilt static agent binary path required")
	}
	return path
}
func startCommand(t *testing.T, args []string) *commandProcess {
	t.Helper()
	p := &commandProcess{statuses: make(chan Status, 256), wait: make(chan error, 1), stopReader: make(chan struct{}), readDone: make(chan struct{})}
	p.cmd = exec.Command(agentBinary(t), args...)
	p.cmd.Env = append(os.Environ(), "PATH=/usr/sbin:/usr/bin:/sbin:/bin")
	p.cmd.Stderr = &p.stderr
	stdout, e := p.cmd.StdoutPipe()
	if e != nil {
		t.Fatal(e)
	}
	if e = p.cmd.Start(); e != nil {
		t.Fatal(e)
	}
	go func() { <-p.readDone; p.wait <- p.cmd.Wait() }()
	go func() {
		defer close(p.readDone)
		defer close(p.statuses)
		defer stdout.Close()
		scan := bufio.NewScanner(stdout)
		scan.Buffer(make([]byte, 4096), 1<<20)
		for scan.Scan() {
			var st Status
			if e := json.Unmarshal(scan.Bytes(), &st); e != nil {
				p.stderr.Write([]byte("invalid status JSON: " + e.Error()))
				return
			}
			select {
			case p.statuses <- st:
			case <-p.stopReader:
				return
			}
		}
	}()
	t.Cleanup(func() {
		close(p.stopReader)
		if !p.stopped {
			p.cmd.Process.Kill()
			<-p.wait
			p.stopped = true
		}
	})
	return p
}
func (p *commandProcess) until(t *testing.T, predicate func(Status) bool) Status {
	t.Helper()
	timer := time.NewTimer(8 * time.Second)
	defer timer.Stop()
	for {
		select {
		case st, ok := <-p.statuses:
			if !ok {
				t.Fatal("agent exited before expected status", p.stderr.String())
			}
			if predicate(st) {
				return st
			}
		case <-timer.C:
			t.Fatal("agent status timeout", p.stderr.String())
		}
	}
}
func (p *commandProcess) stop(t *testing.T, signal os.Signal, wantError bool) {
	t.Helper()
	if e := p.cmd.Process.Signal(signal); e != nil && !errors.Is(e, os.ErrProcessDone) {
		t.Fatal(e)
	}
	select {
	case e := <-p.wait:
		p.stopped = true
		if (e != nil) != wantError {
			t.Fatal("unexpected child result", e, p.stderr.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("agent failed to stop")
	}
}
func onceCommand(t *testing.T, args []string) (Status, error, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, agentBinary(t), append(append([]string{}, args...), "--once")...)
	cmd.Env = append(os.Environ(), "PATH=/usr/sbin:/usr/bin:/sbin:/bin")
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	e := cmd.Run()
	var st Status
	if out.Len() > 0 {
		if decode := json.Unmarshal(out.Bytes(), &st); decode != nil {
			t.Fatal("invalid once status", out.String(), decode)
		}
	}
	return st, e, stderr.String()
}
func confirmed(st Status) bool { return st.EnforcementConfirmed }
func TestEnforceProcessReloadDualStack(t *testing.T) {
	isolatedProcess(t, func() {
		processServers(t)
		raw := example(t)
		args := processArgs(t, noGrants(raw))
		p := startCommand(t, args)
		first := p.until(t, confirmed)
		if first.Desired.Generation != 1 {
			t.Fatal(first)
		}
		processProbe(t, false)
		publish(t, argument(args, "--policy"), raw)
		p.until(t, func(st Status) bool { return st.EnforcementConfirmed && st.Desired.Generation == 2 })
		processProbe(t, true)
		connections := processConnections(t)
		publish(t, argument(args, "--policy"), "broken")
		p.until(t, func(st Status) bool {
			return st.Desired.LastError != "" && st.EnforcementConfirmed && st.Desired.Generation == 2
		})
		processProbe(t, true)
		publish(t, argument(args, "--policy"), noGrants(raw))
		p.until(t, func(st Status) bool { return st.EnforcementConfirmed && st.Desired.Generation == 3 })
		processProbe(t, false)
		processEcho(t, connections)
		publish(t, argument(args, "--policy"), emptyPublication())
		last := p.until(t, func(st Status) bool { return st.EnforcementConfirmed && st.Desired.Generation == 4 })
		processProbe(t, true)
		p.stop(t, syscall.SIGTERM, false)
		t.Logf("actual static binary TCP/UDP IPv4/IPv6 default deny→allow→invalid retained→last Grant default deny→empty withdrawal; generation=%d graceful connections preserved", last.Desired.Generation)
	})
}
func TestEnforceProcessExpiryAndRecovery(t *testing.T) {
	isolatedProcess(t, func() {
		processServers(t)
		expiry := time.Now().Add(3 * time.Second).UTC()
		raw := strings.ReplaceAll(example(t), "validity: {mode: permanent}", "validity: {mode: bounded, expiresAt: '"+expiry.Format(time.RFC3339Nano)+"'}")
		args := processArgs(t, raw)
		p := startCommand(t, args)
		first := p.until(t, confirmed)
		processProbe(t, true)
		connections := processConnections(t)
		p.stop(t, syscall.SIGKILL, true)
		time.Sleep(time.Until(expiry.Add(50 * time.Millisecond)))
		processProbe(t, false)
		processEcho(t, connections)
		restart := startCommand(t, args)
		st := restart.until(t, confirmed)
		if !st.Desired.Restored || st.Desired.Generation != 1 || st.Application.ImageDigest != first.Application.ImageDigest || st.Application.Attempt != first.Application.Attempt {
			t.Fatal("restart changed accepted/applied identity", st, first)
		}
		processProbe(t, false)
		restart.stop(t, syscall.SIGTERM, false)
		t.Logf("SIGKILL/restart actual binary retained absolute expiry %s and image %s; fresh dual-stack TCP/UDP denied, graceful TCP retained", expiry.Format(time.RFC3339Nano), st.Application.ImageDigest)
	})
}
func TestEnforceProcessFutureDeny(t *testing.T) {
	isolatedProcess(t, func() {
		processServers(t)
		start := time.Now().Add(2 * time.Second).UTC()
		end := start.Add(2 * time.Second)
		raw := example(t)
		parts := strings.Split(raw, "---\n")
		for _, grant := range parts[1:3] {
			deny := strings.ReplaceAll(strings.ReplaceAll(grant, "kind: Grant", "kind: Deny"), "grant-loopback", "deny-loopback")
			deny = strings.ReplaceAll(deny, "validity: {mode: permanent}", "validity: {mode: bounded, notBefore: '"+start.Format(time.RFC3339Nano)+"', expiresAt: '"+end.Format(time.RFC3339Nano)+"'}")
			raw += "---\n" + deny
		}
		args := processArgs(t, raw)
		p := startCommand(t, args)
		p.until(t, confirmed)
		processProbe(t, true)
		p.stop(t, syscall.SIGKILL, true)
		time.Sleep(time.Until(start.Add(50 * time.Millisecond)))
		processProbe(t, false)
		time.Sleep(time.Until(end.Add(50 * time.Millisecond)))
		processProbe(t, true)
		st, e, log := onceCommand(t, args)
		if e != nil || !st.EnforcementConfirmed || !st.Desired.Restored {
			t.Fatal(st, e, log)
		}
		t.Logf("actual process died before future deny [%s,%s); kernel independently restricted both protocols/families and reopened once confirmed same state", start.Format(time.RFC3339Nano), end.Format(time.RFC3339Nano))
	})
}
func TestEnforceProcessFutureAllow(t *testing.T) {
	isolatedProcess(t, func() {
		processServers(t)
		start := time.Now().Add(2 * time.Second).UTC()
		end := start.Add(2 * time.Second)
		raw := strings.ReplaceAll(example(t), "validity: {mode: permanent}", "validity: {mode: bounded, notBefore: '"+start.Format(time.RFC3339Nano)+"', expiresAt: '"+end.Format(time.RFC3339Nano)+"'}")
		args := processArgs(t, raw)
		p := startCommand(t, args)
		p.until(t, confirmed)
		processProbe(t, false)
		p.stop(t, syscall.SIGKILL, true)
		time.Sleep(time.Until(start.Add(50 * time.Millisecond)))
		processProbe(t, true)
		time.Sleep(time.Until(end.Add(50 * time.Millisecond)))
		processProbe(t, false)
		t.Log("actual binary killed before future allow; both-family TCP/UDP activated and expired without any process")
	})
}
func TestEnforceProcessStartupRefusals(t *testing.T) {
	isolatedProcess(t, func() {
		processServers(t)
		args := processArgs(t, example(t))
		p := startCommand(t, args)
		p.until(t, confirmed)
		alternate := changedArg(args, "--state-dir", filepath.Join(t.TempDir(), "alternate"))
		if _, e, log := onceCommand(t, alternate); e == nil || !strings.Contains(log, "singleton") {
			t.Fatal("alternate state-dir bypassed namespace singleton", e, log)
		}
		processProbe(t, true)
		p.stop(t, syscall.SIGTERM, false)
		for _, altered := range [][]string{changedArg(args, "--provider", "foreign"), changedArg(args, "--mark-first", "0x504a0001")} {
			if _, e, _ := onceCommand(t, altered); e == nil {
				t.Fatal("startup scope changed")
			}
			processProbe(t, true)
		}
		dir := argument(args, "--state-dir")
		for _, name := range []string{"desired.json", "owner.json"} {
			path := filepath.Join(dir, name)
			original, e := os.ReadFile(path)
			if e != nil {
				t.Fatal(e)
			}
			os.WriteFile(path, []byte("corrupt"), 0600)
			if _, e, _ := onceCommand(t, args); e == nil {
				t.Fatal("corrupt state overwritten", name)
			}
			after, _ := os.ReadFile(path)
			if string(after) != "corrupt" {
				t.Fatal("corrupt state replaced", name)
			}
			processProbe(t, true)
			os.WriteFile(path, original, 0600)
		}
		st, e, log := onceCommand(t, args)
		if e != nil || !st.EnforcementConfirmed {
			t.Fatal(st, e, log)
		}
		t.Log("real process rejected alternate state-dir, provider/range change and both corrupted journals; prior packets/rules retained; SIGTERM released singleton")
	})
}
func TestEnforceProcessDrift(t *testing.T) {
	isolatedProcess(t, func() {
		processServers(t)
		args := processArgs(t, example(t))
		p := startCommand(t, args)
		p.until(t, confirmed)
		dir := argument(args, "--state-dir")
		before, _ := os.ReadFile(filepath.Join(dir, "owner.json"))
		n, e := nftables.New()
		if e != nil {
			t.Fatal(e)
		}
		defer n.CloseLasting()
		tab := &nftables.Table{Name: "picketx", Family: nftables.TableFamilyINet}
		ch := &nftables.Chain{Name: "picketx_admission", Table: tab}
		n.AddRule(&nftables.Rule{Table: tab, Chain: ch, Exprs: []expr.Any{&expr.Counter{}}})
		if e = n.Flush(); e != nil {
			t.Fatal(e)
		}
		st := p.until(t, func(st Status) bool { return st.LastApplyError != "" })
		if st.EnforcementConfirmed || st.Application.EnforcementConfirmed {
			t.Fatal("drift reported confirmed", st)
		}
		select {
		case e = <-p.wait:
			p.stopped = true
			if e == nil {
				t.Fatal("drift exited success")
			}
		case <-time.After(5 * time.Second):
			t.Fatal("drift did not stop")
		}
		after, _ := os.ReadFile(filepath.Join(dir, "owner.json"))
		if !bytes.Equal(before, after) {
			t.Fatal("drift rewrote ownership journal")
		}
		if _, e, _ := onceCommand(t, args); e == nil {
			t.Fatal("restart took over drift")
		}
		t.Log("actual command reported unconfirmed drift, exited nonzero, retained journal/table and refused restart takeover")
	})
}
func TestEnforceProcessForeignAndInvalidOnce(t *testing.T) {
	isolatedProcess(t, func() {
		processServers(t)
		n, e := nftables.New()
		if e != nil {
			t.Fatal(e)
		}
		defer n.CloseLasting()
		tab := n.CreateTable(&nftables.Table{Name: "picketx", Family: nftables.TableFamilyINet})
		n.AddChain(&nftables.Chain{Name: "foreign", Table: tab})
		if e = n.Flush(); e != nil {
			t.Fatal(e)
		}
		args := processArgs(t, example(t))
		if _, e, _ := onceCommand(t, args); e == nil {
			t.Fatal("foreign table adopted")
		}
		chains, e := n.ListChainsOfTableFamily(nftables.TableFamilyINet)
		if e != nil || len(chains) != 1 || chains[0].Name != "foreign" {
			t.Fatal(chains, e)
		}
		n.DelTable(tab)
		if e = n.Flush(); e != nil {
			t.Fatal(e)
		}
		publish(t, argument(args, "--policy"), "broken")
		st, e, _ := onceCommand(t, args)
		if e == nil || st.EnforcementConfirmed || st.Desired.Prepared || st.AppliedGeneration != 0 {
			t.Fatal("bad initial once granted enforcement", st, e)
		}
		publish(t, argument(args, "--policy"), example(t))
		st, e, log := onceCommand(t, args)
		if e != nil || !st.EnforcementConfirmed {
			t.Fatal(st, e, log)
		}
		processProbe(t, true)
		publish(t, argument(args, "--policy"), "broken")
		st, e, log = onceCommand(t, args)
		if e == nil || !st.EnforcementConfirmed || st.Desired.LastError == "" || !st.Desired.Restored {
			t.Fatal("cached once hid current publication failure", st, e, log)
		}
		processProbe(t, true)
		t.Log("actual once refused foreign/invalid startup; valid once retained filtering after exit; invalid cached once emitted retained proof and exited nonzero")
	})
}

func TestEnforceExampleIndependentOfWorkingDirectory(t *testing.T) {
	t.Chdir(t.TempDir())
	if raw := example(t); !strings.Contains(raw, "uid: resource-demo-v1") {
		t.Fatal("embedded process fixture is not the runnable policy")
	}
}
