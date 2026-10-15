package planbridge

import (
	"net/netip"
	"reflect"
	"testing"
	"time"

	"github.com/PicketX/PicketX/pkg/core"
	"github.com/PicketX/PicketX/pkg/networkplan"
)

var testAt = time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)

func testFacts(mode networkplan.HandlingMode) Facts {
	return Facts{AuthorityID: "fixture-authority", Epoch: "epoch-1", NamespaceID: "ns-123", Context: networkplan.ExecutionContext{AgentRef: "agent-1", ProfileRef: "fixture", ID: "transit-1", NetworkSpaceID: "ns-123", Class: networkplan.Transit, Stage: networkplan.OriginalDestination, Destinations: []netip.Prefix{prefix("192.0.2.100/32"), prefix("2001:db8:1:0:2::100/128")}, Handling: mode}}
}
func testInput(mode networkplan.HandlingMode) networkplan.Input {
	f := testFacts(mode)
	cap := networkplan.CapabilityProfile{Classes: []networkplan.TrafficClass{networkplan.Transit}, Protocols: []networkplan.Protocol{networkplan.TCP, networkplan.UDP}, Families: []networkplan.Family{networkplan.IPv4, networkplan.IPv6}, Handling: []networkplan.HandlingMode{mode}, OriginalDestination: true}
	in := networkplan.Input{AuthorityID: f.AuthorityID, Logical: core.Snapshot{Resources: []core.Resource{{Name: "gitea", Endpoints: []core.Endpoint{{Name: "https"}, {Name: "ssh"}, {Name: "udp"}}}}}, Agents: []networkplan.Agent{{Name: f.Context.AgentRef, Capabilities: &cap}}, Contexts: []networkplan.ExecutionContext{f.Context}, Networks: []networkplan.NetworkDefinition{{Entry: networkplan.EntryRef{ResourceName: "gitea", EndpointName: "https"}, Protocol: networkplan.TCP, Port: 443}, {Entry: networkplan.EntryRef{ResourceName: "gitea", EndpointName: "ssh"}, Protocol: networkplan.TCP, Port: 22}, {Entry: networkplan.EntryRef{ResourceName: "gitea", EndpointName: "udp"}, Protocol: networkplan.UDP, Port: 8002}}}
	in.Bindings = []networkplan.Binding{{ID: "binding-a", Resources: []networkplan.ResourceSelection{{ResourceName: "gitea"}}, Agents: []networkplan.AgentSelection{{AgentRef: f.Context.AgentRef}}, Context: networkplan.ContextRequest{ProfileRef: f.Context.ProfileRef, Class: f.Context.Class, Stage: f.Context.Stage, AllowedDestinations: f.Context.Destinations, Handling: mode}}}
	for i, src := range []string{"192.0.2.10/32", "2001:db8:1:0:1::10/128"} {
		in.Logical.Grants = append(in.Logical.Grants, core.Grant{ID: []string{"v4", "v6"}[i], Source: prefix(src), Scopes: []core.Scope{{ResourceName: "gitea"}}, Validity: core.Validity{Mode: core.LifetimeBounded, ExpiresAt: testAt.Add(time.Hour)}})
	}
	return in
}
func compile(t *testing.T, in networkplan.Input) *networkplan.Plan {
	t.Helper()
	p, e := networkplan.Compile(in)
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func lifecycleScope(t *testing.T, mode networkplan.HandlingMode) *Scope {
	t.Helper()
	s, e := scopeFromFacts(testFacts(mode))
	if e != nil {
		t.Fatal(e)
	}
	return s
}

func TestConversionPreservesProvenanceDeadlinesAndFamilies(t *testing.T) {
	in := testInput(networkplan.Strict)
	b := in.Bindings[0]
	b.ID = "binding-b"
	in.Bindings = append(in.Bindings, b)
	deny := core.Deny(in.Logical.Grants[0])
	deny.ID = "v4"
	in.Logical.Denies = []core.Deny{deny} // separate ID domains
	got, e := convertInput(lifecycleScope(t, networkplan.Strict), in, testAt)
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Snapshot.Protected) != 6 || len(got.Snapshot.Allow) != 6 || len(got.Snapshot.Deny) != 3 || len(got.Provenance) != 9 {
		t.Fatalf("%+v", got)
	}
	for _, m := range got.Provenance {
		if m.Rule.Source.Addr().Is4() != m.Rule.Endpoint.Destination.Addr().Is4() || !m.Rule.ExpiresAt.Equal(testAt.Add(time.Hour)) || !reflect.DeepEqual(m.BindingIDs, []string{"binding-a", "binding-b"}) {
			t.Fatal(m)
		}
	}
	got.Provenance[0].BindingIDs[0] = "mutated"
	got.Inventory[0].Protection.BindingIDs[0] = "mutated"
	again, e := convertInput(lifecycleScope(t, networkplan.Strict), in, testAt)
	if e != nil || again.Provenance[0].BindingIDs[0] != "binding-a" {
		t.Fatal(again, e)
	}
}
func TestInactiveInventoryNeverBecomesActive(t *testing.T) {
	in := testInput(networkplan.Graceful)
	for _, v := range []struct {
		name    string
		v       core.Validity
		revoked bool
	}{{"future", core.Validity{Mode: core.LifetimeBounded, NotBefore: testAt.Add(time.Minute), ExpiresAt: testAt.Add(time.Hour)}, false}, {"expired", core.Validity{Mode: core.LifetimeBounded, ExpiresAt: testAt}, false}, {"revoked", core.Validity{Mode: core.LifetimePermanent}, true}} {
		g := in.Logical.Grants[0]
		g.ID = v.name
		g.Validity = v.v
		g.Revoked = v.revoked
		in.Logical.Grants = append(in.Logical.Grants, g)
	}
	in.Logical.Grants[0].Validity = core.Validity{Mode: core.LifetimePermanent}
	got, e := convertInput(lifecycleScope(t, networkplan.Graceful), in, testAt)
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Provenance) != 6 || len(got.Inventory[0].Grants) != 5 {
		t.Fatal(got)
	}
	for _, m := range got.Provenance {
		if m.ID == "v4" && !m.Rule.ExpiresAt.IsZero() {
			t.Fatal(m)
		}
		if m.ID != "v4" && m.ID != "v6" {
			t.Fatal(m)
		}
	}
}
func TestRejectUnprovenScopeAndTuple(t *testing.T) {
	for _, mutate := range []func(*networkplan.Input){func(in *networkplan.Input) { in.AuthorityID = "other" }, func(in *networkplan.Input) { in.Contexts[0].NetworkSpaceID = "other" }, func(in *networkplan.Input) { in.Contexts[0].ID = "other" }, func(in *networkplan.Input) { in.Networks[0].Port = 8443 }, func(in *networkplan.Input) {
		in.Contexts[0].Class = networkplan.HostLocal
		in.Bindings[0].Context.Class = networkplan.HostLocal
		in.Agents[0].Capabilities.Classes = []networkplan.TrafficClass{networkplan.HostLocal}
	}} {
		in := testInput(networkplan.Strict)
		mutate(&in)
		if _, e := convertInput(lifecycleScope(t, networkplan.Strict), in, testAt); e == nil {
			t.Fatal("unsupported projection accepted")
		}
	}
	f := testFacts(networkplan.Strict)
	f.NamespaceID = ""
	if _, e := scopeFromFacts(f); e == nil {
		t.Fatal("missing actual namespace identity accepted")
	}
}

func scopeFromFacts(f Facts) (*Scope, error) {
	catalog := Ordinary
	if f.NAT {
		catalog = NAT
	}
	return NewScope(Identity{AuthorityID: f.AuthorityID, Epoch: f.Epoch, AgentRef: f.Context.AgentRef, NamespaceID: f.NamespaceID}, f.Context, catalog)
}
func convertInput(s *Scope, in networkplan.Input, at time.Time) (Materialized, error) {
	p, e := Prepare(s, nil, in)
	if e != nil {
		return Materialized{}, e
	}
	return p.Materialize(at)
}
