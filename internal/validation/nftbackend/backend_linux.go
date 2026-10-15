// Package nftbackend is an M0 adapter, not a production Agent backend.
package nftbackend

import (
	"context"
	"encoding/binary"
	"fmt"
	"net/netip"
	"sync"
	"time"

	"github.com/PicketX/PicketXD/internal/validation/fixture"
	"github.com/google/nftables"
	"github.com/google/nftables/binaryutil"
	"github.com/google/nftables/expr"
	"golang.org/x/sys/unix"
)

const TestMark uint32 = 0x5049434b

type Backend struct {
	conn       *nftables.Conn
	mu         sync.Mutex
	batch      func(context.Context, fixture.Snapshot, time.Time) error
	now        func() time.Time
	generation uint64
	// Test-only invalid-operation seam; nil outside package integration tests.
	inject func(*nftables.Conn)
}

func New(c *nftables.Conn) *Backend {
	b := &Backend{conn: c, now: time.Now}
	b.batch = b.applyBatch
	return b
}
func (b *Backend) Close() error { return b.conn.CloseLasting() }
func (b *Backend) Apply(ctx context.Context, s fixture.Snapshot, now time.Time) error {
	if err := fixture.Validate(s); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	return b.batch(ctx, s, b.now())
}
func (b *Backend) applyBatch(ctx context.Context, s fixture.Snapshot, now time.Time) error {
	_, err := b.applyReceiptLocked(ctx, s, Options{}, now)
	return err
}
func (b *Backend) queueBatch(ctx context.Context, c *nftables.Conn, s fixture.Snapshot, now time.Time, receipt *Receipt, opts Options) error {
	addChain := func(chain *nftables.Chain) *nftables.Chain {
		receipt.manifest.chains[chain.Name] = chain
		return c.AddChain(chain)
	}
	addRule := func(rule *nftables.Rule) *nftables.Rule {
		receipt.manifest.rules[rule.Chain.Name] = append(receipt.manifest.rules[rule.Chain.Name], rule.Exprs)
		return c.AddRule(rule)
	}

	tables, err := c.ListTablesOfFamily(nftables.TableFamilyINet)
	if err != nil {
		return fmt.Errorf("list owned table: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, tab := range tables {
		if tab.Name == "picketx" {
			c.DelTable(tab)
		}
	}
	tab := c.AddTable(&nftables.Table{Name: "picketx", Family: nftables.TableFamilyINet})
	priority := nftables.ChainPriority(-110)
	hook := nftables.ChainHookPrerouting
	if opts.HostInputFixture {
		hook = nftables.ChainHookInput
	}
	main := addChain(&nftables.Chain{Name: "picketx_admission", Table: tab, Type: nftables.ChainTypeFilter, Hooknum: hook, Priority: &priority})
	type entry struct {
		rule fixture.Rule
		set  *nftables.Set
	}
	create := func(rules []fixture.Rule, label string) ([]entry, error) {
		var entries []entry
		for i, r := range rules {
			addrType := nftables.TypeIPAddr
			if r.Source.Addr().Is6() {
				addrType = nftables.TypeIP6Addr
			}
			set := &nftables.Set{Table: tab, Name: fmt.Sprintf("picketx_%s_%d", label, i), KeyType: nftables.MustConcatSetType(addrType, addrType, nftables.TypeInetProto, nftables.TypeInetService), Interval: true, Concatenation: true, HasTimeout: !r.ExpiresAt.IsZero()}
			if err := c.AddSet(set, nil); err != nil {
				return nil, err
			}
			entries = append(entries, entry{r, set})
			receipt.manifest.sets[set.Name] = setExpectation{rule: r, set: set}
			receipt.manifest.pending = append(receipt.manifest.pending, pendingMember{rule: r, set: set, deny: label == "deny"})
		}
		return entries, nil
	}
	denies, err := create(s.Deny, "deny")
	if err != nil {
		return err
	}
	allows, err := create(s.Allow, "allow")
	if err != nil {
		return err
	}
	for i, e := range s.Protected {
		chain := addChain(&nftables.Chain{Name: fmt.Sprintf("picketx_scope_%d", i), Table: tab})
		match := endpointMatch(e)
		permissionLookup := lookup
		if opts.HostInputFixture {
			// Original tuple reads cannot match untracked traffic. Protect the
			// current tuple explicitly before relying on conntrack dispatch.
			guard := append(endpointMatch(e), untrackedDrop()...)
			addRule(&nftables.Rule{Table: tab, Chain: main, Exprs: guard})
			match = hostEndpointMatch(e)
			permissionLookup = hostLookup
		}
		addRule(&nftables.Rule{Table: tab, Chain: main, Exprs: append(match, &expr.Verdict{Kind: expr.VerdictJump, Chain: chain.Name})})
		for _, en := range denies {
			if en.rule.Source.Addr().Is4() == e.Destination.Addr().Is4() {
				addRule(&nftables.Rule{Table: tab, Chain: chain, Exprs: append(permissionLookup(en.rule, en.set), &expr.Verdict{Kind: expr.VerdictDrop})})
			}
		}
		// Preserve established authorized traffic on graceful expiry. Explicit denies above win.
		addRule(&nftables.Rule{Table: tab, Chain: chain, Exprs: []expr.Any{
			&expr.Ct{Key: expr.CtKeyMARK, Register: 1}, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: binaryutil.NativeEndian.PutUint32(TestMark)},
			&expr.Ct{Key: expr.CtKeySTATE, Register: 1}, &expr.Bitwise{SourceRegister: 1, DestRegister: 1, Len: 4, Mask: binaryutil.NativeEndian.PutUint32(2 | 4), Xor: make([]byte, 4)},
			&expr.Cmp{Op: expr.CmpOpNeq, Register: 1, Data: make([]byte, 4)}, &expr.Verdict{Kind: expr.VerdictAccept},
		}})
		for _, en := range allows {
			if en.rule.Source.Addr().Is4() != e.Destination.Addr().Is4() {
				continue
			}
			ex := permissionLookup(en.rule, en.set)
			if en.rule.Endpoint.Protocol == 6 {
				// An old ACK/PSH must not recreate authorization after conntrack deletion.
				ex = append(ex, &expr.Payload{DestRegister: 1, Base: expr.PayloadBaseTransportHeader, Offset: 13, Len: 1}, &expr.Bitwise{SourceRegister: 1, DestRegister: 1, Len: 1, Mask: []byte{0x12}, Xor: []byte{0}}, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{0x02}})
			}
			ex = append(ex, &expr.Immediate{Register: 1, Data: binaryutil.NativeEndian.PutUint32(TestMark)}, &expr.Ct{Key: expr.CtKeyMARK, Register: 1, SourceRegister: true}, &expr.Verdict{Kind: expr.VerdictAccept})
			addRule(&nftables.Rule{Table: tab, Chain: chain, Exprs: ex})
		}
		addRule(&nftables.Rule{Table: tab, Chain: chain, Exprs: []expr.Any{&expr.Counter{}, &expr.Verdict{Kind: expr.VerdictDrop}}})
	}
	if opts.FixtureNAT {
		appendFixtureNAT(tab, addChain, addRule)
	}
	return nil
}
func membership(r fixture.Rule, now time.Time) (nftables.SetElement, bool) {
	var timeout time.Duration
	if !r.ExpiresAt.IsZero() {
		timeout = r.ExpiresAt.Sub(now).Truncate(time.Millisecond)
		if timeout <= 0 {
			return nftables.SetElement{}, false
		}
	}
	return nftables.SetElement{Key: key(r.Source.Addr(), r.Endpoint.Destination.Addr(), r.Endpoint.Protocol, r.Endpoint.PortFirst), KeyEnd: key(last(r.Source), last(r.Endpoint.Destination), r.Endpoint.Protocol, r.Endpoint.PortLast), Timeout: timeout}, true
}
func last(p netip.Prefix) netip.Addr {
	b := append([]byte{}, p.Addr().AsSlice()...)
	for bit := p.Bits(); bit < len(b)*8; bit++ {
		b[bit/8] |= 1 << uint(7-bit%8)
	}
	a, _ := netip.AddrFromSlice(b)
	return a
}
func key(src, dst netip.Addr, protocol uint8, port uint16) []byte {
	b := append(append([]byte{}, src.AsSlice()...), dst.AsSlice()...)
	b = append(b, protocol, 0, 0, 0)
	p := make([]byte, 4)
	binary.BigEndian.PutUint16(p, port)
	return append(b, p...)
}
func lookup(r fixture.Rule, s *nftables.Set) []expr.Any {
	size := uint32(4)
	srcOffset := uint32(12)
	dstOffset := uint32(16)
	if r.Source.Addr().Is6() {
		size = 16
		srcOffset = 8
		dstOffset = 24
	}
	reg := uint32(unix.NFT_REG32_00)
	protoReg := reg + 2*size/4
	portReg := protoReg + 1
	return []expr.Any{
		&expr.Payload{DestRegister: reg, Base: expr.PayloadBaseNetworkHeader, Offset: srcOffset, Len: size},
		&expr.Payload{DestRegister: reg + size/4, Base: expr.PayloadBaseNetworkHeader, Offset: dstOffset, Len: size},
		&expr.Immediate{Register: protoReg, Data: make([]byte, 4)}, &expr.Meta{Key: expr.MetaKeyL4PROTO, Register: protoReg},
		&expr.Immediate{Register: portReg, Data: make([]byte, 4)}, &expr.Payload{DestRegister: portReg, Base: expr.PayloadBaseTransportHeader, Offset: 2, Len: 2},
		&expr.Lookup{SourceRegister: reg, SetName: s.Name, SetID: s.ID},
	}
}
func endpointMatch(e fixture.Endpoint) []expr.Any {
	family := byte(unix.NFPROTO_IPV4)
	size := uint32(4)
	offset := uint32(16)
	if e.Destination.Addr().Is6() {
		family = unix.NFPROTO_IPV6
		size = 16
		offset = 24
	}
	mask := make([]byte, size)
	for bit := 0; bit < e.Destination.Bits(); bit++ {
		mask[bit/8] |= 1 << uint(7-bit%8)
	}
	return []expr.Any{
		&expr.Meta{Key: expr.MetaKeyNFPROTO, Register: 1}, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{family}},
		&expr.Meta{Key: expr.MetaKeyL4PROTO, Register: 1}, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{e.Protocol}},
		&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseNetworkHeader, Offset: offset, Len: size}, &expr.Bitwise{SourceRegister: 1, DestRegister: 1, Len: size, Mask: mask, Xor: make([]byte, size)}, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: e.Destination.Addr().AsSlice()},
		&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseTransportHeader, Offset: 2, Len: 2}, &expr.Range{Op: expr.CmpOpEq, Register: 1, FromData: binaryutil.BigEndian.PutUint16(e.PortFirst), ToData: binaryutil.BigEndian.PutUint16(e.PortLast)},
	}
}
