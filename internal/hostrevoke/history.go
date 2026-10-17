// SPDX-License-Identifier: GPL-3.0-or-later
package hostrevoke

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/PicketX/PicketX/pkg/networkplan"
	"github.com/PicketX/PicketXD/internal/hostpolicy"
	"github.com/PicketX/PicketXD/internal/hostrules"
	"github.com/PicketX/PicketXD/internal/markstate"
	"net/netip"
	"reflect"
	"slices"
	"strings"
)

const MaxHistoryTargets = markstate.MaxMarks
const MaxHistoryMembers = 65536
const MaxHistoryBytes = 8 << 20

// History retains potential historical ownership, without current permission,
// lifetimes, application receipts or a completeness assertion.
type History struct {
	Version     int                          `json:"version"`
	AuthorityID string                       `json:"authorityID"`
	Context     networkplan.ExecutionContext `json:"context"`
	Targets     []HistoricalTarget           `json:"targets"`
}
type HistoricalTarget struct {
	Context   networkplan.ExecutionContext `json:"context"`
	Identity  markstate.Target             `json:"identity"`
	Mark      uint32                       `json:"mark"`
	Protected []hostpolicy.Endpoint        `json:"protected"`
	Members   []HistoricalMember           `json:"members"`
}
type HistoricalMember struct {
	Source   netip.Prefix        `json:"source"`
	Endpoint hostpolicy.Endpoint `json:"endpoint"`
}

func memberKey(m HistoricalMember) string {
	return fmt.Sprintf("%s|%s|%03d|%05d", m.Source, m.Endpoint.Destination, m.Endpoint.Protocol, m.Endpoint.Port)
}
func validPrefix(p netip.Prefix) bool {
	return p.IsValid() && !p.Addr().Is4In6() && p.Addr().Zone() == "" && p == p.Masked()
}
func (h History) validateShape() error {
	if h.Version != 1 || len(h.Targets) > MaxHistoryTargets {
		return fmt.Errorf("invalid/bounded history version/targets")
	}
	if _, e := hostpolicy.NewScope(h.AuthorityID, h.Context); e != nil {
		return e
	}
	normalized, e := networkplan.NormalizeExecutionContext(h.Context)
	if e != nil || !reflect.DeepEqual(normalized, h.Context) {
		return fmt.Errorf("history context not canonical")
	}
	count := 0
	var last uint32
	for _, t := range h.Targets {
		if t.Mark == 0 || t.Mark <= last || len(t.Protected) == 0 {
			return fmt.Errorf("invalid/duplicate history target")
		}
		last = t.Mark
		physical, e := networkplan.NormalizeExecutionContext(t.Context)
		if e != nil || !reflect.DeepEqual(physical, t.Context) {
			return fmt.Errorf("historical physical context not canonical")
		}
		// Physical bounds can be narrower than the trusted runtime scope.
		identityContext := physical
		identityContext.Destinations = h.Context.Destinations
		if !reflect.DeepEqual(identityContext, h.Context) {
			return fmt.Errorf("historical physical context differs from trusted scope")
		}
		for _, destination := range physical.Destinations {
			within := false
			for _, bound := range h.Context.Destinations {
				if bound.Addr().BitLen() == destination.Addr().BitLen() && bound.Bits() <= destination.Bits() && bound.Contains(destination.Addr()) {
					within = true
				}
			}
			if !within {
				return fmt.Errorf("historical physical context exceeds trusted scope")
			}
		}
		if t.Identity.Key.AgentRef != h.Context.AgentRef || t.Identity.Key.ContextID != h.Context.ID {
			return fmt.Errorf("historical target differs from scope")
		}
		eps := map[hostpolicy.Endpoint]bool{}
		var matches []networkplan.Match
		for _, ep := range t.Protected {
			if !validPrefix(ep.Destination) || ep.Port == 0 || (ep.Protocol != 6 && ep.Protocol != 17) || eps[ep] {
				return fmt.Errorf("invalid historical endpoint")
			}
			within := false
			for _, b := range physical.Destinations {
				if b.Addr().BitLen() == ep.Destination.Addr().BitLen() && b.Bits() <= ep.Destination.Bits() && b.Contains(ep.Destination.Addr()) {
					within = true
				}
			}
			if !within {
				return fmt.Errorf("historical endpoint outside context")
			}
			eps[ep] = true
			protocol := networkplan.TCP
			if ep.Protocol == 17 {
				protocol = networkplan.UDP
			}
			matches = append(matches, networkplan.Match{Destination: ep.Destination, Protocol: protocol, Port: ep.Port})
		}
		target, e := hostpolicy.TargetIdentity(t.Identity.Key, physical, matches)
		if e != nil || target != t.Identity {
			return fmt.Errorf("historical physical shape differs from identity")
		}
		prior := ""
		for _, m := range t.Members {
			count++
			if count > MaxHistoryMembers {
				return fmt.Errorf("history member bound exceeded")
			}
			key := memberKey(m)
			if !validPrefix(m.Source) || m.Source.Addr().BitLen() != m.Endpoint.Destination.Addr().BitLen() || !eps[m.Endpoint] || prior != "" && strings.Compare(prior, key) >= 0 {
				return fmt.Errorf("invalid/duplicate historical member")
			}
			prior = key
		}
	}
	data, e := json.Marshal(h)
	if e != nil {
		return e
	}
	if len(data) > MaxHistoryBytes {
		return fmt.Errorf("compact history exceeds byte bound")
	}
	return nil
}
func (h History) Validate(marks markstate.State) error {
	if e := h.validateShape(); e != nil {
		return e
	}
	if e := marks.Validate(); e != nil {
		return e
	}
	known := map[uint32]markstate.Target{}
	for _, a := range marks.Entries {
		known[a.Mark] = a.Target
	}
	for _, t := range h.Targets {
		if known[t.Mark] != t.Identity {
			return fmt.Errorf("historical target/mark absent from durable ledger")
		}
	}
	return nil
}
func (h History) clone() (History, error) {
	data, e := json.Marshal(h)
	if e != nil {
		return History{}, e
	}
	var out History
	e = json.Unmarshal(data, &out)
	return out, e
}

