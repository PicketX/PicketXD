// SPDX-License-Identifier: GPL-3.0-or-later
package hostowner

import (
	"encoding/json"
	"fmt"
	"github.com/PicketX/PicketXD/internal/hostrevoke"
	"github.com/PicketX/PicketXD/internal/hostrules"
)

type cleanupHistory struct {
	Complete bool                `json:"complete"`
	State    *hostrevoke.History `json:"state,omitempty"`
}
type HistoryStatus struct {
	Tracked  bool   `json:"tracked"`
	Complete bool   `json:"complete"`
	Targets  int    `json:"targets"`
	Members  int    `json:"members"`
	Bytes    int    `json:"bytes"`
	Digest   string `json:"digest,omitempty"`
}

func historiesFor(a *applicationRecord, im hostrules.Image) (cleanupHistory, cleanupHistory, error) {
	before := cleanupHistory{Complete: a == nil}
	if a != nil {
		if a.History != nil {
			before = *a.History
		} else if a.Confirmed != nil {
			h, e := hostrevoke.Remember(nil, *a.Confirmed)
			if e != nil {
				return before, before, e
			}
			before.State = &h
		}
	}
	candidate, e := hostrevoke.Remember(before.State, im)
	if e != nil {
		return before, before, e
	}
	after := before
	after.State = &candidate
	return before, after, nil
}
func validateHistory(r record) error {
	a := r.Application
	if a == nil {
		return nil
	}
	if r.Version < 4 {
		if a.History != nil {
			return fmt.Errorf("legacy application cannot contain v4 history")
		}
		return nil
	}
	if a.History == nil {
		return fmt.Errorf("v4 history tracking metadata missing")
	}
	h := a.History
	if h.State != nil {
		if e := h.State.Validate(*r.Marks); e != nil {
			return e
		}
		if h.State.AuthorityID != r.ProviderID || h.State.Context.AgentRef != r.AgentRef {
			return fmt.Errorf("recorded history scope differs from owner")
		}
	}
	if a.Confirmed != nil {
		if h.State == nil {
			return fmt.Errorf("confirmed image without recorded history")
		}
		if e := h.State.Covers(*a.Confirmed); e != nil {
			return e
		}
	}
	if a.Pending != nil {
		_, after, e := historiesFor(a, *a.Pending)
		if e != nil {
			return e
		}
		if e = after.State.Validate(*r.Marks); e != nil {
			return e
		}
	}
	return nil
}
func (s *Session) historyStatus() HistoryStatus {
	st := HistoryStatus{}
	a := s.record.Application
	if a == nil || a.History == nil {
		return st
	}
	st.Tracked = true
	h := a.History
	if h.State != nil {
		st.Targets = len(h.State.Targets)
		for _, t := range h.State.Targets {
			st.Members += len(t.Members)
		}
		b, _ := json.Marshal(h.State)
		st.Bytes = len(b)
		st.Digest = h.State.Digest()
	}
	st.Complete = h.Complete && h.State != nil && a.Confirmed != nil && a.Pending == nil && s.applicationVerified && s.verified && !s.closed && !s.faulted
	return st
}
func (s *Session) HistoryStatus() HistoryStatus {
	if s == nil {
		return HistoryStatus{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.historyStatus()
}
