// SPDX-License-Identifier: GPL-3.0-or-later
package planbridge

import (
	"fmt"
	"slices"
	"time"

	"github.com/PicketX/PicketX/pkg/core"
	"github.com/PicketX/PicketX/pkg/networkplan"
	"github.com/PicketX/PicketXD/internal/validation/fixture"
)

type Domain string

const (
	GrantDomain Domain = "grant"
	DenyDomain  Domain = "deny"
)

type Member struct {
	Domain          Domain
	ID, AuthorityID string
	Target          networkplan.TargetKey
	BindingIDs      []string
	Rule            fixture.Rule
	Validity        core.Validity
	Revoked         bool
}

type Materialized struct {
	Identity       Identity
	At, NextChange time.Time
	Snapshot       fixture.Snapshot
	Provenance     []Member
	Inventory      []networkplan.TargetInventory
	Withdrawals    []networkplan.Withdrawal
	AgentRemoved   bool
}

type Prepared struct {
	scope       *Scope
	plan        *networkplan.Plan
	withdrawals []networkplan.Withdrawal
	removed     bool
	empty       bool
}

func Prepare(scope *Scope, previous *Prepared, input networkplan.Input) (*Prepared, error) {
	if scope == nil || !scope.valid {
		return nil, fmt.Errorf("fixture scope is not validated")
	}
	if input.AuthorityID != scope.identity.AuthorityID {
		return nil, fmt.Errorf("candidate authority differs from fixture authority")
	}
	var plan *networkplan.Plan
	var withdrawals []networkplan.Withdrawal
	if previous == nil {
		var err error
		plan, err = networkplan.Compile(input)
		if err != nil {
			return nil, err
		}
	} else {
		if previous.scope != scope || previous.plan == nil {
			return nil, fmt.Errorf("accepted predecessor belongs to another fixture scope")
		}
		replacement, err := networkplan.Replace(previous.plan, input)
		if err != nil {
			return nil, err
		}
		plan, withdrawals = replacement.Plan, replacement.Withdrawals
	}
	// Trusted context facts cannot be rewritten, even if no active Grant would
	// currently make the changed destination visible in an allow projection.
	for _, raw := range input.Contexts {
		if raw.AgentRef == scope.identity.AgentRef {
			c, err := networkplan.NormalizeExecutionContext(raw)
			if err != nil || !sameContext(c, scope.context) {
				return nil, fmt.Errorf("candidate context differs from fixed fixture facts")
			}
		}
	}
	// Replace can include withdrawals for many Agents; this fixture owns one.
	withdrawals = slices.DeleteFunc(withdrawals, func(w networkplan.Withdrawal) bool {
		return w.Former.Protection.Key.AgentRef != scope.identity.AgentRef
	})
	known := slices.ContainsFunc(input.Agents, func(a networkplan.Agent) bool { return a.Name == scope.identity.AgentRef })
	if !known {
		if previous == nil || (!previous.empty && len(withdrawals) == 0) {
			return nil, fmt.Errorf("absent fixture Agent requires accepted replacement withdrawal or explicit empty predecessor")
		}
		return &Prepared{scope: scope, plan: plan, withdrawals: withdrawals, removed: true, empty: true}, nil
	}
	inventory, err := plan.Inventory(scope.identity.AgentRef)
	if err != nil {
		return nil, err
	}
	for _, target := range inventory {
		p := target.Protection
		c := p.Context
		// Identity and resolved context facts remain fixed; Endpoint addresses
		// can narrow the physical matches within that boundary.
		if !sameContext(c, scope.context) {
			return nil, fmt.Errorf("selected target has unsupported fixture context")
		}
		for _, match := range p.Matches {
			if !scope.supports(match) {
				return nil, fmt.Errorf("selected tuple %s %s/%d is outside proven fixture catalog", match.Destination, match.Protocol, match.Port)
			}
		}
	}
	return &Prepared{scope: scope, plan: plan, withdrawals: withdrawals, empty: len(inventory) == 0}, nil
}

