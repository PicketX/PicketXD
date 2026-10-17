// SPDX-License-Identifier: GPL-3.0-or-later
package hostrevoke

import (
	"context"
	"errors"
	"fmt"
	"github.com/PicketX/PicketXD/internal/hostrules"
	"syscall"
	"testing"
	"time"
)

func cleanupPlan(t *testing.T) (*Plan, []Flow) {
	t.Helper()
	old, m := testImage(t, rawPolicy(t), nil)
	cur, m := testImage(t, noGrant(rawPolicy(t)), &m)
	p, e := New([]hostrules.Image{old}, cur, m)
	if e != nil {
		t.Fatal(e)
	}
	return p, []Flow{testFlow(old, false, 6), testFlow(old, true, 17)}
}
func testEffects(p *Plan, flows []Flow) (Effects, *[]string) {
	events := &[]string{}
	live := append([]Flow{}, flows...)
	e := Effects{Now: time.Now, Confirm: func(context.Context) error { *events = append(*events, "confirm"); return nil }, Dump: func(ctx context.Context, f uint8) ([]Flow, error) {
		*events = append(*events, fmt.Sprint("dump", f))
		var out []Flow
		for _, v := range live {
			if v.Family == f {
				out = append(out, v)
			}
		}
		return out, nil
	}, Delete: func(ctx context.Context, f Flow) error {
		*events = append(*events, fmt.Sprint("delete", f.Family))
		for i, v := range live {
			if v == f {
				live = append(live[:i], live[i+1:]...)
				return nil
			}
		}
		return syscall.ENOENT
	}}
	return e, events
}
func TestRunConfirmsAndCompletesBothFamilies(t *testing.T) {
	p, flows := cleanupPlan(t)
	e, events := testEffects(p, flows)
	r, err := Run(context.Background(), p, e)
	if err != nil || r.Selected != 2 || r.Deleted != 2 || r.Absent != 0 || r.Remaining != 0 {
		t.Fatal(r, err)
	}
	want := "[confirm dump2 dump10 confirm delete2 confirm delete10 dump2 dump10 confirm]"
	if fmt.Sprint(*events) != want {
		t.Fatal(*events)
	}
	r, err = Run(context.Background(), p, e)
	if err != nil || r.Selected != 0 || r.Remaining != 0 {
		t.Fatal("retry not idempotent", r, err)
	}
}
func TestRunFailedPreflightNeverDeletes(t *testing.T) {
	for _, stage := range []string{"confirm", "family4", "family6", "wrong family", "invalid flow"} {
		t.Run(stage, func(t *testing.T) {
			p, flows := cleanupPlan(t)
			e, _ := testEffects(p, flows)
			deletes := 0
			e.Delete = func(context.Context, Flow) error { deletes++; return nil }
			switch stage {
			case "confirm":
				e.Confirm = func(context.Context) error { return errors.New("drift") }
			case "family4", "family6":
				orig := e.Dump
				e.Dump = func(ctx context.Context, f uint8) ([]Flow, error) {
					if f == FamilyV4 && stage == "family4" || f == FamilyV6 && stage == "family6" {
						return flows, errors.New("interrupted")
					}
					return orig(ctx, f)
				}
			case "wrong family":
				e.Dump = func(context.Context, uint8) ([]Flow, error) { return flows, nil }
			case "invalid flow":
				e.Dump = func(context.Context, uint8) ([]Flow, error) { return []Flow{{}}, nil }
			}
			r, err := Run(context.Background(), p, e)
			if err == nil || deletes != 0 || r.Remaining != -1 {
				t.Fatal(r, err, deletes)
			}
		})
	}
}
func TestRunPartialDeleteAndRemainingAreErrors(t *testing.T) {
	p, flows := cleanupPlan(t)
	e, _ := testEffects(p, flows)
	e.Delete = func(ctx context.Context, f Flow) error {
		if f.Family == FamilyV4 {
			return errors.New("delete failed")
		}
		return syscall.ENOENT
	}
	r, err := Run(context.Background(), p, e)
	if err == nil || r.Selected != 2 || r.Deleted != 0 || r.Absent != 1 || r.Remaining != 2 {
		t.Fatal(r, err)
	}
}
func TestRunIncompleteConfirmationAndDrift(t *testing.T) {
	for _, phase := range []string{"redump4", "redump6", "before delete", "final confirm"} {
		t.Run(phase, func(t *testing.T) {
			p, flows := cleanupPlan(t)
			e, _ := testEffects(p, flows)
			dump, conf := 0, 0
			orig := e.Dump
			e.Dump = func(ctx context.Context, f uint8) ([]Flow, error) {
				dump++
				if dump == 3 && phase == "redump4" || dump == 4 && phase == "redump6" {
					return nil, errors.New("truncated")
				}
				return orig(ctx, f)
			}
			e.Confirm = func(context.Context) error {
				conf++
				if conf == 2 && phase == "before delete" || conf == 4 && phase == "final confirm" {
					return errors.New("drift")
				}
				return nil
			}
			r, err := Run(context.Background(), p, e)
			if err == nil || r.Remaining != -1 {
				t.Fatal(r, err)
			}
		})
	}
}
func TestRunCancellationClockAndBoundary(t *testing.T) {
	for _, mode := range []string{"cancel", "backward", "boundary"} {
		t.Run(mode, func(t *testing.T) {
			p, flows := cleanupPlan(t)
			at := time.Now().UTC()
			if mode == "boundary" {
				old, m := testImage(t, rawPolicy(t), nil)
				raw := rawPolicy(t)
				raw = replaceValidity(raw, at.Add(time.Second), at.Add(time.Hour))
				cur, m := testImage(t, raw, &m)
				var err error
				p, err = New([]hostrules.Image{old}, cur, m)
				if err != nil {
					t.Fatal(err)
				}
			}
			e, _ := testEffects(p, flows)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls, deletes := 0, 0
			e.Now = func() time.Time {
				calls++
				if calls == 1 {
					return at
				}
				if mode == "backward" {
					return at.Add(-time.Second)
				}
				if mode == "boundary" {
					return at.Add(time.Second)
				}
				cancel()
				return at
			}
			e.Delete = func(context.Context, Flow) error { deletes++; return nil }
			r, err := Run(ctx, p, e)
			if err == nil || deletes != 0 || r.Remaining != -1 {
				t.Fatal(r, err, deletes)
			}
		})
	}
	p, _ := cleanupPlan(t)
	if _, e := Run(context.Background(), p, Effects{}); e == nil {
		t.Fatal("missing operations")
	}
}
