// SPDX-License-Identifier: GPL-3.0-or-later
// Package hostrules generates bounded host INPUT rule images, without effects.
package hostrules

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/netip"
	"reflect"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/PicketX/PicketX/pkg/core"
	"github.com/PicketX/PicketX/pkg/networkplan"
	"github.com/PicketX/PicketXD/internal/hostpolicy"
	"github.com/PicketX/PicketXD/internal/markstate"
)

const MaxImageBytes = 8 << 20
const MaxRules = 8192

type Target struct {
	Identity    markstate.Target
	Mark        uint32
	Protected   []hostpolicy.Endpoint
	Allow, Deny []hostpolicy.Member
}
type Image struct {
	Version     int
	AuthorityID string
	Context     networkplan.ExecutionContext
	Targets     []Target
	Input       networkplan.Input
	KnownMarks  []uint32 `json:"knownMarks,omitempty"`
}

func Build(p *hostpolicy.Policy, marks markstate.State) (Image, error) {
	if e := marks.Validate(); e != nil {
		return Image{}, e
	}
	assignments := map[markstate.Target]uint32{}
	for _, a := range marks.Entries {
		if !a.Retired {
			assignments[a.Target] = a.Mark
		}
	}
	out, e := materialize(p, assignments)
	if e != nil {
		return Image{}, e
	}
	if out.Context.Handling == networkplan.Strict {
		for _, a := range marks.Entries {
			if a.Target.Key.AgentRef == out.Context.AgentRef && a.Target.Key.ContextID == out.Context.ID {
				out.KnownMarks = append(out.KnownMarks, a.Mark)
			}
		}
	}
	if e = out.ValidateMarks(marks); e != nil {
		return Image{}, e
	}
	return out, nil
}
func materialize(p *hostpolicy.Policy, assignments map[markstate.Target]uint32) (Image, error) {
	input, e := p.Input()
	if e != nil {
		return Image{}, e
	}
	desired, e := p.MarkTargets()
	if e != nil {
		return Image{}, e
	}
	inv, e := p.Inventory()
	if e != nil {
		return Image{}, e
	}
	snapshot, e := p.Evaluate(time.Unix(1, 0))
	if e != nil {
		return Image{}, e
	}
	out := Image{Version: 1, AuthorityID: snapshot.AuthorityID, Context: snapshot.Context, Input: input}
	if out.Context.Handling == networkplan.Strict {
		out.Version = 2
	}
	for i, item := range inv {
		identity := desired[i]
		assigned := assignments[identity]
		if assigned == 0 {
			return Image{}, fmt.Errorf("target has no active matching mark reservation")
		}
		target := Target{Identity: identity, Mark: assigned}
		for _, m := range item.Protection.Matches {
			protocol := uint8(6)
			if m.Protocol == networkplan.UDP {
				protocol = 17
			}
			endpoint := hostpolicy.Endpoint{Destination: m.Destination, Protocol: protocol, Port: m.Port}
			target.Protected = append(target.Protected, endpoint)
			for _, domain := range []struct {
				members []core.ResolvedContribution
				deny    bool
			}{{item.Grants, false}, {item.Denies, true}} {
				for _, c := range domain.members {
					if c.Revoked || c.Source.Addr().BitLen() != m.Destination.Addr().BitLen() {
						continue
					}
					member := hostpolicy.Member{ID: c.ID, Source: c.Source, Endpoint: endpoint, Validity: c.Validity}
					if domain.deny {
						target.Deny = append(target.Deny, member)
					} else {
						target.Allow = append(target.Allow, member)
					}
				}
			}
		}
		out.Targets = append(out.Targets, target)
	}
	return out, nil
}
func bounded(s string) bool {
	return s != "" && len(s) <= 256 && utf8.ValidString(s) && !strings.ContainsRune(s, 0)
}
func prefix(p netip.Prefix) bool {
	return p.IsValid() && !p.Addr().Is4In6() && p.Addr().Zone() == "" && p == p.Masked()
}
func within(p netip.Prefix, bounds []netip.Prefix) bool {
	for _, b := range bounds {
		if b.Addr().BitLen() == p.Addr().BitLen() && b.Bits() <= p.Bits() && b.Contains(p.Addr()) {
			return true
		}
	}
	return false
}
func endpointValid(e hostpolicy.Endpoint) bool {
	return prefix(e.Destination) && e.Port != 0 && (e.Protocol == 6 || e.Protocol == 17)
}
func timestamp(t time.Time) bool {
	return t.IsZero() || t.Unix() > 0 && t.Unix() <= 9223372036 && t.Equal(time.Unix(0, t.UnixNano()))
}
func (im Image) Validate() error {
	if (im.Version != 1 && im.Version != 2) || !bounded(im.AuthorityID) || (im.Context.Handling == networkplan.Strict) != (im.Version == 2) {
		return fmt.Errorf("invalid rule image version/authority")
	}
	if _, e := hostpolicy.NewScope(im.AuthorityID, im.Context); e != nil {
		return e
	}
	if len(im.Targets) > markstate.MaxMarks {
		return fmt.Errorf("too many rule targets")
	}
	if len(im.KnownMarks) > markstate.MaxMarks || im.Version == 1 && len(im.KnownMarks) != 0 {
		return fmt.Errorf("invalid/bounded strict known marks")
	}
	var prior uint32
	for _, mark := range im.KnownMarks {
		if mark == 0 || mark <= prior {
			return fmt.Errorf("invalid/duplicate strict known mark")
		}
		prior = mark
	}
	count := 2
	if im.Version == 2 && len(im.Targets) > 0 {
		count += len(im.KnownMarks) + 1
	}
	keys := map[networkplan.TargetKey]bool{}
	marks := map[uint32]bool{}
	var endpoints []hostpolicy.Endpoint
	for _, t := range im.Targets {
		if e := t.Identity.Validate(); e != nil {
			return e
		}
		k := t.Identity.Key
		if k.AgentRef != im.Context.AgentRef || k.ContextID != im.Context.ID || keys[k] || t.Mark == 0 || marks[t.Mark] || len(t.Protected) == 0 {
			return fmt.Errorf("invalid/duplicate target identity or mark")
		}
		keys[k] = true
		marks[t.Mark] = true
		count += 2 + 2*len(t.Protected) + len(t.Allow) + len(t.Deny)
		if im.Version == 2 {
			if !slices.Contains(im.KnownMarks, t.Mark) {
				return fmt.Errorf("current strict target missing from known marks")
			}
			count-- // no unconditional established bypass
			for _, m := range t.Allow {
				if m.Endpoint.Protocol == 6 {
					count++
				}
			}
		}
		if count > MaxRules {
			return fmt.Errorf("rule image exceeds %d rules", MaxRules)
		}
		for _, ep := range t.Protected {
			if !endpointValid(ep) || !within(ep.Destination, im.Context.Destinations) {
				return fmt.Errorf("endpoint outside host scope")
			}
			for _, old := range endpoints {
				if ep.Protocol == old.Protocol && ep.Port == old.Port && ep.Destination.Addr().BitLen() == old.Destination.Addr().BitLen() && (ep.Destination.Contains(old.Destination.Addr()) || old.Destination.Contains(ep.Destination.Addr())) {
					return fmt.Errorf("overlapping protected endpoints")
				}
			}
			endpoints = append(endpoints, ep)
		}
		for _, members := range [][]hostpolicy.Member{t.Allow, t.Deny} {
			for _, m := range members {
				if !bounded(m.ID) || !prefix(m.Source) || !slices.Contains(t.Protected, m.Endpoint) || m.Source.Addr().BitLen() != m.Endpoint.Destination.Addr().BitLen() {
					return fmt.Errorf("invalid membership target/source/identity")
				}
				if _, e := core.NormalizeValidity(m.Validity); e != nil {
					return e
				}
				if !timestamp(m.Validity.NotBefore) || !timestamp(m.Validity.ExpiresAt) {
					return fmt.Errorf("validity outside supported Unix nanosecond range")
				}
			}
		}
	}
	b, e := json.Marshal(im)
	if e != nil {
		return e
	}
	if len(b) > MaxImageBytes {
		return fmt.Errorf("rule image exceeds serialized bound")
	}
	sc, e := hostpolicy.NewScope(im.AuthorityID, im.Context)
	if e != nil {
		return e
	}
	p, e := hostpolicy.CompileInput(im.Input, sc)
	if e != nil {
		return e
	}
	assignments := map[markstate.Target]uint32{}
	for _, t := range im.Targets {
		assignments[t.Identity] = t.Mark
	}
	expected, e := materialize(p, assignments)
	if e != nil {
		return e
	}
	if im.Version == 2 {
		expected.KnownMarks = slices.Clone(im.KnownMarks)
	}
	if !reflect.DeepEqual(im, expected) {
		return fmt.Errorf("rule image differs from complete compiled input")
	}
	return nil
}
func (im Image) Digest() string { b, _ := json.Marshal(im); return fmt.Sprintf("%x", sha256.Sum256(b)) }