func (p *Prepared) Materialize(at time.Time) (Materialized, error) {
	if p == nil || p.plan == nil || p.scope == nil || !p.scope.valid {
		return Materialized{}, fmt.Errorf("candidate is not prepared")
	}
	if at.IsZero() {
		return Materialized{}, fmt.Errorf("evaluation time must not be zero")
	}
	if p.removed {
		return Materialized{Identity: p.scope.identity, At: at.Round(0).UTC(), Withdrawals: cloneWithdrawals(p.withdrawals), AgentRemoved: true}, nil
	}
	projection, err := p.plan.Projection(p.scope.identity.AgentRef, at)
	if err != nil {
		return Materialized{}, err
	}
	inventory, err := p.plan.Inventory(p.scope.identity.AgentRef)
	if err != nil {
		return Materialized{}, err
	}
	out := Materialized{Identity: p.scope.identity, At: projection.At, NextChange: projection.NextChange, Inventory: inventory, Withdrawals: cloneWithdrawals(p.withdrawals)}
	bindings := make(map[networkplan.TargetKey][]string)
	seen := make(map[fixture.Endpoint]bool)
	for _, protection := range projection.Protected {
		bindings[protection.Key] = protection.BindingIDs
		for _, match := range protection.Matches {
			e := endpoint(match)
			if !seen[e] {
				out.Snapshot.Protected = append(out.Snapshot.Protected, e)
				seen[e] = true
			}
		}
	}
	for _, domain := range []struct {
		kind    Domain
		members []networkplan.Membership
	}{{GrantDomain, projection.Allow}, {DenyDomain, projection.Deny}} {
		for _, membership := range domain.members {
			c := membership.Contribution
			rule := fixture.Rule{Source: c.Source, Endpoint: endpoint(membership.Match), ExpiresAt: c.Validity.ExpiresAt}
			if domain.kind == GrantDomain {
				out.Snapshot.Allow = append(out.Snapshot.Allow, rule)
			} else {
				out.Snapshot.Deny = append(out.Snapshot.Deny, rule)
			}
			out.Provenance = append(out.Provenance, Member{Domain: domain.kind, ID: c.ID, AuthorityID: projection.AuthorityID, Target: membership.Target, BindingIDs: slices.Clone(bindings[membership.Target]), Rule: rule, Validity: c.Validity, Revoked: c.Revoked})
		}
	}
	return out, nil
}

func endpoint(m networkplan.Match) fixture.Endpoint {
	protocol := uint8(6)
	if m.Protocol == networkplan.UDP {
		protocol = 17
	}
	return fixture.Endpoint{Destination: m.Destination, Protocol: protocol, PortFirst: m.Port, PortLast: m.Port}
}

func sameContext(a, b networkplan.ExecutionContext) bool {
	return a.AgentRef == b.AgentRef && a.ProfileRef == b.ProfileRef && a.ID == b.ID && a.NetworkSpaceID == b.NetworkSpaceID && a.Class == b.Class && a.Stage == b.Stage && a.Handling == b.Handling && slices.Equal(a.Destinations, b.Destinations)
}

func cloneWithdrawals(in []networkplan.Withdrawal) []networkplan.Withdrawal {
	out := slices.Clone(in)
	for i := range out {
		t := &out[i].Former
		t.Protection.BindingIDs = slices.Clone(t.Protection.BindingIDs)
		t.Protection.Matches = slices.Clone(t.Protection.Matches)
		t.Protection.Context.Destinations = slices.Clone(t.Protection.Context.Destinations)
		c := &t.Protection.Required
		c.Classes = slices.Clone(c.Classes)
		c.Protocols = slices.Clone(c.Protocols)
		c.Families = slices.Clone(c.Families)
		c.Handling = slices.Clone(c.Handling)
		t.Grants = slices.Clone(t.Grants)
		t.Denies = slices.Clone(t.Denies)
	}
	return out
}
