// SPDX-License-Identifier: GPL-3.0-or-later
package nftbackend

import (
	"context"
	"net/netip"
	"testing"

	"github.com/PicketX/PicketXD/internal/validation/fixture"
	"github.com/google/nftables"
	"github.com/google/nftables/expr"
)

func TestHostOriginalTupleExpressions(t *testing.T) {
	for _, prefix := range []string{"192.0.2.100/32", "2001:db8::100/128"} {
		ep := fixture.Endpoint{Destination: netip.MustParsePrefix(prefix), Protocol: 6, PortFirst: 443, PortLast: 443}
		match := hostEndpointMatch(ep)
		foundIP, foundPort := false, false
		for _, v := range match {
			if ct, ok := v.(*expr.Ct); ok {
				if ct.Direction != 0 || ct.SourceRegister {
					t.Fatal("host match must read original direction")
				}
				foundIP = foundIP || (ct.Key == expr.CtKeyDSTIP || ct.Key == expr.CtKeyDSTIP6)
				foundPort = foundPort || ct.Key == expr.CtKeyPROTODST
			}
		}
		if !foundIP || !foundPort {
			t.Fatal("host match reads translated payload instead of original tuple")
		}
		r := fixture.Rule{Source: ep.Destination, Endpoint: ep}
		look := hostLookup(r, &nftables.Set{Name: "picketx_allow", ID: 1})
		source := false
		for _, v := range look {
			if ct, ok := v.(*expr.Ct); ok && (ct.Key == expr.CtKeySRCIP || ct.Key == expr.CtKeySRCIP6) {
				source = true
			}
		}
		if !source {
			t.Fatal("host source not anchored in original tuple")
		}
	}
}
func TestHostAndTransitNATOptionsCannotMix(t *testing.T) {
	ep := fixture.Endpoint{Destination: netip.MustParsePrefix("192.0.2.100/32"), Protocol: 6, PortFirst: 443, PortLast: 443}
	b := New(&nftables.Conn{})
	if _, e := b.ApplyReceipt(context.Background(), fixture.Snapshot{Protected: []fixture.Endpoint{ep}}, Options{HostInputFixture: true, FixtureNAT: true}); e == nil {
		t.Fatal("mixed observation paths accepted")
	}
}
