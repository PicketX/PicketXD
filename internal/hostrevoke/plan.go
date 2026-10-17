// SPDX-License-Identifier: GPL-3.0-or-later
// Package hostrevoke scopes cleanup to known owned original tuples. Plans are
// not application receipts or proof that caller-supplied history is complete.
package hostrevoke

import (
	"encoding/json"
	"fmt"
	"github.com/PicketX/PicketXD/internal/hostpolicy"
	"github.com/PicketX/PicketXD/internal/hostrules"
	"github.com/PicketX/PicketXD/internal/markstate"
	"net/netip"
	"reflect"
	"time"
)

const MaxHistoryImages = 32
const MaxPlanBytes = 32 << 20
const MaxFlows = 262144
const FamilyV4 uint8 = 2
const FamilyV6 uint8 = 10

type Tuple struct {
	Source, Destination         netip.Addr
	SourcePort, DestinationPort uint16
	Protocol                    uint8
}
type Flow struct {
	Family          uint8
	Mark            uint32
	Zone            uint16
	ID              [4]byte
	Original, Reply Tuple
}

func (f Flow) Validate() error {
	bits := 32
	if f.Family == FamilyV6 {
		bits = 128
	} else if f.Family != FamilyV4 {
		return fmt.Errorf("unsupported flow family")
	}
	for _, t := range []Tuple{f.Original, f.Reply} {
		for _, a := range []netip.Addr{t.Source, t.Destination} {
			if !a.IsValid() || a.Is4In6() || a.Zone() != "" || a.BitLen() != bits {
				return fmt.Errorf("invalid flow address/family")
			}
		}
		if t.Protocol != 6 && t.Protocol != 17 {
			return fmt.Errorf("unsupported flow protocol")
		}
	}
	if f.Original.Protocol != f.Reply.Protocol {
		return fmt.Errorf("inconsistent tuple protocols")
	}
	return nil
}

type Plan struct {
	current  hostrules.Image
	history  []hostrules.Image
	recorded *History
}

func New(history []hostrules.Image, current hostrules.Image, marks markstate.State) (*Plan, error) {
	return newPlan(history, current, marks, false)
}

func newPlan(history []hostrules.Image, current hostrules.Image, marks markstate.State, retiredCurrent bool) (*Plan, error) {
	if len(history) > MaxHistoryImages {
		return nil, fmt.Errorf("too many history images")
	}
	if e := marks.Validate(); e != nil {
		return nil, e
	}
	if e := current.Validate(); e != nil {
		return nil, e
	}
	p := &Plan{}
	total := 0
	seen := map[string]bool{}
	for i, im := range append([]hostrules.Image{current}, history...) {
		if e := im.Validate(); e != nil {
			return nil, e
		}
		if im.AuthorityID != current.AuthorityID || !reflect.DeepEqual(im.Context, current.Context) {
			return nil, fmt.Errorf("history scope differs from current")
		}
		for _, t := range im.Targets {
			found := false
			for _, a := range marks.Entries {
				if a.Target == t.Identity && a.Mark == t.Mark && (i != 0 || !a.Retired || retiredCurrent) {
					found = true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("image target/mark missing from durable ownership")
			}
		}
		data, e := json.Marshal(im)
		if e != nil {
			return nil, e
		}
		total += len(data)
		if total > MaxPlanBytes {
			return nil, fmt.Errorf("cleanup plan exceeds bound")
		}
		var copy hostrules.Image
		if e = json.Unmarshal(data, &copy); e != nil {
			return nil, e
		}
		if i == 0 {
			p.current = copy
		}
		digest := im.Digest()
		if !seen[digest] {
			p.history = append(p.history, copy)
			seen[digest] = true
		}
	}
	return p, nil
}
func matches(ep hostpolicy.Endpoint, f Flow) bool {
	t := f.Original
	return ep.Protocol == t.Protocol && ep.Port == t.DestinationPort && ep.Destination.Contains(t.Destination)
}
func memberMatches(m hostpolicy.Member, f Flow) bool {
	return m.Source.Contains(f.Original.Source) && matches(m.Endpoint, f)
}
func (p *Plan) withdrawn(f Flow, at time.Time) bool {
	if f.Mark == 0 {
		return false
	}
	owned := false
	for _, im := range p.history {
		for _, t := range im.Targets {
			if t.Mark != f.Mark {
				continue
			}
			for _, m := range t.Allow {
				if memberMatches(m, f) {
					owned = true
					break
				}
			}
		}
	}
	if p.recorded != nil {
		for _, t := range p.recorded.Targets {
			if t.Mark == f.Mark {
				for _, m := range t.Members {
					if m.Source.Contains(f.Original.Source) && matches(m.Endpoint, f) {
						owned = true
						break
					}
				}
			}
		}
	}
	if !owned {
		return false
	}
	for _, t := range p.current.Targets {
		for _, m := range t.Deny {
			if m.Validity.ActiveAt(at) && memberMatches(m, f) {
				return true
			}
		}
	}
	for _, t := range p.current.Targets {
		for _, m := range t.Allow {
			if m.Validity.ActiveAt(at) && memberMatches(m, f) {
				return false
			}
		}
	}
	return true
}
func (p *Plan) Select(flows []Flow, at time.Time) ([]Flow, error) {
	if p == nil || at.IsZero() {
		return nil, fmt.Errorf("validated plan and nonzero time required")
	}
	if len(flows) > MaxFlows {
		return nil, fmt.Errorf("flow snapshot exceeds bound")
	}
	var out []Flow
	seen := map[Flow]bool{}
	for _, f := range flows {
		if e := f.Validate(); e != nil {
			return nil, e
		}
		if seen[f] {
			return nil, fmt.Errorf("duplicate flow observation")
		}
		seen[f] = true
		if p.withdrawn(f, at) {
			out = append(out, f)
		}
	}
	return out, nil
}
func (p *Plan) nextBoundary(at time.Time) time.Time {
	var next time.Time
	for _, t := range p.current.Targets {
		for _, ms := range [][]hostpolicy.Member{t.Allow, t.Deny} {
			for _, m := range ms {
				n := m.Validity.NextBoundary(at)
				if !n.IsZero() && (next.IsZero() || n.Before(next)) {
					next = n
				}
			}
		}
	}
	return next
}
