// SPDX-License-Identifier: GPL-3.0-or-later
package hostrules

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/PicketX/PicketX/pkg/networkplan"
	"github.com/PicketX/PicketXD/internal/hostpolicy"
	"github.com/PicketX/PicketXD/internal/markstate"
	"github.com/google/nftables/expr"
)

func strictPolicy(t *testing.T) *hostpolicy.Policy {
	t.Helper()
	in, e := policy(t).Input()
	if e != nil {
		t.Fatal(e)
	}
	in.Contexts[0].Handling = networkplan.Strict
	in.Bindings[0].Context.Handling = networkplan.Strict
	in.Agents[0].Capabilities.Handling = append(in.Agents[0].Capabilities.Handling, networkplan.Strict)
	scope, e := hostpolicy.NewScope("static", in.Contexts[0])
	if e != nil {
		t.Fatal(e)
	}
	p, e := hostpolicy.CompileInput(in, scope)
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func strictImage(t *testing.T) (Image, markstate.State) {
	t.Helper()
	p := strictPolicy(t)
	targets, e := p.MarkTargets()
	if e != nil {
		t.Fatal(e)
	}
	marks, e := markstate.Reconcile(nil, markstate.Range{First: 100, Last: 107}, targets, nil)
	if e != nil {
		t.Fatal(e)
	}
	old := marks.Entries[0]
	old.Target.Shape = strings.Repeat("f", 64)
	old.Mark = 101
	old.Retired = true
	foreign := old
	foreign.Mark = 102
	foreign.Target.Key.ContextID = "foreign"
	marks.Entries = append(marks.Entries, old, foreign)
	im, e := Build(p, marks)
	if e != nil {
		t.Fatal(e)
	}
	return im, marks
}
func TestStrictImageAndLedger(t *testing.T) {
	im, marks := strictImage(t)
	if im.Version != 2 || !reflect.DeepEqual(im.KnownMarks, []uint32{100, 101}) {
		t.Fatal(im)
	}
	if e := im.ValidateMarks(marks); e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(im)
	for _, mode := range []string{"unknown", "foreign", "zero", "duplicate", "unsorted", "missing current", "too many", "version"} {
		t.Run(mode, func(t *testing.T) {
			var x Image
			json.Unmarshal(raw, &x)
			switch mode {
			case "unknown":
				x.KnownMarks = append(x.KnownMarks, 999)
			case "foreign":
				x.KnownMarks = append(x.KnownMarks, 102)
			case "zero":
				x.KnownMarks[0] = 0
			case "duplicate":
				x.KnownMarks = append(x.KnownMarks, 101)
			case "unsorted":
				x.KnownMarks = []uint32{101, 100}
			case "missing current":
				x.KnownMarks = []uint32{101}
			case "too many":
				x.KnownMarks = make([]uint32, markstate.MaxMarks+1)
			case "version":
				x.Version = 1
			}
			if mode == "unknown" || mode == "foreign" {
				if e := x.Validate(); e != nil {
					t.Fatal("structurally valid alias rejected before ledger validation", e)
				}
			} else if x.Validate() == nil {
				t.Fatal("invalid image accepted", mode)
			}
			if x.ValidateMarks(marks) == nil {
				t.Fatal("unproven alias accepted", mode)
			}
		})
	}
	layout, e := im.Layout("owner")
	if e != nil {
		t.Fatal(e)
	}
	shared, jumps, fresh := 0, 0, 0
	for _, r := range layout.Rules {
		if r.Chain.Name == "picketx_strict_established" {
			shared++
			continue
		}
		if !strings.HasPrefix(r.Chain.Name, "picketx_target_") {
			continue
		}
		hasSource, hasTime, hasJump, hasState, hasWrite := false, false, false, false, false
		for _, x := range r.Exprs {
			switch v := x.(type) {
			case *expr.Ct:
				hasSource = hasSource || v.Key == expr.CtKeySRCIP || v.Key == expr.CtKeySRCIP6
				hasState = hasState || v.Key == expr.CtKeySTATE
				hasWrite = hasWrite || v.Key == expr.CtKeyMARK && v.SourceRegister
			case *expr.Meta:
				hasTime = hasTime || uint32(v.Key) == 30
			case *expr.Verdict:
				hasJump = hasJump || v.Kind == expr.VerdictJump && v.Chain == "picketx_strict_established"
			}
		}
		if hasJump {
			if !hasSource || !hasTime {
				t.Fatal("established path bypasses current membership")
			}
			jumps++
		}
		if hasWrite {
			if !hasSource || !hasTime || !hasState {
				t.Fatal("new TCP path lacks current membership/NEW")
			}
			fresh++
		}
	}
	if shared != 3 || jumps != 1 || fresh != 1 {
		t.Fatal("missing strict shared/permission/new paths", shared, jumps, fresh)
	}
	member := im.Targets[0].Allow[0]
	im.Targets[0].Allow = make([]hostpolicy.Member, MaxRules/2+1)
	for i := range im.Targets[0].Allow {
		im.Targets[0].Allow[i] = member
	}
	if e = im.Validate(); e == nil || !strings.Contains(e.Error(), "rules") {
		t.Fatal("strict doubled rule count not bounded", e)
	}
}
func TestGracefulImageEncodingStable(t *testing.T) {
	im := built(t)
	legacy := struct {
		Version     int
		AuthorityID string
		Context     networkplan.ExecutionContext
		Targets     []Target
		Input       networkplan.Input
	}{im.Version, im.AuthorityID, im.Context, im.Targets, im.Input}
	a, _ := json.Marshal(im)
	b, _ := json.Marshal(legacy)
	if string(a) != string(b) || im.Version != 1 {
		t.Fatal("graceful v1 encoding changed")
	}
}

func TestStrictReplyReconstructionGuard(t *testing.T) {
	im, _ := strictImage(t)
	layout, err := im.Layout("owner")
	if err != nil {
		t.Fatal(err)
	}
	var guarded []hostpolicy.Endpoint
	seenGuard := false
	for _, r := range layout.Rules {
		if r.Chain.Name != "picketx_admission" {
			continue
		}
		var direction bool
		for _, x := range r.Exprs {
			if v, ok := x.(*expr.Ct); ok && v.Key == expr.CtKeyDIRECTION {
				direction = true
			}
		}
		if !direction {
			if seenGuard {
				t.Fatal("original-destination dispatch follows fallback drop")
			}
			continue
		}
		seenGuard = true
		for _, target := range im.Targets {
			for _, ep := range target.Protected {
				expected := append(endpointMatch(ep, false), &expr.Ct{Key: expr.CtKeyDIRECTION, Register: 1}, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{1}}, &expr.Verdict{Kind: expr.VerdictDrop})
				if reflect.DeepEqual(r.Exprs, expected) {
					guarded = append(guarded, ep)
				}
			}
		}
	}
	var expected []hostpolicy.Endpoint
	for _, target := range im.Targets {
		expected = append(expected, target.Protected...)
	}
	if !reflect.DeepEqual(guarded, expected) {
		t.Fatalf("missing exact raw reply guards: got %v want %v", guarded, expected)
	}
	graceful := built(t)
	layout, err = graceful.Layout("owner")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range layout.Rules {
		for _, x := range r.Exprs {
			if v, ok := x.(*expr.Ct); ok && v.Key == expr.CtKeyDIRECTION {
				t.Fatal("graceful gained strict reply guard")
			}
		}
	}
}
