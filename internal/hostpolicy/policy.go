// SPDX-License-Identifier: GPL-3.0-or-later
// Package hostpolicy compiles desired host INPUT intent. Its values are not
// installation receipts, mark reservations or proof of kernel enforcement.
package hostpolicy

import (
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/PicketX/PicketX/pkg/core"
	"github.com/PicketX/PicketX/pkg/networkplan"
)

// ContractVersion versions this internal compiler output, not kernel structure.
const ContractVersion = 1

type Endpoint struct {
	Destination netip.Prefix
	Protocol    uint8
	Port        uint16
}
type Member struct {
	ID       string
	Source   netip.Prefix
	Endpoint Endpoint
	Validity core.Validity
}
type Target struct {
	Key         networkplan.TargetKey
	BindingIDs  []string
	Protected   []Endpoint
	Allow, Deny []Member
}
type Snapshot struct {
	Version        int
	AuthorityID    string
	Context        networkplan.ExecutionContext
	At, NextChange time.Time
	Targets        []Target
}

// Scope is an immutable copy of trusted runtime facts, not policy-authored facts.
type Scope struct {
	authority string
	context   networkplan.ExecutionContext
}
type Policy struct {
	scope     *Scope
	plan      *networkplan.Plan
	inputJSON []byte
}

func NewScope(authorityID string, context networkplan.ExecutionContext) (*Scope, error) {
	for _, id := range []string{authorityID, context.AgentRef, context.ProfileRef, context.ID, context.NetworkSpaceID} {
		if id == "" || len(id) > 256 || !utf8.ValidString(id) || strings.ContainsRune(id, 0) {
			return nil, fmt.Errorf("host scope requires bounded nonempty UTF-8 identities without NUL")
		}
	}
	if context.Class != networkplan.HostLocal || context.Stage != networkplan.OriginalDestination || (context.Handling != networkplan.Graceful && context.Handling != networkplan.Strict) {
		return nil, fmt.Errorf("host compiler supports only explicit host-local/original-destination with explicit graceful or strict context")
	}
	c, err := networkplan.NormalizeExecutionContext(context)
	if err != nil {
		return nil, err
	}
	return &Scope{authority: authorityID, context: c}, nil
}

func Compile(plan *networkplan.Plan, scope *Scope) (*Policy, error) {
	if scope == nil || scope.authority == "" {
		return nil, fmt.Errorf("host scope is not validated")
	}
	// Inventory validates the plan and Agent even when the selected target set is
	// empty. Check complete targets before evaluation so dormant intent cannot
	// evade capability and namespace validation.
	inventory, err := plan.Inventory(scope.context.AgentRef)
	if err != nil {
		return nil, err
	}
	projection, err := plan.Projection(scope.context.AgentRef, time.Unix(0, 0))
	if err != nil {
		return nil, err
	}
	if projection.AuthorityID != scope.authority {
		return nil, fmt.Errorf("plan authority differs from trusted host authority")
	}
	for _, target := range inventory {
		c, want := target.Protection.Context, scope.context
		if c.AgentRef != want.AgentRef || c.ProfileRef != want.ProfileRef || c.ID != want.ID || c.NetworkSpaceID != want.NetworkSpaceID || c.Class != want.Class || c.Stage != want.Stage || c.Handling != want.Handling {
			return nil, fmt.Errorf("target %v differs from trusted host context", target.Protection.Key)
		}
		for _, destination := range c.Destinations {
			if !within(destination, want.Destinations) {
				return nil, fmt.Errorf("target context exceeds trusted host destination bounds")
			}
		}
		for _, m := range target.Protection.Matches {
			if !within(m.Destination, c.Destinations) || !within(m.Destination, want.Destinations) || m.Port == 0 || (m.Protocol != networkplan.TCP && m.Protocol != networkplan.UDP) {
				return nil, fmt.Errorf("target match exceeds supported host context")
			}
		}
	}
	return &Policy{scope: scope, plan: plan}, nil
}
func within(p netip.Prefix, bounds []netip.Prefix) bool {
	for _, b := range bounds {
		if b.Addr().BitLen() == p.Addr().BitLen() && b.Bits() <= p.Bits() && b.Contains(p.Addr()) {
			return true
		}
	}
	return false
}
func endpoint(m networkplan.Match) Endpoint {
	protocol := uint8(6)
	if m.Protocol == networkplan.UDP {
		protocol = 17
	}
	return Endpoint{Destination: m.Destination, Protocol: protocol, Port: m.Port}
}
func (p *Policy) valid() bool { return p != nil && p.scope != nil && p.plan != nil }

// Inventory returns complete desired membership, including future, expired and
// revoked contributions. It never proves which members reached the kernel.
func (p *Policy) Inventory() ([]networkplan.TargetInventory, error) {
	if !p.valid() {
		return nil, fmt.Errorf("host policy is not compiled")
	}
	return p.plan.Inventory(p.scope.context.AgentRef)
}

// Evaluate returns independently owned data at an explicit instant. NextChange
// is a desired reevaluation boundary, not a process-loss enforcement guarantee.
func (p *Policy) Evaluate(at time.Time) (Snapshot, error) {
	if !p.valid() {
		return Snapshot{}, fmt.Errorf("host policy is not compiled")
	}
	projection, err := p.plan.Projection(p.scope.context.AgentRef, at)
	if err != nil {
		return Snapshot{}, err
	}
	context := p.scope.context
	context.Destinations = slices.Clone(context.Destinations)
	out := Snapshot{Version: ContractVersion, AuthorityID: projection.AuthorityID, Context: context, At: projection.At, NextChange: projection.NextChange}
	indices := make(map[networkplan.TargetKey]int, len(projection.Protected))
	for _, protection := range projection.Protected {
		target := Target{Key: protection.Key, BindingIDs: slices.Clone(protection.BindingIDs)}
		for _, m := range protection.Matches {
			target.Protected = append(target.Protected, endpoint(m))
		}
		indices[target.Key] = len(out.Targets)
		out.Targets = append(out.Targets, target)
	}
	for _, domain := range []struct {
		members []networkplan.Membership
		deny    bool
	}{{projection.Allow, false}, {projection.Deny, true}} {
		for _, m := range domain.members {
			index, ok := indices[m.Target]
			if !ok {
				return Snapshot{}, fmt.Errorf("membership has no protected target")
			}
			c := m.Contribution
			member := Member{ID: c.ID, Source: c.Source, Endpoint: endpoint(m.Match), Validity: c.Validity}
			target := &out.Targets[index]
			if domain.deny {
				target.Deny = append(target.Deny, member)
			} else {
				target.Allow = append(target.Allow, member)
			}
		}
	}
	return out, nil
}