// Remember returns an independent candidate. Callers must publish it only with
// actual live application confirmation, and validate the durable mark ledger.
func Remember(previous *History, im hostrules.Image) (History, error) {
	if e := im.Validate(); e != nil {
		return History{}, e
	}
	out := History{Version: 1, AuthorityID: im.AuthorityID, Context: im.Context}
	var e error
	if previous != nil {
		if e = previous.validateShape(); e != nil {
			return History{}, e
		}
		if previous.AuthorityID != im.AuthorityID || !reflect.DeepEqual(previous.Context, im.Context) {
			return History{}, fmt.Errorf("same-boot history scope migration requires explicit deployment")
		}
		out, e = previous.clone()
		if e != nil {
			return History{}, e
		}
	}
	plan, e := networkplan.Compile(im.Input)
	if e != nil {
		return History{}, e
	}
	inventory, e := plan.Inventory(im.Context.AgentRef)
	if e != nil {
		return History{}, e
	}
	contexts := map[networkplan.TargetKey]networkplan.ExecutionContext{}
	for _, item := range inventory {
		contexts[item.Protection.Key] = item.Protection.Context
	}
	for _, target := range im.Targets {
		index := -1
		for i, t := range out.Targets {
			if t.Mark == target.Mark {
				if t.Identity != target.Identity || !reflect.DeepEqual(t.Protected, target.Protected) || !reflect.DeepEqual(t.Context, contexts[target.Identity.Key]) {
					return History{}, fmt.Errorf("historical mark cannot change physical identity")
				}
				index = i
				break
			}
		}
		if index < 0 {
			out.Targets = append(out.Targets, HistoricalTarget{Context: contexts[target.Identity.Key], Identity: target.Identity, Mark: target.Mark, Protected: append([]hostpolicy.Endpoint{}, target.Protected...)})
			index = len(out.Targets) - 1
		}
		t := &out.Targets[index]
		seen := map[HistoricalMember]bool{}
		for _, m := range t.Members {
			seen[m] = true
		}
		for _, m := range target.Allow {
			member := HistoricalMember{Source: m.Source, Endpoint: m.Endpoint}
			if !seen[member] {
				t.Members = append(t.Members, member)
				seen[member] = true
			}
		}
		slices.SortFunc(t.Members, func(a, b HistoricalMember) int { return strings.Compare(memberKey(a), memberKey(b)) })
	}
	slices.SortFunc(out.Targets, func(a, b HistoricalTarget) int {
		if a.Mark < b.Mark {
			return -1
		}
		if a.Mark > b.Mark {
			return 1
		}
		return 0
	})
	if e = out.validateShape(); e != nil {
		return History{}, e
	}
	return out.clone()
}
func (h History) Digest() string { b, _ := json.Marshal(h); return fmt.Sprintf("%x", sha256.Sum256(b)) }
func (h History) Covers(im hostrules.Image) error {
	merged, e := Remember(&h, im)
	if e != nil {
		return e
	}
	if merged.Digest() != h.Digest() {
		return fmt.Errorf("confirmed image absent from history")
	}
	return nil
}
func NewRecorded(history History, current hostrules.Image, marks markstate.State) (*Plan, error) {
	if e := history.Validate(marks); e != nil {
		return nil, e
	}
	if e := history.Covers(current); e != nil {
		return nil, e
	}
	// Retirement is monotonic allocation history, not evidence the recovered
	// live image is absent. The held Session still confirms this exact image.
	p, e := newPlan(nil, current, marks, true)
	if e != nil {
		return nil, e
	}
	h, e := history.clone()
	if e != nil {
		return nil, e
	}
	p.recorded = &h
	return p, nil
}
