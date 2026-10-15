//go:build integration

// SPDX-License-Identifier: GPL-3.0-or-later
package hostbridge

import (
	"context"
	_ "embed"
	"fmt"
	"net/netip"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PicketX/PicketX/pkg/configuration"
	"github.com/PicketX/PicketX/pkg/networkplan"
	"github.com/PicketX/PicketXD/internal/agentstate"
	"github.com/PicketX/PicketXD/internal/validation/fixture"
	"github.com/PicketX/PicketXD/internal/validation/kerneltest"
	"github.com/PicketX/PicketXD/internal/validation/nftbackend"
	"github.com/google/nftables"
	"github.com/google/nftables/binaryutil"
	"github.com/google/nftables/expr"
	"golang.org/x/sys/unix"
)

//go:embed testdata/policy.yaml
var policyFixture []byte

type hostCase struct {
	x       *kerneltest.Topology
	backend *nftbackend.Backend
	cfg     agentstate.Config
	store   *agentstate.Store
}

func withHost(t *testing.T, fn func(*hostCase)) {
	t.Helper()
	kerneltest.WithTopology(t, func(x *kerneltest.Topology) {
		// Before policy prove both actual host destinations, both transports and
		// unrelated controls. A failed listener must never count as a successful deny.
		for i := range kerneltest.Sources {
			for _, p := range []struct {
				proto string
				port  uint16
			}{{"tcp", 443}, {"udp", 22}, {"tcp", 9000}} {
				packet(t, x, i, false, p.proto, p.port, true)
			}
		}
		c, e := nftables.New(nftables.WithNetNSFd(int(x.Server)))
		if e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() { c.CloseLasting() })
		if c.NetNS != int(x.Server) || x.Server.Equal(x.Enforcement) || x.Server.Equal(x.Client) {
			t.Fatal("host fixture escaped its owned namespace")
		}
		caps := &networkplan.CapabilityProfile{Classes: []networkplan.TrafficClass{networkplan.HostLocal}, Protocols: []networkplan.Protocol{networkplan.TCP, networkplan.UDP}, Families: []networkplan.Family{networkplan.IPv4, networkplan.IPv6}, Handling: []networkplan.HandlingMode{networkplan.Graceful}, OriginalDestination: true}
		var dest []netip.Prefix
		for _, a := range kerneltest.Destinations {
			dest = append(dest, netip.PrefixFrom(a, a.BitLen()))
		}
		f := configuration.Facts{AuthorityID: "host-fixture:" + x.Server.UniqueId(), Agents: []networkplan.Agent{{Name: "agent-a", Capabilities: caps}}, Contexts: []networkplan.ExecutionContext{{AgentRef: "agent-a", ProfileRef: "host", ID: "host:" + x.Server.UniqueId(), NetworkSpaceID: x.Server.UniqueId(), Class: networkplan.HostLocal, Stage: networkplan.OriginalDestination, Destinations: dest, Handling: networkplan.Graceful}}}
		cfg := agentstate.Config{Path: filepath.Join(t.TempDir(), "private", "accepted.json"), ProviderID: f.AuthorityID, AgentRef: "agent-a", RuntimeKey: "fixture-host-v1", BootID: "isolated-fixture-boot", Facts: f}
		s, e := agentstate.New(cfg)
		if e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() { s.Close() })
		fn(&hostCase{x, nftbackend.New(c), cfg, s})
	})
}
func publication(t *testing.T, until4, until6 time.Time, grants bool) []byte {
	t.Helper()
	raw := policyFixture
	s := strings.NewReplacer("198.51.100.9", kerneltest.Sources[0].String(), "2001:db8:1::9", kerneltest.Sources[1].String(), "2001:db8::100", kerneltest.Destinations[1].String(), "2026-10-15T10:00:00Z", until4.UTC().Format(time.RFC3339Nano), "2026-10-15T12:00:00Z", until6.UTC().Format(time.RFC3339Nano), "      endpointNames: [https]\n", "", "      endpointNames: [ssh]\n", "", "protocol: TCP, port: 22", "protocol: UDP, port: 22").Replace(string(raw))
	if !grants {
		parts := strings.Split(s, "---\n")
		s = parts[0] + "---\n" + parts[len(parts)-1]
	}
	return []byte(s)
}
func materialize(t *testing.T, s *agentstate.Store) fixture.Snapshot {
	t.Helper()
	c := s.Current()
	if c == nil {
		t.Fatal("no accepted desired state")
	}
	p, e := c.Plan().Projection("agent-a", time.Now())
	if e != nil {
		t.Fatal(e)
	}
	out := fixture.Snapshot{}
	endpoint := func(m networkplan.Match) fixture.Endpoint {
		protocol := uint8(6)
		if m.Protocol == networkplan.UDP {
			protocol = 17
		}
		return fixture.Endpoint{Destination: m.Destination, Protocol: protocol, PortFirst: m.Port, PortLast: m.Port}
	}
	for _, protection := range p.Protected {
		if protection.Context.Class != networkplan.HostLocal {
			t.Fatal("non-host target")
		}
		for _, m := range protection.Matches {
			out.Protected = append(out.Protected, endpoint(m))
		}
	}
	for _, domain := range []struct {
		members []networkplan.Membership
		deny    bool
	}{{p.Allow, false}, {p.Deny, true}} {
		for _, m := range domain.members {
			r := fixture.Rule{Source: m.Contribution.Source, Endpoint: endpoint(m.Match), ExpiresAt: m.Contribution.Validity.ExpiresAt}
			if domain.deny {
				out.Deny = append(out.Deny, r)
			} else {
				out.Allow = append(out.Allow, r)
			}
		}
	}
	return out
}
func (h *hostCase) apply(t *testing.T, raw []byte) {
	t.Helper()
	if e := h.store.Accept(context.Background(), raw); e != nil {
		t.Fatal(e)
	}
	receipt, e := h.backend.ApplyReceipt(context.Background(), materialize(t, h.store), nftbackend.Options{HostInputFixture: true})
	if e != nil {
		t.Fatal(e)
	}
	if e = h.backend.Confirm(context.Background(), receipt); e != nil {
		t.Fatal(e)
	}
}
func packet(t *testing.T, x *kerneltest.Topology, i int, other bool, proto string, port uint16, want bool) {
	t.Helper()
	src := kerneltest.Sources[i]
	if other {
		src = kerneltest.Others[i]
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	e := x.Probe(ctx, src, kerneltest.Destinations[i], proto, port)
	if (e == nil) != want {
		t.Fatalf("host packet %s -> %s %s/%d reachable=%v want=%v error=%v", src, kerneltest.Destinations[i], proto, port, e == nil, want, e)
	}
	t.Logf("host observed %s -> %s %s/%d reachable=%v", src, kerneltest.Destinations[i], proto, port, want)
}
func TestHostConfigurationDefaultDeny(t *testing.T) {
	withHost(t, func(h *hostCase) {
		h.apply(t, publication(t, time.Now(), time.Now(), false))
		for i := range kerneltest.Sources {
			packet(t, h.x, i, false, "tcp", 443, false)
			packet(t, h.x, i, false, "udp", 22, false)
			packet(t, h.x, i, false, "tcp", 9000, true)
		}
	})
}
func TestHostConfigurationSourcesAndDeny(t *testing.T) {
	withHost(t, func(h *hostCase) {
		raw := string(publication(t, time.Now().Add(time.Minute), time.Now().Add(time.Minute), true))
		parts := strings.Split(raw, "---\n")
		broad4 := strings.NewReplacer("grant-browser-v1", "grant-broad4", "name: browser", "name: broad4", kerneltest.Sources[0].String(), "192.0.2.0/26").Replace(parts[1])
		broad6 := strings.NewReplacer("grant-ssh6-v1", "grant-broad6", "name: ssh6", "name: broad6", kerneltest.Sources[1].String(), "2001:db8:1:0:1::/80").Replace(parts[2])
		union := raw + "---\n" + broad4 + "---\n" + broad6
		h.apply(t, []byte(union))
		for i := range kerneltest.Sources {
			packet(t, h.x, i, true, "tcp", 443, true)
		}
		deny4 := strings.NewReplacer("kind: Grant", "kind: NetworkDeny", "grant-browser-v1", "deny-other4", "name: browser", "name: deny4", kerneltest.Sources[0].String(), kerneltest.Others[0].String()).Replace(parts[1])
		deny6 := strings.NewReplacer("kind: Grant", "kind: NetworkDeny", "grant-ssh6-v1", "deny-other6", "name: ssh6", "name: deny6", kerneltest.Sources[1].String(), kerneltest.Others[1].String()).Replace(parts[2])
		// Revoke the exact host grants; overlapping CIDRs still independently allow.
		// Only mark original exact grants revoked, preserving broad contributions.
		withdrawn := strings.Join([]string{parts[0], strings.Replace(parts[1], "  subject:", "  revoked: true\n  subject:", 1), strings.Replace(parts[2], "  subject:", "  revoked: true\n  subject:", 1), parts[3], broad4, broad6, deny4, deny6}, "---\n")
		h.apply(t, []byte(withdrawn))
		for i := range kerneltest.Sources {
			packet(t, h.x, i, false, "tcp", 443, true)
			packet(t, h.x, i, false, "udp", 22, true)
			packet(t, h.x, i, true, "tcp", 443, false)
			packet(t, h.x, i, true, "udp", 22, false)
			packet(t, h.x, i, true, "tcp", 9000, true)
		}
	})
}
func TestHostConfigurationExpiryWithInvalidReload(t *testing.T) {
	withHost(t, func(h *hostCase) {
		end := time.Now().Add(1500 * time.Millisecond)
		h.apply(t, publication(t, end, time.Now().Add(time.Minute), true))
		packet(t, h.x, 0, false, "tcp", 443, true)
		if e := h.store.Accept(context.Background(), []byte("broken")); e == nil {
			t.Fatal("invalid candidate accepted")
		}
		for time.Now().Before(end.Add(80 * time.Millisecond)) {
			time.Sleep(20 * time.Millisecond)
		}
		packet(t, h.x, 0, false, "tcp", 443, false)
		packet(t, h.x, 1, false, "tcp", 443, true)
		packet(t, h.x, 0, true, "tcp", 9000, true)
		st, e := h.store.Status(time.Now())
		if e != nil || st.LastError == "" || len(st.Projection.Allow) != 2 {
			t.Fatalf("deadline/error not preserved: %#v %v", st, e)
		}
	})
}
func TestHostConfigurationRecoveryDeadline(t *testing.T) {
	withHost(t, func(h *hostCase) {
		end := time.Now().Add(1600 * time.Millisecond)
		raw := publication(t, end, end, true)
		h.apply(t, raw)
		for i := range kerneltest.Sources {
			packet(t, h.x, i, false, "tcp", 443, true)
		}
		h.store.Close()
		h.cfg.BootID = "new-fixture-boot"
		restored, e := agentstate.New(h.cfg)
		if e != nil {
			t.Fatal(e)
		}
		h.store = restored
		defer restored.Close()
		st, e := restored.Status(time.Now())
		if e != nil || !st.Restored || st.SameBoot || st.EnforcementConfirmed {
			t.Fatalf("restore misreported apply: %#v %v", st, e)
		}
		h.apply(t, raw)
		for time.Now().Before(end.Add(80 * time.Millisecond)) {
			time.Sleep(20 * time.Millisecond)
		}
		for i := range kerneltest.Sources {
			packet(t, h.x, i, false, "tcp", 443, false)
			packet(t, h.x, i, false, "tcp", 9000, true)
		}
	})
}
func addHostNAT(t *testing.T, h *hostCase) {
	t.Helper()
	c, e := nftables.New(nftables.WithNetNSFd(int(h.x.Server)))
	if e != nil {
		t.Fatal(e)
	}
	defer c.CloseLasting()
	tab := c.AddTable(&nftables.Table{Name: "picketx_host_nat_test", Family: nftables.TableFamilyINet})
	priority := nftables.ChainPriority(-100)
	chain := c.AddChain(&nftables.Chain{Name: "picketx_dnat", Table: tab, Type: nftables.ChainTypeNAT, Hooknum: nftables.ChainHookPrerouting, Priority: &priority})
	for i, a := range kerneltest.Destinations {
		family := byte(unix.NFPROTO_IPV4)
		af := uint32(unix.NFPROTO_IPV4)
		if i == 1 {
			family = unix.NFPROTO_IPV6
			af = unix.NFPROTO_IPV6
		}
		c.AddRule(&nftables.Rule{Table: tab, Chain: chain, Exprs: []expr.Any{&expr.Meta{Key: expr.MetaKeyNFPROTO, Register: 1}, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{family}}, &expr.Meta{Key: expr.MetaKeyL4PROTO, Register: 1}, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{6}}, &expr.Payload{DestRegister: 1, Base: expr.PayloadBaseTransportHeader, Offset: 2, Len: 2}, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: binaryutil.BigEndian.PutUint16(8001)}, &expr.Immediate{Register: 1, Data: a.AsSlice()}, &expr.Immediate{Register: 2, Data: binaryutil.BigEndian.PutUint16(8000)}, &expr.NAT{Type: expr.NATTypeDestNAT, Family: af, RegAddrMin: 1, RegAddrMax: 1, RegProtoMin: 2, RegProtoMax: 2, Specified: true}}})
	}
	if e = c.Flush(); e != nil {
		t.Fatal(e)
	}
}
func TestHostOriginalDestinationDNAT(t *testing.T) {
	withHost(t, func(h *hostCase) {
		addHostNAT(t, h)
		for i := range kerneltest.Sources {
			packet(t, h.x, i, false, "tcp", 8001, true)
			packet(t, h.x, i, true, "tcp", 8001, true)
		}
		raw := strings.Replace(string(publication(t, time.Now().Add(time.Minute), time.Now().Add(time.Minute), true)), "port: 443", "port: 8001", 1)
		h.apply(t, []byte(raw))
		for i := range kerneltest.Sources {
			packet(t, h.x, i, false, "tcp", 8001, true)
			packet(t, h.x, i, true, "tcp", 8001, false)
			packet(t, h.x, i, true, "tcp", 8000, true)
		}
	})
}
func TestHostUntrackedProtectedTraffic(t *testing.T) {
	withHost(t, func(h *hostCase) {
		c, e := nftables.New(nftables.WithNetNSFd(int(h.x.Server)))
		if e != nil {
			t.Fatal(e)
		}
		defer c.CloseLasting()
		tab := c.AddTable(&nftables.Table{Name: "picketx_host_untracked_test", Family: nftables.TableFamilyINet})
		priority := nftables.ChainPriority(-300)
		chain := c.AddChain(&nftables.Chain{Name: "picketx_raw", Table: tab, Type: nftables.ChainTypeFilter, Hooknum: nftables.ChainHookPrerouting, Priority: &priority})
		c.AddRule(&nftables.Rule{Table: tab, Chain: chain, Exprs: []expr.Any{&expr.Meta{Key: expr.MetaKeyL4PROTO, Register: 1}, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{6}}, &expr.Payload{DestRegister: 1, Base: expr.PayloadBaseTransportHeader, Offset: 2, Len: 2}, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: binaryutil.BigEndian.PutUint16(443)}, &expr.Notrack{}}})
		if e = c.Flush(); e != nil {
			t.Fatal(e)
		}
		for i := range kerneltest.Sources {
			packet(t, h.x, i, false, "tcp", 443, true)
		}
		h.apply(t, publication(t, time.Now().Add(time.Minute), time.Now().Add(time.Minute), true))
		for i := range kerneltest.Sources {
			packet(t, h.x, i, false, "tcp", 443, false)
			packet(t, h.x, i, false, "tcp", 9000, true)
		}
	})
}
func TestHostInputDoesNotClaimTransit(t *testing.T) {
	withHost(t, func(h *hostCase) {
		c, e := nftables.New(nftables.WithNetNSFd(int(h.x.Enforcement)))
		if e != nil {
			t.Fatal(e)
		}
		defer c.CloseLasting()
		if e = h.store.Accept(context.Background(), publication(t, time.Now(), time.Now(), false)); e != nil {
			t.Fatal(e)
		}
		b := nftbackend.New(c)
		r, e := b.ApplyReceipt(context.Background(), materialize(t, h.store), nftbackend.Options{HostInputFixture: true})
		if e != nil {
			t.Fatal(e)
		}
		if e = b.Confirm(context.Background(), r); e != nil {
			t.Fatal(e)
		}
		for i := range kerneltest.Sources {
			packet(t, h.x, i, false, "tcp", 443, true)
			packet(t, h.x, i, false, "udp", 22, true)
		}
		t.Log(fmt.Sprintf("INPUT-only fixture did not claim forwarded service coverage; namespace=%s", h.x.Enforcement.UniqueId()))
	})
}
