// SPDX-License-Identifier: GPL-3.0-or-later
package hostpolicy

import (
	"github.com/PicketX/PicketX/pkg/core"
	"github.com/PicketX/PicketX/pkg/networkplan"
	"net/netip"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func prefix(s string) netip.Prefix { return netip.MustParsePrefix(s) }
func instant(s string) time.Time {
	v, e := time.Parse(time.RFC3339, "2026-10-16T"+s+":00Z")
	if e != nil {
		panic(e)
	}
	return v
}
func contextFacts() networkplan.ExecutionContext {
	return networkplan.ExecutionContext{AgentRef: "agent-a", ProfileRef: "host", ID: "context-1", NetworkSpaceID: "boot/netns", Class: networkplan.HostLocal, Stage: networkplan.OriginalDestination, Handling: networkplan.Graceful, Destinations: []netip.Prefix{prefix("203.0.113.0/24"), prefix("fd66::/64")}}
}
func input() networkplan.Input {
	c := contextFacts()
	return networkplan.Input{AuthorityID: "static-owner", Logical: core.Snapshot{Resources: []core.Resource{{Name: "resource-uid", Endpoints: []core.Endpoint{{Name: "web"}, {Name: "dns"}}}}, Grants: []core.Grant{
		{ID: "same-id", Source: prefix("198.51.100.7/32"), Scopes: []core.Scope{{ResourceName: "resource-uid"}}, Validity: core.Validity{Mode: core.LifetimeBounded, NotBefore: instant("09:00"), ExpiresAt: instant("10:00")}},
		{ID: "v6", Source: prefix("fd77::/80"), Scopes: []core.Scope{{ResourceName: "resource-uid"}}, Validity: core.Validity{Mode: core.LifetimePermanent}},
	}}, Networks: []networkplan.NetworkDefinition{{Entry: networkplan.EntryRef{ResourceName: "resource-uid", EndpointName: "web"}, Protocol: networkplan.TCP, Port: 8443}, {Entry: networkplan.EntryRef{ResourceName: "resource-uid", EndpointName: "dns"}, Protocol: networkplan.UDP, Port: 5353}}, Agents: []networkplan.Agent{{Name: "agent-a", Capabilities: &networkplan.CapabilityProfile{Classes: []networkplan.TrafficClass{networkplan.HostLocal, networkplan.Transit, networkplan.PublishedIngress}, Protocols: []networkplan.Protocol{networkplan.TCP, networkplan.UDP}, Families: []networkplan.Family{networkplan.IPv4, networkplan.IPv6}, Handling: []networkplan.HandlingMode{networkplan.Graceful, networkplan.Strict}, OriginalDestination: true}}}, Contexts: []networkplan.ExecutionContext{c}, Bindings: []networkplan.Binding{{ID: "binding-uid", Resources: []networkplan.ResourceSelection{{ResourceName: "resource-uid"}}, Agents: []networkplan.AgentSelection{{AgentRef: "agent-a"}}, Context: networkplan.ContextRequest{ProfileRef: c.ProfileRef, Class: c.Class, Stage: c.Stage, AllowedDestinations: c.Destinations, Handling: c.Handling}}}}
}
func shared(t *testing.T, in networkplan.Input) *networkplan.Plan {
	t.Helper()
	p, e := networkplan.Compile(in)
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func scope(t *testing.T) *Scope {
	t.Helper()
	s, e := NewScope("static-owner", contextFacts())
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func compiled(t *testing.T, in networkplan.Input) *Policy {
	t.Helper()
	p, e := Compile(shared(t, in), scope(t))
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func evaluated(t *testing.T, p *Policy, at time.Time) Snapshot {
	t.Helper()
	s, e := p.Evaluate(at)
	if e != nil {
		t.Fatal(e)
	}
	return s
}

// A fixture catalog or lost protocol conversion would reject these arbitrary tuples.
func TestDualStackArbitraryTargets(t *testing.T) {
	in := input()
	deny := in.Logical.Grants[0]
	deny.Validity.ExpiresAt = instant("09:30")
	in.Logical.Denies = []core.Deny{core.Deny(deny)}
	s := evaluated(t, compiled(t, in), instant("09:00"))
	if s.Version != 1 || s.AuthorityID != "static-owner" || s.Context.ID != "context-1" || !s.At.Equal(instant("09:00")) || len(s.Targets) != 2 {
		t.Fatalf("lost identity/targets: %+v", s)
	}
	for _, target := range s.Targets {
		if target.Key.Entry.ResourceName != "resource-uid" || target.Key.AgentRef != "agent-a" || !reflect.DeepEqual(target.BindingIDs, []string{"binding-uid"}) || len(target.Protected) != 2 || len(target.Allow) != 2 || len(target.Deny) != 1 {
			t.Fatalf("lost provenance/union: %+v", target)
		}
		protocol, port := uint8(6), uint16(8443)
		if target.Key.Entry.EndpointName == "dns" {
			protocol, port = 17, 5353
		}
		for _, e := range target.Protected {
			if e.Protocol != protocol || e.Port != port {
				t.Fatal("rewritten tuple", e)
			}
		}
		for _, m := range target.Allow {
			if m.ID == "same-id" && (!m.Validity.ExpiresAt.Equal(instant("10:00")) || m.Source != prefix("198.51.100.7/32")) {
				t.Fatal(m)
			}
		}
		if target.Deny[0].ID != "same-id" || !target.Deny[0].Validity.ExpiresAt.Equal(instant("09:30")) {
			t.Fatal("deny domain merged", target.Deny)
		}
	}
	if !s.NextChange.Equal(instant("09:30")) {
		t.Fatal(s.NextChange)
	}
}
func TestLifetimeAndInventory(t *testing.T) {
	in := input()
	in.Logical.Grants = in.Logical.Grants[:1]
	d := in.Logical.Grants[0]
	d.Validity.NotBefore = instant("09:30")
	d.Validity.ExpiresAt = instant("11:00")
	in.Logical.Denies = []core.Deny{core.Deny(d)}
	revoked := in.Logical.Grants[0]
	revoked.ID = "revoked"
	revoked.Revoked = true
	in.Logical.Grants = append(in.Logical.Grants, revoked)
	p := compiled(t, in)
	for _, tc := range []struct {
		at          string
		allow, deny int
		next        string
	}{{"08:00", 0, 0, "09:00"}, {"09:00", 1, 0, "09:30"}, {"09:30", 1, 1, "10:00"}, {"10:00", 0, 1, "11:00"}, {"11:00", 0, 0, ""}} {
		s := evaluated(t, p, instant(tc.at))
		if len(s.Targets) != 2 {
			t.Fatal("protection disappeared", s)
		}
		for _, target := range s.Targets {
			if len(target.Protected) != 2 || len(target.Allow) != tc.allow || len(target.Deny) != tc.deny {
				t.Fatalf("%s: %+v", tc.at, target)
			}
		}
		next := time.Time{}
		if tc.next != "" {
			next = instant(tc.next)
		}
		if !s.NextChange.Equal(next) {
			t.Fatal(tc.at, s.NextChange, next)
		}
	}
	inventory, e := p.Inventory()
	if e != nil || len(inventory) != 2 {
		t.Fatal(inventory, e)
	}
	for _, target := range inventory {
		if len(target.Grants) != 2 || len(target.Denies) != 1 {
			t.Fatal("inactive/revoked inventory lost", target)
		}
	}
	if _, e := p.Evaluate(time.Time{}); e == nil {
		t.Fatal("zero time accepted")
	}
	if _, e := (*Policy)(nil).Evaluate(instant("09:00")); e == nil {
		t.Fatal("nil policy accepted")
	}
	if _, e := (&Policy{}).Inventory(); e == nil {
		t.Fatal("zero policy accepted")
	}
}
func TestScopeRejections(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*networkplan.ExecutionContext)
	}{
		{"empty-agent", func(c *networkplan.ExecutionContext) { c.AgentRef = "" }}, {"empty-namespace", func(c *networkplan.ExecutionContext) { c.NetworkSpaceID = "" }}, {"invalid-utf8", func(c *networkplan.ExecutionContext) { c.ID = string([]byte{255}) }}, {"oversized", func(c *networkplan.ExecutionContext) { c.ID = strings.Repeat("a", 257) }}, {"nul", func(c *networkplan.ExecutionContext) { c.ProfileRef = "host\x00" }},
		{"transit", func(c *networkplan.ExecutionContext) { c.Class = networkplan.Transit }}, {"published", func(c *networkplan.ExecutionContext) { c.Class = networkplan.PublishedIngress }}, {"unknown-handling", func(c *networkplan.ExecutionContext) { c.Handling = "unknown" }}, {"default-handling", func(c *networkplan.ExecutionContext) { c.Handling = "" }}, {"stage", func(c *networkplan.ExecutionContext) { c.Stage = "post-dnat" }}, {"no-bounds", func(c *networkplan.ExecutionContext) { c.Destinations = nil }}, {"host-bits", func(c *networkplan.ExecutionContext) { c.Destinations[0] = prefix("203.0.113.1/24") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := contextFacts()
			tc.change(&c)
			if s, e := NewScope("static-owner", c); e == nil || s != nil {
				t.Fatalf("invalid scope accepted: %v %v", s, e)
			}
		})
	}
	if s, e := NewScope("", contextFacts()); e == nil || s != nil {
		t.Fatal("empty authority accepted")
	}
	if _, e := Compile(nil, scope(t)); e == nil {
		t.Fatal("nil plan accepted")
	}
	if _, e := Compile(&networkplan.Plan{}, scope(t)); e == nil {
		t.Fatal("zero plan accepted")
	}
	if _, e := Compile(shared(t, input()), nil); e == nil {
		t.Fatal("nil scope accepted")
	}
	if _, e := Compile(shared(t, input()), &Scope{}); e == nil {
		t.Fatal("zero scope accepted")
	}
}
func TestSelectedTargetRejections(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*networkplan.Input)
	}{
		{"authority", func(in *networkplan.Input) { in.AuthorityID = "other" }}, {"namespace", func(in *networkplan.Input) { in.Contexts[0].NetworkSpaceID = "other" }}, {"context", func(in *networkplan.Input) { in.Contexts[0].ID = "other" }}, {"profile", func(in *networkplan.Input) {
			in.Contexts[0].ProfileRef = "other"
			in.Bindings[0].Context.ProfileRef = "other"
		}}, {"strict", func(in *networkplan.Input) {
			in.Contexts[0].Handling = networkplan.Strict
			in.Bindings[0].Context.Handling = networkplan.Strict
		}}, {"transit", func(in *networkplan.Input) {
			in.Contexts[0].Class = networkplan.Transit
			in.Bindings[0].Context.Class = networkplan.Transit
		}}, {"published", func(in *networkplan.Input) {
			in.Contexts[0].Class = networkplan.PublishedIngress
			in.Bindings[0].Context.Class = networkplan.PublishedIngress
		}}, {"widened", func(in *networkplan.Input) {
			in.Contexts[0].Destinations = []netip.Prefix{prefix("203.0.112.0/23")}
			in.Bindings[0].Context.AllowedDestinations = in.Contexts[0].Destinations
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := input()
			in.Logical.Grants = nil
			tc.change(&in)
			if p, e := Compile(shared(t, in), scope(t)); e == nil || p != nil {
				t.Fatalf("dormant unsupported target accepted %v %v", p, e)
			}
		})
	}
	in := input()
	in.Networks[0].Addresses = []netip.Prefix{prefix("203.0.113.99/32")}
	in.Networks[1].Addresses = []netip.Prefix{prefix("203.0.113.99/32")}
	s := evaluated(t, compiled(t, in), instant("09:00"))
	if len(s.Targets) != 2 || s.Targets[0].Protected[0].Destination != prefix("203.0.113.99/32") {
		t.Fatal("narrowing rejected/rewritten", s)
	}
	in = input()
	in.Agents = nil
	in.Contexts = nil
	in.Bindings = nil
	if _, e := Compile(shared(t, in), scope(t)); e == nil {
		t.Fatal("unknown agent accepted")
	}
}
func TestIndependentReturnedData(t *testing.T) {
	facts := contextFacts()
	s, e := NewScope("static-owner", facts)
	if e != nil {
		t.Fatal(e)
	}
	facts.Destinations[0] = prefix("10.0.0.0/8")
	p, e := Compile(shared(t, input()), s)
	if e != nil {
		t.Fatal("scope aliases facts", e)
	}
	original := evaluated(t, p, instant("09:00"))
	if len(original.Targets) != 2 {
		t.Fatal("missing targets before mutation", original)
	}
	mutated := evaluated(t, p, instant("09:00"))
	mutated.Context.Destinations[0] = prefix("10.0.0.0/8")
	mutated.Targets[0].BindingIDs[0] = "changed"
	mutated.Targets[0].Protected[0].Port = 1
	mutated.Targets[0].Allow[0].ID = "changed"
	inv, e := p.Inventory()
	if e != nil {
		t.Fatal(e)
	}
	inv[0].Protection.BindingIDs[0] = "changed"
	inv[0].Protection.Context.Destinations[0] = prefix("10.0.0.0/8")
	inv[0].Protection.Matches[0].Port = 1
	inv[0].Protection.Required.Protocols[0] = "bad"
	inv[0].Grants[0].ID = "changed"
	if got := evaluated(t, p, instant("09:00")); !reflect.DeepEqual(got, original) {
		t.Fatal("returned mutation changed policy", got)
	}
	fresh, _ := p.Inventory()
	if fresh[0].Grants[0].ID == "changed" || fresh[0].Protection.BindingIDs[0] == "changed" {
		t.Fatal("inventory aliases internal state")
	}
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, e := p.Evaluate(instant("09:00"))
			if e != nil || !reflect.DeepEqual(got, original) {
				t.Error("concurrent evaluation changed", e)
			}
		}()
	}
	wg.Wait()
}
