// SPDX-License-Identifier: GPL-3.0-or-later
package planbridge

import (
	"net/netip"
	"testing"

	"github.com/PicketX/PicketX/pkg/networkplan"
)

func fixtureFacts() (Identity, networkplan.ExecutionContext) {
	return Identity{"fixture-authority", "epoch-one", "agent-a", "fixture-netns"}, networkplan.ExecutionContext{
		AgentRef: "agent-a", ProfileRef: "transit", ID: "ordinary-v1", NetworkSpaceID: "fixture-netns",
		Class: networkplan.Transit, Stage: networkplan.OriginalDestination, Handling: networkplan.Graceful,
		Destinations: []netip.Prefix{netip.MustParsePrefix("192.0.2.100/32"), netip.MustParsePrefix("2001:db8:1:0:2::100/128")},
	}
}

// Removing one of these validations admits a scope the fixture cannot enforce.
func TestScopeRejectsUnsupportedFacts(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Identity, *networkplan.ExecutionContext, *Catalog)
	}{
		{"authority", func(i *Identity, _ *networkplan.ExecutionContext, _ *Catalog) { i.AuthorityID = "" }},
		{"epoch", func(i *Identity, _ *networkplan.ExecutionContext, _ *Catalog) { i.Epoch = "" }},
		{"agent", func(i *Identity, _ *networkplan.ExecutionContext, _ *Catalog) { i.AgentRef = "" }},
		{"namespace", func(i *Identity, _ *networkplan.ExecutionContext, _ *Catalog) { i.NamespaceID = "" }},
		{"wrong-agent", func(_ *Identity, c *networkplan.ExecutionContext, _ *Catalog) { c.AgentRef = "other" }},
		{"wrong-namespace", func(_ *Identity, c *networkplan.ExecutionContext, _ *Catalog) { c.NetworkSpaceID = "other" }},
		{"missing-context", func(_ *Identity, c *networkplan.ExecutionContext, _ *Catalog) { c.ID = "" }},
		{"missing-profile", func(_ *Identity, c *networkplan.ExecutionContext, _ *Catalog) { c.ProfileRef = "" }},
		{"host", func(_ *Identity, c *networkplan.ExecutionContext, _ *Catalog) { c.Class = networkplan.HostLocal }},
		{"published", func(_ *Identity, c *networkplan.ExecutionContext, _ *Catalog) { c.Class = networkplan.PublishedIngress }},
		{"post-dnat", func(_ *Identity, c *networkplan.ExecutionContext, _ *Catalog) { c.Stage = "post-dnat" }},
		{"handling", func(_ *Identity, c *networkplan.ExecutionContext, _ *Catalog) { c.Handling = "unknown" }},
		{"empty-boundary", func(_ *Identity, c *networkplan.ExecutionContext, _ *Catalog) { c.Destinations = nil }},
		{"wide-boundary", func(_ *Identity, c *networkplan.ExecutionContext, _ *Catalog) {
			c.Destinations = []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")}
		}},
		{"foreign-host", func(_ *Identity, c *networkplan.ExecutionContext, _ *Catalog) {
			c.Destinations = []netip.Prefix{netip.MustParsePrefix("192.0.2.99/32")}
		}},
		{"noncanonical", func(_ *Identity, c *networkplan.ExecutionContext, _ *Catalog) {
			c.Destinations = []netip.Prefix{netip.MustParsePrefix("192.0.2.100/24")}
		}},
		{"unknown-catalog", func(_ *Identity, _ *networkplan.ExecutionContext, k *Catalog) { *k = 0 }},
		{"wrong-catalog", func(_ *Identity, _ *networkplan.ExecutionContext, k *Catalog) { *k = NAT }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			i, c := fixtureFacts()
			k := Ordinary
			tc.change(&i, &c, &k)
			if got, err := NewScope(i, c, k); err == nil || got != nil {
				t.Fatalf("unsupported scope accepted: %v, %v", got, err)
			}
		})
	}
	for _, mode := range []networkplan.HandlingMode{networkplan.Graceful, networkplan.Strict} {
		i, c := fixtureFacts()
		c.Handling = mode
		if got, err := NewScope(i, c, Ordinary); err != nil || got == nil {
			t.Fatal(got, err)
		}
	}
	i, c := fixtureFacts()
	c.Destinations = []netip.Prefix{netip.MustParsePrefix("192.0.2.101/32"), netip.MustParsePrefix("2001:db8:1:0:2::101/128")}
	if got, err := NewScope(i, c, NAT); err != nil || got == nil {
		t.Fatal(got, err)
	}
}

// Retaining the caller's slice would let it change a previously validated boundary.
func TestScopeOwnsFacts(t *testing.T) {
	i, c := fixtureFacts()
	s, err := NewScope(i, c, Ordinary)
	if err != nil {
		t.Fatal(err)
	}
	c.Destinations[0] = netip.MustParsePrefix("0.0.0.0/0")
	if s.context.Destinations[0] != netip.MustParsePrefix("192.0.2.100/32") {
		t.Fatal("scope boundary aliases caller data")
	}
}
