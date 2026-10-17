// SPDX-License-Identifier: GPL-3.0-or-later
package hostrevoke

import (
	"github.com/PicketX/PicketX/pkg/configuration"
	"github.com/PicketX/PicketX/pkg/networkplan"
	"github.com/PicketX/PicketXD/internal/hostpolicy"
	"github.com/PicketX/PicketXD/internal/hostrules"
	"github.com/PicketX/PicketXD/internal/markstate"
	"net/netip"
	"os"
	"strings"
	"testing"
	"time"
)

func rawPolicy(t *testing.T) string {
	t.Helper()
	b, e := os.ReadFile("../../examples/enforce/policy.yaml")
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func testImage(t *testing.T, raw string, prior *markstate.State) (hostrules.Image, markstate.State) {
	t.Helper()
	c := networkplan.ExecutionContext{AgentRef: "node", ProfileRef: "host", ID: "ctx", NetworkSpaceID: "space", Class: networkplan.HostLocal, Stage: networkplan.OriginalDestination, Handling: networkplan.Graceful, Destinations: []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32"), netip.MustParsePrefix("::1/128")}}
	f := configuration.Facts{AuthorityID: "static", Contexts: []networkplan.ExecutionContext{c}, Agents: []networkplan.Agent{{Name: "node", Capabilities: &networkplan.CapabilityProfile{Classes: []networkplan.TrafficClass{c.Class}, Protocols: []networkplan.Protocol{networkplan.TCP, networkplan.UDP}, Families: []networkplan.Family{networkplan.IPv4, networkplan.IPv6}, Handling: []networkplan.HandlingMode{c.Handling}, OriginalDestination: true}}}}
	b, e := configuration.Decode([]byte(raw))
	if e != nil {
		t.Fatal(e)
	}
	cand, e := configuration.Compile(b, f)
	if e != nil {
		t.Fatal(e)
	}
	sc, e := hostpolicy.NewScope("static", c)
	if e != nil {
		t.Fatal(e)
	}
	p, e := hostpolicy.CompileInput(cand.Input(), sc)
	if e != nil {
		t.Fatal(e)
	}
	ts, e := p.MarkTargets()
	if e != nil {
		t.Fatal(e)
	}
	marks, e := markstate.Reconcile(prior, markstate.Range{First: 100, Last: 131}, ts, nil)
	if e != nil {
		t.Fatal(e)
	}
	im, e := hostrules.Build(p, marks)
	if e != nil {
		t.Fatal(e)
	}
	return im, marks
}
func noGrant(raw string) string {
	p := strings.Split(raw, "---\n")
	return p[0] + "---\n" + p[len(p)-1]
}
func testFlow(im hostrules.Image, ipv6 bool, protocol uint8) Flow {
	src, dst := netip.MustParseAddr("127.0.0.2"), netip.MustParseAddr("127.0.0.1")
	family := uint8(2)
	if ipv6 {
		src, dst, family = netip.IPv6Loopback(), netip.IPv6Loopback(), 10
	}
	for _, t := range im.Targets {
		for _, ep := range t.Protected {
			if ep.Protocol == protocol && ep.Destination.Contains(dst) {
				return Flow{Family: family, Mark: t.Mark, Zone: 7, ID: [4]byte{1, 2, 3, 4}, Original: Tuple{Source: src, Destination: dst, SourcePort: 30000, DestinationPort: ep.Port, Protocol: protocol}, Reply: Tuple{Source: dst, Destination: src, SourcePort: ep.Port, DestinationPort: 30000, Protocol: protocol}}
			}
		}
	}
	panic("no target")
}
func TestPlanScopesAndImmutable(t *testing.T) {
	old, marks := testImage(t, rawPolicy(t), nil)
	current, marks := testImage(t, noGrant(rawPolicy(t)), &marks)
	p, e := New([]hostrules.Image{old}, current, marks)
	if e != nil {
		t.Fatal(e)
	}
	var all []Flow
	for _, v6 := range []bool{false, true} {
		for _, proto := range []uint8{6, 17} {
			f := testFlow(old, v6, proto)
			all = append(all, f)
			other := f
			other.Mark = 0
			all = append(all, other)
			other = f
			other.Mark = 999
			all = append(all, other)
			other = f
			other.Original.DestinationPort++
			all = append(all, other)
		}
	}
	other := testFlow(old, false, 6)
	other.Original.Source = netip.MustParseAddr("192.0.2.1")
	other.Reply.Destination = other.Original.Source
	all = append(all, other)
	old.Targets[0].Allow = nil
	current.Targets[0].Allow = nil
	marks.Entries[0].Mark = 999
	got, e := p.Select(all, time.Now())
	if e != nil || len(got) != 4 {
		t.Fatal(len(got), e)
	}
	got[0].Mark = 0
	again, e := p.Select(all, time.Now())
	if e != nil || len(again) != 4 || again[0].Mark == 0 {
		t.Fatal("selection aliases caller", e)
	}
	if _, e = (*Plan)(nil).Select(all, time.Now()); e == nil {
		t.Fatal("nil plan")
	}
	if _, e = p.Select(all, time.Time{}); e == nil {
		t.Fatal("zero instant")
	}
	malformed := testFlow(current, false, 6)
	malformed.Reply.Protocol = 17
	if _, e = p.Select([]Flow{malformed}, time.Now()); e == nil {
		t.Fatal("malformed flow accepted")
	}
}
func TestPlanCurrentUnionDenyAndDeadlines(t *testing.T) {
	raw := rawPolicy(t)
	old, marks := testImage(t, raw, nil)
	at := time.Now().Round(0).UTC()
	start, end := at.Add(time.Hour), at.Add(2*time.Hour)
	future := strings.ReplaceAll(raw, "validity: {mode: permanent}", "validity: {mode: bounded, notBefore: '"+start.Format(time.RFC3339Nano)+"', expiresAt: '"+end.Format(time.RFC3339Nano)+"'}")
	cur, m := testImage(t, future, &marks)
	p, e := New([]hostrules.Image{old}, cur, m)
	if e != nil {
		t.Fatal(e)
	}
	f := testFlow(old, false, 6)
	for _, tc := range []struct {
		at time.Time
		n  int
	}{{at, 1}, {start, 0}, {end.Add(-time.Nanosecond), 0}, {end, 1}} {
		got, e := p.Select([]Flow{f}, tc.at)
		if e != nil || len(got) != tc.n {
			t.Fatal(tc, len(got), e)
		}
	}
	// A new physical incarnation has a fresh mark; its independent allow still
	// preserves flows bearing the retired mark from the previous incarnation.
	empty := "apiVersion: picketx.io/v1alpha1\nkind: Configuration\nmetadata: {name: empty, uid: empty}\nspec: {empty: true}\n"
	_, retired := testImage(t, empty, &marks)
	recreated, newMarks := testImage(t, raw, &retired)
	p, e = New([]hostrules.Image{old}, recreated, newMarks)
	if e != nil {
		t.Fatal(e)
	}
	if f.Mark == testFlow(recreated, false, 6).Mark {
		t.Fatal("test did not retire mark")
	}
	got, e := p.Select([]Flow{f}, at)
	if e != nil || len(got) != 0 {
		t.Fatal("independent current mark allowance lost", got, e)
	}
	parts := strings.Split(raw, "---\n")
	deny := strings.NewReplacer("kind: Grant", "kind: NetworkDeny", "grant-loopback", "deny-loopback").Replace(parts[1])
	withDeny, m := testImage(t, raw+"---\n"+deny, &marks)
	p, e = New([]hostrules.Image{old}, withDeny, m)
	if e != nil {
		t.Fatal(e)
	}
	got, e = p.Select([]Flow{f}, at)
	if e != nil || len(got) != 1 {
		t.Fatal("deny did not win", got, e)
	}
}
func TestPlanRejectsUnprovenHistory(t *testing.T) {
	old, marks := testImage(t, rawPolicy(t), nil)
	cur, _ := testImage(t, noGrant(rawPolicy(t)), &marks)
	for _, edit := range []func(*hostrules.Image, *markstate.State){func(im *hostrules.Image, m *markstate.State) { im.Targets[0].Mark++ }, func(im *hostrules.Image, m *markstate.State) { m.Entries[0].Retired = true }, func(im *hostrules.Image, m *markstate.State) { im.Context.ID = "foreign" }, func(im *hostrules.Image, m *markstate.State) { im.Targets[0].Protected[0].Port++ }} {
		im, m := testImage(t, rawPolicy(t), nil)
		edit(&im, &m)
		if _, e := New([]hostrules.Image{im}, cur, m); e == nil {
			t.Fatal("invalid ownership/history accepted")
		}
	}
	hs := make([]hostrules.Image, MaxHistoryImages+1)
	for i := range hs {
		hs[i] = old
	}
	if _, e := New(hs, cur, marks); e == nil {
		t.Fatal("unbounded history")
	}
}
func replaceValidity(raw string, start, end time.Time) string {
	return strings.ReplaceAll(raw, "validity: {mode: permanent}", "validity: {mode: bounded, notBefore: '"+start.Format(time.RFC3339Nano)+"', expiresAt: '"+end.Format(time.RFC3339Nano)+"'}")
}
