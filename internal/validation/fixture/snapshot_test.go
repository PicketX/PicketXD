package fixture

import (
	"net/netip"
	"testing"
)

// Removing any invariant check must admit one of these malformed fixtures.
func TestValidateSnapshot(t *testing.T) {
	endpoint := Endpoint{Destination: netip.MustParsePrefix("192.0.2.100/32"), Protocol: 6, PortFirst: 443, PortLast: 443}
	valid := func() Snapshot {
		return Snapshot{Protected: []Endpoint{endpoint}, Allow: []Rule{{Source: netip.MustParsePrefix("192.0.2.10/32"), Endpoint: endpoint}}}
	}
	tests := []struct {
		name    string
		mutate  func(*Snapshot)
		invalid bool
	}{
		{"valid", func(*Snapshot) {}, false},
		{"dual stack", func(s *Snapshot) {
			e := Endpoint{Destination: netip.MustParsePrefix("2001:db8:1::100/128"), Protocol: 17, PortFirst: 53, PortLast: 55}
			s.Protected = append(s.Protected, e)
			s.Allow = append(s.Allow, Rule{Source: netip.MustParsePrefix("2001:db8:1::10/128"), Endpoint: e})
		}, false},
		{"empty scope", func(s *Snapshot) { s.Protected = nil }, true},
		{"invalid destination", func(s *Snapshot) { s.Protected[0].Destination = netip.Prefix{} }, true},
		{"noncanonical destination", func(s *Snapshot) { s.Protected[0].Destination = netip.MustParsePrefix("192.0.2.100/24") }, true},
		{"invalid source", func(s *Snapshot) { s.Allow[0].Source = netip.Prefix{} }, true},
		{"noncanonical source", func(s *Snapshot) { s.Allow[0].Source = netip.MustParsePrefix("192.0.2.10/24") }, true},
		{"mapped IPv4", func(s *Snapshot) { s.Allow[0].Source = netip.MustParsePrefix("::ffff:192.0.2.10/128") }, true},
		{"family mismatch", func(s *Snapshot) { s.Allow[0].Source = netip.MustParsePrefix("2001:db8:1::10/128") }, true},
		{"protocol", func(s *Snapshot) { s.Protected[0].Protocol = 1 }, true},
		{"zero port", func(s *Snapshot) { s.Protected[0].PortFirst = 0 }, true},
		{"reversed range", func(s *Snapshot) { s.Protected[0].PortLast = 442 }, true},
		{"deny validation", func(s *Snapshot) { s.Deny = []Rule{{Source: netip.Prefix{}, Endpoint: endpoint}} }, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := valid()
			tc.mutate(&s)
			err := Validate(s)
			if (err != nil) != tc.invalid {
				t.Fatalf("Validate error=%v, invalid=%v", err, tc.invalid)
			}
		})
	}
}
