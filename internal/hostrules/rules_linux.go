// SPDX-License-Identifier: GPL-3.0-or-later
package hostrules

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"time"

	"github.com/PicketX/PicketX/pkg/networkplan"
	"github.com/PicketX/PicketXD/internal/hostpolicy"
	"github.com/google/nftables"
	"github.com/google/nftables/binaryutil"
	"github.com/google/nftables/expr"
	"golang.org/x/sys/unix"
)

// NFT_META_TIME_NS is stable Linux UAPI key 30, absent from the pinned x/sys.
const metaTimeNS expr.MetaKey = 30
const OwnerChain = "picketx__owner"

type Layout struct {
	Table  *nftables.Table
	Chains []*nftables.Chain
	Rules  []*nftables.Rule
}

func (im Image) Layout(marker string) (Layout, error) {
	if e := im.Validate(); e != nil {
		return Layout{}, e
	}
	tab := &nftables.Table{Name: "picketx", Family: nftables.TableFamilyINet}
	out := Layout{Table: tab}
	owner := &nftables.Chain{Name: OwnerChain, Table: tab}
	out.Chains = append(out.Chains, owner)
	out.Rules = append(out.Rules, &nftables.Rule{Table: tab, Chain: owner, UserData: []byte(marker), Exprs: []expr.Any{&expr.Counter{}}})
	out.Rules = append(out.Rules, &nftables.Rule{Table: tab, Chain: owner, UserData: []byte("picketx-image-v1:" + im.Digest()), Exprs: []expr.Any{&expr.Counter{}}})
	if len(im.Targets) == 0 {
		return out, nil
	}
	priority := nftables.ChainPriority(-110)
	policy := nftables.ChainPolicyAccept
	main := &nftables.Chain{Name: "picketx_admission", Table: tab, Type: nftables.ChainTypeFilter, Hooknum: nftables.ChainHookInput, Priority: &priority, Policy: &policy}
	out.Chains = append(out.Chains, main)
	add := func(chain *nftables.Chain, ex []expr.Any) {
		out.Rules = append(out.Rules, &nftables.Rule{Table: tab, Chain: chain, Exprs: ex})
	}
	if im.Context.Handling == networkplan.Strict {
		shared := &nftables.Chain{Name: strictEstablishedChain, Table: tab}
		out.Chains = append(out.Chains, shared)
		for _, mark := range im.KnownMarks {
			add(shared, ownedEstablished(mark))
		}
		add(shared, []expr.Any{&expr.Verdict{Kind: expr.VerdictReturn}})
	}
	for i, t := range im.Targets {
		ch := &nftables.Chain{Name: fmt.Sprintf("picketx_target_%d", i), Table: tab}
		out.Chains = append(out.Chains, ch)
		for _, ep := range t.Protected {
			invalid := append(endpointMatch(ep, false), &expr.Ct{Key: expr.CtKeySTATE, Register: 1}, &expr.Bitwise{SourceRegister: 1, DestRegister: 1, Len: 4, Mask: binaryutil.NativeEndian.PutUint32(expr.CtStateBitINVALID | expr.CtStateBitUNTRACKED), Xor: make([]byte, 4)}, &expr.Cmp{Op: expr.CmpOpNeq, Register: 1, Data: make([]byte, 4)}, &expr.Verdict{Kind: expr.VerdictDrop})
			add(main, invalid)
			add(main, append(endpointMatch(ep, true), &expr.Verdict{Kind: expr.VerdictJump, Chain: ch.Name}))
		}
		for _, m := range t.Deny {
			add(ch, append(member(m), &expr.Verdict{Kind: expr.VerdictDrop}))
		}
		if im.Context.Handling == networkplan.Graceful {
			add(ch, []expr.Any{&expr.Ct{Key: expr.CtKeyMARK, Register: 1}, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: binaryutil.NativeEndian.PutUint32(t.Mark)}, &expr.Ct{Key: expr.CtKeySTATE, Register: 1}, &expr.Bitwise{SourceRegister: 1, DestRegister: 1, Len: 4, Mask: binaryutil.NativeEndian.PutUint32(expr.CtStateBitESTABLISHED | expr.CtStateBitRELATED), Xor: make([]byte, 4)}, &expr.Cmp{Op: expr.CmpOpNeq, Register: 1, Data: make([]byte, 4)}, &expr.Verdict{Kind: expr.VerdictAccept}})
		}

		for _, m := range t.Allow {
			ex := member(m)
			if m.Endpoint.Protocol == 6 {
				if im.Context.Handling == networkplan.Strict {
					add(ch, append(member(m), &expr.Verdict{Kind: expr.VerdictJump, Chain: strictEstablishedChain}))
					ex = append(ex, conntrackState(expr.CtStateBitNEW)...)
				}
				ex = append(ex, &expr.Payload{DestRegister: 1, Base: expr.PayloadBaseTransportHeader, Offset: 13, Len: 1}, &expr.Bitwise{SourceRegister: 1, DestRegister: 1, Len: 1, Mask: []byte{0x12}, Xor: []byte{0}}, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{0x02}})
			}
			ex = append(ex, &expr.Immediate{Register: 1, Data: binaryutil.NativeEndian.PutUint32(t.Mark)}, &expr.Ct{Key: expr.CtKeyMARK, Register: 1, SourceRegister: true}, &expr.Verdict{Kind: expr.VerdictAccept})
			add(ch, ex)
		}
		add(ch, []expr.Any{&expr.Counter{}, &expr.Verdict{Kind: expr.VerdictDrop}})
	}
	if im.Context.Handling == networkplan.Strict {
		// A server packet can recreate deleted conntrack in reverse orientation.
		// After all original-destination dispatches, refuse reply-direction
		// packets addressed to a protected service endpoint. Original-direction
		// DNAT aliases outside our scope keep their existing behavior.
		for _, t := range im.Targets {
			for _, ep := range t.Protected {
				add(main, append(endpointMatch(ep, false), &expr.Ct{Key: expr.CtKeyDIRECTION, Register: 1}, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{1}}, &expr.Verdict{Kind: expr.VerdictDrop}))
			}
		}
	}
	return out, nil
}
func prefixMatch(p netip.Prefix, read expr.Any) []expr.Any {
	size := len(p.Addr().AsSlice())
	mask := make([]byte, size)
	for bit := 0; bit < p.Bits(); bit++ {
		mask[bit/8] |= 1 << uint(7-bit%8)
	}
	return []expr.Any{read, &expr.Bitwise{SourceRegister: 1, DestRegister: 1, Len: uint32(size), Mask: mask, Xor: make([]byte, size)}, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: p.Addr().AsSlice()}}
}
func endpointMatch(ep hostpolicy.Endpoint, original bool) []expr.Any {
	family := byte(unix.NFPROTO_IPV4)
	offset, size := uint32(16), uint32(4)
	key := expr.CtKeyDSTIP
	if ep.Destination.Addr().Is6() {
		family = unix.NFPROTO_IPV6
		offset = 24
		size = 16
		key = expr.CtKeyDSTIP6
	}
	ex := []expr.Any{&expr.Meta{Key: expr.MetaKeyNFPROTO, Register: 1}, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{family}}}
	var address expr.Any = &expr.Payload{DestRegister: 1, Base: expr.PayloadBaseNetworkHeader, Offset: offset, Len: size}
	var protocol expr.Any = &expr.Meta{Key: expr.MetaKeyL4PROTO, Register: 1}
	var port expr.Any = &expr.Payload{DestRegister: 1, Base: expr.PayloadBaseTransportHeader, Offset: 2, Len: 2}
	if original {
		address = &expr.Ct{Key: key, Register: 1, Direction: expr.CtDirOriginal}
		protocol = &expr.Ct{Key: expr.CtKeyPROTOCOL, Register: 1}
		port = &expr.Ct{Key: expr.CtKeyPROTODST, Register: 1, Direction: expr.CtDirOriginal}
	}
	ex = append(ex, protocol, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{ep.Protocol}})
	ex = append(ex, prefixMatch(ep.Destination, address)...)
	ex = append(ex, port, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: binaryutil.BigEndian.PutUint16(ep.Port)})
	return ex
}
func boundary(t time.Time, op expr.CmpOp) []expr.Any {
	if t.IsZero() {
		return nil
	}
	data := make([]byte, 8)
	binary.BigEndian.PutUint64(data, uint64(t.UnixNano()))
	return []expr.Any{&expr.Meta{Key: metaTimeNS, Register: 1}, &expr.Byteorder{SourceRegister: 1, DestRegister: 1, Op: expr.ByteorderHton, Len: 8, Size: 8}, &expr.Cmp{Op: op, Register: 1, Data: data}}
}
func member(m hostpolicy.Member) []expr.Any {
	ex := endpointMatch(m.Endpoint, true)
	key := expr.CtKeySRCIP
	if m.Source.Addr().Is6() {
		key = expr.CtKeySRCIP6
	}
	ex = append(ex, prefixMatch(m.Source, &expr.Ct{Key: key, Register: 1, Direction: expr.CtDirOriginal})...)
	ex = append(ex, boundary(m.Validity.NotBefore, expr.CmpOpGte)...)
	return append(ex, boundary(m.Validity.ExpiresAt, expr.CmpOpLt)...)
}
