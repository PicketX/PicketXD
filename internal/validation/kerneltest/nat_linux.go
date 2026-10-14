package kerneltest

import (
	"fmt"
	"github.com/google/nftables"
	"github.com/google/nftables/binaryutil"
	"github.com/google/nftables/expr"
	"golang.org/x/sys/unix"
)

// DNAT belongs to the same owned test table, and is reapplied after a snapshot.
func (x *Topology) DNAT() error {
	table := &nftables.Table{Name: "picketx", Family: nftables.TableFamilyINet}
	chain := x.NFT.AddChain(&nftables.Chain{Name: "picketx_nat", Table: table, Type: nftables.ChainTypeNAT, Hooknum: nftables.ChainHookPrerouting, Priority: nftables.ChainPriorityNATDest})
	for i, vip := range NATDestinations {
		family := byte(unix.NFPROTO_IPV4)
		natFamily := uint32(unix.AF_INET)
		offset := uint32(16)
		size := uint32(4)
		if i == 1 {
			family = unix.NFPROTO_IPV6
			natFamily = unix.AF_INET6
			offset = 24
			size = 16
		}
		x.NFT.AddRule(&nftables.Rule{Table: table, Chain: chain, Exprs: []expr.Any{
			&expr.Meta{Key: expr.MetaKeyNFPROTO, Register: 1}, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{family}},
			&expr.Meta{Key: expr.MetaKeyL4PROTO, Register: 1}, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{6}},
			&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseNetworkHeader, Offset: offset, Len: size}, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: vip.AsSlice()},
			&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseTransportHeader, Offset: 2, Len: 2}, &expr.Range{Op: expr.CmpOpEq, Register: 1, FromData: binaryutil.BigEndian.PutUint16(8000), ToData: binaryutil.BigEndian.PutUint16(8001)},
			&expr.Immediate{Register: 1, Data: Destinations[i].AsSlice()}, &expr.Immediate{Register: 2, Data: binaryutil.BigEndian.PutUint16(9000)},
			&expr.NAT{Type: expr.NATTypeDestNAT, Family: natFamily, RegAddrMin: 1, RegProtoMin: 2},
		}})
	}
	if err := x.NFT.Flush(); err != nil {
		return fmt.Errorf("test DNAT: %w", err)
	}
	return nil
}
