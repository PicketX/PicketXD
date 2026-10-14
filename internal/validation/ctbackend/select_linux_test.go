package ctbackend

import (
	"github.com/PicketX/PicketXD/internal/validation/fixture"
	"github.com/vishvananda/netlink"
	"net"
	"net/netip"
	"testing"
	"time"
)

func input(v6 bool) fixture.Snapshot {
	dst, src := "192.0.2.100/32", "192.0.2.10/32"
	if v6 {
		dst, src = "2001:db8:1:0:2::100/128", "2001:db8:1:0:1::10/128"
	}
	e := fixture.Endpoint{Destination: netip.MustParsePrefix(dst), Protocol: 6, PortFirst: 8000, PortLast: 8002}
	return fixture.Snapshot{Protected: []fixture.Endpoint{e}, Allow: []fixture.Rule{{Source: netip.MustParsePrefix(src), Endpoint: e}}}
}
func flow(v6 bool) *netlink.ConntrackFlow {
	src, dst := "192.0.2.10", "192.0.2.100"
	family := uint8(2)
	if v6 {
		src, dst = "2001:db8:1:0:1::10", "2001:db8:1:0:2::100"
		family = 10
	}
	return &netlink.ConntrackFlow{FamilyType: family, Mark: 123, Forward: netlink.IPTuple{SrcIP: net.ParseIP(src), DstIP: net.ParseIP(dst), SrcPort: 41000, DstPort: 8000, Protocol: 6}, Reverse: netlink.IPTuple{SrcIP: net.ParseIP(dst), DstIP: net.ParseIP(src), SrcPort: 8000, DstPort: 41000, Protocol: 6}}
}
func TestSelectWithdrawn(t *testing.T) {
	now := time.Unix(100, 0)
	for _, v6 := range []bool{false, true} {
		cases := []struct {
			name   string
			modify func(*netlink.ConntrackFlow, *fixture.Snapshot, *fixture.Snapshot)
			want   int
		}{
			{"withdrawn", func(*netlink.ConntrackFlow, *fixture.Snapshot, *fixture.Snapshot) {}, 1},
			{"unmarked", func(f *netlink.ConntrackFlow, b, a *fixture.Snapshot) { f.Mark = 0 }, 0},
			{"different marker", func(f *netlink.ConntrackFlow, b, a *fixture.Snapshot) { f.Mark = 124 }, 0},
			{"another port", func(f *netlink.ConntrackFlow, b, a *fixture.Snapshot) { f.Forward.DstPort = 9000 }, 0},
			{"another protocol", func(f *netlink.ConntrackFlow, b, a *fixture.Snapshot) { f.Forward.Protocol = 17 }, 0},
			{"remaining prefix grant", func(f *netlink.ConntrackFlow, b, a *fixture.Snapshot) {
				*a = input(v6)
				bits := 26
				if v6 {
					bits = 80
				}
				a.Allow[0].Source = netip.PrefixFrom(a.Allow[0].Source.Addr(), bits).Masked()
			}, 0},
			{"new deny", func(f *netlink.ConntrackFlow, b, a *fixture.Snapshot) {
				*a = input(v6)
				a.Deny = append([]fixture.Rule{}, a.Allow...)
			}, 1},
			{"expired grant", func(f *netlink.ConntrackFlow, b, a *fixture.Snapshot) {
				b.Allow[0].ExpiresAt = now.Add(-time.Second)
				*a = input(v6)
				a.Allow[0].ExpiresAt = now
			}, 1},
			{"expired deny", func(f *netlink.ConntrackFlow, b, a *fixture.Snapshot) {
				*a = input(v6)
				a.Deny = append([]fixture.Rule{}, a.Allow...)
				a.Deny[0].ExpiresAt = now
			}, 0},
			{"original NAT boundary", func(f *netlink.ConntrackFlow, b, a *fixture.Snapshot) {
				f.Reverse.SrcIP = net.ParseIP("192.0.2.200")
				f.Reverse.SrcPort = 9000
			}, 1},
			{"reply tuple alone cannot select", func(f *netlink.ConntrackFlow, b, a *fixture.Snapshot) { f.Forward.DstPort = 9000 }, 0},
			{"outside new protected scope", func(f *netlink.ConntrackFlow, b, a *fixture.Snapshot) { a.Protected[0].PortFirst = 8001 }, 1},
		}
		for _, tc := range cases {
			t.Run(tc.name+map[bool]string{false: "/IPv4", true: "/IPv6"}[v6], func(t *testing.T) {
				b := input(v6)
				a := input(v6)
				a.Allow = nil
				f := flow(v6)
				tc.modify(f, &b, &a)
				got := SelectWithdrawn([]*netlink.ConntrackFlow{nil, f}, b, a, 123, now)
				if len(got) != tc.want {
					t.Fatalf("selected=%d want=%d", len(got), tc.want)
				}
			})
		}
	}
}
