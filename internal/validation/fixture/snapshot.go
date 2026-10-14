// Package fixture defines private M0 inputs, not a production policy schema.
package fixture

import (
	"fmt"
	"net/netip"
	"time"
)

type Endpoint struct {
	Destination         netip.Prefix
	Protocol            uint8
	PortFirst, PortLast uint16
}
type Rule struct {
	Source   netip.Prefix
	Endpoint Endpoint
	// Zero denotes a non-expiring test fixture only.
	ExpiresAt time.Time
}
type Snapshot struct {
	Protected   []Endpoint
	Allow, Deny []Rule
}

func Validate(s Snapshot) error {
	if len(s.Protected) == 0 {
		return fmt.Errorf("empty protection scope")
	}
	for i, e := range s.Protected {
		if err := validateEndpoint(e); err != nil {
			return fmt.Errorf("protected[%d]: %w", i, err)
		}
	}
	for _, group := range []struct {
		name  string
		rules []Rule
	}{{"allow", s.Allow}, {"deny", s.Deny}} {
		for i, r := range group.rules {
			if err := validatePrefix(r.Source); err != nil {
				return fmt.Errorf("%s[%d] source: %w", group.name, i, err)
			}
			if err := validateEndpoint(r.Endpoint); err != nil {
				return fmt.Errorf("%s[%d]: %w", group.name, i, err)
			}
			if r.Source.Addr().Is4() != r.Endpoint.Destination.Addr().Is4() {
				return fmt.Errorf("%s[%d]: address family mismatch", group.name, i)
			}
		}
	}
	return nil
}
func validatePrefix(p netip.Prefix) error {
	if !p.IsValid() || p.Addr().Is4In6() || p != p.Masked() {
		return fmt.Errorf("invalid or noncanonical prefix %s", p)
	}
	return nil
}
func validateEndpoint(e Endpoint) error {
	if err := validatePrefix(e.Destination); err != nil {
		return err
	}
	if e.Protocol != 6 && e.Protocol != 17 {
		return fmt.Errorf("unsupported protocol %d", e.Protocol)
	}
	if e.PortFirst == 0 || e.PortLast < e.PortFirst {
		return fmt.Errorf("invalid port range %d-%d", e.PortFirst, e.PortLast)
	}
	return nil
}
