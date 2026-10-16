// SPDX-License-Identifier: GPL-3.0-or-later
package hostrules

import (
	"encoding/binary"
	"encoding/json"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/PicketX/PicketX/pkg/core"
	"github.com/PicketX/PicketX/pkg/networkplan"
	"github.com/PicketX/PicketXD/internal/hostpolicy"
	"github.com/PicketX/PicketXD/internal/markstate"
	"github.com/google/nftables/expr"
)

func policy(t *testing.T) *hostpolicy.Policy {
	t.Helper()
	c := networkplan.ExecutionContext{AgentRef: "node", ProfileRef: "host", ID: "ctx", NetworkSpaceID: "space", Class: networkplan.HostLocal, Stage: networkplan.OriginalDestination, Handling: networkplan.Graceful, Destinations: []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32"), netip.MustParsePrefix("::1/128")}}
	v := core.Validity{Mode: core.LifetimeBounded, NotBefore: time.Unix(2000000000, 0).UTC(), ExpiresAt: time.Unix(2000000060, 0).UTC()}
	in := networkplan.Input{AuthorityID: "static", Logical: core.Snapshot{Resources: []core.Resource{{Name: "uid", Endpoints: []core.Endpoint{{Name: "web"}}}}, Grants: []core.Grant{{ID: "future", Source: netip.MustParsePrefix("127.0.0.0/8"), Scopes: []core.Scope{{ResourceName: "uid"}}, Validity: v}, {ID: "revoked", Source: netip.MustParsePrefix("::1/128"), Scopes: []core.Scope{{ResourceName: "uid"}}, Validity: core.Validity{Mode: core.LifetimePermanent}, Revoked: true}}, Denies: []core.Deny{{ID: "future-deny", Source: netip.MustParsePrefix("127.0.0.1/32"), Scopes: []core.Scope{{ResourceName: "uid"}}, Validity: v}}}, Networks: []networkplan.NetworkDefinition{{Entry: networkplan.EntryRef{ResourceName: "uid", EndpointName: "web"}, Protocol: networkplan.TCP, Port: 8443}}, Agents: []networkplan.Agent{{Name: "node", Capabilities: &networkplan.CapabilityProfile{Classes: []networkplan.TrafficClass{networkplan.HostLocal}, Protocols: []networkplan.Protocol{networkplan.TCP, networkplan.UDP}, Families: []networkplan.Family{networkplan.IPv4, networkplan.IPv6}, Handling: []networkplan.HandlingMode{networkplan.Graceful}, OriginalDestination: true}}}, Contexts: []networkplan.ExecutionContext{c}, Bindings: []networkplan.Binding{{ID: "binding", Resources: []networkplan.ResourceSelection{{ResourceName: "uid"}}, Agents: []networkplan.AgentSelection{{AgentRef: "node"}}, Context: networkplan.ContextRequest{ProfileRef: "host", Class: c.Class, Stage: c.Stage, Handling: c.Handling, AllowedDestinations: c.Destinations}}}}
	plan, e := networkplan.Compile(in)
	if e != nil {
		t.Fatal(e)
	}
	scope, e := hostpolicy.NewScope("static", c)
	if e != nil {
		t.Fatal(e)
	}
	p, e := hostpolicy.Compile(plan, scope)
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func built(t *testing.T) Image {
	t.Helper()
	p := policy(t)
	targets, e := p.MarkTargets()
	if e != nil {
		t.Fatal(e)
	}
	state, e := markstate.Reconcile(nil, markstate.Range{First: 100, Last: 107}, targets, nil)
	if e != nil {
		t.Fatal(e)
	}
	im, e := Build(p, state)
	if e != nil {
		t.Fatal(e)
	}
	return im
}
func TestImageCompleteAndIndependent(t *testing.T) {
	im := built(t)
	if im.Version != 1 || im.AuthorityID != "static" || len(im.Targets) != 1 || im.Targets[0].Mark != 100 || len(im.Targets[0].Allow) != 1 || len(im.Targets[0].Deny) != 1 {
		t.Fatal(im)
	}
	if im.Targets[0].Allow[0].ID != "future" || !im.Targets[0].Allow[0].Validity.NotBefore.Equal(time.Unix(2000000000, 0)) {
		t.Fatal("future member omitted")
	}
	data, _ := json.Marshal(im)
	var restored Image
	if e := json.Unmarshal(data, &restored); e != nil {
		t.Fatal(e)
	}
	if e := restored.Validate(); e != nil || restored.Digest() != im.Digest() {
		t.Fatal(e)
	}
	p := policy(t)
	targets, _ := p.MarkTargets()
	state, _ := markstate.Reconcile(nil, markstate.Range{First: 100, Last: 107}, targets, nil)
	state.Entries[0].Target.Shape = strings.Repeat("f", 64)
	if _, e := Build(p, state); e == nil {
		t.Fatal("forged shape accepted")
	}
	state.Entries[0].Retired = true
	if _, e := Build(p, state); e == nil {
		t.Fatal("retired mark accepted")
	}
}
func TestImageRejectsInvalidInput(t *testing.T) {
	for _, which := range []string{"version", "zero-mark", "duplicate", "bad-prefix", "bad-port", "bad-family", "bad-date", "bad-context", "unprotected-member", "bad-validity"} {
		t.Run(which, func(t *testing.T) {
			im := built(t)
			switch which {
			case "version":
				im.Version = 99
			case "zero-mark":
				im.Targets[0].Mark = 0
			case "duplicate":
				im.Targets = append(im.Targets, im.Targets[0])
			case "bad-prefix":
				im.Targets[0].Protected[0].Destination = netip.Prefix{}
			case "bad-port":
				im.Targets[0].Protected[0].Port = 0
			case "bad-family":
				im.Targets[0].Allow[0].Source = netip.MustParsePrefix("::1/128")
			case "bad-date":
				im.Targets[0].Allow[0].Validity.ExpiresAt = time.Date(2500, 1, 1, 0, 0, 0, 0, time.UTC)
			case "bad-context":
				im.Context.Handling = networkplan.Strict
			case "unprotected-member":
				im.Targets[0].Allow[0].Endpoint.Port++
			case "bad-validity":
				im.Targets[0].Allow[0].Validity.Mode = 99
			}
			if im.Validate() == nil {
				t.Fatal("invalid image accepted")
			}
		})
	}
}
func TestRulesProgramAbsoluteBoundaries(t *testing.T) {
	im := built(t)
	layout, e := im.Layout("marker")
	if e != nil {
		t.Fatal(e)
	}
	var before, after, mark, ctDst, kernelClock bool
	for _, rule := range layout.Rules {
		for _, x := range rule.Exprs {
			switch v := x.(type) {
			case *expr.Meta:
				kernelClock = kernelClock || uint32(v.Key) == 30
			case *expr.Cmp:
				if len(v.Data) == 8 {
					n := binary.BigEndian.Uint64(v.Data)
					before = before || v.Op == expr.CmpOpGte && n == 2000000000000000000
					after = after || v.Op == expr.CmpOpLt && n == 2000000060000000000
				}
			case *expr.Immediate:
				if len(v.Data) == 4 && binary.NativeEndian.Uint32(v.Data) == 100 {
					mark = true
				}
			case *expr.Ct:
				ctDst = ctDst || v.Key == expr.CtKeyDSTIP && v.Direction == expr.CtDirOriginal
			}
		}
	}
	if !before || !after || !mark || !ctDst || !kernelClock {
		t.Fatalf("missing native contract %v %v %v %v", before, after, mark, ctDst)
	}
	if len(layout.Chains) != 3 {
		t.Fatal("owner, input and target chains required", len(layout.Chains))
	}
}
func TestImageLimits(t *testing.T) {
	im := built(t)
	im.Targets = make([]Target, 4097)
	if im.Validate() == nil {
		t.Fatal("target bound ignored")
	}
	im = built(t)
	for i := 0; i < 9000; i++ {
		im.Targets[0].Allow = append(im.Targets[0].Allow, im.Targets[0].Allow[0])
	}
	if im.Validate() == nil {
		t.Fatal("rule bound ignored")
	}
}
func TestEscapedImageByteBound(t *testing.T) {
	im := built(t)
	m := im.Targets[0].Allow[0]
	m.ID = strings.Repeat("<", 256)
	im.Targets[0].Allow = nil
	for i := 0; i < 6000; i++ {
		im.Targets[0].Allow = append(im.Targets[0].Allow, m)
	}
	if e := im.Validate(); e == nil || !strings.Contains(e.Error(), "serialized") {
		t.Fatal("escaped image byte bound missing", e)
	}
}
