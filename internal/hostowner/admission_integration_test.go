//go:build integration

// SPDX-License-Identifier: GPL-3.0-or-later
package hostowner

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/PicketX/PicketX/pkg/core"
	"github.com/PicketX/PicketX/pkg/networkplan"
	"github.com/PicketX/PicketXD/internal/hostpolicy"
	"github.com/PicketX/PicketXD/internal/hostrules"
	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	nl "github.com/mdlayher/netlink"
	vnet "github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

var admissionAddresses = []string{"127.0.0.1:18443", "[::1]:18443"}

func admissionLoopback(t *testing.T) {
	t.Helper()
	link, e := vnet.LinkByName("lo")
	if e != nil {
		t.Fatal(e)
	}
	if e = vnet.LinkSetUp(link); e != nil {
		t.Fatal(e)
	}
}
func admissionServers(t *testing.T) {
	t.Helper()
	admissionLoopback(t)
	for i, addr := range admissionAddresses {
		network := "tcp4"
		if i == 1 {
			network = "tcp6"
		}
		listener, e := net.Listen(network, addr)
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
	}
}
func admissionConn(t *testing.T, address string, allowed bool) net.Conn {
	t.Helper()
	c, e := net.DialTimeout("tcp", address, 250*time.Millisecond)
	if allowed {
		if e != nil {
			t.Fatalf("expected admission %s: %v", address, e)
		}
		admissionEcho(t, c)
		return c
	}
	if e == nil {
		c.Close()
		t.Fatalf("unexpected admission %s", address)
	}
	return nil
}
func admissionEcho(t *testing.T, c net.Conn) {
	t.Helper()
	c.SetDeadline(time.Now().Add(500 * time.Millisecond))
	if _, e := c.Write([]byte("p")); e != nil {
		t.Fatal(e)
	}
	var b [1]byte
	if _, e := io.ReadFull(c, b[:]); e != nil || b[0] != 'p' {
		t.Fatal("admitted connection lost", e)
	}
	c.SetDeadline(time.Time{})
}
func dualAdmissionPolicy(t *testing.T, allow, empty bool, edit ...func(*networkplan.Input)) *hostpolicy.Policy {
	t.Helper()
	all := []func(*networkplan.Input){func(in *networkplan.Input) {
		if allow {
			g := in.Logical.Grants[0]
			g.ID = "allow-v6"
			g.Source = prefix6()
			in.Logical.Grants = append(in.Logical.Grants, g)
		}
	}}
	all = append(all, edit...)
	return admissionPolicy(t, allow, empty, all...)
}
func prefix6() netip.Prefix { return netip.MustParsePrefix("::1/128") }
func applyAdmission(t *testing.T, s *Session, p *hostpolicy.Policy) ApplicationStatus {
	t.Helper()
	st, e := s.ApplyPolicy(context.Background(), p, kernelMarkRange())
	if e != nil || !st.EnforcementConfirmed {
		t.Fatal(st, e)
	}
	return st
}
func TestOwnerKernelAdmissionDualStack(t *testing.T) {
	isolated(t, func() {
		admissionServers(t)
		c := kernelConfig(t)
		s := openKernel(t, c)
		defer s.Close()
		applyAdmission(t, s, dualAdmissionPolicy(t, false, false))
		for _, a := range admissionAddresses {
			admissionConn(t, a, false)
		}
		applyAdmission(t, s, dualAdmissionPolicy(t, true, false))
		for _, a := range admissionAddresses {
			conn := admissionConn(t, a, true)
			conn.Close()
		}
		p := dualAdmissionPolicy(t, true, false, func(in *networkplan.Input) {
			for _, g := range in.Logical.Grants {
				d := core.Deny(g)
				d.ID = "deny-" + g.ID
				in.Logical.Denies = append(in.Logical.Denies, d)
			}
		})
		applyAdmission(t, s, p)
		for _, a := range admissionAddresses {
			admissionConn(t, a, false)
		}
		t.Log("production Session INPUT: IPv4/IPv6 default deny, scoped allow and deny precedence observed")
	})
}
func TestOwnerKernelAdmissionAbsoluteTime(t *testing.T) {
	isolated(t, func() {
		admissionServers(t)
		c := kernelConfig(t)
		s := openKernel(t, c)
		expiry := time.Now().Add(2 * time.Second).UTC()
		p := dualAdmissionPolicy(t, true, false, func(in *networkplan.Input) {
			for i := range in.Logical.Grants {
				in.Logical.Grants[i].Validity = core.Validity{Mode: core.LifetimeBounded, ExpiresAt: expiry}
			}
		})
		applyAdmission(t, s, p)
		var established []net.Conn
		for _, a := range admissionAddresses {
			conn := admissionConn(t, a, true)
			defer conn.Close()
			established = append(established, conn)
		}
		s.Close()
		time.Sleep(time.Until(expiry.Add(50 * time.Millisecond)))
		for _, conn := range established {
			admissionEcho(t, conn)
		}
		for _, a := range admissionAddresses {
			admissionConn(t, a, false)
		}
		s = openKernel(t, c)
		defer s.Close()
		if !s.ApplicationStatus().EnforcementConfirmed {
			t.Fatal("expired image could not be reobserved")
		}
		for _, a := range admissionAddresses {
			admissionConn(t, a, false)
		}
		t.Logf("absolute expiry %s enforced without Session; marked established flows survived and restart did not extend deadline", expiry.Format(time.RFC3339Nano))
	})
}
func TestOwnerKernelAdmissionFutureDeny(t *testing.T) {
	isolated(t, func() {
		admissionServers(t)
		c := kernelConfig(t)
		s := openKernel(t, c)
		start := time.Now().Add(2 * time.Second).UTC()
		end := start.Add(2 * time.Second)
		p := dualAdmissionPolicy(t, true, false, func(in *networkplan.Input) {
			for _, g := range in.Logical.Grants {
				d := core.Deny(g)
				d.ID = "deny-" + g.ID
				d.Validity = core.Validity{Mode: core.LifetimeBounded, NotBefore: start, ExpiresAt: end}
				in.Logical.Denies = append(in.Logical.Denies, d)
			}
		})
		applyAdmission(t, s, p)
		for _, a := range admissionAddresses {
			conn := admissionConn(t, a, true)
			conn.Close()
		}
		s.Close()
		time.Sleep(time.Until(start.Add(50 * time.Millisecond)))
		for _, a := range admissionAddresses {
			admissionConn(t, a, false)
		}
		time.Sleep(time.Until(end.Add(50 * time.Millisecond)))
		for _, a := range admissionAddresses {
			conn := admissionConn(t, a, true)
			conn.Close()
		}
		t.Logf("future deny [%s,%s) activated and expired for both families without a running Session", start.Format(time.RFC3339Nano), end.Format(time.RFC3339Nano))
	})
}
func TestOwnerKernelAdmissionRecoveryAndDrift(t *testing.T) {
	isolated(t, func() {
		admissionServers(t)
		c := kernelConfig(t)
		s := openKernel(t, c)
		boom := errors.New("crash before confirmed publication")
		s.persist = func(r record) error {
			if r.Application != nil && r.Application.Pending == nil {
				return boom
			}
			return writeRecord(c.StatePath, r)
		}
		if st, e := s.ApplyPolicy(context.Background(), dualAdmissionPolicy(t, true, false), kernelMarkRange()); !errors.Is(e, boom) || st.EnforcementConfirmed {
			t.Fatal(st, e)
		}
		s.Close()
		s = openKernel(t, c)
		defer s.Close()
		if !s.ApplicationStatus().EnforcementConfirmed || s.ApplicationStatus().Attempt != 1 {
			t.Fatal("pending real image not recovered")
		}
		for _, a := range admissionAddresses {
			conn := admissionConn(t, a, true)
			conn.Close()
		}
		n := connectKernel(t)
		tab := kernelTable()
		n.AddRule(&nftables.Rule{Table: tab, Chain: &nftables.Chain{Table: tab, Name: "picketx_admission"}, Exprs: []expr.Any{&expr.Counter{}}})
		if e := n.Flush(); e != nil {
			t.Fatal(e)
		}
		before, _ := os.ReadFile(c.StatePath)
		if _, e := s.ApplyPolicy(context.Background(), dualAdmissionPolicy(t, false, false), kernelMarkRange()); e == nil || s.Status().EnforcementConfirmed {
			t.Fatal("drift replaced or confirmed")
		}
		after, _ := os.ReadFile(c.StatePath)
		if string(before) != string(after) {
			t.Fatal("drift changed journal")
		}
		t.Log("pending applied image recovered from exact full live structure; later extra rule rejected without publication")
	})
}

