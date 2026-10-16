// SPDX-License-Identifier: GPL-3.0-or-later
package markstate

import (
	"github.com/PicketX/PicketX/pkg/networkplan"
	"math"
	"reflect"
	"strings"
	"testing"
)

func target(id string) Target {
	return Target{Key: networkplan.TargetKey{AgentRef: "node", ContextID: "ctx", Entry: networkplan.EntryRef{ResourceName: id, EndpointName: "https"}}, Shape: strings.Repeat("a", 64)}
}
func reconcile(t *testing.T, previous *State, r Range, targets []Target, occupied ...uint32) State {
	t.Helper()
	s, e := Reconcile(previous, r, targets, occupied)
	if e != nil {
		t.Fatal(e)
	}
	if len(s.Entries) == 0 {
		t.Fatal("missing assignments")
	}
	if e = s.Validate(); e != nil {
		t.Fatal(e)
	}
	return s
}
func active(s State, id string) uint32 {
	for _, a := range s.Entries {
		if a.Target.Key.Entry.ResourceName == id && !a.Retired {
			return a.Mark
		}
	}
	return 0
}
func TestMarkLifecycle(t *testing.T) {
	r := Range{100, 104}
	a, b := target("a"), target("b")
	first := reconcile(t, nil, r, []Target{b, a})
	if active(first, "a") != 100 || active(first, "b") != 101 {
		t.Fatal("allocation not deterministic", first)
	}
	same := reconcile(t, &first, r, []Target{a, b}, 100, 101, 9000)
	if !reflect.DeepEqual(first, same) {
		t.Fatal("stable targets rotated", same)
	}
	retired := reconcile(t, &first, r, []Target{b}, 100)
	if !retired.Entries[0].Retired || active(retired, "b") != 101 || first.Entries[0].Retired {
		t.Fatal("retirement changed history/input", retired, first)
	}
	again := reconcile(t, &retired, r, []Target{a, b}, 100)
	if active(again, "a") != 102 {
		t.Fatal("recreated target reused mark", again)
	}
	b.Shape = strings.Repeat("b", 64)
	changed := reconcile(t, &again, r, []Target{a, b}, 100, 101)
	if active(changed, "b") != 103 || !changed.Entries[1].Retired {
		t.Fatal("physical revision inherited stale authority", changed)
	}
	empty, e := Reconcile(&changed, r, nil, []uint32{100, 103})
	if e != nil {
		t.Fatal(e)
	}
	for _, a := range empty.Entries {
		if !a.Retired {
			t.Fatal("withdrawal retained active mark")
		}
	}
}
func TestMarkCollisionAndValidation(t *testing.T) {
	r := Range{100, 104}
	a := target("a")
	for _, mark := range []uint32{100, 104} {
		s, e := Reconcile(nil, r, []Target{a}, []uint32{mark})
		if e == nil || len(s.Entries) != 0 {
			t.Fatal("occupied fresh reservation accepted", mark, s, e)
		}
	}
	first := reconcile(t, nil, r, []Target{a}, 0, 99, 105)
	if _, e := Reconcile(&first, r, []Target{a}, []uint32{104}); e == nil {
		t.Fatal("far unused reserved collision accepted")
	}
	if _, e := Reconcile(&first, Range{100, 105}, []Target{a}, nil); e == nil {
		t.Fatal("reservation changed")
	}
	for _, bad := range []Range{{0, 10}, {5, 4}, {1, 4097}, {1, math.MaxUint32}} {
		if bad.Validate() == nil {
			t.Fatal("invalid range accepted", bad)
		}
	}
	for _, change := range []func(*Target){func(a *Target) { a.Key.AgentRef = "" }, func(a *Target) { a.Key.ContextID = "x\x00" }, func(a *Target) { a.Key.Entry.ResourceName = string([]byte{255}) }, func(a *Target) { a.Key.Entry.EndpointName = strings.Repeat("a", 257) }, func(a *Target) { a.Shape = "a" }, func(a *Target) { a.Shape = strings.Repeat("A", 64) }} {
		bad := a
		change(&bad)
		if bad.Validate() == nil {
			t.Fatal("invalid target accepted", bad)
		}
		if s, e := Reconcile(&first, r, []Target{bad}, nil); e == nil || len(s.Entries) != 0 {
			t.Fatal("invalid request published state", s, e)
		}
	}
	duplicate := a
	duplicate.Shape = strings.Repeat("b", 64)
	if _, e := Reconcile(&first, r, []Target{a, duplicate}, nil); e == nil {
		t.Fatal("duplicate key revisions accepted")
	}
}
func TestMarkExhaustionAndHistory(t *testing.T) {
	r := Range{math.MaxUint32 - 1, math.MaxUint32}
	a, b := target("a"), target("b")
	s := reconcile(t, nil, r, []Target{a, b})
	if active(s, "b") != math.MaxUint32 {
		t.Fatal("u32 boundary changed", s)
	}
	next, e := Reconcile(&s, r, []Target{a, target("c")}, nil)
	if e == nil || len(next.Entries) != 0 || s.Entries[1].Retired {
		t.Fatal("partial state or wrap on exhaustion", next, e, s)
	}
	for _, change := range []func(*State){func(s *State) { s.Entries[0].Mark++ }, func(s *State) { s.Range.First = 0 }, func(s *State) { s.Entries[1].Target = s.Entries[0].Target }, func(s *State) { s.Entries[0].Target.Shape = "bad" }} {
		bad := s
		bad.Entries = append([]Assignment(nil), s.Entries...)
		change(&bad)
		if bad.Validate() == nil {
			t.Fatal("corrupt history accepted", bad)
		}
		if _, e := Reconcile(&bad, r, nil, nil); e == nil {
			t.Fatal("corrupt previous accepted")
		}
	}
}
func TestMarkIsolation(t *testing.T) {
	r := Range{1, 4}
	requests := []Target{target("b"), target("a")}
	s := reconcile(t, nil, r, requests)
	if requests[0].Key.Entry.ResourceName != "b" {
		t.Fatal("requests sorted in place")
	}
	clone := s.Clone()
	clone.Entries[0].Retired = true
	if s.Entries[0].Retired {
		t.Fatal("clone aliases entries")
	}
	next := reconcile(t, &s, r, requests)
	next.Entries[0].Target.Shape = "bad"
	if s.Entries[0].Target.Shape == "bad" {
		t.Fatal("result aliases prior history")
	}
}
