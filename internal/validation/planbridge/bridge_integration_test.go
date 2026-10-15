//go:build integration

package planbridge

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/PicketX/PicketX/pkg/core"
	"github.com/PicketX/PicketX/pkg/networkplan"
	"github.com/PicketX/PicketXD/internal/validation/ctbackend"
	"github.com/PicketX/PicketXD/internal/validation/fixture"
	"github.com/PicketX/PicketXD/internal/validation/kerneltest"
	"github.com/PicketX/PicketXD/internal/validation/nftbackend"
	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"github.com/vishvananda/netlink"
)

type liveFixture struct {
	x        *kerneltest.Topology
	c        *Coordinator
	b        *nftbackend.Backend
	r        *ctbackend.Backend
	in       networkplan.Input
	facts    Facts
	expected *core.Engine
}

func withBridge(t *testing.T, mode networkplan.HandlingMode, nat bool, run func(*liveFixture)) {
	t.Helper()
	kerneltest.WithTopology(t, func(x *kerneltest.Topology) {
		stopObservation, err := x.ObserveConntrack()
		if err != nil {
			t.Fatal(err)
		}
		f := testFacts(mode)
		f.NamespaceID = x.Enforcement.UniqueId()
		f.Context.NetworkSpaceID = f.NamespaceID
		f.Epoch = fmt.Sprintf("%x", rand.Text())
		f.NAT = nat
		if nat {
			f.Context.ID = "nat-original"
			f.Context.Destinations = []netip.Prefix{prefix("192.0.2.101/32"), prefix("2001:db8:1:0:2::101/128")}
		}
		scope, e := scopeFromFacts(f)
		if e != nil {
			t.Fatal(e)
		}
		in := testInput(mode)
		in.Contexts = []networkplan.ExecutionContext{f.Context}
		in.Bindings[0].Context.AllowedDestinations = slices.Clone(f.Context.Destinations)
		for i := range in.Logical.Grants {
			in.Logical.Grants[i].Validity = core.Validity{Mode: core.LifetimePermanent}
		}
		if nat {
			in.Logical.Resources[0].Endpoints = in.Logical.Resources[0].Endpoints[:2]
			in.Networks = in.Networks[:2]
			in.Networks[0].Port = 8000
			in.Networks[1].Port = 8001
		}
		b, r := nftbackend.New(x.NFT), ctbackend.New(x.CT)
		effects := Effects{Now: time.Now, Revalidate: func(ctx context.Context, got Facts) error {
			if got.Epoch != f.Epoch || got.NamespaceID != f.NamespaceID {
				return fmt.Errorf("fixture epoch/namespace changed")
			}
			return x.ValidateTransit(ctx, f.NamespaceID)
		}, Apply: b.ApplyReceipt, Confirm: b.Confirm, Remove: b.RemoveOwned, ConfirmAbsent: b.ConfirmAbsent, Revoke: r.Revoke, RevokeEmpty: r.RevokeEmpty}
		c, e := NewCoordinator(scope, effects)
		if e != nil {
			t.Fatal(e)
		}
		// Verify the finite catalog before installing any admission policy. NAT VIPs
		// have listeners too; after translation, the server replies from TCP/9000.
		for i, src := range kerneltest.Sources {
			dest := kerneltest.Destinations[i]
			ports := []uint16{443, 22}
			if nat {
				dest = kerneltest.NATDestinations[i]
				ports = []uint16{8000, 8001}
			}
			for _, port := range ports {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				e = x.Probe(ctx, src, dest, "tcp", port)
				cancel()
				if e != nil {
					t.Fatal("unfiltered catalog", src, dest, port, e)
				}
			}
			if !nat {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				e = x.Probe(ctx, src, dest, "udp", 8002)
				cancel()
				if e != nil {
					t.Fatal(e)
				}
			}
		}
		t.Logf("fixture epoch=%s namespace=%s context=%+v shared=e39f96e2787154a8af8e7d996b2f65aa10b6c3eb", f.Epoch, f.NamespaceID, f.Context)
		// Observe the actual original source/tuple in the enforcement namespace;
		// labels and successful Go conversion cannot establish this contract.
		for i, family := range []netlink.InetFamily{netlink.FAMILY_V4, netlink.FAMILY_V6} {
			flows, e := x.CT.ConntrackTableList(netlink.ConntrackTable, family)
			if e != nil {
				t.Fatal(e)
			}
			seen := false
			dest := kerneltest.Destinations[i]
			if nat {
				dest = kerneltest.NATDestinations[i]
			}
			for _, flow := range flows {
				if flow.Forward.SrcIP.Equal(net.IP(kerneltest.Sources[i].AsSlice())) && flow.Forward.DstIP.Equal(net.IP(dest.AsSlice())) {
					seen = true
					t.Logf("unfiltered observed original=%+v reply=%+v", flow.Forward, flow.Reverse)
				}
			}
			if !seen {
				t.Fatal("source/tuple not observed at bound enforcement namespace", family)
			}
		}
		if err := stopObservation(); err != nil {
			t.Fatal(err)
		}
		run(&liveFixture{x: x, c: c, b: b, r: r, in: in, facts: f})
	})
}
func (l *liveFixture) update(t *testing.T) {
	t.Helper()
	st, e := l.c.Update(context.Background(), l.in)
	logState(t, st, e)
	if e != nil {
		t.Fatal(e)
	}
	l.expected, e = core.Compile(l.in.Logical)
	if e != nil {
		t.Fatal(e)
	}
}
func logState(t *testing.T, s State, e error) {
	t.Helper()
	t.Logf("apply generation=%d observed=%s next=%s empty=%v current=%+v error=%v", s.Generation, s.AppliedAt.Format(time.RFC3339Nano), s.NextChange.Format(time.RFC3339Nano), s.Empty, s.Current, e)
	t.Logf("revocation selected=%d deleted=%d remaining=%d", s.Cleanup.Selected, s.Cleanup.Deleted, s.Cleanup.Remaining)
}
func (l *liveFixture) probe(t *testing.T, i int, other bool, proto string, port uint16, want bool) {
	t.Helper()
	src := kerneltest.Sources[i]
	if other {
		src = kerneltest.Others[i]
	}
	dst := kerneltest.Destinations[i]
	if l.facts.NAT {
		dst = kerneltest.NATDestinations[i]
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Millisecond)
	defer cancel()
	at := time.Now()
	if !l.c.State().Empty && l.expected != nil {
		for _, n := range l.in.Networks {
			if string(n.Protocol) == proto && n.Port == port {
				decision, e := l.expected.Decide(core.Query{Source: src, ResourceName: n.Entry.ResourceName, EndpointName: n.Entry.EndpointName, At: at})
				if e != nil || decision.Allowed != want {
					t.Fatal("literal packet expectation differs from shared Core", decision, e)
				}
				t.Logf("Core at=%s entry=%+v allowed=%v grants=%v denies=%v", at.Format(time.RFC3339Nano), n.Entry, decision.Allowed, decision.GrantIDs, decision.DenyIDs)
			}
		}
	}
	e := l.x.Probe(ctx, src, dst, proto, port)
	t.Logf("packet at=%s original=%s->%s %s/%d allowed=%v", time.Now().Format(time.RFC3339Nano), src, dst, proto, port, e == nil)
	if (e == nil) != want {
		t.Fatalf("packet want=%v err=%v", want, e)
	}
}
func (l *liveFixture) open(t *testing.T, i int, proto string, port uint16) net.Conn {
	t.Helper()
	dst := kerneltest.Destinations[i]
	if l.facts.NAT {
		dst = kerneltest.NATDestinations[i]
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var c net.Conn
	var e error
	if proto == "tcp" {
		c, e = l.x.OpenTCP(ctx, kerneltest.Sources[i], dst, port)
	} else {
		c, e = l.x.OpenUDP(ctx, kerneltest.Sources[i], dst, port)
	}
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { c.Close() })
	exchangeBridge(t, c, true)
	return c
}
func exchangeBridge(t *testing.T, c net.Conn, want bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Millisecond)
	defer cancel()
	e := kerneltest.Exchange(ctx, c)
	if (e == nil) != want {
		t.Fatalf("existing %s->%s want=%v error=%v", c.LocalAddr(), c.RemoteAddr(), want, e)
	}
}
func waitUntil(t *testing.T, at time.Time) {
	t.Helper()
	if d := time.Until(at); d > 0 {
		time.Sleep(d)
	}
}
func (l *liveFixture) boundaryDenied(t *testing.T, end time.Time, port uint16) {
	t.Helper()
	waitUntil(t, end.Add(5*time.Millisecond))
	for i := range kerneltest.Sources {
		if time.Since(end) > time.Second {
			t.Fatal("missed 1s observation window", time.Since(end))
		}
		l.probe(t, i, false, "tcp", port, false)
		t.Logf("first denied observation=%s boundary=%s elapsed=%s", time.Now().Format(time.RFC3339Nano), end.Format(time.RFC3339Nano), time.Since(end))
	}
}
func entryScope(name string) []core.Scope {
	return []core.Scope{{ResourceName: "gitea", Endpoints: core.EndpointSelection{Mode: core.SelectionNames, Names: []string{name}}}}
}

