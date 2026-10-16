// SPDX-License-Identifier: GPL-3.0-or-later
package hostowner

import (
	"context"
	"errors"
	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	nl "github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"
	"testing"
)

func TestOwnershipNetlinkErrorsFailClosed(t *testing.T) {
	boom := errors.New("interrupted dump")
	c, err := nftables.New(nftables.WithTestDial(func([]nl.Message) ([]nl.Message, error) { return nil, boom }))
	if err != nil {
		t.Fatal(err)
	}
	b := &nftKernel{conn: c, reader: dumpFunc(func(context.Context, uint16, []nl.Attribute) ([]nl.Message, error) { return nil, boom })}
	defer b.Close()
	if got, err := b.Observe(context.Background()); !errors.Is(err, boom) || got.Exists {
		t.Fatalf("dump error became absence/ownership: %+v %v", got, err)
	}
}
func TestOwnershipCreateIsExclusive(t *testing.T) {
	found := false
	boom := errors.New("transport failure")
	c, err := nftables.New(nftables.WithTestDial(func(msgs []nl.Message) ([]nl.Message, error) {
		for _, m := range msgs {
			if uint16(m.Header.Type) == uint16((unix.NFNL_SUBSYS_NFTABLES<<8)|unix.NFT_MSG_NEWTABLE) {
				found = true
				if m.Header.Flags&nl.Excl == 0 || m.Header.Flags&nl.Replace != 0 {
					t.Errorf("nonexclusive table mutation: %v", m.Header.Flags)
				}
			}
		}
		return nil, boom
	}))
	if err != nil {
		t.Fatal(err)
	}
	b := &nftKernel{conn: c}
	defer b.Close()
	r := scopeRecord(Config{ProviderID: "p", AgentRef: "a", RuntimeKey: "r"}, namespace{BootID: "b", Device: 1, Inode: 1})
	r.OwnerID, _ = randomID()
	r.Epoch, _ = randomID()
	if err = b.Create(context.Background(), markerFor(r)); err == nil || !found {
		t.Fatalf("create not sent or transport failure hidden: %v found=%v", err, found)
	}
}

func TestOwnershipMarkerShape(t *testing.T) {
	r := scopeRecord(Config{ProviderID: "p", AgentRef: "a", RuntimeKey: "r"}, namespace{BootID: "b", Device: 1, Inode: 2})
	r.OwnerID = "0123456789abcdef0123456789abcdef"
	r.Epoch = r.OwnerID
	m := markerFor(r)
	for _, which := range []string{"valid", "hook", "type", "chain-name", "extra-chain", "extra-rule", "verdict", "marker", "table-flags", "set", "object", "flowtable"} {
		t.Run(which, func(t *testing.T) {
			tab := &nftables.Table{Name: "picketx", Family: nftables.TableFamilyINet}
			ch := &nftables.Chain{Name: "picketx__owner", Table: tab}
			chains := []*nftables.Chain{ch}
			rule := &nftables.Rule{Table: tab, Chain: ch, UserData: []byte(m), Exprs: []expr.Any{&expr.Counter{Packets: 99}}}
			rules := []*nftables.Rule{rule}
			sets, objects, flows := 0, 0, 0
			switch which {
			case "hook":
				ch.Hooknum = nftables.ChainHookInput
			case "type":
				ch.Type = nftables.ChainTypeFilter
			case "chain-name":
				ch.Name = "other"
			case "extra-chain":
				chains = append(chains, &nftables.Chain{Name: "extra", Table: tab})
			case "extra-rule":
				rules = append(rules, rule)
			case "verdict":
				rule.Exprs = []expr.Any{&expr.Verdict{Kind: expr.VerdictAccept}}
			case "marker":
				rule.UserData = []byte("foreign")
			case "table-flags":
				tab.Flags = 1
			case "set":
				sets = 1
			case "object":
				objects = 1
			case "flowtable":
				flows = 1
			}
			got, err := inspectShape(tab, chains, rules, sets, objects, flows)
			if which == "valid" {
				if err != nil || got.Marker != m || !got.Exists {
					t.Fatalf("valid marker: %+v %v", got, err)
				}
			} else if err == nil {
				t.Fatal("unexpected structure accepted")
			}
		})
	}
}
