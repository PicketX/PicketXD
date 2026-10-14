// Package ctbackend scopes M0 revocation to withdrawn owned original tuples.
package ctbackend

import (
	"github.com/PicketX/PicketXD/internal/validation/fixture"
	"github.com/vishvananda/netlink"
	"net/netip"
	"time"
)

func SelectWithdrawn(flows []*netlink.ConntrackFlow, before, after fixture.Snapshot, mark uint32, now time.Time) []*netlink.ConntrackFlow {
	var selected []*netlink.ConntrackFlow
	if mark == 0 {
		return selected
	}
	for _, f := range flows {
		if f == nil || f.Mark != mark {
			continue
		}
		src, ok := netip.AddrFromSlice(f.Forward.SrcIP)
		if !ok {
			continue
		}
		src = src.Unmap()
		dst, ok := netip.AddrFromSlice(f.Forward.DstIP)
		if !ok {
			continue
		}
		dst = dst.Unmap()
		protected := false
		for _, e := range before.Protected {
			if endpointMatches(e, dst, f.Forward.Protocol, f.Forward.DstPort) {
				protected = true
				break
			}
		}
		if !protected {
			continue
		}
		historicalAllow := false
		for _, r := range before.Allow {
			if ruleMatches(r, src, dst, f.Forward.Protocol, f.Forward.DstPort) {
				historicalAllow = true
				break
			}
		}
		// An expired old contribution can still own an established graceful flow.
		if !historicalAllow {
			continue
		}
		if !allowed(after, src, dst, f.Forward.Protocol, f.Forward.DstPort, now) {
			selected = append(selected, f)
		}
	}
	return selected
}
func endpointMatches(e fixture.Endpoint, dst netip.Addr, protocol uint8, port uint16) bool {
	return e.Destination.Contains(dst) && e.Protocol == protocol && port >= e.PortFirst && port <= e.PortLast
}
func ruleMatches(r fixture.Rule, src, dst netip.Addr, protocol uint8, port uint16) bool {
	return r.Source.Contains(src) && endpointMatches(r.Endpoint, dst, protocol, port)
}
func active(r fixture.Rule, now time.Time) bool {
	return r.ExpiresAt.IsZero() || now.Before(r.ExpiresAt)
}
func allowed(s fixture.Snapshot, src, dst netip.Addr, protocol uint8, port uint16, now time.Time) bool {
	for _, r := range s.Deny {
		if active(r, now) && ruleMatches(r, src, dst, protocol, port) {
			return false
		}
	}
	for _, r := range s.Allow {
		if active(r, now) && ruleMatches(r, src, dst, protocol, port) {
			return true
		}
	}
	return false
}