func TestBridgeGiteaIndependentDeadlines(t *testing.T) {
	withBridge(t, networkplan.Graceful, false, func(l *liveFixture) {
		exactEnd := time.Now().Add(1500 * time.Millisecond)
		wideEnd := exactEnd.Add(1500 * time.Millisecond)
		for i, g := range slices.Clone(l.in.Logical.Grants) {
			l.in.Logical.Grants[i].Scopes = entryScope("https")
			l.in.Logical.Grants[i].Validity = core.Validity{Mode: core.LifetimeBounded, ExpiresAt: exactEnd}
			ssh := g
			ssh.ID += "-ssh"
			ssh.Scopes = entryScope("ssh")
			ssh.Validity = core.Validity{Mode: core.LifetimeBounded, ExpiresAt: wideEnd.Add(5 * time.Second)}
			wide := g
			wide.ID += "-wide"
			wide.Scopes = entryScope("https")
			bits := 26
			if g.Source.Addr().Is6() {
				bits = 80
			}
			wide.Source = netip.PrefixFrom(g.Source.Addr(), bits).Masked()
			wide.Validity = core.Validity{Mode: core.LifetimeBounded, ExpiresAt: wideEnd}
			l.in.Logical.Grants = append(l.in.Logical.Grants, ssh, wide)
		}
		l.update(t)
		for i := range kerneltest.Sources {
			l.probe(t, i, false, "tcp", 443, true)
			l.probe(t, i, true, "tcp", 443, true)
			l.probe(t, i, false, "tcp", 22, true)
		}
		waitUntil(t, exactEnd.Add(20*time.Millisecond))
		for i := range kerneltest.Sources {
			l.probe(t, i, false, "tcp", 443, true)
			l.probe(t, i, true, "tcp", 443, true)
		}
		l.boundaryDenied(t, wideEnd, 443)
		for i := range kerneltest.Sources {
			l.probe(t, i, false, "tcp", 22, true)
		}
	})
}
func TestBridgeDenyAndRevocation(t *testing.T) {
	withBridge(t, networkplan.Strict, false, func(l *liveFixture) {
		for _, g := range slices.Clone(l.in.Logical.Grants) {
			g.ID += "-wide"
			bits := 26
			if g.Source.Addr().Is6() {
				bits = 80
			}
			g.Source = netip.PrefixFrom(g.Source.Addr(), bits).Masked()
			l.in.Logical.Grants = append(l.in.Logical.Grants, g)
		}
		l.update(t)
		for i := range kerneltest.Sources {
			l.probe(t, i, true, "tcp", 443, true)
		}
		for i := 0; i < 2; i++ {
			l.in.Logical.Grants[i].Revoked = true
		}
		l.update(t)
		for i := range kerneltest.Sources {
			l.probe(t, i, false, "tcp", 443, true)
		}
		for i := 0; i < 2; i++ {
			d := core.Deny(l.in.Logical.Grants[i])
			d.ID = "deny-" + d.ID
			d.Revoked = false
			d.Scopes = entryScope("https")
			l.in.Logical.Denies = append(l.in.Logical.Denies, d)
		}
		l.update(t)
		for i := range kerneltest.Sources {
			l.probe(t, i, false, "tcp", 443, false)
			l.probe(t, i, true, "tcp", 443, true)
			l.probe(t, i, false, "tcp", 22, true)
		}
	})
}
func TestBridgeUDP(t *testing.T) {
	withBridge(t, networkplan.Strict, false, func(l *liveFixture) {
		l.update(t)
		var old []net.Conn
		for i := range kerneltest.Sources {
			old = append(old, l.open(t, i, "udp", 8002))
		}
		for i := range l.in.Logical.Grants {
			l.in.Logical.Grants[i].Scopes = entryScope("https")
		}
		l.update(t)
		if s := l.c.State(); s.Cleanup.Selected < 2 || s.Cleanup.Remaining != 0 {
			t.Fatal(s)
		}
		for i, c := range old {
			exchangeBridge(t, c, false)
			l.probe(t, i, false, "udp", 8002, false)
			l.probe(t, i, false, "tcp", 443, true)
		}
	})
}
func TestBridgeNativeExpiry(t *testing.T) {
	withBridge(t, networkplan.Graceful, false, func(l *liveFixture) {
		end := time.Now().Add(1500 * time.Millisecond)
		for i := range l.in.Logical.Grants {
			l.in.Logical.Grants[i].Validity = core.Validity{Mode: core.LifetimeBounded, ExpiresAt: end}
		}
		l.update(t)
		var cs []net.Conn
		for i := range kerneltest.Sources {
			cs = append(cs, l.open(t, i, "tcp", 443))
		}
		generation := l.c.State().Generation
		l.boundaryDenied(t, end, 443)
		if l.c.State().Generation != generation {
			t.Fatal("native expiry test reapplied policy")
		}
		for _, c := range cs {
			exchangeBridge(t, c, true)
		}
	})
}
func TestBridgeFutureBoundaries(t *testing.T) {
	withBridge(t, networkplan.Strict, false, func(l *liveFixture) {
		start := time.Now().Add(1500 * time.Millisecond)
		denyStart := start.Add(1500 * time.Millisecond)
		denyEnd := denyStart.Add(1500 * time.Millisecond)
		for i := range l.in.Logical.Grants {
			l.in.Logical.Grants[i].Validity = core.Validity{Mode: core.LifetimeBounded, NotBefore: start, ExpiresAt: denyEnd.Add(5 * time.Second)}
			d := core.Deny(l.in.Logical.Grants[i])
			d.ID = "deny-" + d.ID
			d.Validity.NotBefore = denyStart
			d.Validity.ExpiresAt = denyEnd
			l.in.Logical.Denies = append(l.in.Logical.Denies, d)
		}
		l.update(t)
		for i := range kerneltest.Sources {
			l.probe(t, i, false, "tcp", 443, false)
		}
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- l.c.Run(ctx) }()
		defer func() {
			cancel()
			if e := <-done; !errors.Is(e, context.Canceled) {
				t.Error(e)
			}
		}()
		for _, step := range []struct {
			at    time.Time
			allow bool
		}{{start, true}, {denyStart, false}, {denyEnd, true}} {
			waitUntil(t, step.at.Add(100*time.Millisecond))
			for i := range kerneltest.Sources {
				if time.Since(step.at) > time.Second {
					t.Fatal("missed 1s boundary observation")
				}
				l.probe(t, i, false, "tcp", 443, step.allow)
			}
		}
	})
}
func TestBridgeReplayLateApply(t *testing.T) {
	withBridge(t, networkplan.Graceful, false, func(l *liveFixture) {
		end := time.Now().Add(2200 * time.Millisecond)
		for i := range l.in.Logical.Grants {
			l.in.Logical.Grants[i].Validity = core.Validity{Mode: core.LifetimeBounded, ExpiresAt: end}
		}
		l.update(t)
		waitUntil(t, end.Add(-600*time.Millisecond))
		st, e := l.c.Refresh(context.Background())
		logState(t, st, e)
		if e != nil {
			t.Fatal(e)
		}
		for i := range kerneltest.Sources {
			l.probe(t, i, false, "tcp", 443, true)
		}
		l.boundaryDenied(t, end, 443)
		st, e = l.c.Refresh(context.Background())
		if e != nil || len(st.Current.Allow) != 0 {
			t.Fatal("late apply renewed expired grant", st, e)
		}
	})
}
func TestBridgeHistoricalStrictCleanup(t *testing.T) {
	withBridge(t, networkplan.Strict, false, func(l *liveFixture) {
		end := time.Now().Add(1300 * time.Millisecond)
		for i := range l.in.Logical.Grants {
			l.in.Logical.Grants[i].Validity = core.Validity{Mode: core.LifetimeBounded, ExpiresAt: end}
		}
		l.update(t)
		var cs []net.Conn
		for i := range kerneltest.Sources {
			cs = append(cs, l.open(t, i, "tcp", 443))
		}
		waitUntil(t, end.Add(30*time.Millisecond))
		actual := l.c.effects.Revoke
		l.c.effects.Revoke = func(context.Context, fixture.Snapshot, fixture.Snapshot, uint32, time.Time) (ctbackend.Result, error) {
			return ctbackend.Result{Remaining: -1}, fmt.Errorf("injected cleanup interruption before deletion")
		}
		st, e := l.c.Refresh(context.Background())
		logState(t, st, e)
		if e == nil || !st.Applied || !st.CleanupPending {
			t.Fatal(st, e)
		}
		for _, c := range cs {
			exchangeBridge(t, c, true)
		}
		// Another confirmed apply with no active Grant must not erase old ownership.
		st, e = l.c.Refresh(context.Background())
		if e == nil || len(st.History) != 6 {
			t.Fatal(st, e)
		}
		l.c.effects.Revoke = actual
		st, e = l.c.RetryCleanup(context.Background())
		logState(t, st, e)
		if e != nil || st.Cleanup.Selected < 2 || st.Cleanup.Remaining != 0 {
			t.Fatal(st, e)
		}
		for _, c := range cs {
			exchangeBridge(t, c, false)
		}
	})
}
func TestBridgeStrictPreservation(t *testing.T) {
	withBridge(t, networkplan.Strict, false, func(l *liveFixture) {
		for _, g := range slices.Clone(l.in.Logical.Grants) {
			g.ID += "-wide"
			g.Scopes = entryScope("https")
			bits := 26
			if g.Source.Addr().Is6() {
				bits = 80
			}
			g.Source = netip.PrefixFrom(g.Source.Addr(), bits).Masked()
			l.in.Logical.Grants = append(l.in.Logical.Grants, g)
		}
		l.update(t)
		var kept, lost, unmanaged []net.Conn
		for i := range kerneltest.Sources {
			kept = append(kept, l.open(t, i, "tcp", 443))
			lost = append(lost, l.open(t, i, "tcp", 22))
			unmanaged = append(unmanaged, l.open(t, i, "tcp", 9000))
		}
		for i := 0; i < 2; i++ {
			l.in.Logical.Grants[i].Revoked = true
		}
		l.update(t)
		for i := range kept {
			exchangeBridge(t, kept[i], true)
			exchangeBridge(t, unmanaged[i], true)
			exchangeBridge(t, lost[i], false)
		}
	})
}
func TestBridgeApplyFailure(t *testing.T) {
	withBridge(t, networkplan.Strict, false, func(l *liveFixture) {
		l.update(t)
		generation := l.c.State().Generation
		l.c.effects.Apply = func(_ context.Context, _ fixture.Snapshot, _ nftbackend.Options) (nftbackend.Receipt, error) {
			c := &nftables.Conn{NetNS: int(l.x.Enforcement)}
			c.AddRule(&nftables.Rule{Table: &nftables.Table{Name: "picketx", Family: nftables.TableFamilyINet}, Chain: &nftables.Chain{Name: "missing_chain"}, Exprs: []expr.Any{&expr.Verdict{Kind: expr.VerdictAccept}}})
			e := c.Flush()
			if e == nil {
				t.Fatal("invalid real kernel batch accepted")
			}
			return nftbackend.Receipt{}, &nftbackend.ApplyError{Rejected: true, Err: e}
		}
		for i := range l.in.Logical.Grants {
			l.in.Logical.Grants[i].Revoked = true
		}
		st, e := l.c.Update(context.Background(), l.in)
		logState(t, st, e)
		if e == nil || st.Faulted || st.Generation != generation {
			t.Fatal(st, e)
		}
		for i := range kerneltest.Sources {
			l.probe(t, i, false, "tcp", 443, true)
		}
	})
}
func TestBridgeConfirmationFailure(t *testing.T) {
	withBridge(t, networkplan.Strict, false, func(l *liveFixture) {
		l.update(t)
		var old []net.Conn
		for i := range kerneltest.Sources {
			old = append(old, l.open(t, i, "tcp", 443))
		}
		generation := l.c.State().Generation
		confirm := l.c.effects.Confirm
		l.c.effects.Confirm = func(ctx context.Context, r nftbackend.Receipt) error {
			tab := &nftables.Table{Name: "picketx", Family: nftables.TableFamilyINet}
			chain := &nftables.Chain{Name: "picketx_admission", Table: tab}
			l.x.NFT.AddRule(&nftables.Rule{Table: tab, Chain: chain, Exprs: []expr.Any{&expr.Verdict{Kind: expr.VerdictAccept}}})
			if e := l.x.NFT.Flush(); e != nil {
				return e
			}
			return confirm(ctx, r)
		}
		for i := range l.in.Logical.Grants {
			l.in.Logical.Grants[i].Revoked = true
		}
		st, e := l.c.Update(context.Background(), l.in)
		logState(t, st, e)
		if e == nil || !st.Faulted || st.Generation != generation {
			t.Fatal(st, e)
		}
		for _, c := range old {
			exchangeBridge(t, c, true)
		}
	})
}
func TestBridgePartialCleanup(t *testing.T) {
	withBridge(t, networkplan.Strict, false, func(l *liveFixture) {
		l.update(t)
		for i := range kerneltest.Sources {
			l.open(t, i, "tcp", 443)
		}
		ops := ctbackend.KernelOperations(l.x.CT)
		realDelete := ops.Delete
		ops.Delete = func(f netlink.InetFamily, flows []*netlink.ConntrackFlow) (uint, error) {
			if f == netlink.FAMILY_V6 {
				return 0, fmt.Errorf("injected second-family delete failure")
			}
			return realDelete(f, flows)
		}
		partial := ctbackend.NewWithOperations(ops)
		l.c.effects.Revoke = partial.Revoke
		for i := range l.in.Logical.Grants {
			l.in.Logical.Grants[i].Revoked = true
		}
		st, e := l.c.Update(context.Background(), l.in)
		logState(t, st, e)
		if e == nil || !st.CleanupPending || st.Cleanup.Selected < 2 || st.Cleanup.Deleted < 1 || st.Cleanup.Remaining < 1 {
			t.Fatal(st, e)
		}
		l.c.effects.Revoke = l.r.Revoke
		st, e = l.c.RetryCleanup(context.Background())
		logState(t, st, e)
		if e != nil || st.Cleanup.Remaining != 0 || st.Cleanup.Selected < 1 {
			t.Fatal(st, e)
		}
	})
}
func TestBridgeBindingOwnership(t *testing.T) {
	withBridge(t, networkplan.Strict, false, func(l *liveFixture) {
		b := l.in.Bindings[0]
		b.ID = "second"
		l.in.Bindings = append(l.in.Bindings, b)
		l.update(t)
		l.in.Bindings = l.in.Bindings[1:]
		l.update(t)
		for i := range kerneltest.Sources {
			l.probe(t, i, false, "tcp", 443, true)
		}
		for _, m := range l.c.State().History {
			if len(m.BindingIDs) == 2 {
				t.Logf("retained historical bindings=%v", m.BindingIDs)
			}
		}
		l.in.Logical.Grants = nil
		l.update(t)
		if len(l.c.State().Current.Protected) != 6 {
			t.Fatal("last Grant removed protection")
		}
		for i := range kerneltest.Sources {
			l.probe(t, i, false, "tcp", 443, false)
		}
	})
}
func TestBridgeEmptyOwnership(t *testing.T) {
	for _, mode := range []networkplan.HandlingMode{networkplan.Graceful, networkplan.Strict} {
		for _, removeAgent := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/agent=%v", mode, removeAgent), func(t *testing.T) {
				withBridge(t, mode, false, func(l *liveFixture) {
					l.update(t)
					for i := range kerneltest.Sources {
						l.open(t, i, "tcp", 443)
					}
					l.in.Bindings = nil
					if removeAgent {
						l.in.Agents = nil
						l.in.Contexts = nil
					}
					l.update(t)
					st := l.c.State()
					if !st.Empty || st.Faulted || mode == networkplan.Strict && st.Cleanup.Remaining != 0 {
						t.Fatal(st)
					}
					if e := l.b.ConfirmAbsent(context.Background()); e != nil {
						t.Fatal(e)
					}
					for i := range kerneltest.Sources {
						l.probe(t, i, false, "tcp", 443, true)
					}
					if _, e := l.c.RetryCleanup(context.Background()); e != nil {
						t.Fatal(e)
					}
				})
			})
		}
	}
}
func TestBridgeNAT(t *testing.T) {
	withBridge(t, networkplan.Strict, true, func(l *liveFixture) {
		l.update(t)
		var kept, lost []net.Conn
		for i := range kerneltest.Sources {
			lost = append(lost, l.open(t, i, "tcp", 8000))
			kept = append(kept, l.open(t, i, "tcp", 8001))
		}
		for _, family := range []netlink.InetFamily{netlink.FAMILY_V4, netlink.FAMILY_V6} {
			flows, e := l.x.CT.ConntrackTableList(netlink.ConntrackTable, family)
			if e != nil {
				t.Fatal(e)
			}
			found := false
			for _, flow := range flows {
				if flow.Mark == nftbackend.TestMark && flow.Forward.DstPort == 8000 {
					found = true
					t.Logf("NAT original=%+v reply=%+v", flow.Forward, flow.Reverse)
					if flow.Reverse.SrcPort != 9000 {
						t.Fatal("translation missing", flow)
					}
				}
			}
			if !found {
				t.Fatal("no observed owned NAT flow", family)
			}
		}
		for i := range l.in.Logical.Grants {
			l.in.Logical.Grants[i].Scopes = entryScope("ssh")
		}
		l.update(t)
		if st := l.c.State(); st.Cleanup.Selected < 2 || st.Cleanup.Remaining != 0 {
			t.Fatal(st)
		}
		for i := range kept {
			exchangeBridge(t, kept[i], true)
			exchangeBridge(t, lost[i], false)
			l.probe(t, i, false, "tcp", 8000, false)
			l.probe(t, i, false, "tcp", 8001, true)
		}
	})
}

