// Package scenario coordinates the M0 apply-before-revoke contract.
package scenario

import (
	"context"
	"fmt"
	"github.com/PicketX/PicketXD/internal/validation/ctbackend"
	"github.com/PicketX/PicketXD/internal/validation/fixture"
	"time"
)

type Applier interface {
	Apply(context.Context, fixture.Snapshot, time.Time) error
}
type Revoker interface {
	Revoke(context.Context, fixture.Snapshot, fixture.Snapshot, uint32, time.Time) (ctbackend.Result, error)
}

func ApplyStrict(ctx context.Context, a Applier, r Revoker, before, after fixture.Snapshot, mark uint32, now time.Time) (ctbackend.Result, error) {
	if err := a.Apply(ctx, after, now); err != nil {
		return ctbackend.Result{}, fmt.Errorf("candidate not applied: %w", err)
	}
	return r.Revoke(ctx, before, after, mark, now)
}
