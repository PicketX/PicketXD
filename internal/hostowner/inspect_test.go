// SPDX-License-Identifier: GPL-3.0-or-later
package hostowner

import (
	"context"
	"encoding/binary"
	"github.com/PicketX/PicketXD/internal/hostrules"
	"testing"

	"github.com/google/nftables/expr"
	nl "github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"
)

func wire(t *testing.T, a ...nl.Attribute) []byte {
	t.Helper()
	b, e := nl.MarshalAttributes(a)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestRawImageExpressionsStrict(t *testing.T) {
	expected := []expr.Any{&expr.Meta{Key: expr.MetaKeyNFPROTO, Register: 1}, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{2}}, &expr.Counter{}}
	encode := func(extra bool) []byte {
		var attrs []nl.Attribute
		for _, ex := range expected {
			b, e := expr.Marshal(unix.NFPROTO_INET, ex)
			if e != nil {
				t.Fatal(e)
			}
			if extra {
				f, _ := rawFields(b)
				f[unix.NFTA_EXPR_DATA] = append(f[unix.NFTA_EXPR_DATA], wire(t, nl.Attribute{Type: 99, Data: []byte{1}})...)
				b = wire(t, nl.Attribute{Type: unix.NFTA_EXPR_NAME, Data: f[unix.NFTA_EXPR_NAME]}, nl.Attribute{Type: unix.NFTA_EXPR_DATA, Data: f[unix.NFTA_EXPR_DATA]})
			}
			attrs = append(attrs, nl.Attribute{Type: unix.NFTA_LIST_ELEM, Data: b})
		}
		return wire(t, attrs...)
	}
	if e := compareRawExpressions(encode(false), expected); e != nil {
		t.Fatal(e)
	}
	if e := compareRawExpressions(encode(true), expected); e == nil {
		t.Fatal("unknown nested attribute accepted")
	}
	for _, bad := range [][]byte{{0, 0, 1, 0}, append(encode(false), 0, 0, 0, 0), encode(false)[:3]} {
		if e := compareRawExpressions(bad, expected); e == nil {
			t.Fatal("malformed image expression accepted")
		}
	}
	expected[1] = &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{10}}
	if e := compareRawExpressions(encode(false), []expr.Any{&expr.Counter{}}); e == nil {
		t.Fatal("changed expression structure accepted")
	}
}
func TestImageTransactionHasNoEchoOrGlobalFlush(t *testing.T) {
	c, e, _ := applicationSetup(t)
	s := mustOpen(t, c, e)
	defer s.Close()
	p := admissionPolicy(t, true, false)
	targets, _ := p.MarkTargets()
	marks := reserve(t, s, targets...)
	im, err := hostrules.Build(p, marks.State)
	if err != nil {
		t.Fatal(err)
	}
	messages, err := imageMessages(markerFor(s.record), im)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) < 5 || messages[0].Header.Type != nl.HeaderType(unix.NFNL_MSG_BATCH_BEGIN) || messages[len(messages)-1].Header.Type != nl.HeaderType(unix.NFNL_MSG_BATCH_END) {
		t.Fatal("not an atomic batch")
	}
	for _, m := range messages {
		if m.Header.Flags&nl.Echo != 0 {
			t.Fatal("unsupported byteorder reply decoder invoked")
		}
		if uint16(m.Header.Type) == uint16(unix.NFNL_SUBSYS_NFTABLES<<8|unix.NFT_MSG_DELTABLE) {
			f, err := rawFields(m.Data[4:])
			if err != nil || string(f[unix.NFTA_TABLE_NAME]) != "picketx\x00" {
				t.Fatal("unscoped table delete", err)
			}
		}
	}
}
func TestStrictImageGenerationAndDrift(t *testing.T) {
	c, e, _ := applicationSetup(t)
	s := mustOpen(t, c, e)
	defer s.Close()
	p := admissionPolicy(t, true, false)
	targets, _ := p.MarkTargets()
	marks := reserve(t, s, targets...)
	im, err := hostrules.Build(p, marks.State)
	if err != nil {
		t.Fatal(err)
	}
	for _, which := range []string{"valid", "generation", "rule", "chain", "extra-set", "malformed"} {
		t.Run(which, func(t *testing.T) {
			layout, err := im.Layout(markerFor(s.record))
			if err != nil {
				t.Fatal(err)
			}
			gen := 0
			r := dumpFunc(func(_ context.Context, cmd uint16, attrs []nl.Attribute) ([]nl.Message, error) {
				msg := func(a ...nl.Attribute) nl.Message {
					return nl.Message{Data: append([]byte{unix.NFPROTO_INET, 0, 0, 0}, wire(t, a...)...)}
				}
				switch cmd {
				case unix.NFT_MSG_GETGEN:
					gen++
					id := uint32(5)
					if which == "generation" && gen > 1 {
						id++
					}
					b := make([]byte, 4)
					binary.BigEndian.PutUint32(b, id)
					return []nl.Message{msg(nl.Attribute{Type: unix.NFTA_GEN_ID, Data: b})}, nil
				case unix.NFT_MSG_GETTABLE:
					return []nl.Message{msg(nl.Attribute{Type: unix.NFTA_TABLE_NAME, Data: []byte("picketx\x00")}, nl.Attribute{Type: unix.NFTA_TABLE_FLAGS, Data: make([]byte, 4)})}, nil
				case unix.NFT_MSG_GETCHAIN:
					var result []nl.Message
					for _, ch := range layout.Chains {
						a := chainAttributes(ch)
						if which == "chain" && ch.Hooknum != nil {
							a = append(a, nl.Attribute{Type: 99, Data: []byte{1}})
						}
						result = append(result, msg(a...))
					}
					return result, nil
				case unix.NFT_MSG_GETRULE:
					var result []nl.Message
					for _, rule := range layout.Rules {
						a, err := ruleAttributes(rule)
						if err != nil {
							t.Fatal(err)
						}
						if which == "rule" {
							a = append(a, nl.Attribute{Type: 99, Data: []byte{1}})
						}
						m := msg(a...)
						if which == "malformed" {
							m.Data = append(m.Data, 0, 0, 0, 0)
						}
						result = append(result, m)
					}
					return result, nil
				case unix.NFT_MSG_GETSET:
					if which == "extra-set" {
						return []nl.Message{msg(nl.Attribute{Type: 1, Data: []byte("picketx\x00")})}, nil
					}
				}
				return nil, nil
			})
			got, err := inspectImages(context.Background(), r, markerFor(s.record), []hostrules.Image{im})
			if which == "valid" {
				if err != nil || got.Image != im.Digest() {
					t.Fatal(got, err)
				}
			} else if err == nil {
				t.Fatal("drift accepted", which)
			}
		})
	}
}
func TestNativeMarkerOnlyGenerationBracket(t *testing.T) {
	c, e, _ := setup(t)
	s := mustOpen(t, c, e)
	defer s.Close()
	marker := markerFor(s.record)
	for _, changed := range []bool{false, true} {
		gen := 0
		r := dumpFunc(func(_ context.Context, cmd uint16, _ []nl.Attribute) ([]nl.Message, error) {
			msg := func(a ...nl.Attribute) []nl.Message {
				return []nl.Message{{Data: append([]byte{unix.NFPROTO_INET, 0, 0, 0}, wire(t, a...)...)}}
			}
			switch cmd {
			case unix.NFT_MSG_GETGEN:
				gen++
				id := uint32(7)
				if changed && gen > 1 {
					id++
				}
				return msg(nl.Attribute{Type: unix.NFTA_GEN_ID, Data: be32(id)}), nil
			case unix.NFT_MSG_GETTABLE:
				return msg(nl.Attribute{Type: unix.NFTA_TABLE_NAME, Data: []byte("picketx\x00")}, nl.Attribute{Type: unix.NFTA_TABLE_FLAGS, Data: make([]byte, 4)}), nil
			case unix.NFT_MSG_GETCHAIN:
				return msg(nl.Attribute{Type: unix.NFTA_CHAIN_TABLE, Data: []byte("picketx\x00")}, nl.Attribute{Type: unix.NFTA_CHAIN_NAME, Data: []byte(ownerChain + "\x00")}), nil
			case unix.NFT_MSG_GETRULE:
				b, _ := expr.Marshal(unix.NFPROTO_INET, &expr.Counter{})
				return msg(nl.Attribute{Type: unix.NFTA_RULE_TABLE, Data: []byte("picketx\x00")}, nl.Attribute{Type: unix.NFTA_RULE_CHAIN, Data: []byte(ownerChain + "\x00")}, nl.Attribute{Type: unix.NFTA_RULE_USERDATA, Data: []byte(marker)}, nl.Attribute{Type: unix.NFTA_RULE_EXPRESSIONS, Data: wire(t, nl.Attribute{Type: unix.NFTA_LIST_ELEM, Data: b})}), nil
			}
			return nil, nil
		})
		b := &nftKernel{reader: r}
		b.ConfigureImages(marker, nil)
		got, err := b.Observe(context.Background())
		if gen < 2 || changed && err == nil || !changed && (err != nil || !got.Exists) {
			t.Fatalf("marker-only generation proof absent changed=%v calls=%d got=%+v err=%v", changed, gen, got, err)
		}
	}
}
func TestExplicitDefaultBitwiseOperation(t *testing.T) {
	ex := &expr.Bitwise{SourceRegister: 1, DestRegister: 1, Len: 4, Mask: []byte{255, 255, 255, 0}, Xor: make([]byte, 4)}
	encoded, _ := expr.Marshal(unix.NFPROTO_INET, ex)
	f, _ := rawFields(encoded)
	for _, op := range []uint32{0, 1, 99} {
		body := append(append([]byte{}, f[unix.NFTA_EXPR_DATA]...), wire(t, nl.Attribute{Type: 6, Data: be32(op)})...)
		actual := wire(t, nl.Attribute{Type: unix.NFTA_LIST_ELEM, Data: wire(t, nl.Attribute{Type: unix.NFTA_EXPR_NAME, Data: f[unix.NFTA_EXPR_NAME]}, nl.Attribute{Type: unix.NFTA_EXPR_DATA | unix.NLA_F_NESTED, Data: body})})
		err := compareRawExpressions(actual, []expr.Any{ex})
		if op == 0 && err != nil || op != 0 && err == nil {
			t.Fatalf("explicit default bitwise operation %d: %v", op, err)
		}
	}
}
