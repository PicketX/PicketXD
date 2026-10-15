package planbridge

import (
	"context"
	"errors"
	"github.com/PicketX/PicketX/pkg/core"
	"github.com/PicketX/PicketX/pkg/networkplan"
	"github.com/PicketX/PicketXD/internal/validation/fixture"
	"github.com/PicketX/PicketXD/internal/validation/nftbackend"
	"testing"
	"time"
)

func liveCoordinator(t *testing.T) *Coordinator {
	t.Helper()
	f := &fakeEffects{}
	e := f.effects()
	e.Now = time.Now
	e.Apply = func(_ context.Context, s fixture.Snapshot, _ nftbackend.Options) (nftbackend.Receipt, error) {
		now := time.Now()
		return nftbackend.Receipt{Emitted: s, DispatchedAt: now, AcknowledgedAt: now}, nil
	}
	c, err := NewCoordinator(lifecycleScope(t, networkplan.Graceful), e)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func waitState(t *testing.T, c *Coordinator, want func(State) bool) {
	t.Helper()
	end := time.Now().Add(2 * time.Second)
	for time.Now().Before(end) {
		if want(c.State()) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("state not reached", c.State())
}
func TestDriverActivatesFutureAllowDenyAndExpiresDeny(t *testing.T) {
	c := liveCoordinator(t)
	in := testInput(networkplan.Graceful)
	now := time.Now()
	for i := range in.Logical.Grants {
		in.Logical.Grants[i].Validity = core.Validity{Mode: core.LifetimeBounded, NotBefore: now.Add(80 * time.Millisecond), ExpiresAt: now.Add(time.Second)}
	}
	d := core.Deny(in.Logical.Grants[0])
	d.ID = "future-deny"
	d.Validity = core.Validity{Mode: core.LifetimeBounded, NotBefore: now.Add(180 * time.Millisecond), ExpiresAt: now.Add(280 * time.Millisecond)}
	in.Logical.Denies = []core.Deny{d}
	if _, e := c.Update(context.Background(), in); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()
	defer func() {
		cancel()
		if e := <-done; !errors.Is(e, context.Canceled) {
			t.Error(e)
		}
	}()
	waitState(t, c, func(s State) bool { return len(s.Current.Allow) == 6 && len(s.Current.Deny) == 0 })
	waitState(t, c, func(s State) bool { return len(s.Current.Deny) == 3 })
	waitState(t, c, func(s State) bool { return s.Generation >= 4 && len(s.Current.Deny) == 0 })
}
func TestDriverNeverReplaysReplacedOrCanceledPlan(t *testing.T) {
	c := liveCoordinator(t)
	in := testInput(networkplan.Graceful)
	start := time.Now().Add(120 * time.Millisecond)
	for i := range in.Logical.Grants {
		in.Logical.Grants[i].Validity = core.Validity{Mode: core.LifetimeBounded, NotBefore: start, ExpiresAt: start.Add(time.Second)}
	}
	if _, e := c.Update(context.Background(), in); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()
	for i := range in.Logical.Grants {
		in.Logical.Grants[i].Revoked = true
	}
	if _, e := c.Update(context.Background(), in); e != nil {
		t.Fatal(e)
	}
	generation := c.State().Generation
	time.Sleep(150 * time.Millisecond)
	if s := c.State(); len(s.Current.Allow) != 0 || s.Generation != generation {
		t.Fatal("old boundary replayed", s)
	}
	cancel()
	if e := <-done; !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	// A canceled driver cannot later apply a newly accepted future generation.
	in = testInput(networkplan.Graceful)
	now := time.Now()
	for i := range in.Logical.Grants {
		in.Logical.Grants[i].Validity = core.Validity{Mode: core.LifetimeBounded, NotBefore: now.Add(50 * time.Millisecond), ExpiresAt: now.Add(time.Second)}
	}
	if _, e := c.Update(context.Background(), in); e != nil {
		t.Fatal(e)
	}
	generation = c.State().Generation
	time.Sleep(80 * time.Millisecond)
	if s := c.State(); s.Generation != generation || len(s.Current.Allow) != 0 {
		t.Fatal("apply after cancellation", s)
	}
}
