// SPDX-License-Identifier: GPL-3.0-or-later
package planbridge

import (
	"net/netip"
	"reflect"
	"slices"
	"sync"
	"testing"

	"github.com/PicketX/PicketX/pkg/networkplan"
)

// Rejected complete replacements must not change the accepted predecessor.
func TestReplacementPreservesAcceptedCandidate(t *testing.T) {
	in := compilerInput()
	s := scopeOK(t, in, Ordinary)
	p := prepareOK(t, s, nil, in)
	before := materializeOK(t, p, at("09:45"))
	for _, tc := range []struct {
		name   string
		change func(*networkplan.Input)
	}{
		{"malformed", func(i *networkplan.Input) { i.Logical.Grants[0].Source = netip.Prefix{} }},
		{"authority", func(i *networkplan.Input) { i.AuthorityID = "another" }},
		{"context", func(i *networkplan.Input) { i.Contexts[0].ID = "changed" }},
		{"context-without-bindings", func(i *networkplan.Input) { i.Contexts[0].ID = "changed"; i.Bindings = nil }},
		{"retained-id-retarget", func(i *networkplan.Input) { i.Networks[0].Addresses = []netip.Prefix{prefix("192.0.2.100/32")} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := compilerInput()
			tc.change(&candidate)
			rejectPrepare(t, s, p, candidate)
			if got := materializeOK(t, p, at("09:45")); !reflect.DeepEqual(got, before) {
				t.Fatal("failed candidate mutated predecessor")
			}
		})
	}
	i, c := fixtureFacts()
	i.Epoch = "epoch-two"
	other, e := NewScope(i, c, Ordinary)
	if e != nil {
		t.Fatal(e)
	}
	rejectPrepare(t, other, p, compilerInput())
	rejectPrepare(t, s, &Prepared{}, compilerInput())
	withTwo := compilerInput()
	b := withTwo.Bindings[0]
	b.ID = "binding-b"
	withTwo.Bindings = append(withTwo.Bindings, b)
	two := prepareOK(t, s, p, withTwo)
	remaining := compilerInput()
	remaining.Bindings[0].ID = "binding-b"
	one := prepareOK(t, s, two, remaining)
	v := materializeOK(t, one, at("09:45"))
	if len(v.Snapshot.Protected) != 4 || len(v.Provenance) != 7 || len(v.Withdrawals) != 0 {
		t.Fatal(v)
	}
	for _, m := range v.Provenance {
		if !reflect.DeepEqual(m.BindingIDs, []string{"binding-b"}) {
			t.Fatal(m)
		}
	}
	remaining.Logical.Grants = nil
	emptyGrants := prepareOK(t, s, one, remaining)
	v = materializeOK(t, emptyGrants, at("09:00"))
	if len(v.Snapshot.Allow) != 0 || len(v.Snapshot.Protected) != 4 {
		t.Fatal("lost protection without Grants", v)
	}
}

// Removed Agent requires explicit complete replacement evidence, not an ignored query error.
func TestRemovedAgentRequiresReplacementEvidence(t *testing.T) {
	in := compilerInput()
	s := scopeOK(t, in, Ordinary)
	p := prepareOK(t, s, nil, in)
	removed := compilerInput()
	removed.Agents = nil
	removed.Contexts = nil
	removed.Bindings = nil
	rejectPrepare(t, s, nil, removed)
	next := prepareOK(t, s, p, removed)
	v := materializeOK(t, next, at("09:45"))
	if !v.AgentRemoved || len(v.Snapshot.Protected) != 0 || len(v.Snapshot.Allow) != 0 || len(v.Snapshot.Deny) != 0 || len(v.Provenance) != 0 || len(v.Inventory) != 0 || len(v.Withdrawals) != 2 || !v.NextChange.IsZero() {
		t.Fatalf("removal not explicit: %+v", v)
	}
	for _, w := range v.Withdrawals {
		if w.Reason != "removed" || w.Former.Protection.Key.AgentRef != "agent-a" || len(w.Former.Protection.Matches) != 2 || len(w.Former.Grants) != 3 || !reflect.DeepEqual(w.Former.Protection.BindingIDs, []string{"binding-a"}) {
			t.Fatalf("lost former target: %+v", w)
		}
	}
	again := prepareOK(t, s, next, removed)
	v = materializeOK(t, again, at("10:00"))
	if !v.AgentRemoved || len(v.Withdrawals) != 0 || len(v.Snapshot.Protected) != 0 {
		t.Fatal(v)
	}
	back := prepareOK(t, s, again, compilerInput())
	v = materializeOK(t, back, at("09:45"))
	if v.AgentRemoved || len(v.Snapshot.Protected) != 4 {
		t.Fatal(v)
	}
	bad := compilerInput()
	bad.Bindings[0].Agents[0].AgentRef = "unknown"
	rejectPrepare(t, s, p, bad)
	// An accepted known-but-unbound empty state also permits subsequent Agent removal.
	unbound := compilerInput()
	unbound.Bindings = nil
	empty := prepareOK(t, s, p, unbound)
	v = materializeOK(t, empty, at("09:45"))
	if v.AgentRemoved || len(v.Withdrawals) != 2 || len(v.Snapshot.Protected) != 0 {
		t.Fatal(v)
	}
	v = materializeOK(t, prepareOK(t, s, empty, removed), at("09:45"))
	if !v.AgentRemoved {
		t.Fatal(v)
	}
}

