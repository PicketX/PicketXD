// SPDX-License-Identifier: GPL-3.0-or-later
package planbridge

import (
	"net/netip"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/PicketX/PicketX/pkg/core"
	"github.com/PicketX/PicketX/pkg/networkplan"
	"github.com/PicketX/PicketXD/internal/validation/fixture"
)

func at(hhmm string) time.Time {
	v, err := time.Parse(time.RFC3339, "2026-10-15T"+hhmm+":00Z")
	if err != nil {
		panic(err)
	}
	return v
}
func prefix(v string) netip.Prefix { return netip.MustParsePrefix(v) }
func grant(id, source, entry, end string) core.Grant {
	g := core.Grant{ID: id, Source: prefix(source), Scopes: []core.Scope{{ResourceName: "gitea"}}, Validity: core.Validity{Mode: core.LifetimeBounded, ExpiresAt: at(end)}}
	if entry != "" {
		g.Scopes[0].Endpoints = core.EndpointSelection{Mode: core.SelectionNames, Names: []string{entry}}
	}
	return g
}
func compilerInput() networkplan.Input {
	id, c := fixtureFacts()
	in := networkplan.Input{
		AuthorityID: id.AuthorityID,
		Logical: core.Snapshot{Resources: []core.Resource{{Name: "gitea", Endpoints: []core.Endpoint{{Name: "https"}, {Name: "ssh"}}}}, Grants: []core.Grant{
			grant("exact", "192.0.2.10/32", "https", "10:00"),
			grant("wide", "192.0.2.0/26", "", "11:00"),
			grant("v6", "2001:db8:1:0:1::10/128", "", "11:00"),
			grant("ssh", "192.0.2.10/32", "ssh", "12:00"),
		}},
		Networks: []networkplan.NetworkDefinition{{Entry: networkplan.EntryRef{ResourceName: "gitea", EndpointName: "https"}, Protocol: networkplan.TCP, Port: 443}, {Entry: networkplan.EntryRef{ResourceName: "gitea", EndpointName: "ssh"}, Protocol: networkplan.TCP, Port: 22}},
		Agents:   []networkplan.Agent{{Name: id.AgentRef, Capabilities: &networkplan.CapabilityProfile{Classes: []networkplan.TrafficClass{networkplan.Transit}, Protocols: []networkplan.Protocol{networkplan.TCP, networkplan.UDP}, Families: []networkplan.Family{networkplan.IPv4, networkplan.IPv6}, Handling: []networkplan.HandlingMode{networkplan.Graceful, networkplan.Strict}, OriginalDestination: true}}},
		Contexts: []networkplan.ExecutionContext{c},
		Bindings: []networkplan.Binding{{ID: "binding-a", Resources: []networkplan.ResourceSelection{{ResourceName: "gitea", Endpoints: core.EndpointSelection{Mode: core.SelectionNames, Names: []string{"https", "ssh"}}}}, Agents: []networkplan.AgentSelection{{AgentRef: id.AgentRef}}, Context: networkplan.ContextRequest{ProfileRef: c.ProfileRef, Class: c.Class, Stage: c.Stage, Handling: c.Handling, AllowedDestinations: slices.Clone(c.Destinations)}}},
	}
	d := grant("exact", "192.0.2.10/32", "https", "10:30")
	d.Validity.NotBefore = at("09:30")
	in.Logical.Denies = []core.Deny{core.Deny(d)}
	return in
}
func scopeOK(t *testing.T, in networkplan.Input, catalog Catalog) *Scope {
	t.Helper()
	id, _ := fixtureFacts()
	s, e := NewScope(id, in.Contexts[0], catalog)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func prepareOK(t *testing.T, s *Scope, previous *Prepared, in networkplan.Input) *Prepared {
	t.Helper()
	p, e := Prepare(s, previous, in)
	if e != nil || p == nil {
		t.Fatalf("Prepare: %v %v", p, e)
	}
	return p
}
func materializeOK(t *testing.T, p *Prepared, when time.Time) Materialized {
	t.Helper()
	v, e := p.Materialize(when)
	if e != nil {
		t.Fatal(e)
	}
	return v
}

// Catch protocol/port rewrites, merged deadlines, lost bindings and conflated ID domains.
func TestMaterializeIndependentDualStackMembership(t *testing.T) {
	in := compilerInput()
	b := in.Bindings[0]
	b.ID = "binding-b"
	in.Bindings = append(in.Bindings, b)
	p := prepareOK(t, scopeOK(t, in, Ordinary), nil, in)
	v := materializeOK(t, p, at("09:45"))
	want := []fixture.Endpoint{
		{Destination: prefix("192.0.2.100/32"), Protocol: 6, PortFirst: 443, PortLast: 443},
		{Destination: prefix("2001:db8:1:0:2::100/128"), Protocol: 6, PortFirst: 443, PortLast: 443},
		{Destination: prefix("192.0.2.100/32"), Protocol: 6, PortFirst: 22, PortLast: 22},
		{Destination: prefix("2001:db8:1:0:2::100/128"), Protocol: 6, PortFirst: 22, PortLast: 22},
	}
	if !reflect.DeepEqual(v.Snapshot.Protected, want) || len(v.Snapshot.Allow) != 6 || len(v.Snapshot.Deny) != 1 || len(v.Provenance) != 7 || len(v.Inventory) != 2 {
		t.Fatalf("wrong materialized fixture: %+v", v)
	}
	if err := fixture.Validate(v.Snapshot); err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, m := range v.Provenance {
		if !reflect.DeepEqual(m.BindingIDs, []string{"binding-a", "binding-b"}) || m.AuthorityID != "fixture-authority" || m.Target.AgentRef != "agent-a" || m.Target.ContextID != "ordinary-v1" || m.Revoked {
			t.Fatalf("lost provenance: %+v", m)
		}
		end := map[string]time.Time{"exact": at("10:00"), "wide": at("11:00"), "v6": at("11:00"), "ssh": at("12:00")}[m.ID]
		if m.Domain == DenyDomain {
			end = at("10:30")
		}
		wantSource := map[string]netip.Prefix{"exact": prefix("192.0.2.10/32"), "wide": prefix("192.0.2.0/26"), "v6": prefix("2001:db8:1:0:1::10/128"), "ssh": prefix("192.0.2.10/32")}[m.ID]
		if m.Rule.Source != wantSource {
			t.Fatalf("source range changed: %+v", m)
		}
		if !m.Validity.ExpiresAt.Equal(end) || !m.Rule.ExpiresAt.Equal(end) {
			t.Fatalf("deadline changed: %+v", m)
		}
		if m.Rule.Source.Addr().Is4() != m.Rule.Endpoint.Destination.Addr().Is4() {
			t.Fatal("cross-family member")
		}
		counts[string(m.Domain)+":"+m.ID]++
	}
	if !reflect.DeepEqual(counts, map[string]int{"grant:exact": 1, "grant:wide": 2, "grant:v6": 2, "grant:ssh": 1, "deny:exact": 1}) {
		t.Fatal(counts)
	}
	if v.Identity != (Identity{"fixture-authority", "epoch-one", "agent-a", "fixture-netns"}) || !v.At.Equal(at("09:45")) {
		t.Fatal(v.Identity, v.At)
	}

	t.Run("UDP", func(t *testing.T) {
		u := compilerInput()
		u.Logical.Resources[0].Endpoints = []core.Endpoint{{Name: "echo"}}
		u.Logical.Grants = []core.Grant{grant("udp", "192.0.2.10/32", "", "10:00")}
		u.Logical.Denies = nil
		u.Networks = []networkplan.NetworkDefinition{{Entry: networkplan.EntryRef{ResourceName: "gitea", EndpointName: "echo"}, Protocol: networkplan.UDP, Port: 8002}}
		u.Bindings[0].Resources[0].Endpoints = core.EndpointSelection{}
		v := materializeOK(t, prepareOK(t, scopeOK(t, u, Ordinary), nil, u), at("09:00"))
		if len(v.Snapshot.Protected) != 2 || len(v.Snapshot.Allow) != 1 {
			t.Fatal(v)
		}
		for _, e := range v.Snapshot.Protected {
			if e.Protocol != 17 || e.PortFirst != 8002 || e.PortLast != 8002 {
				t.Fatal(e)
			}
		}
	})
	t.Run("NAT-original", func(t *testing.T) {
		n := compilerInput()
		n.Contexts[0].ID = "nat-v1"
		n.Contexts[0].ProfileRef = "nat"
		n.Contexts[0].Destinations = []netip.Prefix{prefix("192.0.2.101/32"), prefix("2001:db8:1:0:2::101/128")}
		n.Bindings[0].Context.ProfileRef = "nat"
		n.Bindings[0].Context.AllowedDestinations = slices.Clone(n.Contexts[0].Destinations)
		n.Networks[0].Port = 8000
		n.Networks[1].Port = 8001
		v := materializeOK(t, prepareOK(t, scopeOK(t, n, NAT), nil, n), at("09:00"))
		if len(v.Snapshot.Protected) != 4 {
			t.Fatal(v)
		}
		for i, e := range v.Snapshot.Protected {
			port := uint16(8000)
			if i >= 2 {
				port = 8001
			}
			dst := prefix("192.0.2.101/32")
			if i%2 == 1 {
				dst = prefix("2001:db8:1:0:2::101/128")
			}
			if e.Protocol != 6 || e.PortFirst != port || e.PortLast != port || e.Destination != dst {
				t.Fatalf("original tuple rewritten: %+v", e)
			}
		}
	})
}

// Catch inclusive ends, lost future boundaries, revocation leakage and bounded-to-permanent conversion.
func TestMaterializeLifetimeBoundaries(t *testing.T) {
	in := compilerInput()
	active := grant("start", "192.0.2.10/32", "https", "10:00")
	active.Validity.NotBefore = at("09:00")
	permanent := grant("permanent", "2001:db8:1:0:1::10/128", "ssh", "12:00")
	permanent.Validity = core.Validity{Mode: core.LifetimePermanent}
	future := grant("future", "192.0.2.10/32", "ssh", "11:00")
	future.Validity.NotBefore = at("09:30")
	expired := grant("expired", "192.0.2.10/32", "https", "09:00")
	revoked := grant("revoked", "192.0.2.10/32", "https", "10:00")
	revoked.Revoked = true
	in.Logical.Grants = []core.Grant{active, permanent, future, expired, revoked}
	deny := future
	deny.ID = "future-deny"
	deny.Source = prefix("2001:db8:1:0:1::10/128")
	deny.Validity.ExpiresAt = at("10:00")
	end := expired
	end.ID = "ended-deny"
	in.Logical.Denies = []core.Deny{core.Deny(deny), core.Deny(end)}
	p := prepareOK(t, scopeOK(t, in, Ordinary), nil, in)
	if _, e := p.Materialize(time.Time{}); e == nil {
		t.Fatal("zero evaluation time accepted")
	}
	for _, tc := range []struct {
		when        string
		allow, deny int
		next        string
	}{{"09:00", 2, 0, "09:30"}, {"09:30", 3, 1, "10:00"}, {"10:00", 2, 0, "11:00"}, {"11:00", 1, 0, ""}} {
		v := materializeOK(t, p, at(tc.when))
		next := time.Time{}
		if tc.next != "" {
			next = at(tc.next)
		}
		if len(v.Snapshot.Protected) != 4 || len(v.Snapshot.Allow) != tc.allow || len(v.Snapshot.Deny) != tc.deny || !v.NextChange.Equal(next) {
			t.Fatalf("%s: %+v", tc.when, v)
		}
		var gs, ds int
		for _, i := range v.Inventory {
			gs += len(i.Grants)
			ds += len(i.Denies)
		}
		if gs != 5 || ds != 2 {
			t.Fatal("inactive membership lost from inventory", gs, ds)
		}
		for _, m := range v.Provenance {
			if m.Rule.ExpiresAt.IsZero() != (m.ID == "permanent") {
				t.Fatal("invalid lifetime conversion", m)
			}
		}
	}
	base := compilerInput()
	v := materializeOK(t, prepareOK(t, scopeOK(t, base, Ordinary), nil, base), at("12:00"))
	if len(v.Snapshot.Protected) != 4 || len(v.Snapshot.Allow) != 0 || len(v.Snapshot.Deny) != 0 {
		t.Fatal("last expiry removed protection", v)
	}
}

func rejectPrepare(t *testing.T, s *Scope, previous *Prepared, in networkplan.Input) {
	t.Helper()
	p, e := Prepare(s, previous, in)
	if e == nil || p != nil {
		t.Fatalf("invalid candidate accepted: %v %v", p, e)
	}
}

// Catch silently dropped context fields, excessive selected tuples, or validation limited to bound entries.
func TestPrepareRejectsUnsupportedTargets(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*networkplan.Input)
	}{
		{"authority", func(i *networkplan.Input) { i.AuthorityID = "other" }},
		{"port", func(i *networkplan.Input) { i.Networks[0].Port = 8443 }},
		{"protocol", func(i *networkplan.Input) { i.Networks[0].Protocol = networkplan.UDP }},
		{"host-class", func(i *networkplan.Input) {
			i.Contexts[0].Class = networkplan.HostLocal
			i.Bindings[0].Context.Class = networkplan.HostLocal
			i.Agents[0].Capabilities.Classes = []networkplan.TrafficClass{networkplan.HostLocal}
		}},
		{"published-class", func(i *networkplan.Input) {
			i.Contexts[0].Class = networkplan.PublishedIngress
			i.Bindings[0].Context.Class = networkplan.PublishedIngress
			i.Agents[0].Capabilities.Classes = []networkplan.TrafficClass{networkplan.PublishedIngress}
		}},
		{"stage", func(i *networkplan.Input) {
			i.Contexts[0].Stage = "post-dnat"
			i.Bindings[0].Context.Stage = "post-dnat"
		}},
		{"profile", func(i *networkplan.Input) {
			i.Contexts[0].ProfileRef = "other"
			i.Bindings[0].Context.ProfileRef = "other"
		}},
		{"context-id", func(i *networkplan.Input) { i.Contexts[0].ID = "other" }},
		{"namespace", func(i *networkplan.Input) { i.Contexts[0].NetworkSpaceID = "other" }},
		{"handling", func(i *networkplan.Input) {
			i.Contexts[0].Handling = networkplan.Strict
			i.Bindings[0].Context.Handling = networkplan.Strict
		}},
		{"wide-boundary", func(i *networkplan.Input) {
			i.Contexts[0].Destinations = []netip.Prefix{prefix("192.0.2.0/24")}
			i.Bindings[0].Context.AllowedDestinations = slices.Clone(i.Contexts[0].Destinations)
		}},
		{"unknown-agent", func(i *networkplan.Input) { i.Agents = nil; i.Bindings = nil; i.Contexts = nil }},
		{"capability", func(i *networkplan.Input) { i.Agents[0].Capabilities.OriginalDestination = false }},
		{"bounded-zero", func(i *networkplan.Input) { i.Logical.Grants[0].Validity.ExpiresAt = time.Time{} }},
		{"invalid-unbound", func(i *networkplan.Input) {
			i.Logical.Resources[0].Endpoints = append(i.Logical.Resources[0].Endpoints, core.Endpoint{Name: "unbound"})
			i.Networks = append(i.Networks, networkplan.NetworkDefinition{Entry: networkplan.EntryRef{ResourceName: "gitea", EndpointName: "unbound"}, Protocol: networkplan.TCP, Port: 0})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := compilerInput()
			s := scopeOK(t, in, Ordinary)
			tc.change(&in)
			rejectPrepare(t, s, nil, in)
		})
	}
	in := compilerInput()
	rejectPrepare(t, nil, nil, in)
	rejectPrepare(t, &Scope{}, nil, in)
	if _, e := (*Prepared)(nil).Materialize(at("09:00")); e == nil {
		t.Fatal("nil prepared accepted")
	}
	if _, e := (&Prepared{}).Materialize(at("09:00")); e == nil {
		t.Fatal("zero prepared accepted")
	}
}
