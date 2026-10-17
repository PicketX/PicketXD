//go:build integration

// SPDX-License-Identifier: GPL-3.0-or-later
package hostowner

import (
	"context"
	"errors"
	"github.com/PicketX/PicketX/pkg/core"
	"github.com/PicketX/PicketX/pkg/networkplan"
	"github.com/PicketX/PicketXD/internal/hostpolicy"
	"github.com/PicketX/PicketXD/internal/hostrevoke"
	"github.com/PicketX/PicketXD/internal/hostrules"
	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	nl "github.com/mdlayher/netlink"
	vnet "github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"io"
	"net"
	"net/netip"
	"os"
	"reflect"
	"syscall"
	"testing"
	"time"
)

func cleanupPolicy(t *testing.T, keep bool, edit ...func(*networkplan.Input)) *hostpolicy.Policy {
	t.Helper()
	all := []func(*networkplan.Input){func(in *networkplan.Input) {
		in.Logical.Resources[0].Endpoints = append(in.Logical.Resources[0].Endpoints, core.Endpoint{Name: "dns"})
		in.Networks = append(in.Networks, networkplan.NetworkDefinition{Entry: networkplan.EntryRef{ResourceName: "uid", EndpointName: "dns"}, Protocol: networkplan.UDP, Port: 18444})
		in.Logical.Grants[1].Source = netip.MustParsePrefix("::/0")
		if keep {
			for i := range in.Logical.Grants {
				in.Logical.Grants[i].Revoked = true
			}
			for _, source := range []string{"127.0.0.2/32", "::2/128"} {
				g := in.Logical.Grants[0]
				g.ID = "keep-" + source
				g.Source = netip.MustParsePrefix(source)
				g.Revoked = false
				in.Logical.Grants = append(in.Logical.Grants, g)
			}
		}
	}}
	all = append(all, edit...)
	return dualAdmissionPolicy(t, true, false, all...)
}
func cleanupServers(t *testing.T) {
	t.Helper()
	admissionServers(t)
	lo, e := vnet.LinkByName("lo")
	if e != nil {
		t.Fatal(e)
	}
	if e = vnet.AddrAdd(lo, &vnet.Addr{IPNet: &net.IPNet{IP: net.ParseIP("::2"), Mask: net.CIDRMask(128, 128)}}); e != nil {
		t.Fatal(e)
	}
	for _, addr := range []string{"127.0.0.1:18444", "[::1]:18444"} {
		p, e := net.ListenPacket("udp", addr)
		if e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() { p.Close() })
		go func() {
			b := make([]byte, 32)
			for {
				n, a, e := p.ReadFrom(b)
				if e != nil {
					return
				}
				p.WriteTo(b[:n], a)
			}
		}()
	}
}
func cleanupDial(t *testing.T, protocol, source, destination string, port string) net.Conn {
	t.Helper()
	d := net.Dialer{Timeout: time.Second}
	if protocol == "tcp" {
		d.LocalAddr = &net.TCPAddr{IP: net.ParseIP(source)}
	} else {
		d.LocalAddr = &net.UDPAddr{IP: net.ParseIP(source)}
	}
	c, e := d.Dial(protocol, net.JoinHostPort(destination, port))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { c.Close() })
	cleanupEcho(t, c, true)
	return c
}
func cleanupEcho(t *testing.T, c net.Conn, allowed bool) {
	t.Helper()
	c.SetDeadline(time.Now().Add(250 * time.Millisecond))
	_, e := c.Write([]byte("c"))
	var b [1]byte
	if e == nil {
		_, e = io.ReadFull(c, b[:])
	}
	if allowed && (e != nil || b[0] != 'c') || !allowed && e == nil {
		t.Fatal("cleanup echo", allowed, e)
	}
	c.SetDeadline(time.Time{})
}
func cleanupClients(t *testing.T) (removed, kept []net.Conn) {
	t.Helper()
	for _, s := range []struct {
		src, dst string
		keep     bool
	}{{"127.0.0.1", "127.0.0.1", false}, {"127.0.0.2", "127.0.0.1", true}, {"::1", "::1", false}, {"::2", "::1", true}} {
		for _, proto := range []string{"tcp", "udp"} {
			port := "18443"
			if proto == "udp" {
				port = "18444"
			}
			c := cleanupDial(t, proto, s.src, s.dst, port)
			if s.keep {
				kept = append(kept, c)
			} else {
				removed = append(removed, c)
			}
		}
	}
	return
}
func cleanupSnapshot(t *testing.T, b *nftKernel) []hostrevoke.Flow {
	t.Helper()
	var all []hostrevoke.Flow
	for _, family := range []uint8{2, 10} {
		f, e := b.ConntrackFlows(context.Background(), family)
		if e != nil {
			t.Fatal(e)
		}
		all = append(all, f...)
	}
	return all
}
func cleanupPlan(t *testing.T, s *Session, old hostrules.Image) *hostrevoke.Plan {
	t.Helper()
	p, e := hostrevoke.New([]hostrules.Image{old}, *s.record.Application.Confirmed, *s.record.Marks)
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func selectedCleanup(t *testing.T, p *hostrevoke.Plan, flows []hostrevoke.Flow) []hostrevoke.Flow {
	t.Helper()
	f, e := p.Select(flows, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	return f
}
func containsFlow(flows []hostrevoke.Flow, f hostrevoke.Flow) bool {
	for _, v := range flows {
		if v == f {
			return true
		}
	}
	return false
}
func seedCleanupFlow(t *testing.T, b *nftKernel, f hostrevoke.Flow) {
	t.Helper()
	m, e := hostrevoke.DeleteMessage(f)
	if e != nil {
		t.Fatal(e)
	}
	a, e := nl.UnmarshalAttributes(m.Data[4:])
	if e != nil {
		t.Fatal(e)
	}
	a = a[:2] // original tuple and explicit zone; ID is kernel assigned
	reply := f
	reply.Original = f.Reply
	m2, e := hostrevoke.DeleteMessage(reply)
	if e != nil {
		t.Fatal(e)
	}
	ra, e := nl.UnmarshalAttributes(m2.Data[4:])
	if e != nil {
		t.Fatal(e)
	}
	a = append(a, nl.Attribute{Type: 2 | nl.Nested, Data: ra[0].Data}, nl.Attribute{Type: 8, Data: be32(f.Mark)}, nl.Attribute{Type: 7, Data: be32(120)})
	data, e := nl.MarshalAttributes(a)
	if e != nil {
		t.Fatal(e)
	}
	if e = b.reader.(*strictNetlink).applyMessages(context.Background(), []nl.Message{{Header: nl.Header{Type: 256, Flags: nl.Request | nl.Acknowledge | nl.Create | nl.Excl}, Data: append([]byte{f.Family, 0, 0, 0}, data...)}}); e != nil {
		t.Fatal("seed exact test flow", e)
	}
}
func flowForCleanup(im hostrules.Image, family uint8, port uint16) hostrevoke.Flow {
	src, dst := netip.MustParseAddr("127.0.0.1"), netip.MustParseAddr("127.0.0.1")
	if family == 10 {
		src, dst = netip.IPv6Loopback(), netip.IPv6Loopback()
	}
	var mark uint32
	for _, t := range im.Targets {
		for _, ep := range t.Protected {
			if ep.Protocol == 17 && ep.Destination.Contains(dst) {
				mark = t.Mark
			}
		}
	}
	return hostrevoke.Flow{Family: family, Mark: mark, Original: hostrevoke.Tuple{Source: src, Destination: dst, SourcePort: port, DestinationPort: 18444, Protocol: 17}, Reply: hostrevoke.Tuple{Source: dst, Destination: src, SourcePort: 18444, DestinationPort: port, Protocol: 17}}
}
func TestOwnerKernelCleanupPreservation(t *testing.T) {
	isolated(t, func() {
		cleanupServers(t)
		c := kernelConfig(t)
		s := openKernel(t, c)
		defer s.Close()
		applyAdmission(t, s, cleanupPolicy(t, false))
		old := *s.record.Application.Confirmed
		removed, kept := cleanupClients(t)
		b := s.backend.(*nftKernel)
		var unrelated []hostrevoke.Flow
		for _, family := range []uint8{2, 10} {
			for i, mark := range []uint32{0, 999} {
				f := flowForCleanup(old, family, uint16(37000+i))
				f.Mark = mark
				seedCleanupFlow(t, b, f)
			}
			f := flowForCleanup(old, family, 37003)
			if family == 2 {
				f.Original.Source = netip.MustParseAddr("192.0.2.1")
			} else { // old IPv6 grant covers ::/0, so use a wrong physical target instead.
				f.Original.DestinationPort = 19999
				f.Reply.SourcePort = 19999
			}
			f.Reply.Destination = f.Original.Source
			seedCleanupFlow(t, b, f)
		}
		applyAdmission(t, s, cleanupPolicy(t, true))
		p := cleanupPlan(t, s, old)
		before := cleanupSnapshot(t, b)
		if n := len(selectedCleanup(t, p, before)); n != 4 {
			t.Fatal("expected four withdrawn packet flows", n, before)
		}
		for _, f := range before {
			if len(selectedCleanup(t, p, []hostrevoke.Flow{f})) == 0 {
				unrelated = append(unrelated, f)
			}
		}
		journal, _ := os.ReadFile(c.StatePath)
		r, e := s.RevokeWithdrawn(context.Background(), []hostrules.Image{old})
		if e != nil || r.Selected != 4 || r.Deleted != 4 || r.Remaining != 0 {
			t.Fatal(r, e)
		}
		after := cleanupSnapshot(t, b)
		for _, f := range unrelated {
			if !containsFlow(after, f) {
				t.Fatal("unrelated/independently allowed exact flow lost", f)
			}
		}
		for _, c := range kept {
			cleanupEcho(t, c, true)
		}
		for _, c := range removed {
			cleanupEcho(t, c, false)
		}
		// The kernel operation does not update the application journal.
		afterJournal, _ := os.ReadFile(c.StatePath)
		if !reflect.DeepEqual(journal, afterJournal) {
			t.Fatal("cleanup wrote journal")
		}
		t.Logf("production cleanup selected=%d deleted=%d remaining=%d; both-family TCP/UDP revoked, independent source grants and unmarked/unrelated marks/targets/sources preserved", r.Selected, r.Deleted, r.Remaining)
	})
}
func cleanupNAT(t *testing.T) {
	t.Helper()
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
}
func TestOwnerKernelCleanupNATZoneID(t *testing.T) {
	isolated(t, func() {
		cleanupServers(t)
		cleanupNAT(t)
		s := openKernel(t, kernelConfig(t))
		defer s.Close()
		edit := func(in *networkplan.Input) { in.Networks[0].Port = 18543 }
		applyAdmission(t, s, cleanupPolicy(t, false, edit))
		old := *s.record.Application.Confirmed
		b := s.backend.(*nftKernel)
		for _, addr := range []string{"127.0.0.1", "::1"} {
			cleanupDial(t, "tcp", addr, addr, "18543")
		}
		for _, family := range []uint8{2, 10} {
			for _, zone := range []uint16{0, 7} {
				f := flowForCleanup(old, family, 38000)
				f.Zone = zone
				seedCleanupFlow(t, b, f)
			}
		}
		all := cleanupSnapshot(t, b)
		nat := 0
		for _, f := range all {
			if f.Original.Protocol == 6 && f.Original.DestinationPort == 18543 {
				if f.Reply.SourcePort != 18443 {
					t.Fatal("test did not traverse DNAT", f)
				}
				nat++
			}
		}
		if nat != 2 {
			t.Fatal("missing actual dual-stack NAT flows", all)
		}
		for _, family := range []uint8{2, 10} {
			var zero, zoned hostrevoke.Flow
			for _, f := range all {
				if f.Family == family && f.Original.Protocol == 17 && f.Original.SourcePort == 38000 {
					if f.Zone == 7 {
						zoned = f
					} else if f.Zone == 0 {
						zero = f
					}
				}
			}
			if zoned.Family == 0 || zero.Family == 0 {
				t.Fatal("missing zones", all)
			}
			wrong := zoned
			wrong.ID[0] ^= 0xff
			if e := b.DeleteConntrackFlow(context.Background(), wrong); !errors.Is(e, syscall.ENOENT) {
				t.Fatal("ID mismatch not refused", e)
			}
			wrong = zoned
			wrong.Zone = 8
			if e := b.DeleteConntrackFlow(context.Background(), wrong); !errors.Is(e, syscall.ENOENT) {
				t.Fatal("zone mismatch not refused", e)
			}
			after := cleanupSnapshot(t, b)
			if !containsFlow(after, zoned) || !containsFlow(after, zero) {
				t.Fatal("mismatched request changed entries")
			}
			if e := b.DeleteConntrackFlow(context.Background(), zoned); e != nil {
				t.Fatal(e)
			}
			after = cleanupSnapshot(t, b)
			if containsFlow(after, zoned) || !containsFlow(after, zero) {
				t.Fatal("exact zone7 delete damaged zone0")
			}
		}
		applyAdmission(t, s, cleanupPolicy(t, true, edit))
		r, e := s.RevokeWithdrawn(context.Background(), []hostrules.Image{old})
		if e != nil || r.Selected != 4 || r.Deleted != 4 || r.Remaining != 0 {
			t.Fatal("NAT/original cleanup", r, e)
		}
		t.Log("actual original DNAT TCP tuples deleted in both families; wrong opaque ID and zone rejected, exact zone7 deletion preserved identical zone0 tuple; final scoped cleanup confirmed zero remaining")
	})
}

type cleanupFaultKernel struct {
	*nftKernel
	badDump, failDelete bool
	deleteCalls         int
}

func (b *cleanupFaultKernel) ConntrackFlows(ctx context.Context, f uint8) ([]hostrevoke.Flow, error) {
	flows, e := b.nftKernel.ConntrackFlows(ctx, f)
	if e != nil {
		return nil, e
	}
	if b.badDump && f == 10 {
		return flows, errors.New("injected interrupted IPv6 after native read")
	}
	return flows, nil
}
func (b *cleanupFaultKernel) DeleteConntrackFlow(ctx context.Context, f hostrevoke.Flow) error {
	b.deleteCalls++
	if b.failDelete && f.Family == 10 {
		return errors.New("injected IPv6 delete failure")
	}
	return b.nftKernel.DeleteConntrackFlow(ctx, f)
}
func cleanupSeedPair(t *testing.T, s *Session) hostrules.Image {
	t.Helper()
	applyAdmission(t, s, cleanupPolicy(t, false))
	old := *s.record.Application.Confirmed
	b := s.backend.(*nftKernel)
	for _, family := range []uint8{2, 10} {
		seedCleanupFlow(t, b, flowForCleanup(old, family, 39000))
	}
	applyAdmission(t, s, cleanupPolicy(t, true))
	return old
}
func TestOwnerKernelCleanupPreflightDrift(t *testing.T) {
	for _, mode := range []string{"incomplete family", "drift"} {
		t.Run(mode, func(t *testing.T) {
			isolated(t, func() {
				cleanupServers(t)
				c := kernelConfig(t)
				s := openKernel(t, c)
				defer s.Close()
				old := cleanupSeedPair(t, s)
				native := s.backend.(*nftKernel)
				before := cleanupSnapshot(t, native)
				journal, e := os.ReadFile(c.StatePath)
				if e != nil {
					t.Fatal(e)
				}
				b := &cleanupFaultKernel{nftKernel: native, badDump: mode == "incomplete family"}
				s.backend = b
				if mode == "drift" {
					n := connectKernel(t)
					tab := kernelTable()
					n.AddRule(&nftables.Rule{Table: tab, Chain: &nftables.Chain{Table: tab, Name: "picketx_admission"}, Exprs: []expr.Any{&expr.Counter{}}})
					if e = n.Flush(); e != nil {
						t.Fatal(e)
					}
				}
				r, e := s.RevokeWithdrawn(context.Background(), []hostrules.Image{old})
				if e == nil || b.deleteCalls != 0 || r.Remaining != -1 || s.ApplicationStatus().EnforcementConfirmed {
					t.Fatal("unsafe preflight", r, e, b.deleteCalls)
				}
				after := cleanupSnapshot(t, native)
				for _, f := range before {
					if !containsFlow(after, f) {
						t.Fatal("preflight lost exact flow", f)
					}
				}
				afterJournal, e := os.ReadFile(c.StatePath)
				if e != nil || !reflect.DeepEqual(journal, afterJournal) {
					t.Fatal("preflight changed journal", e)
				}
				t.Log("actual two-family connection records preserved with zero delete calls after incomplete-family evidence or real nft drift; session requires reopen")
			})
		})
	}
}
func TestOwnerKernelCleanupPartialFailure(t *testing.T) {
	isolated(t, func() {
		cleanupServers(t)
		c := kernelConfig(t)
		s := openKernel(t, c)
		old := cleanupSeedPair(t, s)
		native := s.backend.(*nftKernel)
		b := &cleanupFaultKernel{nftKernel: native, failDelete: true}
		s.backend = b
		r, e := s.RevokeWithdrawn(context.Background(), []hostrules.Image{old})
		if e == nil || r.Selected != 2 || r.Deleted != 1 || r.Remaining != 1 || s.ApplicationStatus().EnforcementConfirmed {
			t.Fatal("partial delete claimed completion", r, e)
		}
		left := selectedCleanup(t, cleanupPlan(t, s, old), cleanupSnapshot(t, native))
		if len(left) != 1 || left[0].Family != 10 {
			t.Fatal("wrong partial native state", left)
		}
		s.Close()
		s = openKernel(t, c)
		defer s.Close()
		r, e = s.RevokeWithdrawn(context.Background(), []hostrules.Image{old})
		if e != nil || r.Selected != 1 || r.Deleted != 1 || r.Remaining != 0 {
			t.Fatal("reopened partial retry failed", r, e)
		}
		t.Log("actual IPv4 deletion succeeded, injected IPv6 deletion failed and complete native readback reported one remaining; reopen/retry deleted only remaining exact flow")
	})
}
