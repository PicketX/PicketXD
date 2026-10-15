package planbridge

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sync"
	"time"

	"github.com/PicketX/PicketX/pkg/networkplan"
	"github.com/PicketX/PicketXD/internal/validation/ctbackend"
	"github.com/PicketX/PicketXD/internal/validation/fixture"
	"github.com/PicketX/PicketXD/internal/validation/nftbackend"
)

// Effects are private test-fixture boundaries. Revalidate must check the actual
// namespace epoch, topology and handles, not simply echo desired context strings.
type Effects struct {
	Now                   func() time.Time
	Revalidate            func(context.Context, Facts) error
	Apply                 func(context.Context, fixture.Snapshot, nftbackend.Options) (nftbackend.Receipt, error)
	Confirm               func(context.Context, nftbackend.Receipt) error
	Remove, ConfirmAbsent func(context.Context) error
	Revoke                func(context.Context, fixture.Snapshot, fixture.Snapshot, uint32, time.Time) (ctbackend.Result, error)
	RevokeEmpty           func(context.Context, fixture.Snapshot, uint32, time.Time) (ctbackend.Result, error)
}
type State struct {
	Generation                              uint64
	Applied, Empty, Faulted, CleanupPending bool
	AppliedAt, NextChange                   time.Time
	Cleanup                                 ctbackend.Result
	History                                 []Member
	Current                                 fixture.Snapshot
}
type Coordinator struct {
	mu         sync.Mutex
	scope      *Scope
	effects    Effects
	plan       *Prepared
	removed    bool
	receipt    nftbackend.Receipt
	state      State
	protection []fixture.Endpoint
	wake       chan struct{}
}

func NewCoordinator(scope *Scope, e Effects) (*Coordinator, error) {
	if scope == nil || !scope.valid || e.Now == nil || e.Revalidate == nil || e.Apply == nil || e.Confirm == nil || e.Remove == nil || e.ConfirmAbsent == nil || e.Revoke == nil || e.RevokeEmpty == nil {
		return nil, fmt.Errorf("complete fixture scope and effects are required")
	}
	return &Coordinator{scope: scope, effects: e, wake: make(chan struct{}, 1), state: State{Cleanup: ctbackend.Result{Remaining: -1}}}, nil
}
func cloneMember(m Member) Member {
	m.BindingIDs = slices.Clone(m.BindingIDs)
	return m
}
func cloneMembers(ms []Member) []Member {
	out := make([]Member, len(ms))
	for i, m := range ms {
		out[i] = cloneMember(m)
	}
	return out
}
func (c *Coordinator) stateLocked() State {
	s := c.state
	s.History = cloneMembers(s.History)
	s.Current = cloneSnapshot(s.Current)
	return s
}
func cloneSnapshot(s fixture.Snapshot) fixture.Snapshot {
	return fixture.Snapshot{Protected: slices.Clone(s.Protected), Allow: slices.Clone(s.Allow), Deny: slices.Clone(s.Deny)}
}
func (c *Coordinator) State() State { c.mu.Lock(); defer c.mu.Unlock(); return c.stateLocked() }
func (c *Coordinator) signal() {
	select {
	case c.wake <- struct{}{}:
	default:
	}
}
func (c *Coordinator) check(ctx context.Context) error {
	if c.state.Faulted {
		return fmt.Errorf("fixture application is unknown; teardown required")
	}
	return ctx.Err()
}

func (c *Coordinator) Update(ctx context.Context, in networkplan.Input) (State, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.check(ctx); e != nil {
		return c.stateLocked(), e
	}
	p, e := Prepare(c.scope, c.plan, in)
	if e != nil {
		return c.stateLocked(), e
	}
	return c.applyLocked(ctx, p, p.removed)
}
func (c *Coordinator) prepared(p *Prepared, _ bool, now time.Time) (Materialized, error) {
	return p.Materialize(now)
}

