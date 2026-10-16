// SPDX-License-Identifier: GPL-3.0-or-later
package hostowner

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/PicketX/PicketXD/internal/hostrules"
	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	nl "github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"
)

const attributeTypeMask uint16 = 0x3fff

func attributeMap(data []byte) (map[uint16]nl.Attribute, error) {
	if err := validateConntrackEnvelopes(data); err != nil {
		return nil, err
	}
	attrs, err := nl.UnmarshalAttributes(data)
	if err != nil {
		return nil, err
	}
	out := map[uint16]nl.Attribute{}
	for _, a := range attrs {
		k := a.Type & attributeTypeMask
		if _, ok := out[k]; ok {
			return nil, fmt.Errorf("duplicate semantic attribute")
		}
		if a.Type&^(uint16(attributeTypeMask)|uint16(unix.NLA_F_NESTED)) != 0 {
			return nil, fmt.Errorf("unsupported attribute flags")
		}
		out[k] = a
	}
	return out, nil
}
func compareAttributes(actual, expected []byte) error {
	a, e := attributeMap(actual)
	if e != nil {
		return e
	}
	b, e := attributeMap(expected)
	if e != nil {
		return e
	}
	if len(a) != len(b) {
		return fmt.Errorf("semantic attribute count differs")
	}
	for key, want := range b {
		got, ok := a[key]
		if !ok {
			return fmt.Errorf("semantic attribute %d absent", key)
		}
		if got.Type&unix.NLA_F_NESTED != 0 || want.Type&unix.NLA_F_NESTED != 0 {
			if e = compareAttributes(got.Data, want.Data); e != nil {
				return e
			}
		} else if !bytes.Equal(got.Data, want.Data) {
			return fmt.Errorf("semantic attribute %d differs: %x/%x", key, got.Data, want.Data)
		}
	}
	return nil
}
func compareRawExpressions(actual []byte, expected []expr.Any) error {
	if err := validateConntrackEnvelopes(actual); err != nil {
		return err
	}
	attrs, err := nl.UnmarshalAttributes(actual)
	if err != nil {
		return err
	}
	if len(attrs) != len(expected) {
		return fmt.Errorf("expression count differs")
	}
	for i, a := range attrs {
		if a.Type&attributeTypeMask != unix.NFTA_LIST_ELEM {
			return fmt.Errorf("unexpected expression envelope")
		}
		f, e := rawFields(a.Data)
		if e != nil {
			return e
		}
		if e = allowedFields(f, unix.NFTA_EXPR_NAME, unix.NFTA_EXPR_DATA); e != nil {
			return e
		}
		if _, ok := expected[i].(*expr.Counter); ok {
			wrapper, _ := nl.MarshalAttributes([]nl.Attribute{{Type: unix.NFTA_LIST_ELEM, Data: a.Data}})
			if e = checkRawCounter(wrapper); e != nil {
				return e
			}
			continue
		}
		want, e := expr.Marshal(unix.NFPROTO_INET, expected[i])
		if e != nil {
			return e
		}
		if e = compareAttributes(a.Data, want); e != nil {
			return fmt.Errorf("expression %d (%s): %w", i, strings.TrimRight(string(f[unix.NFTA_EXPR_NAME]), "\x00"), e)
		}
	}
	return nil
}
func generation(ctx context.Context, r dumpReader) (uint32, error) {
	msgs, e := r.Dump(ctx, unix.NFT_MSG_GETGEN, nil)
	if e != nil {
		return 0, e
	}
	if len(msgs) != 1 || len(msgs[0].Data) < 4 {
		return 0, fmt.Errorf("missing nft generation")
	}
	f, e := rawFields(msgs[0].Data[4:])
	if e != nil {
		return 0, e
	}
	if e = allowedFields(f, unix.NFTA_GEN_ID, 2, 3); e != nil {
		return 0, e
	}
	if len(f[unix.NFTA_GEN_ID]) != 4 {
		return 0, fmt.Errorf("malformed nft generation")
	}
	return binary.BigEndian.Uint32(f[unix.NFTA_GEN_ID]), nil
}

type rawImage struct {
	chains map[string]map[uint16][]byte
	rules  map[string][]map[uint16][]byte
}

