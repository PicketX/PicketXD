//go:build integration

package ctbackend_test

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/PicketX/PicketXD/internal/validation/ctbackend"
	"github.com/PicketX/PicketXD/internal/validation/fixture"
	"github.com/PicketX/PicketXD/internal/validation/kerneltest"
	"github.com/PicketX/PicketXD/internal/validation/nftbackend"
	"github.com/PicketX/PicketXD/internal/validation/scenario"
	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"github.com/vishvananda/netlink"
)

func snapshot() fixture.Snapshot {
	s := fixture.Snapshot{}
	for i, dst := range kerneltest.Destinations {
		for _, p := range []uint8{6, 17} {
			e := fixture.Endpoint{Destination: netip.PrefixFrom(dst, dst.BitLen()), Protocol: p, PortFirst: 8000, PortLast: 8002}
			s.Protected = append(s.Protected, e)
			s.Allow = append(s.Allow, fixture.Rule{Source: netip.PrefixFrom(kerneltest.Sources[i], kerneltest.Sources[i].BitLen()), Endpoint: e})
		}
	}
	return s
}
func apply(t *testing.T, b *nftbackend.Backend, s fixture.Snapshot) {
	t.Helper()
	if err := b.Apply(context.Background(), s, time.Now()); err != nil {
		t.Fatal(err)
	}
}
func open(t *testing.T, x *kerneltest.Topology, i int, dst netip.Addr, proto string, port uint16) net.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var c net.Conn
	var err error
	if proto == "tcp" {
		c, err = x.OpenTCP(ctx, kerneltest.Sources[i], dst, port)
	} else {
		c, err = x.OpenUDP(ctx, kerneltest.Sources[i], dst, port)
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	exchange(t, c, true)
	return c
}
func exchange(t *testing.T, c net.Conn, want bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	err := kerneltest.Exchange(ctx, c)
	if (err == nil) != want {
		t.Fatalf("existing %s -> %s err=%v want reachable=%v", c.LocalAddr(), c.RemoteAddr(), err, want)
	}
	t.Logf("observed existing %s -> %s reachable=%v", c.LocalAddr(), c.RemoteAddr(), want)
}
func revoke(t *testing.T, b *nftbackend.Backend, r *ctbackend.Backend, before, after fixture.Snapshot, minSelected int) ctbackend.Result {
	t.Helper()
	got, err := scenario.ApplyStrict(context.Background(), b, r, before, after, nftbackend.TestMark, time.Now())
	if err != nil || got.Selected < minSelected || got.Remaining != 0 {
		t.Fatalf("strict result=%+v err=%v require selected>=%d and no remaining targets", got, err, minSelected)
	}
	t.Logf("revocation selected=%d deleted=%d remaining=%d", got.Selected, got.Deleted, got.Remaining)
	return got
}
func TestStrictRevokeDualStack(t *testing.T) {
	kerneltest.WithTopology(t, func(x *kerneltest.Topology) {
		b := nftbackend.New(x.NFT)
		r := ctbackend.New(x.CT)
		before := snapshot()
		apply(t, b, before)
		var cs []net.Conn
		for i, dst := range kerneltest.Destinations {
			cs = append(cs, open(t, x, i, dst, "tcp", 8000))
		}
		after := snapshot()
		after.Allow = nil
		revoke(t, b, r, before, after, 2)
		for _, c := range cs {
			exchange(t, c, false)
		}
	})
}
func TestStrictRevokePreservesOtherGrant(t *testing.T) {
	kerneltest.WithTopology(t, func(x *kerneltest.Topology) {
		b := nftbackend.New(x.NFT)
		r := ctbackend.New(x.CT)
		before := snapshot()
		after := snapshot()
		for i := range after.Allow {
			bits := 26
			if after.Allow[i].Source.Addr().Is6() {
				bits = 80
			}
			after.Allow[i].Source = netip.PrefixFrom(after.Allow[i].Source.Addr(), bits).Masked()
			after.Allow[i].Endpoint.PortLast = 8000
		}
		before.Allow = append(before.Allow, after.Allow...)
		apply(t, b, before)
		var kept, withdrawn []net.Conn
		for i, dst := range kerneltest.Destinations {
			kept = append(kept, open(t, x, i, dst, "tcp", 8000))
			withdrawn = append(withdrawn, open(t, x, i, dst, "tcp", 8001))
		}
		revoke(t, b, r, before, after, 2)
		for _, c := range kept {
			exchange(t, c, true)
		}
		for _, c := range withdrawn {
			exchange(t, c, false)
		}
	})
}
func TestStrictRevokePreservesUnrelatedFlow(t *testing.T) {
	kerneltest.WithTopology(t, func(x *kerneltest.Topology) {
		b := nftbackend.New(x.NFT)
		r := ctbackend.New(x.CT)
		before := snapshot()
		apply(t, b, before)
		var kept, withdrawn []net.Conn
		for i, dst := range kerneltest.Destinations {
			kept = append(kept, open(t, x, i, dst, "tcp", 9000))
			withdrawn = append(withdrawn, open(t, x, i, dst, "tcp", 8000))
		}
		after := snapshot()
		after.Allow = nil
		revoke(t, b, r, before, after, 2)
		for _, c := range kept {
			exchange(t, c, true)
		}
		for _, c := range withdrawn {
			exchange(t, c, false)
		}
	})
}
func TestOldTCPPacketsCannotResurrect(t *testing.T) {
	kerneltest.WithTopology(t, func(x *kerneltest.Topology) {
		b := nftbackend.New(x.NFT)
		r := ctbackend.New(x.CT)
		before := snapshot()
		apply(t, b, before)
		var cs []net.Conn
		for i, dst := range kerneltest.Destinations {
			cs = append(cs, open(t, x, i, dst, "tcp", 8000))
		}
		after := snapshot()
		after.Allow = nil
		revoke(t, b, r, before, after, 2)
		for _, c := range cs {
			exchange(t, c, false)
		}
		apply(t, b, before)
		for _, c := range cs {
			exchange(t, c, false)
		}
		for i, dst := range kerneltest.Destinations {
			open(t, x, i, dst, "tcp", 8000)
		}
	})
}
func TestUDPAfterDeletionRequiresCurrentAllow(t *testing.T) {
	kerneltest.WithTopology(t, func(x *kerneltest.Topology) {
		b := nftbackend.New(x.NFT)
		r := ctbackend.New(x.CT)
		before := snapshot()
		apply(t, b, before)
		var cs []net.Conn
		for i, dst := range kerneltest.Destinations {
			cs = append(cs, open(t, x, i, dst, "udp", 8000))
		}
		after := snapshot()
		after.Allow = nil
		revoke(t, b, r, before, after, 2)
		for _, c := range cs {
			exchange(t, c, false)
		}
		apply(t, b, before)
		for _, c := range cs {
			exchange(t, c, true)
		}
	})
}
func TestNATScopedDeletion(t *testing.T) {
	kerneltest.WithTopology(t, func(x *kerneltest.Topology) {
		b := nftbackend.New(x.NFT)
		r := ctbackend.New(x.CT)
		before := snapshot()
		for i := range before.Protected {
			family := 0
			if before.Protected[i].Destination.Addr().Is6() {
				family = 1
			}
			before.Protected[i].Destination = netip.PrefixFrom(kerneltest.NATDestinations[family], kerneltest.NATDestinations[family].BitLen())
			before.Allow[i].Endpoint = before.Protected[i]
		}
		apply(t, b, before)
		if err := x.DNAT(); err != nil {
			t.Fatal(err)
		}
		var kept, withdrawn []net.Conn
		for i, vip := range kerneltest.NATDestinations {
			withdrawn = append(withdrawn, open(t, x, i, vip, "tcp", 8000))
			kept = append(kept, open(t, x, i, vip, "tcp", 8001))
			family := netlink.FAMILY_V4
			if i == 1 {
				family = netlink.FAMILY_V6
			}
			flows, err := x.CT.ConntrackTableList(netlink.ConntrackTable, netlink.InetFamily(family))
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, f := range flows {
				if f.Mark == nftbackend.TestMark && f.Forward.DstIP.Equal(net.IP(vip.AsSlice())) && f.Forward.DstPort == 8000 {
					if !f.Reverse.SrcIP.Equal(net.IP(kerneltest.Destinations[i].AsSlice())) || f.Reverse.SrcPort != 9000 {
						t.Fatalf("wrong real NAT reply tuple: %v", f)
					}
					found = true
				}
			}
			if !found {
				t.Fatal("owned NAT flow not observed")
			}
		}
		after := before
		after.Allow = append([]fixture.Rule{}, before.Allow...)
		for i := range after.Allow {
			after.Allow[i].Endpoint.PortFirst = 8001
			after.Allow[i].Endpoint.PortLast = 8001
		}
		apply(t, b, after)
		if err := x.DNAT(); err != nil {
			t.Fatal(err)
		}
		got, err := r.Revoke(context.Background(), before, after, nftbackend.TestMark, time.Now())
		if err != nil || got.Selected < 2 || got.Remaining != 0 {
			t.Fatalf("NAT result=%+v err=%v", got, err)
		}
		t.Logf("revocation selected=%d deleted=%d remaining=%d", got.Selected, got.Deleted, got.Remaining)
		for _, c := range kept {
			exchange(t, c, true)
		}
		for _, c := range withdrawn {
			exchange(t, c, false)
		}
	})
}

