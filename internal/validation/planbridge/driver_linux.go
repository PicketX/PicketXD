package planbridge

import (
	"context"
	"time"
)

// Run is a test-only boundary driver. Each wake queries the latest accepted
// state; a timer never captures a plan. Callers cancel and join it before teardown.
func (c *Coordinator) Run(ctx context.Context) error {
	for {
		c.mu.Lock()
		next := c.state.NextChange
		faulted := c.state.Faulted
		now := c.effects.Now()
		c.mu.Unlock()
		if e := ctx.Err(); e != nil {
			return e
		}
		if faulted {
			return c.checkDriverFault()
		}
		var tick <-chan time.Time
		var timer *time.Timer
		if !next.IsZero() {
			timer = time.NewTimer(max(time.Duration(0), next.Sub(now)))
			tick = timer.C
		}
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return ctx.Err()
		case <-c.wake:
			if timer != nil {
				timer.Stop()
			}
			continue
		case <-tick:
			// Refresh locks, checks cancellation and rematerializes the current plan.
			if _, e := c.Refresh(ctx); e != nil {
				return e
			}
		}
	}
}
func (c *Coordinator) checkDriverFault() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.check(context.Background())
}