func inspectImages(ctx context.Context, r dumpReader, marker string, images []hostrules.Image) (observation, error) {
	before, e := generation(ctx, r)
	if e != nil {
		return observation{}, e
	}
	tab, e := rawTable(ctx, r)
	if e != nil {
		return observation{}, e
	}
	if tab == nil {
		return observation{}, nil
	}
	live := rawImage{chains: map[string]map[uint16][]byte{}, rules: map[string][]map[uint16][]byte{}}
	msgs, e := r.Dump(ctx, unix.NFT_MSG_GETCHAIN, []nl.Attribute{{Type: unix.NFTA_CHAIN_TABLE, Data: []byte("picketx\x00")}})
	if e != nil {
		return observation{}, e
	}
	for _, m := range msgs {
		f, owned, e := ownedFields(m, unix.NFTA_CHAIN_TABLE)
		if e != nil {
			return observation{}, e
		}
		if !owned {
			continue
		}
		if e = allowedFields(f, unix.NFTA_CHAIN_TABLE, unix.NFTA_CHAIN_NAME, unix.NFTA_CHAIN_HANDLE, unix.NFTA_CHAIN_USE, unix.NFTA_CHAIN_PAD, nftaChainFlags, unix.NFTA_CHAIN_HOOK, unix.NFTA_CHAIN_TYPE, unix.NFTA_CHAIN_POLICY); e != nil {
			return observation{}, e
		}
		name := string(f[unix.NFTA_CHAIN_NAME])
		if !strings.HasSuffix(name, "\x00") || strings.ContainsRune(name[:len(name)-1], 0) {
			return observation{}, fmt.Errorf("invalid chain name")
		}
		name = strings.TrimSuffix(name, "\x00")
		if _, ok := live.chains[name]; ok {
			return observation{}, fmt.Errorf("duplicate image chain")
		}
		live.chains[name] = f
	}
	// Dump all rules of the table, rather than only known chains. This also
	// detects an unexpected rule attributed to a missing/foreign chain.
	msgs, e = r.Dump(ctx, unix.NFT_MSG_GETRULE, []nl.Attribute{{Type: unix.NFTA_RULE_TABLE, Data: []byte("picketx\x00")}})
	if e != nil {
		return observation{}, e
	}
	for _, m := range msgs {
		f, owned, e := ownedFields(m, unix.NFTA_RULE_TABLE)
		if e != nil {
			return observation{}, e
		}
		if !owned {
			continue
		}
		if e = allowedFields(f, unix.NFTA_RULE_TABLE, unix.NFTA_RULE_CHAIN, unix.NFTA_RULE_HANDLE, unix.NFTA_RULE_EXPRESSIONS, unix.NFTA_RULE_USERDATA, unix.NFTA_RULE_POSITION, unix.NFTA_RULE_PAD); e != nil {
			return observation{}, e
		}
		name := strings.TrimSuffix(string(f[unix.NFTA_RULE_CHAIN]), "\x00")
		if _, ok := live.chains[name]; !ok {
			return observation{}, fmt.Errorf("rule has unknown chain")
		}
		live.rules[name] = append(live.rules[name], f)
	}
	for _, cmd := range []uint16{unix.NFT_MSG_GETSET, unix.NFT_MSG_GETOBJ, nftables.NFT_MSG_GETFLOWTABLE} {
		msgs, e = r.Dump(ctx, cmd, []nl.Attribute{{Type: 1, Data: []byte("picketx\x00")}})
		if e != nil {
			return observation{}, e
		}
		for _, m := range msgs {
			_, owned, e := ownedFields(m, 1)
			if e != nil {
				return observation{}, e
			}
			if owned {
				return observation{}, fmt.Errorf("unexpected set/object/flowtable in admission image")
			}
		}
	}
	after, e := generation(ctx, r)
	if e != nil {
		return observation{}, e
	}
	if after != before {
		return observation{}, fmt.Errorf("nft generation changed during inspection")
	}
	var mismatch error
	for _, im := range images {
		layout, e := im.Layout(marker)
		if e != nil {
			return observation{}, e
		}
		if e = live.matches(layout); e == nil {
			return observation{Exists: true, Marker: marker, Image: im.Digest()}, nil
		}
		mismatch = e
	}
	// Pending first application may still have the exact marker-only previous
	// image. Recovery, rather than the inspector, decides if it was expected.
	owner := &nftables.Chain{Name: ownerChain, Table: tab}
	layout := hostrules.Layout{Table: tab, Chains: []*nftables.Chain{owner}, Rules: []*nftables.Rule{{Table: tab, Chain: owner, UserData: []byte(marker), Exprs: []expr.Any{&expr.Counter{}}}}}
	if e = live.matches(layout); e == nil {
		return observation{Exists: true, Marker: marker}, nil
	}
	return observation{}, fmt.Errorf("unrecognized admission table: %w", mismatch)
}
func (live rawImage) matches(layout hostrules.Layout) error {
	if len(live.chains) != len(layout.Chains) {
		return fmt.Errorf("chain count differs")
	}
	for _, ch := range layout.Chains {
		actual, ok := live.chains[ch.Name]
		if !ok {
			return fmt.Errorf("expected chain missing")
		}
		expected := map[uint16][]byte{}
		for _, a := range chainAttributes(ch) {
			expected[a.Type&attributeTypeMask] = a.Data
		}
		for _, key := range []uint16{unix.NFTA_CHAIN_HOOK, unix.NFTA_CHAIN_POLICY, unix.NFTA_CHAIN_TYPE} {
			got, g := actual[key]
			want, w := expected[key]
			if g != w {
				return fmt.Errorf("chain semantics absent/unexpected")
			}
			if key == unix.NFTA_CHAIN_HOOK {
				if g {
					if e := compareAttributes(got, want); e != nil {
						return e
					}
				}
			} else if !bytes.Equal(got, want) {
				return fmt.Errorf("chain scope/policy/type differs")
			}
		}
		if flags, ok := actual[nftaChainFlags]; ok {
			want := uint32(0)
			if ch.Hooknum != nil {
				want = 1
			}
			if len(flags) != 4 || binary.BigEndian.Uint32(flags) != want {
				return fmt.Errorf("chain flags differ")
			}
		}
		var rules []*nftables.Rule
		for _, r := range layout.Rules {
			if r.Chain.Name == ch.Name {
				rules = append(rules, r)
			}
		}
		if len(live.rules[ch.Name]) != len(rules) {
			return fmt.Errorf("rule count differs in %s", ch.Name)
		}
		for i, r := range rules {
			a := live.rules[ch.Name][i]
			if !bytes.Equal(a[unix.NFTA_RULE_USERDATA], r.UserData) {
				return fmt.Errorf("rule identity differs in %s", ch.Name)
			}
			if e := compareRawExpressions(a[unix.NFTA_RULE_EXPRESSIONS], r.Exprs); e != nil {
				return fmt.Errorf("chain %s rule %d: %w", ch.Name, i, e)
			}
		}
	}
	return nil
}