type invalidBatchApplier struct {
	b *nftbackend.Backend
	c *nftables.Conn
}

func (a invalidBatchApplier) Apply(ctx context.Context, s fixture.Snapshot, now time.Time) error {
	a.c.AddRule(&nftables.Rule{Table: &nftables.Table{Name: "picketx_nonexistent", Family: nftables.TableFamilyINet}, Chain: &nftables.Chain{Name: "picketx_missing"}, Exprs: []expr.Any{&expr.Verdict{Kind: expr.VerdictDrop}}})
	return a.b.Apply(ctx, s, now)
}
func TestApplyFailureDoesNotTriggerDeletion(t *testing.T) {
	kerneltest.WithTopology(t, func(x *kerneltest.Topology) {
		b := nftbackend.New(x.NFT)
		r := ctbackend.New(x.CT)
		before := snapshot()
		apply(t, b, before)
		var cs []net.Conn
		for i, dst := range kerneltest.Destinations {
			cs = append(cs, open(t, x, i, dst, "tcp", 8000))
		}
		after := snapshot()
		after.Allow = nil
		got, err := scenario.ApplyStrict(context.Background(), invalidBatchApplier{b, x.NFT}, r, before, after, nftbackend.TestMark, time.Now())
		if err == nil || got.Deleted != 0 {
			t.Fatalf("failed candidate result=%+v err=%v", got, err)
		}
		for _, c := range cs {
			exchange(t, c, true)
		}
		for i, dst := range kerneltest.Destinations {
			open(t, x, i, dst, "tcp", 8000)
		}
	})
}
