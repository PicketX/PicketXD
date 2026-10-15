//go:build integration

package nftbackend

import (
	"context"
	"github.com/PicketX/PicketXD/internal/validation/fixture"
	"github.com/PicketX/PicketXD/internal/validation/kerneltest"
	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"net/netip"
	"testing"
	"time"
)

func bridgeSnapshot(nat bool, end time.Time) fixture.Snapshot {
	s := fixture.Snapshot{}
	destinations := kerneltest.Destinations
	ports := []uint16{443}
	if nat {
		destinations = kerneltest.NATDestinations
		ports = []uint16{8000, 8001}
	}
	for i, dst := range destinations {
		for _, port := range ports {
			ep := fixture.Endpoint{Destination: netip.PrefixFrom(dst, dst.BitLen()), Protocol: 6, PortFirst: port, PortLast: port}
			s.Protected = append(s.Protected, ep)
			s.Allow = append(s.Allow, fixture.Rule{Source: netip.PrefixFrom(kerneltest.Sources[i], kerneltest.Sources[i].BitLen()), Endpoint: ep, ExpiresAt: end})
		}
	}
	return s
}
func TestBridgeBackendDispatchDeadline(t *testing.T) {
	for _, stage := range []string{"lock", "prepare"} {
		t.Run(stage, func(t *testing.T) {
			kerneltest.WithTopology(t, func(x *kerneltest.Topology) {
				b := New(x.NFT)
				end := time.Now().Add(150 * time.Millisecond)
				s := bridgeSnapshot(false, end)
				var receipt Receipt
				var err error
				if stage == "lock" {
					b.mu.Lock()
					done := make(chan struct{})
					go func() { receipt, err = b.ApplyReceipt(context.Background(), s, Options{}); close(done) }()
					time.Sleep(time.Until(end.Add(time.Millisecond)))
					b.mu.Unlock()
					<-done
				} else {
					b.inject = func(*nftables.Conn) {
						if d := time.Until(end.Add(time.Millisecond)); d > 0 {
							time.Sleep(d)
						}
					}
					receipt, err = b.ApplyReceipt(context.Background(), s, Options{})
				}
				if err != nil || len(receipt.Emitted.Allow) != 0 || len(receipt.Omitted.Allow) != 2 {
					t.Fatal("expired dispatch became active", receipt, err)
				}
				if err = b.Confirm(context.Background(), receipt); err != nil {
					t.Fatal(err)
				}
				t.Logf("deadline=%s dispatch=%s ack=%s emitted=%+v omitted=%+v", end.Format(time.RFC3339Nano), receipt.DispatchedAt.Format(time.RFC3339Nano), receipt.AcknowledgedAt.Format(time.RFC3339Nano), receipt.Emitted, receipt.Omitted)
				for i := range kerneltest.Sources {
					probe(t, x, i, false, "tcp", 443, false)
				}
			})
		})
	}
}
func TestBridgeBackendSubmillisecondReceipt(t *testing.T) {
	kerneltest.WithTopology(t, func(x *kerneltest.Topology) {
		b := New(x.NFT)
		end := time.Now().Add(200 * time.Millisecond)
		s := bridgeSnapshot(false, end)
		b.inject = func(*nftables.Conn) {
			if d := time.Until(end.Add(-3 * time.Millisecond)); d > 0 {
				time.Sleep(d)
			}
			for time.Now().Before(end.Add(-500 * time.Microsecond)) {
			}
		}
		receipt, err := b.ApplyReceipt(context.Background(), s, Options{})
		if err != nil {
			t.Fatal(err)
		}
		remaining := end.Sub(receipt.DispatchedAt)
		t.Logf("absolute=%s dispatch=%s remaining=%s actual=%+v omitted=%+v", end.Format(time.RFC3339Nano), receipt.DispatchedAt.Format(time.RFC3339Nano), remaining, receipt.Emitted, receipt.Omitted)
		if remaining <= 0 || remaining >= time.Millisecond {
			t.Fatal("missed explicit submillisecond dispatch window", remaining)
		}
		if len(receipt.Emitted.Allow) != 0 || len(receipt.Omitted.Allow) != 2 {
			t.Fatal("submillisecond became permanent", receipt)
		}
		if err = b.Confirm(context.Background(), receipt); err != nil {
			t.Fatal(err)
		}
		for i := range kerneltest.Sources {
			probe(t, x, i, false, "tcp", 443, false)
		}
	})
}
func TestBridgeNATAtomicRejection(t *testing.T) {
	kerneltest.WithTopology(t, func(x *kerneltest.Topology) {
		b := New(x.NFT)
		s := bridgeSnapshot(true, time.Time{})
		receipt, err := b.ApplyReceipt(context.Background(), s, Options{FixtureNAT: true})
		if err != nil {
			t.Fatal(err)
		}
		if err = b.Confirm(context.Background(), receipt); err != nil {
			t.Fatal(err)
		}
		b.inject = func(c *nftables.Conn) {
			c.AddRule(&nftables.Rule{Table: &nftables.Table{Name: "picketx", Family: nftables.TableFamilyINet}, Chain: &nftables.Chain{Name: "invalid_fixture_chain"}, Exprs: []expr.Any{&expr.Verdict{Kind: expr.VerdictAccept}}})
		}
		s.Allow = nil
		if _, err = b.ApplyReceipt(context.Background(), s, Options{FixtureNAT: true}); err == nil {
			t.Fatal("invalid combined batch accepted")
		}
		b.inject = nil
		if err = b.Confirm(context.Background(), receipt); err != nil {
			t.Fatal("rejected combined transaction changed previous admission/NAT", err)
		}
		for i, src := range kerneltest.Sources {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			err = x.Probe(ctx, src, kerneltest.NATDestinations[i], "tcp", 8000)
			cancel()
			if err != nil {
				t.Fatal("old admission/translation lost", err)
			}
		}
	})
}
func TestBridgeRejectsPermanentNativeMutation(t *testing.T) {
	kerneltest.WithTopology(t, func(x *kerneltest.Topology) {
		b := New(x.NFT)
		s := bridgeSnapshot(false, time.Now().Add(10*time.Second))
		receipt, err := b.ApplyReceipt(context.Background(), s, Options{})
		if err != nil {
			t.Fatal(err)
		}
		if err = b.Confirm(context.Background(), receipt); err != nil {
			t.Fatal(err)
		}
		want := receipt.manifest.sets["picketx_allow_0"]
		if err = x.NFT.SetDeleteElements(want.set, []nftables.SetElement{want.element}); err != nil {
			t.Fatal(err)
		}
		permanent := want.element
		permanent.Timeout = 0
		if err = x.NFT.SetAddElements(want.set, []nftables.SetElement{permanent}); err != nil {
			t.Fatal(err)
		}
		if err = x.NFT.Flush(); err != nil {
			t.Fatal(err)
		}
		if err = b.Confirm(context.Background(), receipt); err == nil {
			t.Fatal("permanent bounded membership falsely confirmed")
		}
	})
}