type rejectedImageKernel struct{ *nftKernel }

func (b *rejectedImageKernel) ApplyImage(ctx context.Context, marker string, im hostrules.Image) error {
	messages, e := imageMessages(marker, im)
	if e != nil {
		return e
	}
	// Test-only malformed operation in the same native atomic transaction.
	invalid := nl.Message{Header: nl.Header{Type: nl.HeaderType(unix.NFNL_SUBSYS_NFTABLES<<8 | unix.NFT_MSG_NEWCHAIN), Flags: nl.Request | nl.Acknowledge | nl.Create}, Data: []byte{unix.NFPROTO_INET, 0, 0, 0}}
	messages = append(messages[:len(messages)-1], invalid, messages[len(messages)-1])
	return b.reader.(*strictNetlink).applyMessages(ctx, messages)
}
func TestOwnerKernelAdmissionAtomicRejection(t *testing.T) {
	isolated(t, func() {
		admissionServers(t)
		c := kernelConfig(t)
		s := openKernel(t, c)
		first := applyAdmission(t, s, dualAdmissionPolicy(t, true, false))
		native := s.backend.(*nftKernel)
		s.backend = &rejectedImageKernel{native}
		st, e := s.ApplyPolicy(context.Background(), dualAdmissionPolicy(t, false, false), kernelMarkRange())
		if e == nil || st.EnforcementConfirmed {
			t.Fatal("invalid batch succeeded")
		}
		s.Close()
		s = openKernel(t, c)
		defer s.Close()
		if s.ApplicationStatus().ImageDigest != first.ImageDigest || !s.ApplicationStatus().EnforcementConfirmed {
			t.Fatal("previous live image not retained")
		}
		for _, a := range admissionAddresses {
			conn := admissionConn(t, a, true)
			conn.Close()
		}
		t.Logf("atomic rejection retained both-family allowed image %s; pending attempt resolved to old image", first.ImageDigest)
	})
}
func TestOwnerKernelAdmissionWithdrawal(t *testing.T) {
	isolated(t, func() {
		admissionServers(t)
		c := kernelConfig(t)
		s := openKernel(t, c)
		applyAdmission(t, s, dualAdmissionPolicy(t, true, false))
		for _, a := range admissionAddresses {
			conn := admissionConn(t, a, true)
			conn.Close()
		}
		applyAdmission(t, s, dualAdmissionPolicy(t, false, false))
		for _, a := range admissionAddresses {
			admissionConn(t, a, false)
		}
		applyAdmission(t, s, dualAdmissionPolicy(t, false, true))
		for _, a := range admissionAddresses {
			conn := admissionConn(t, a, true)
			conn.Close()
		}
		s.Close()
		s = openKernel(t, c)
		defer s.Close()
		if !s.ApplicationStatus().EnforcementConfirmed {
			t.Fatal("empty withdrawal image not recoverable")
		}
		t.Log("last Grant retained default deny; explicit empty Binding inventory withdrew protection and recovered its distinct empty image")
	})
}
func TestOwnerKernelAdmissionProcessRecovery(t *testing.T) {
	isolated(t, func() {
		admissionServers(t)
		c := kernelConfig(t)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		child := exec.CommandContext(ctx, os.Args[0], "-test.run", "^TestOwnerKernelHelper$", "-test.v")
		child.Env = append(os.Environ(), "PICKETX_OWNER_HELPER_PATH="+c.StatePath, "PICKETX_OWNER_HELPER_ADMISSION=1")
		stdout, e := child.StdoutPipe()
		if e != nil {
			t.Fatal(e)
		}
		stdin, e := child.StdinPipe()
		if e != nil {
			t.Fatal(e)
		}
		defer stdin.Close()
		child.Stderr = os.Stderr
		if e = child.Start(); e != nil {
			t.Fatal(e)
		}
		defer func() {
			if child.ProcessState == nil {
				child.Process.Kill()
				child.Wait()
			}
		}()
		reader := bufio.NewReader(stdout)
		var ready ApplicationStatus
		for {
			line, e := reader.ReadBytes('\n')
			if e != nil {
				t.Fatal("child before admission readiness", e)
			}
			if json.Unmarshal(line, &ready) == nil && ready.EnforcementConfirmed {
				break
			}
		}
		for _, a := range admissionAddresses {
			conn := admissionConn(t, a, true)
			conn.Close()
		}
		if e = child.Process.Kill(); e != nil {
			t.Fatal(e)
		}
		if e = child.Wait(); e == nil {
			t.Fatal("helper not killed")
		}
		io.Copy(io.Discard, reader)
		for _, a := range admissionAddresses {
			conn := admissionConn(t, a, true)
			conn.Close()
		}
		s := openKernel(t, c)
		defer s.Close()
		if s.ApplicationStatus().ImageDigest != ready.ImageDigest || !s.ApplicationStatus().EnforcementConfirmed {
			t.Fatal("killed process image recovery failed")
		}
		t.Log(fmt.Sprintf("SIGKILL kept packet filtering; independent reopened Session confirmed image %s", ready.ImageDigest))
	})
}
func TestOwnerKernelAdmissionUDPFutureAllow(t *testing.T) {
	isolated(t, func() {
		admissionLoopback(t)
		for i, address := range admissionAddresses {
			network := "udp4"
			if i == 1 {
				network = "udp6"
			}
			server, e := net.ListenPacket(network, address)
			if e != nil {
				t.Fatal(e)
			}
			t.Cleanup(func() { server.Close() })
			go func() {
				b := make([]byte, 128)
				for {
					n, peer, e := server.ReadFrom(b)
					if e != nil {
						return
					}
					server.WriteTo(b[:n], peer)
				}
			}()
		}
		probe := func(allowed bool) {
			for _, address := range admissionAddresses {
				conn, e := net.Dial("udp", address)
				if e != nil {
					t.Fatal(e)
				}
				conn.SetDeadline(time.Now().Add(250 * time.Millisecond))
				_, e = conn.Write([]byte("u"))
				var b [1]byte
				if e == nil {
					_, e = io.ReadFull(conn, b[:])
				}
				conn.Close()
				if allowed && (e != nil || b[0] != 'u') || !allowed && e == nil {
					t.Fatalf("UDP %s allowed=%v err=%v", address, allowed, e)
				}
			}
		}
		c := kernelConfig(t)
		s := openKernel(t, c)
		start := time.Now().Add(2 * time.Second).UTC()
		end := start.Add(2 * time.Second)
		p := dualAdmissionPolicy(t, true, false, func(in *networkplan.Input) {
			in.Networks[0].Protocol = networkplan.UDP
			for i := range in.Logical.Grants {
				in.Logical.Grants[i].Validity = core.Validity{Mode: core.LifetimeBounded, NotBefore: start, ExpiresAt: end}
			}
		})
		applyAdmission(t, s, p)
		s.Close()
		probe(false)
		time.Sleep(time.Until(start.Add(50 * time.Millisecond)))
		probe(true)
		time.Sleep(time.Until(end.Add(50 * time.Millisecond)))
		probe(false)
		t.Log("native dual-stack UDP future allow activated and expired without Session; new post-expiry tuples denied")
	})
}
func TestOwnerKernelAdmissionOriginalDNAT(t *testing.T) {
	isolated(t, func() {
		admissionServers(t)
		n := connectKernel(t)
		tab := n.CreateTable(&nftables.Table{Name: "picketx_test_nat", Family: nftables.TableFamilyINet})
		priority := nftables.ChainPriority(-100)
		ch := n.AddChain(&nftables.Chain{Name: "picketx_test_output", Table: tab, Type: nftables.ChainTypeNAT, Hooknum: nftables.ChainHookOutput, Priority: &priority})
		for _, family := range []byte{unix.NFPROTO_IPV4, unix.NFPROTO_IPV6} {
			n.AddRule(&nftables.Rule{Table: tab, Chain: ch, Exprs: []expr.Any{&expr.Meta{Key: expr.MetaKeyNFPROTO, Register: 1}, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{family}}, &expr.Meta{Key: expr.MetaKeyL4PROTO, Register: 1}, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{6}}, &expr.Payload{DestRegister: 1, Base: expr.PayloadBaseTransportHeader, Offset: 2, Len: 2}, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{0x48, 0x6f}}, &expr.Immediate{Register: 1, Data: []byte{0x48, 0x0b}}, &expr.NAT{Type: expr.NATTypeDestNAT, Family: uint32(family), RegProtoMin: 1, RegProtoMax: 1}}})
		}
		if e := n.Flush(); e != nil {
			t.Fatal(e)
		}
		c := kernelConfig(t)
		s := openKernel(t, c)
		defer s.Close()
		edit := func(in *networkplan.Input) { in.Networks[0].Port = 18543 }
		applyAdmission(t, s, dualAdmissionPolicy(t, false, false, edit))
		for _, address := range []string{"127.0.0.1:18543", "[::1]:18543"} {
			admissionConn(t, address, false)
		}
		applyAdmission(t, s, dualAdmissionPolicy(t, true, false, edit))
		for _, address := range []string{"127.0.0.1:18543", "[::1]:18543"} {
			conn := admissionConn(t, address, true)
			conn.Close()
		}
		t.Log("production INPUT rules matched pre-DNAT original port 18543 while packets reached listening port 18443 in both families; test NAT table preserved")
	})
}