func (c *Coordinator) Refresh(ctx context.Context) (State, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.check(ctx); e != nil {
		return c.stateLocked(), e
	}
	if c.plan == nil {
		return c.stateLocked(), fmt.Errorf("no accepted plan")
	}
	return c.applyLocked(ctx, c.plan, c.removed)
}
func (c *Coordinator) fault(e error) (State, error) {
	c.state.Faulted = true
	return c.stateLocked(), fmt.Errorf("unknown fixture application; no cleanup: %w", e)
}
func (c *Coordinator) applyLocked(ctx context.Context, p *Prepared, removed bool) (State, error) {
	// Full shared validation and scoped conversion precede the first side effect.
	for attempts := 0; attempts < 100; attempts++ {
		if e := c.check(ctx); e != nil {
			return c.stateLocked(), e
		}
		prepared, e := c.prepared(p, removed, c.effects.Now())
		if e != nil {
			return c.stateLocked(), e
		}
		if e = c.effects.Revalidate(ctx, c.scope.Facts()); e != nil {
			return c.stateLocked(), e
		}
		var r nftbackend.Receipt
		empty := len(prepared.Snapshot.Protected) == 0
		if empty {
			if e = c.effects.Remove(ctx); e != nil {
				return c.fault(e)
			}
			if e = c.effects.ConfirmAbsent(ctx); e != nil {
				return c.fault(e)
			}
		} else {
			r, e = c.effects.Apply(ctx, cloneSnapshot(prepared.Snapshot), nftbackend.Options{FixtureNAT: c.scope.catalog == NAT})
			if e != nil {
				var ae *nftbackend.ApplyError
				if errors.As(e, &ae) && ae.Rejected {
					return c.stateLocked(), e
				}
				return c.fault(e)
			}
			if e = validateReceipt(prepared, r); e != nil {
				return c.fault(e)
			}
			if e = c.effects.Confirm(ctx, r); e != nil {
				return c.fault(e)
			}
		}
		// This state is actually confirmed even if later cleanup fails. Do not roll
		// it back or extend deadlines. Retain every confirmed emitted contribution.
		c.plan = p
		c.removed = removed
		c.receipt = r
		c.state.Applied = true
		c.state.Empty = empty
		c.state.Generation++
		c.state.AppliedAt = c.effects.Now()
		c.state.NextChange = prepared.NextChange
		c.state.Current = cloneSnapshot(r.Emitted)
		c.remember(prepared, r)
		c.signal()
		if !prepared.NextChange.IsZero() && !c.effects.Now().Before(prepared.NextChange) {
			continue
		}
		return c.cleanupLocked(ctx)
	}
	return c.fault(fmt.Errorf("too many boundaries crossed during fixture application"))
}
func validateReceipt(p Materialized, r nftbackend.Receipt) error {
	if r.DispatchedAt.IsZero() || r.AcknowledgedAt.IsZero() || r.AcknowledgedAt.Before(r.DispatchedAt) || r.DispatchedAt.Before(p.At) {
		return fmt.Errorf("missing or inconsistent batch receipt timestamps")
	}
	if !slices.Equal(p.Snapshot.Protected, r.Emitted.Protected) {
		return fmt.Errorf("receipt protection differs from prepared scope")
	}
	for _, domain := range []struct{ wanted, emitted, omitted []fixture.Rule }{{p.Snapshot.Allow, r.Emitted.Allow, r.Omitted.Allow}, {p.Snapshot.Deny, r.Emitted.Deny, r.Omitted.Deny}} {
		counts := make(map[fixture.Rule]int)
		for _, rule := range domain.wanted {
			counts[rule]++
		}
		for _, list := range [][]fixture.Rule{domain.emitted, domain.omitted} {
			for _, rule := range list {
				if counts[rule] == 0 {
					return fmt.Errorf("receipt contains unknown or duplicated permission")
				}
				counts[rule]--
			}
		}
		for _, count := range counts {
			if count != 0 {
				return fmt.Errorf("receipt silently lost permission")
			}
		}
		for _, rule := range domain.omitted {
			if rule.ExpiresAt.IsZero() || rule.ExpiresAt.Sub(r.DispatchedAt) >= time.Millisecond {
				return fmt.Errorf("receipt omitted a live representable permission")
			}
		}
		for _, rule := range domain.emitted {
			if !rule.ExpiresAt.IsZero() && rule.ExpiresAt.Sub(r.DispatchedAt) < time.Millisecond {
				return fmt.Errorf("receipt emitted expired/submillisecond permission")
			}
		}
	}
	return nil
}
func (c *Coordinator) remember(p Materialized, r nftbackend.Receipt) {
	for _, e := range r.Emitted.Protected {
		if !slices.Contains(c.protection, e) {
			c.protection = append(c.protection, e)
		}
	}
	for _, m := range p.Provenance {
		rules := r.Emitted.Allow
		if m.Domain == DenyDomain {
			rules = r.Emitted.Deny
		}
		if !slices.Contains(rules, m.Rule) {
			continue
		}
		found := false
		for _, old := range c.state.History {
			if reflect.DeepEqual(old, m) {
				found = true
				break
			}
		}
		if !found {
			c.state.History = append(c.state.History, cloneMember(m))
		}
	}
}
func (c *Coordinator) cleanupLocked(ctx context.Context) (State, error) {
	if c.scope.context.Handling != networkplan.Strict {
		return c.stateLocked(), nil
	}
	before := fixture.Snapshot{Protected: slices.Clone(c.protection)}
	for _, m := range c.state.History {
		if m.Domain == GrantDomain && !slices.Contains(before.Allow, m.Rule) {
			before.Allow = append(before.Allow, m.Rule)
		}
	}
	if len(before.Allow) == 0 {
		c.state.CleanupPending = false
		c.state.Cleanup = ctbackend.Result{Remaining: 0}
		return c.stateLocked(), nil
	}
	c.state.CleanupPending = true
	if e := c.effects.Revalidate(ctx, c.scope.Facts()); e != nil {
		return c.stateLocked(), e
	}
	if c.state.Empty {
		if e := c.effects.ConfirmAbsent(ctx); e != nil {
			return c.fault(e)
		}
	} else {
		if e := c.effects.Confirm(ctx, c.receipt); e != nil {
			return c.fault(e)
		}
	}
	now := c.effects.Now()
	if !c.state.NextChange.IsZero() && !now.Before(c.state.NextChange) {
		return c.applyLocked(ctx, c.plan, c.removed)
	}
	var result ctbackend.Result
	var e error
	if c.state.Empty {
		result, e = c.effects.RevokeEmpty(ctx, before, nftbackend.TestMark, now)
	} else {
		result, e = c.effects.Revoke(ctx, before, cloneSnapshot(c.state.Current), nftbackend.TestMark, now)
	}
	c.state.Cleanup = result
	if e == nil && result.Remaining != 0 {
		e = fmt.Errorf("cleanup did not confirm zero remaining targets: %d", result.Remaining)
	}
	c.state.CleanupPending = e != nil
	return c.stateLocked(), e
}
func (c *Coordinator) RetryCleanup(ctx context.Context) (State, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.check(ctx); e != nil {
		return c.stateLocked(), e
	}
	if c.plan == nil {
		return c.stateLocked(), fmt.Errorf("no confirmed history")
	}
	return c.cleanupLocked(ctx)
}
