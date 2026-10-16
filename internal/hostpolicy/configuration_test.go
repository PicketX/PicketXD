// SPDX-License-Identifier: GPL-3.0-or-later
package hostpolicy

import (
	"github.com/PicketX/PicketX/pkg/configuration"
	"github.com/PicketX/PicketX/pkg/networkplan"
	"net/netip"
	"os"
	"strings"
	"testing"
)

func TestConfigurationIdentityAndWithdrawal(t *testing.T) {
	raw, e := os.ReadFile("../../examples/prepare/policy.yaml")
	if e != nil {
		t.Fatal(e)
	}
	facts := contextFacts()
	facts.Destinations = []netip.Prefix{prefix("192.0.2.100/32"), prefix("2001:db8::100/128")}
	in := input()
	f := configuration.Facts{AuthorityID: "static-owner", Agents: in.Agents, Contexts: []networkplan.ExecutionContext{facts}}
	b, e := configuration.Decode(raw)
	if e != nil {
		t.Fatal(e)
	}
	c, e := configuration.Compile(b, f)
	if e != nil {
		t.Fatal(e)
	}
	sc, e := NewScope(f.AuthorityID, facts)
	if e != nil {
		t.Fatal(e)
	}
	p, e := Compile(c.Plan(), sc)
	if e != nil {
		t.Fatal(e)
	}
	s := evaluated(t, p, instant("09:00"))
	if len(s.Targets) != 2 || s.Targets[0].Key.Entry.ResourceName != "resource-gitea-v1" || s.Targets[0].BindingIDs[0] != "binding-gitea-v1" {
		t.Fatal("configuration UID lost", s)
	}
	parts := strings.Split(string(raw), "---\n")
	withoutGrants := parts[0] + "---\n" + parts[len(parts)-1]
	b, e = configuration.Decode([]byte(withoutGrants))
	if e != nil {
		t.Fatal(e)
	}
	noGrants, _, e := configuration.Replace(c, b, f)
	if e != nil {
		t.Fatal(e)
	}
	p, e = Compile(noGrants.Plan(), sc)
	if e != nil {
		t.Fatal(e)
	}
	s = evaluated(t, p, instant("09:00"))
	if len(s.Targets) != 2 || len(s.Targets[0].Allow) != 0 {
		t.Fatal("last Grant removed protection", s)
	}
	b, e = configuration.Decode([]byte(parts[0]))
	if e != nil {
		t.Fatal(e)
	}
	empty, _, e := configuration.Replace(noGrants, b, f)
	if e != nil {
		t.Fatal(e)
	}
	p, e = Compile(empty.Plan(), sc)
	if e != nil {
		t.Fatal(e)
	}
	s = evaluated(t, p, instant("09:00"))
	if len(s.Targets) != 0 || s.AuthorityID != "static-owner" {
		t.Fatal("empty desired policy lost identity", s)
	}
	renewed := strings.ReplaceAll(string(raw), "resource-gitea-v1", "resource-gitea-v2")
	b, e = configuration.Decode([]byte(renewed))
	if e != nil {
		t.Fatal(e)
	}
	next, _, e := configuration.Replace(empty, b, f)
	if e != nil {
		t.Fatal(e)
	}
	p, e = Compile(next.Plan(), sc)
	if e != nil {
		t.Fatal(e)
	}
	s = evaluated(t, p, instant("09:00"))
	if s.Targets[0].Key.Entry.ResourceName != "resource-gitea-v2" {
		t.Fatal("new UID reused old identity", s)
	}
}
