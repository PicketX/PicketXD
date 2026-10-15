package planbridge

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/PicketX/PicketX/pkg/core"
	"github.com/PicketX/PicketX/pkg/networkplan"
	"github.com/PicketX/PicketXD/internal/validation/ctbackend"
	"github.com/PicketX/PicketXD/internal/validation/fixture"
	"github.com/PicketX/PicketXD/internal/validation/nftbackend"
)

type fakeEffects struct {
	at                               time.Time
	calls                            []string
	applyErr, confirmErr, cleanupErr error
	before, after                    fixture.Snapshot
	omit                             bool
}

func (f *fakeEffects) effects() Effects {
	return Effects{
		Now:        func() time.Time { return f.at },
		Revalidate: func(context.Context, Facts) error { f.calls = append(f.calls, "validate"); return nil },
		Apply: func(_ context.Context, s fixture.Snapshot, _ nftbackend.Options) (nftbackend.Receipt, error) {
			f.calls = append(f.calls, "apply")
			r := nftbackend.Receipt{Emitted: s, DispatchedAt: f.at, AcknowledgedAt: f.at}
			if f.omit {
				r.Omitted.Allow = r.Emitted.Allow
				r.Emitted.Allow = nil
			}
			return r, f.applyErr
		},
		Confirm: func(context.Context, nftbackend.Receipt) error {
			f.calls = append(f.calls, "confirm")
			return f.confirmErr
		},
		Remove:        func(context.Context) error { f.calls = append(f.calls, "remove"); return nil },
		ConfirmAbsent: func(context.Context) error { f.calls = append(f.calls, "absent"); return f.confirmErr },
		Revoke: func(_ context.Context, b, a fixture.Snapshot, _ uint32, _ time.Time) (ctbackend.Result, error) {
			f.calls = append(f.calls, "revoke")
			f.before = b
			f.after = a
			return ctbackend.Result{Remaining: 0}, f.cleanupErr
		},
		RevokeEmpty: func(_ context.Context, b fixture.Snapshot, _ uint32, _ time.Time) (ctbackend.Result, error) {
			f.calls = append(f.calls, "revoke-empty")
			f.before = b
			return ctbackend.Result{Remaining: 0}, f.cleanupErr
		},
	}
}
func coordinatorOK(t *testing.T, f *fakeEffects, mode networkplan.HandlingMode) *Coordinator {
	t.Helper()
	c, e := NewCoordinator(lifecycleScope(t, mode), f.effects())
	if e != nil {
		t.Fatal(e)
	}
	return c
}
func TestCoordinatorConfirmsBeforeCleanup(t *testing.T) {
	f := &fakeEffects{at: testAt}
	c := coordinatorOK(t, f, networkplan.Strict)
	st, e := c.Update(context.Background(), testInput(networkplan.Strict))
	if e != nil || !st.Applied || st.Faulted || st.CleanupPending {
		t.Fatal(st, e)
	}
	if !reflect.DeepEqual(f.calls, []string{"validate", "apply", "confirm", "validate", "confirm", "revoke"}) {
		t.Fatal(f.calls)
	}
	if len(st.History) != 6 {
		t.Fatal(st)
	}
	st.History[0].BindingIDs[0] = "mutation"
	if c.State().History[0].BindingIDs[0] != "binding-a" {
		t.Fatal("shared mutable history")
	}
}
func TestCoordinatorRejectsBeforeMutationAndRetainsPrevious(t *testing.T) {
	f := &fakeEffects{at: testAt}
	c := coordinatorOK(t, f, networkplan.Strict)
	in := testInput(networkplan.Strict)
	if _, e := c.Update(context.Background(), in); e != nil {
		t.Fatal(e)
	}
	before := c.State().Generation
	f.calls = nil
	in.Networks[0].Port = 8443
	if _, e := c.Update(context.Background(), in); e == nil || len(f.calls) != 0 || c.State().Generation != before {
		t.Fatal("bad replacement mutated state", f.calls, e)
	}
}
func TestNoCleanupOnRejectedOrAmbiguousApply(t *testing.T) {
	for _, known := range []bool{true, false} {
		t.Run(map[bool]string{true: "rejected", false: "ambiguous"}[known], func(t *testing.T) {
			f := &fakeEffects{at: testAt, applyErr: &nftbackend.ApplyError{Rejected: known, Err: errors.New("failed batch")}}
			c := coordinatorOK(t, f, networkplan.Strict)
			st, e := c.Update(context.Background(), testInput(networkplan.Strict))
			if e == nil || st.Applied || st.Faulted == known || len(st.History) != 0 {
				t.Fatal(st, e)
			}
			if !reflect.DeepEqual(f.calls, []string{"validate", "apply"}) {
				t.Fatal(f.calls)
			}
		})
	}
	f := &fakeEffects{at: testAt, confirmErr: errors.New("lost readback")}
	c := coordinatorOK(t, f, networkplan.Strict)
	st, e := c.Update(context.Background(), testInput(networkplan.Strict))
	if e == nil || !st.Faulted || st.Applied || len(st.History) != 0 {
		t.Fatal(st, e)
	}
	f.calls = nil
	if _, e := c.Update(context.Background(), testInput(networkplan.Strict)); e == nil || len(f.calls) != 0 {
		t.Fatal("faulted fixture reused")
	}
}
func TestHistoricalExpiredMembershipSurvivesInterveningApplyAndRetry(t *testing.T) {
	f := &fakeEffects{at: testAt}
	c := coordinatorOK(t, f, networkplan.Strict)
	in := testInput(networkplan.Strict)
	in.Logical.Grants[0].Validity.ExpiresAt = testAt.Add(time.Second)
	if _, e := c.Update(context.Background(), in); e != nil {
		t.Fatal(e)
	}
	f.at = testAt.Add(2 * time.Second)
	f.cleanupErr = errors.New("partial delete")
	st, e := c.Refresh(context.Background())
	if e == nil || !st.Applied || !st.CleanupPending || len(st.History) != 6 || len(f.before.Allow) != 6 || len(f.after.Allow) != 3 {
		t.Fatal(st, e, f.before, f.after)
	}
	generation := st.Generation
	f.cleanupErr = nil
	f.calls = nil
	st, e = c.RetryCleanup(context.Background())
	if e != nil || st.CleanupPending || st.Generation != generation || len(f.before.Allow) != 6 {
		t.Fatal(st, e)
	}
	if !reflect.DeepEqual(f.calls, []string{"validate", "confirm", "revoke"}) {
		t.Fatal(f.calls)
	}
}
func TestExplicitEmptyBindingAndAgentWithdrawal(t *testing.T) {
	for _, removeAgent := range []bool{false, true} {
		for _, mode := range []networkplan.HandlingMode{networkplan.Graceful, networkplan.Strict} {
			f := &fakeEffects{at: testAt}
			c := coordinatorOK(t, f, mode)
			in := testInput(mode)
			if _, e := c.Update(context.Background(), in); e != nil {
				t.Fatal(e)
			}
			in.Bindings = nil
			if removeAgent {
				in.Agents = nil
				in.Contexts = nil
			}
			f.calls = nil
			st, e := c.Update(context.Background(), in)
			if e != nil || !st.Applied || !st.Empty {
				t.Fatal(st, e)
			}
			want := []string{"validate", "remove", "absent"}
			if mode == networkplan.Strict {
				want = append(want, "validate", "absent", "revoke-empty")
			}
			if !reflect.DeepEqual(f.calls, want) {
				t.Fatal(f.calls, want)
			}
			if _, e := c.RetryCleanup(context.Background()); e != nil {
				t.Fatal("retained explicit empty transition cannot retry", e)
			}
		}
	}
	f := &fakeEffects{at: testAt}
	c := coordinatorOK(t, f, networkplan.Strict)
	in := testInput(networkplan.Strict)
	in.Bindings = nil
	in.Agents = nil
	in.Contexts = nil
	if _, e := c.Update(context.Background(), in); e == nil || len(f.calls) != 0 {
		t.Fatal("unknown initial Agent treated as empty")
	}
}
func TestOmittedMembershipNeverSeedsHistory(t *testing.T) {
	f := &fakeEffects{at: testAt, omit: true}
	c := coordinatorOK(t, f, networkplan.Graceful)
	// Only a truly expiring submillisecond membership may be omitted.
	in := testInput(networkplan.Graceful)
	for i := range in.Logical.Grants {
		in.Logical.Grants[i].Validity = core.Validity{Mode: core.LifetimeBounded, ExpiresAt: testAt.Add(time.Microsecond)}
	}
	st, e := c.Update(context.Background(), in)
	if e != nil || len(st.History) != 0 {
		t.Fatal(st, e)
	}
}
