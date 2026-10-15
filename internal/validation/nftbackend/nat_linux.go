package nftbackend

import (
	"github.com/google/nftables"
	"github.com/google/nftables/binaryutil"
	"github.com/google/nftables/expr"
	"golang.org/x/sys/unix"
	"net/netip"
)

// Only the proven M0 fixture translation; no configurable product port mapping.
func appendFixtureNAT(tab *nftables.Table, chain func(*nftables.Chain) *nftables.Chain, rule func(*nftables.Rule) *nftables.Rule) {
	c := chain(&nftables.Chain{Name: "picketx_nat", Table: tab, Type: nftables.ChainTypeNAT, Hooknum: nftables.ChainHookPrerouting, Priority: nftables.ChainPriorityNATDest})
	for _, pair := range [][2]string{{"192.0.2.101", "192.0.2.100"}, {"2001:db8:1:0:2::101", "2001:db8:1:0:2::100"}} {
		vip, dst := netip.MustParseAddr(pair[0]), netip.MustParseAddr(pair[1])
		family := uint32(unix.AF_INET)
		nf := byte(unix.NFPROTO_IPV4)
		offset, size := uint32(16), uint32(4)
		if vip.Is6() {
			family = unix.AF_INET6
			nf = unix.NFPROTO_IPV6
			offset = 24
			size = 16
		}
		rule(&nftables.Rule{Table: tab, Chain: c, Exprs: []expr.Any{
			&expr.Meta{Key: expr.MetaKeyNFPROTO, Register: 1}, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{nf}},
			&expr.Meta{Key: expr.MetaKeyL4PROTO, Register: 1}, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{6}},
			&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseNetworkHeader, Offset: offset, Len: size}, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: vip.AsSlice()},
			&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseTransportHeader, Offset: 2, Len: 2}, &expr.Range{Op: expr.CmpOpEq, Register: 1, FromData: binaryutil.BigEndian.PutUint16(8000), ToData: binaryutil.BigEndian.PutUint16(8001)},
			&expr.Immediate{Register: 1, Data: dst.AsSlice()}, &expr.Immediate{Register: 2, Data: binaryutil.BigEndian.PutUint16(9000)}, &expr.NAT{Type: expr.NATTypeDestNAT, Family: family, RegAddrMin: 1, RegProtoMin: 2},
		}})
	}
}
