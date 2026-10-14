package scenario

import (
	"context"
	"errors"
	"github.com/PicketX/PicketXD/internal/validation/ctbackend"
	"github.com/PicketX/PicketXD/internal/validation/fixture"
	"testing"
	"time"
)

type failingApplier struct{ err error }

func (f failingApplier) Apply(context.Context, fixture.Snapshot, time.Time) error { return f.err }

type forbiddenRevoker struct{ called bool }

func (r *forbiddenRevoker) Revoke(context.Context, fixture.Snapshot, fixture.Snapshot, uint32, time.Time) (ctbackend.Result, error) {
	r.called = true
	return ctbackend.Result{Deleted: 1}, nil
}
func TestApplyStrictRejectsUnappliedState(t *testing.T) {
	failure := errors.New("batch failed")
	r := &forbiddenRevoker{}
	got, err := ApplyStrict(context.Background(), failingApplier{failure}, r, fixture.Snapshot{}, fixture.Snapshot{}, 123, time.Now())
	if !errors.Is(err, failure) || r.called || got.Deleted != 0 {
		t.Fatalf("result=%+v err=%v revoker called=%v", got, err, r.called)
	}
}
