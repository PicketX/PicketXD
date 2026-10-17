// SPDX-License-Identifier: GPL-3.0-or-later
package hostowner

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/PicketX/PicketX/pkg/networkplan"
	"math"
	"reflect"
	"time"

	"github.com/PicketX/PicketXD/internal/hostpolicy"
	"github.com/PicketX/PicketXD/internal/hostrules"
	"github.com/PicketX/PicketXD/internal/markstate"
)

type applicationRecord struct {
	History   *cleanupHistory    `json:"history,omitempty"`
	Baseline  *networkplan.Input `json:"baseline,omitempty"`
	Attempt   uint64             `json:"attempt"`
	Confirmed *hostrules.Image   `json:"confirmed,omitempty"`
	Pending   *hostrules.Image   `json:"pending,omitempty"`
}
type ApplicationStatus struct {
	Phase                string    `json:"phase"`
	Attempt              uint64    `json:"attempt"`
	ImageDigest          string    `json:"imageDigest,omitempty"`
	EnforcementConfirmed bool      `json:"enforcementConfirmed"`
	ObservedAt           time.Time `json:"observedAt"`
}
type imageKernel interface {
	ConfigureImages(string, []hostrules.Image)
	ApplyImage(context.Context, string, hostrules.Image) error
}

func validateApplication(r record) error {
	if r.Application == nil {
		return nil
	}
	a := r.Application
	if a.Baseline != nil {
		data, e := json.Marshal(a.Baseline)
		if e != nil || len(data) > hostrules.MaxImageBytes {
			return fmt.Errorf("invalid application baseline size")
		}
		plan, e := networkplan.Compile(*a.Baseline)
		if e != nil {
			return e
		}
		if a.Baseline.AuthorityID != r.ProviderID {
			return fmt.Errorf("baseline authority differs from owner")
		}
		if _, e = plan.Inventory(r.AgentRef); e != nil {
			return e
		}
	}
	if a.Confirmed != nil && (a.Baseline == nil || !reflect.DeepEqual(*a.Baseline, a.Confirmed.Input)) {
		return fmt.Errorf("confirmed image differs from replacement baseline")
	}
	if (a.Confirmed != nil || a.Pending != nil) && a.Attempt == 0 {
		return fmt.Errorf("application image without attempt identity")
	}
	for _, im := range []*hostrules.Image{a.Confirmed, a.Pending} {
		if im == nil {
			continue
		}
		if e := im.Validate(); e != nil {
			return e
		}
		if im.AuthorityID != r.ProviderID || im.Context.AgentRef != r.AgentRef {
			return fmt.Errorf("application scope differs from owner")
		}
		for _, target := range im.Targets {
			found := false
			for _, entry := range r.Marks.Entries {
				if entry.Target == target.Identity && entry.Mark == target.Mark {
					found = true
				}
			}
			if !found {
				return fmt.Errorf("application mark missing from durable history")
			}
		}
	}
	return validateHistory(r)
}
func confirmedDigest(r record) string {
	if r.Application != nil && r.Application.Confirmed != nil {
		return r.Application.Confirmed.Digest()
	}
	return ""
}
func configureImages(k kernel, r record, newBoot bool) error {
	b, ok := k.(imageKernel)
	if !ok {
		if r.Application != nil {
			return fmt.Errorf("application-aware backend required")
		}
		return nil
	}
	var images []hostrules.Image
	if !newBoot && r.Application != nil {
		if r.Application.Pending != nil {
			images = append(images, *r.Application.Pending)
		}
		if r.Application.Confirmed != nil {
			images = append(images, *r.Application.Confirmed)
		}
	}
	b.ConfigureImages(markerFor(r), images)
	return nil
}
func recoverApplication(r *record, live observation, newBoot bool, save func(string, record) error, path string) error {
	if r.Application == nil {
		return nil
	}
	if newBoot {
		return nil
	}
	a := *r.Application
	want := confirmedDigest(*r)
	if a.Pending != nil && live.Image == a.Pending.Digest() {
		a.Confirmed = a.Pending
		a.Baseline = &a.Pending.Input
		if r.Version == 4 {
			_, next, e := historiesFor(r.Application, *a.Pending)
			if e != nil {
				return e
			}
			a.History = &next
		}
		a.Pending = nil
	} else if live.Image == want {
		a.Pending = nil
	} else {
		return fmt.Errorf("live application differs from recorded previous and pending images")
	}
	if r.Application.Pending != nil {
		updated := *r
		updated.Application = &a
		if e := save(path, updated); e != nil {
			return fmt.Errorf("persist recovered application: %w", e)
		}
		*r = updated
	}
	return nil
}
func (s *Session) applicationStatus() ApplicationStatus {
	st := ApplicationStatus{Phase: "none", ObservedAt: s.observedAt}
	if s.record.Application != nil {
		st.Attempt = s.record.Application.Attempt
		st.ImageDigest = confirmedDigest(s.record)
		if st.ImageDigest != "" {
			st.Phase = "confirmed"
		}
		if s.record.Application.Pending != nil {
			st.Phase = "unknown"
			st.ImageDigest = s.record.Application.Pending.Digest()
		}
	}
	st.EnforcementConfirmed = s.applicationVerified && s.verified && !s.closed && !s.faulted
	if (s.faulted || !s.verified || s.closed) && st.Phase == "confirmed" {
		st.Phase = "unknown"
	}
	if s.faulted {
		st.Phase = "unknown"
	}
	return st
}
func (s *Session) ApplicationStatus() ApplicationStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.applicationStatus()
}

