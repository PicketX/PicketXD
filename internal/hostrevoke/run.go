// SPDX-License-Identifier: GPL-3.0-or-later
package hostrevoke

import (
	"context"
	"errors"
	"fmt"
	"syscall"
	"time"
)

type Result struct{ Selected, Deleted, Absent, Remaining int }
type Effects struct {
	Confirm func(context.Context) error
	Dump    func(context.Context, uint8) ([]Flow, error)
	Delete  func(context.Context, Flow) error
	Now     func() time.Time
}

// Run returns scoped observations, never a persisted strict-mode receipt. The
// caller must serialize desired changes and provide complete historical images.
func Run(ctx context.Context, p *Plan, e Effects) (Result, error) {
	r := Result{Remaining: -1}
	if p == nil || e.Confirm == nil || e.Dump == nil || e.Delete == nil || e.Now == nil {
		return r, fmt.Errorf("validated plan and complete effects required")
	}
	check := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		return e.Confirm(ctx)
	}
	if err := check(); err != nil {
		return r, err
	}
	start := e.Now()
	if start.IsZero() {
		return r, fmt.Errorf("nonzero clock required")
	}
	last := start
	boundary := p.nextBoundary(start)
	clock := func() (time.Time, error) {
		if err := ctx.Err(); err != nil {
			return time.Time{}, err
		}
		at := e.Now()
		if at.IsZero() || at.Before(last) {
			return at, fmt.Errorf("cleanup clock moved backward/invalid")
		}
		last = at
		if !boundary.IsZero() && !at.Before(boundary) {
			return at, fmt.Errorf("current membership boundary crossed; retry cleanup")
		}
		return at, nil
	}
	dump := func() ([]Flow, error) {
		var all []Flow
		for _, family := range []uint8{FamilyV4, FamilyV6} {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			flows, err := e.Dump(ctx, family)
			if err != nil {
				return nil, fmt.Errorf("complete family %d dump: %w", family, err)
			}
			if len(flows) > MaxFlows-len(all) {
				return nil, fmt.Errorf("flow snapshot exceeds bound")
			}
			for _, f := range flows {
				if f.Family != family {
					return nil, fmt.Errorf("dump returned wrong flow family")
				}
			}
			all = append(all, flows...)
		}
		return all, nil
	}
	flows, err := dump()
	if err != nil {
		return r, err
	}
	at, err := clock()
	if err != nil {
		return r, err
	}
	selected, err := p.Select(flows, at)
	if err != nil {
		return r, err
	}
	r.Selected = len(selected)
	var problems []error
	for _, f := range selected {
		if err = check(); err != nil {
			return r, errors.Join(append(problems, err)...)
		}
		at, err = clock()
		if err != nil {
			return r, errors.Join(append(problems, err)...)
		}
		if !p.withdrawn(f, at) {
			continue
		}
		err = e.Delete(ctx, f)
		if err == nil {
			r.Deleted++
		} else if errors.Is(err, syscall.ENOENT) {
			r.Absent++
		} else {
			problems = append(problems, fmt.Errorf("delete family %d: %w", f.Family, err))
		}
	}
	flows, err = dump()
	if err != nil {
		return r, errors.Join(append(problems, err)...)
	}
	if err = check(); err != nil {
		return r, errors.Join(append(problems, err)...)
	}
	at, err = clock()
	if err != nil {
		return r, errors.Join(append(problems, err)...)
	}
	remaining, err := p.Select(flows, at)
	if err != nil {
		return r, errors.Join(append(problems, err)...)
	}
	r.Remaining = len(remaining)
	if r.Remaining != 0 {
		problems = append(problems, fmt.Errorf("%d withdrawn owned flows remain", r.Remaining))
	}
	return r, errors.Join(problems...)
}
