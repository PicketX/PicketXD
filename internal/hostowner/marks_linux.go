// SPDX-License-Identifier: GPL-3.0-or-later
package hostowner

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/PicketX/PicketXD/internal/markstate"
)

type MarkStatus struct {
	State      markstate.State `json:"state"`
	ObservedAt time.Time       `json:"observedAt"`
}
type markObserver interface {
	OccupiedMarks(context.Context, markstate.Range) ([]uint32, error)
}

// ReconcileMarks reserves complete marks durably before returning them. It
// neither writes conntrack marks nor authorizes kernel filtering or cleanup.
func (s *Session) ReconcileMarks(ctx context.Context, reservation markstate.Range, targets []markstate.Target) (MarkStatus, error) {
	if s == nil {
		return MarkStatus{}, fmt.Errorf("ownership session required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.check(ctx); err != nil {
		return MarkStatus{}, err
	}
	for _, target := range targets {
		if target.Key.AgentRef != s.record.AgentRef {
			return MarkStatus{}, fmt.Errorf("mark target Agent differs from owner")
		}
	}
	// Validate the complete candidate before native observations, but do not
	// publish this speculative result or return assignments yet.
	if _, err := markstate.Reconcile(s.record.Marks, reservation, targets, nil); err != nil {
		return MarkStatus{}, err
	}
	observer, ok := s.backend.(markObserver)
	if !ok || s.persist == nil {
		return MarkStatus{}, fmt.Errorf("durable namespace-bound mark observer required")
	}
	if _, err := s.verifyLocked(ctx); err != nil {
		return MarkStatus{}, err
	}
	occupied, err := observer.OccupiedMarks(ctx, reservation)
	observedAt := time.Now().Round(0).UTC()
	if err != nil {
		return MarkStatus{}, fmt.Errorf("observe reserved conntrack marks: %w", err)
	}
	next, err := markstate.Reconcile(s.record.Marks, reservation, targets, occupied)
	if err != nil {
		return MarkStatus{}, err
	}
	if _, err = s.verifyLocked(ctx); err != nil {
		return MarkStatus{}, err
	}
	if s.record.Marks == nil || !reflect.DeepEqual(*s.record.Marks, next) {
		updated := s.record
		stored := next.Clone()
		updated.Version = 2
		updated.Marks = &stored
		if err = s.check(ctx); err != nil {
			return MarkStatus{}, err
		}
		if err = s.persist(updated); err != nil {
			s.faulted = true
			s.verified = false
			return MarkStatus{}, fmt.Errorf("persist mark ledger; close and reopen: %w", err)
		}
		s.record = updated
	}
	if err = s.check(ctx); err != nil {
		return MarkStatus{}, err
	}
	return MarkStatus{State: next, ObservedAt: observedAt}, nil
}
func (b *nftKernel) OccupiedMarks(ctx context.Context, reservation markstate.Range) ([]uint32, error) {
	observer, ok := b.reader.(markObserver)
	if !ok {
		return nil, fmt.Errorf("strict namespace-bound conntrack reader required")
	}
	return observer.OccupiedMarks(ctx, reservation)
}