// ApplyPolicy reserves target marks and durably records an attempt before any
// atomic nft transaction. Only complete live readback plus durable publication
// returns enforcement confirmation. It never performs conntrack deletion.
func (s *Session) ApplyPolicy(ctx context.Context, p *hostpolicy.Policy, reservation markstate.Range) (ApplicationStatus, error) {
	if s == nil {
		return ApplicationStatus{}, fmt.Errorf("ownership session required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.check(ctx); e != nil {
		return s.applicationStatus(), e
	}
	backend, ok := s.backend.(imageKernel)
	if !ok {
		return s.applicationStatus(), fmt.Errorf("application-aware backend required")
	}
	targets, e := p.MarkTargets()
	if e != nil {
		return s.applicationStatus(), e
	}
	speculative, e := markstate.Reconcile(s.record.Marks, reservation, targets, nil)
	if e != nil {
		return s.applicationStatus(), e
	}
	im, e := hostrules.Build(p, speculative)
	if e != nil {
		return s.applicationStatus(), e
	}
	if im.AuthorityID != s.record.ProviderID || im.Context.AgentRef != s.record.AgentRef {
		return s.applicationStatus(), fmt.Errorf("policy scope differs from owner")
	}
	if s.record.Application != nil && s.record.Application.Baseline != nil {
		previous, e := networkplan.Compile(*s.record.Application.Baseline)
		if e != nil {
			return s.applicationStatus(), e
		}
		if _, e = networkplan.Replace(previous, im.Input); e != nil {
			return s.applicationStatus(), fmt.Errorf("invalid policy replacement: %w", e)
		}
	}
	if s.record.Application != nil && s.record.Application.Attempt == math.MaxUint64 {
		return s.applicationStatus(), fmt.Errorf("application attempt counter exhausted")
	}
	beforeHistory, candidateHistory, e := historiesFor(s.record.Application, im)
	if e != nil {
		return s.applicationStatus(), e
	}
	if e = candidateHistory.State.Validate(speculative); e != nil {
		return s.applicationStatus(), e
	}
	if _, e = s.verifyLocked(ctx); e != nil {
		return s.applicationStatus(), e
	}
	if _, e = s.reconcileMarksLocked(ctx, reservation, targets, true); e != nil {
		return s.applicationStatus(), e
	}
	if im.Digest() == confirmedDigest(s.record) {
		if s.record.Version < 4 {
			upgraded := s.record
			app := *s.record.Application
			app.History = &candidateHistory
			upgraded.Version = 4
			upgraded.Application = &app
			if e = s.persist(upgraded); e != nil {
				s.faulted = true
				s.verified = false
				s.applicationVerified = false
				return s.applicationStatus(), fmt.Errorf("persist legacy history migration; reopen: %w", e)
			}
			s.record = upgraded
		}
		return s.applicationStatus(), nil
	}
	a := applicationRecord{Pending: &im, History: &beforeHistory}
	if old := s.record.Application; old != nil {
		a.Attempt = old.Attempt
		a.Confirmed = old.Confirmed
		a.Baseline = old.Baseline
	}
	a.Attempt++
	updated := s.record
	updated.Version = 4
	updated.Application = &a
	fail := func(err error) (ApplicationStatus, error) {
		s.faulted = true
		s.verified = false
		s.applicationVerified = false
		return s.applicationStatus(), err
	}
	if e = s.persist(updated); e != nil {
		return fail(fmt.Errorf("persist pending application; reopen to inspect: %w", e))
	}
	s.record = updated
	s.applicationVerified = false
	configureImages(s.backend, s.record, false)
	if e = s.check(ctx); e != nil {
		return fail(e)
	}
	if e = backend.ApplyImage(ctx, markerFor(s.record), im); e != nil {
		return fail(fmt.Errorf("application transaction uncertain; reopen to inspect: %w", e))
	}
	if e = s.check(ctx); e != nil {
		return fail(e)
	}
	live, e := s.backend.Observe(ctx)
	observed := time.Now().Round(0).UTC()
	if e != nil {
		return fail(fmt.Errorf("application readback: %w", e))
	}
	if !live.Exists || live.Marker != markerFor(s.record) || live.Image != im.Digest() {
		return fail(fmt.Errorf("application readback mismatch"))
	}
	if e = s.check(ctx); e != nil {
		return fail(e)
	}
	confirmed := a
	confirmed.Confirmed = &im
	confirmed.Baseline = &im.Input
	confirmed.Pending = nil
	confirmed.History = &candidateHistory
	updated.Application = &confirmed
	if e = s.persist(updated); e != nil {
		return fail(fmt.Errorf("persist confirmed application; reopen to inspect: %w", e))
	}
	s.record = updated
	configureImages(s.backend, s.record, false)
	s.verified = true
	s.applicationVerified = true
	s.observedAt = observed
	return s.applicationStatus(), nil
}