// A narrowed request is valid, while withdrawals from unrelated Agents never become this scope's output.
func TestNarrowedTargetsAndScopedWithdrawals(t *testing.T) {
	in := compilerInput()
	s := scopeOK(t, in, Ordinary)
	for i := range in.Networks {
		in.Networks[i].Addresses = []netip.Prefix{prefix("192.0.2.100/32")}
	}
	p := prepareOK(t, s, nil, in)
	v := materializeOK(t, p, at("09:45"))
	if len(v.Snapshot.Protected) != 2 || len(v.Snapshot.Allow) != 4 {
		t.Fatal(v)
	}
	for _, e := range v.Snapshot.Protected {
		if e.Destination != prefix("192.0.2.100/32") {
			t.Fatal(e)
		}
	}
	full := compilerInput()
	b := full.Agents[0]
	b.Name = "agent-b"
	full.Agents = append(full.Agents, b)
	ctx := full.Contexts[0]
	ctx.AgentRef = "agent-b"
	ctx.NetworkSpaceID = "other-fixture"
	full.Contexts = append(full.Contexts, ctx)
	full.Bindings[0].Agents = append(full.Bindings[0].Agents, networkplan.AgentSelection{AgentRef: "agent-b"})
	multi := prepareOK(t, s, nil, full)
	full.Bindings = nil
	v = materializeOK(t, prepareOK(t, s, multi, full), at("09:45"))
	if len(v.Withdrawals) != 2 {
		t.Fatal("wrong scope withdrawals", v.Withdrawals)
	}
	for _, w := range v.Withdrawals {
		if w.Former.Protection.Key.AgentRef != "agent-a" {
			t.Fatal("foreign withdrawal", w)
		}
	}
}

func damage(v Materialized) {
	if len(v.Snapshot.Protected) > 0 {
		v.Snapshot.Protected[0].Destination = netip.Prefix{}
	}
	if len(v.Snapshot.Allow) > 0 {
		v.Snapshot.Allow[0].Source = netip.Prefix{}
	}
	if len(v.Snapshot.Deny) > 0 {
		v.Snapshot.Deny[0].ExpiresAt = at("12:00")
	}
	if len(v.Provenance) > 0 {
		v.Provenance[0].BindingIDs[0] = "changed"
		v.Provenance[0].ID = "changed"
		v.Provenance[0].Rule.Source = netip.Prefix{}
	}
	mutate := func(i *networkplan.TargetInventory) {
		i.Protection.BindingIDs[0] = "changed"
		i.Protection.Context.Destinations[0] = netip.Prefix{}
		i.Protection.Matches[0].Port = 1
		i.Protection.Required.Classes[0] = "changed"
		i.Protection.Required.Protocols[0] = "changed"
		i.Protection.Required.Families[0] = 99
		i.Protection.Required.Handling[0] = "changed"
		if len(i.Grants) > 0 {
			i.Grants[0].Source = netip.Prefix{}
		}
		if len(i.Denies) > 0 {
			i.Denies[0].ID = "changed"
		}
	}
	if len(v.Inventory) > 0 {
		mutate(&v.Inventory[0])
	}
	if len(v.Withdrawals) > 0 {
		mutate(&v.Withdrawals[0].Former)
		v.Withdrawals[0].Reason = "changed"
	}
}

// Catch shared caller input, sibling result slices and concurrent-reader aliasing.
func TestPreparedOutputsOwnData(t *testing.T) {
	in := compilerInput()
	s := scopeOK(t, in, Ordinary)
	p := prepareOK(t, s, nil, in)
	want := materializeOK(t, p, at("09:45"))
	in.Logical.Grants[0].ID = "changed"
	in.Logical.Resources[0].Endpoints[0].Name = "changed"
	in.Networks[0].Port = 1
	in.Contexts[0].Destinations[0] = netip.Prefix{}
	in.Bindings[0].Agents[0].AgentRef = "changed"
	in.Bindings[0].Context.AllowedDestinations[0] = netip.Prefix{}
	in.Agents[0].Capabilities.Protocols[0] = "changed"
	damage(materializeOK(t, p, at("09:45")))
	if got := materializeOK(t, p, at("09:45")); !reflect.DeepEqual(got, want) {
		t.Fatal("input/result mutated candidate")
	}
	// Provenance and inventory within one returned result are also independent.
	v := materializeOK(t, p, at("09:45"))
	v.Provenance[0].BindingIDs[0] = "changed"
	if !reflect.DeepEqual(v.Inventory, want.Inventory) {
		t.Fatal("sidecar aliases returned inventory")
	}
	removed := compilerInput()
	removed.Agents = nil
	removed.Contexts = nil
	removed.Bindings = nil
	next := prepareOK(t, s, p, removed)
	emptyWant := materializeOK(t, next, at("09:45"))
	if len(emptyWant.Withdrawals) != 2 {
		t.Fatal("withdrawal evidence missing")
	}
	damage(materializeOK(t, next, at("09:45")))
	if got := materializeOK(t, next, at("09:45")); !reflect.DeepEqual(got, emptyWant) {
		t.Fatal("withdrawal aliases candidate")
	}
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			for _, item := range []struct {
				p    *Prepared
				want Materialized
			}{{p, want}, {next, emptyWant}} {
				v, e := item.p.Materialize(at("09:45"))
				if e != nil || !reflect.DeepEqual(v, item.want) {
					t.Error("concurrent result changed", e)
					return
				}
				damage(v)
			}
		})
	}
	wg.Wait()
	if !slices.Equal(s.context.Destinations, []netip.Prefix{prefix("192.0.2.100/32"), prefix("2001:db8:1:0:2::100/128")}) {
		t.Fatal("scope corrupted")
	}
}
