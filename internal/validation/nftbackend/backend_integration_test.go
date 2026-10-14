//go:build integration

package nftbackend

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/PicketX/PicketXD/internal/validation/fixture"
	"github.com/PicketX/PicketXD/internal/validation/kerneltest"
	"github.com/google/nftables"
	"github.com/google/nftables/expr"
)

func scope() fixture.Snapshot {
	s := fixture.Snapshot{}
	for _, dst := range kerneltest.Destinations {
		for _, p := range []uint8{6, 17} {
			s.Protected = append(s.Protected, fixture.Endpoint{Destination: netip.PrefixFrom(dst, dst.BitLen()), Protocol: p, PortFirst: 8000, PortLast: 8002})
		}
	}
	return s
}
func allow(s fixture.Snapshot, expiry time.Time) fixture.Snapshot {
	for _, e := range s.Protected {
		i := 0
		if e.Destination.Addr().Is6() {
			i = 1
		}
		s.Allow = append(s.Allow, fixture.Rule{Source: netip.PrefixFrom(kerneltest.Sources[i], kerneltest.Sources[i].BitLen()), Endpoint: e, ExpiresAt: expiry})
	}
	return s
}
func apply(t *testing.T, b *Backend, s fixture.Snapshot) {
	t.Helper()
	if err := b.Apply(context.Background(), s, time.Now()); err != nil {
		t.Fatal(err)
	}
}
func probe(t *testing.T, x *kerneltest.Topology, i int, other bool, proto string, port uint16, want bool) {
	t.Helper()
	src := kerneltest.Sources[i]
	if other {
		src = kerneltest.Others[i]
	}
	ctx, cancel := context.WithTimeout(context.Background(), 350*time.Millisecond)
	defer cancel()
	err := x.Probe(ctx, src, kerneltest.Destinations[i], proto, port)
	if (err == nil) != want {
		t.Fatalf("packet src=%s dst=%s %s/%d err=%v want reachable=%v", src, kerneltest.Destinations[i], proto, port, err, want)
	}
	t.Logf("observed %s -> %s %s/%d reachable=%v", src, kerneltest.Destinations[i], proto, port, want)
}
func deniedEventually(t *testing.T, x *kerneltest.Topology, i int, port uint16) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		err := x.Probe(ctx, kerneltest.Sources[i], kerneltest.Destinations[i], "tcp", port)
		cancel()
		if err != nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("new TCP access still admitted after deadline on family %d port %d", i, port)
}
func TestDefaultDenyDualStack(t *testing.T) {
	kerneltest.WithTopology(t, func(x *kerneltest.Topology) {
		b := New(x.NFT)
		apply(t, b, scope())
		for i := range kerneltest.Sources {
			for _, p := range []string{"tcp", "udp"} {
				probe(t, x, i, false, p, 8000, false)
			}
		}
	})
}
func TestScopedAllowDualStack(t *testing.T) {
	kerneltest.WithTopology(t, func(x *kerneltest.Topology) {
		b := New(x.NFT)
		s := allow(scope(), time.Time{})
		for i := range s.Allow {
			s.Allow[i].Endpoint.PortLast = 8000
		}
		apply(t, b, s)
		for i := range kerneltest.Sources {
			for _, p := range []string{"tcp", "udp"} {
				probe(t, x, i, false, p, 8000, true)
				probe(t, x, i, true, p, 8000, false)
				probe(t, x, i, false, p, 8001, false)
			}
		}
	})
}
func TestDenyPrecedence(t *testing.T) {
	kerneltest.WithTopology(t, func(x *kerneltest.Topology) {
		b := New(x.NFT)
		s := allow(scope(), time.Time{})
		s.Deny = append([]fixture.Rule{}, s.Allow...)
		apply(t, b, s)
		for i := range kerneltest.Sources {
			probe(t, x, i, false, "tcp", 8000, false)
			probe(t, x, i, false, "udp", 8000, false)
		}
	})
}
func TestConcatenatedIntervalSets(t *testing.T) {
	kerneltest.WithTopology(t, func(x *kerneltest.Topology) {
		b := New(x.NFT)
		s := allow(scope(), time.Time{})
		for i := range s.Allow {
			r := &s.Allow[i]
			bits := 26
			if r.Source.Addr().Is6() {
				bits = 80
			}
			r.Source = netip.PrefixFrom(r.Source.Addr(), bits).Masked()
			r.Endpoint.PortFirst = 8001
		}
		apply(t, b, s)
		for i := range kerneltest.Sources {
			probe(t, x, i, true, "tcp", 8001, true)
			probe(t, x, i, false, "udp", 8002, true)
			probe(t, x, i, false, "tcp", 8000, false)
		}
	})
}
func TestOverlapRetainsMembership(t *testing.T) {
	kerneltest.WithTopology(t, func(x *kerneltest.Topology) {
		b := New(x.NFT)
		s := allow(scope(), time.Time{})
		prefix := allow(scope(), time.Time{})
		for i := range prefix.Allow {
			bits := 26
			if prefix.Allow[i].Source.Addr().Is6() {
				bits = 80
			}
			prefix.Allow[i].Source = netip.PrefixFrom(prefix.Allow[i].Source.Addr(), bits).Masked()
		}
		s.Allow = append(s.Allow, prefix.Allow...)
		apply(t, b, s)
		s.Allow = prefix.Allow
		apply(t, b, s)
		for i := range kerneltest.Sources {
			probe(t, x, i, false, "tcp", 8000, true)
		}
	})
}
func TestIndependentDeadlines(t *testing.T) {
	kerneltest.WithTopology(t, func(x *kerneltest.Topology) {
		b := New(x.NFT)
		s := allow(scope(), time.Time{})
		for i := range s.Allow {
			s.Allow[i].Endpoint.PortLast = 8000
			s.Allow[i].ExpiresAt = time.Now().Add(2 * time.Second)
		}
		long := allow(scope(), time.Now().Add(20*time.Second))
		for i := range long.Allow {
			long.Allow[i].Endpoint.PortFirst = 8001
			long.Allow[i].Endpoint.PortLast = 8001
		}
		s.Allow = append(s.Allow, long.Allow...)
		apply(t, b, s)
		for i := range kerneltest.Sources {
			probe(t, x, i, false, "tcp", 8000, true)
		}
		for i := range kerneltest.Sources {
			deniedEventually(t, x, i, 8000)
			probe(t, x, i, false, "tcp", 8001, true)
		}
	})
}
func TestNativeTimeoutWithoutController(t *testing.T) {
	kerneltest.WithTopology(t, func(x *kerneltest.Topology) {
		b := New(x.NFT)
		apply(t, b, allow(scope(), time.Now().Add(2*time.Second)))
		for i := range kerneltest.Sources {
			probe(t, x, i, false, "tcp", 8000, true)
		}
		for i := range kerneltest.Sources {
			deniedEventually(t, x, i, 8000)
		}
	})
}
func TestGracefulExpiry(t *testing.T) {
	kerneltest.WithTopology(t, func(x *kerneltest.Topology) {
		b := New(x.NFT)
		apply(t, b, allow(scope(), time.Now().Add(2*time.Second)))
		var cs []net.Conn
		for i := range kerneltest.Sources {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			c, err := x.OpenTCP(ctx, kerneltest.Sources[i], kerneltest.Destinations[i], 8000)
			if err != nil {
				cancel()
				t.Fatal(err)
			}
			if err = kerneltest.Exchange(ctx, c); err != nil {
				cancel()
				t.Fatal(err)
			}
			cancel()
			cs = append(cs, c)
			t.Cleanup(func() { c.Close() })
		}
		for i, c := range cs {
			deniedEventually(t, x, i, 8000)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			err := kerneltest.Exchange(ctx, c)
			cancel()
			if err != nil {
				t.Fatalf("graceful retained connection: %v", err)
			}
		}
	})
}
func TestRestartPreservesDeadline(t *testing.T) {
	kerneltest.WithTopology(t, func(x *kerneltest.Topology) {
		b := New(x.NFT)
		s := allow(scope(), time.Now().Add(2*time.Second))
		apply(t, b, s)
		for i := range kerneltest.Sources {
			probe(t, x, i, false, "tcp", 8000, true)
		}
		deniedEventually(t, x, 0, 8000)
		b = New(x.NFT)
		apply(t, b, s)
		for i := range kerneltest.Sources {
			probe(t, x, i, false, "tcp", 8000, false)
		}
	})
}
func TestInvalidBatchPreservesPreviousState(t *testing.T) {
	kerneltest.WithTopology(t, func(x *kerneltest.Topology) {
		b := New(x.NFT)
		apply(t, b, allow(scope(), time.Time{}))
		b.inject = func(c *nftables.Conn) {
			c.AddRule(&nftables.Rule{Table: &nftables.Table{Name: "picketx_nonexistent", Family: nftables.TableFamilyINet}, Chain: &nftables.Chain{Name: "picketx_missing"}, Exprs: []expr.Any{&expr.Verdict{Kind: expr.VerdictDrop}}})
		}
		if err := b.Apply(context.Background(), scope(), time.Now()); err == nil {
			t.Fatal("invalid Netlink batch unexpectedly succeeded")
		}
		for i := range kerneltest.Sources {
			probe(t, x, i, false, "tcp", 8000, true)
			probe(t, x, i, true, "tcp", 8000, false)
		}
	})
}