func TestBridgeInterruptedDump(t *testing.T) {
	withBridge(t, networkplan.Strict, false, func(l *liveFixture) {
		l.update(t)
		var old []net.Conn
		for i := range kerneltest.Sources {
			old = append(old, l.open(t, i, "tcp", 443))
		}
		ops := ctbackend.KernelOperations(l.x.CT)
		dump := ops.Dump
		deletes := 0
		ops.Dump = func(f netlink.InetFamily) ([]*netlink.ConntrackFlow, error) {
			flows, e := dump(f)
			if e != nil {
				return flows, e
			}
			if f == netlink.FAMILY_V6 {
				return flows, netlink.ErrDumpInterrupted
			}
			return flows, nil
		}
		del := ops.Delete
		ops.Delete = func(f netlink.InetFamily, flows []*netlink.ConntrackFlow) (uint, error) {
			deletes++
			return del(f, flows)
		}
		interrupted := ctbackend.NewWithOperations(ops)
		l.c.effects.Revoke = interrupted.Revoke
		for i := range l.in.Logical.Grants {
			l.in.Logical.Grants[i].Revoked = true
		}
		st, e := l.c.Update(context.Background(), l.in)
		logState(t, st, e)
		if !errors.Is(e, netlink.ErrDumpInterrupted) || !st.CleanupPending || deletes != 0 || st.Cleanup.Remaining != -1 {
			t.Fatal(st, e, deletes)
		}
		for _, c := range old {
			exchangeBridge(t, c, true)
		}
		l.c.effects.Revoke = l.r.Revoke
		st, e = l.c.RetryCleanup(context.Background())
		logState(t, st, e)
		if e != nil || st.Cleanup.Selected < 2 || st.Cleanup.Remaining != 0 {
			t.Fatal(st, e)
		}
	})
}
