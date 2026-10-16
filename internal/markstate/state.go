// SPDX-License-Identifier: GPL-3.0-or-later
// Package markstate defines desired target mark lifecycles, not kernel receipts.
package markstate

import (
	"cmp"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/PicketX/PicketX/pkg/networkplan"
)

const MaxMarks = 4096

type Range struct {
	First uint32 `json:"first"`
	Last  uint32 `json:"last"`
}
type Target struct {
	Key   networkplan.TargetKey `json:"key"`
	Shape string                `json:"shape"`
}
type Assignment struct {
	Target  Target `json:"target"`
	Mark    uint32 `json:"mark"`
	Retired bool   `json:"retired"`
}
type State struct {
	Range   Range        `json:"range"`
	Entries []Assignment `json:"entries"`
}

func (r Range) Validate() error {
	if r.First == 0 || r.Last < r.First || uint64(r.Last)-uint64(r.First)+1 > MaxMarks {
		return fmt.Errorf("explicit nonzero complete mark range of 1..%d values required", MaxMarks)
	}
	return nil
}
func (t Target) Validate() error {
	for _, id := range []string{t.Key.AgentRef, t.Key.ContextID, t.Key.Entry.ResourceName, t.Key.Entry.EndpointName} {
		if id == "" || len(id) > 256 || !utf8.ValidString(id) || strings.ContainsRune(id, 0) {
			return fmt.Errorf("mark target requires bounded nonempty UTF-8 identities without NUL")
		}
	}
	b, e := hex.DecodeString(t.Shape)
	if e != nil || len(b) != 32 || hex.EncodeToString(b) != t.Shape {
		return fmt.Errorf("mark target shape must be canonical SHA-256 hex")
	}
	return nil
}
func (s State) Validate() error {
	if e := s.Range.Validate(); e != nil {
		return e
	}
	if uint64(len(s.Entries)) > uint64(s.Range.Last)-uint64(s.Range.First)+1 {
		return fmt.Errorf("mark history exceeds reservation")
	}
	active := make(map[networkplan.TargetKey]bool)
	for i, a := range s.Entries {
		if e := a.Target.Validate(); e != nil {
			return e
		}
		if uint64(a.Mark) != uint64(s.Range.First)+uint64(i) {
			return fmt.Errorf("mark history must be contiguous, ordered and never reused")
		}
		if !a.Retired {
			if active[a.Target.Key] {
				return fmt.Errorf("multiple active marks for one target")
			}
			active[a.Target.Key] = true
		}
	}
	return nil
}
func (s State) Clone() State { s.Entries = slices.Clone(s.Entries); return s }
func compareTarget(a, b Target) int {
	x := []string{a.Key.AgentRef, a.Key.ContextID, a.Key.Entry.ResourceName, a.Key.Entry.EndpointName}
	y := []string{b.Key.AgentRef, b.Key.ContextID, b.Key.Entry.ResourceName, b.Key.Entry.EndpointName}
	for i := range x {
		if c := cmp.Compare(x[i], y[i]); c != 0 {
			return c
		}
	}
	return 0
}

// Reconcile returns a complete independent ledger or zero state on error.
// Occupied marks must come from complete observations of both address families.
func Reconcile(previous *State, reservation Range, targets []Target, occupied []uint32) (State, error) {
	if e := reservation.Validate(); e != nil {
		return State{}, e
	}
	out := State{Range: reservation}
	if previous != nil {
		if e := previous.Validate(); e != nil {
			return State{}, e
		}
		if previous.Range != reservation {
			return State{}, fmt.Errorf("mark reservation differs from durable history")
		}
		out = previous.Clone()
	}
	if len(targets) > MaxMarks {
		return State{}, fmt.Errorf("too many desired mark targets")
	}
	desired := make(map[networkplan.TargetKey]Target, len(targets))
	for _, target := range targets {
		if e := target.Validate(); e != nil {
			return State{}, e
		}
		if _, exists := desired[target.Key]; exists {
			return State{}, fmt.Errorf("duplicate desired mark target key")
		}
		desired[target.Key] = target
	}
	known := make(map[uint32]bool, len(out.Entries))
	for _, a := range out.Entries {
		known[a.Mark] = true
	}
	for _, mark := range occupied {
		if mark >= reservation.First && mark <= reservation.Last && !known[mark] {
			return State{}, fmt.Errorf("unallocated reserved conntrack mark %#x is occupied", mark)
		}
	}
	active := make(map[networkplan.TargetKey]bool)
	for i := range out.Entries {
		a := &out.Entries[i]
		if a.Retired {
			continue
		}
		if target, ok := desired[a.Target.Key]; !ok || target != a.Target {
			a.Retired = true
		} else {
			active[a.Target.Key] = true
		}
	}
	ordered := slices.Clone(targets)
	slices.SortFunc(ordered, compareTarget)
	for _, target := range ordered {
		if active[target.Key] {
			continue
		}
		next := uint64(reservation.First) + uint64(len(out.Entries))
		if next > uint64(reservation.Last) {
			return State{}, fmt.Errorf("mark reservation exhausted; retired marks cannot be reused")
		}
		out.Entries = append(out.Entries, Assignment{Target: target, Mark: uint32(next)})
	}
	return out, nil
}
