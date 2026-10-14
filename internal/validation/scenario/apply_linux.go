package scenario

import (
	"context"
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

func ApplyStrict(context.Context, Applier, Revoker, fixture.Snapshot, fixture.Snapshot, uint32, time.Time) (ctbackend.Result, error) {
	return ctbackend.Result{}, nil
}
