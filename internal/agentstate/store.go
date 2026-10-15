// SPDX-License-Identifier: GPL-3.0-or-later
// Package agentstate owns static desired preparation, never applied kernel state.
package agentstate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/PicketX/PicketX/pkg/configuration"
	"github.com/PicketX/PicketX/pkg/networkplan"
)

type Config struct {
	Path, ProviderID, AgentRef, RuntimeKey, BootID string
	Facts                                          configuration.Facts
	Now                                            func() time.Time
}
type Store struct {
	mu                                           sync.Mutex
	cfg                                          Config
	lock                                         *os.File
	current                                      *configuration.Candidate
	record                                       cacheRecord
	restored, sameBoot, closed, faulted, running bool
	lastAttempt                                  time.Time
	lastError                                    string
	wake                                         chan struct{}
}
type Status struct {
	State                string                 `json:"state"`
	Prepared             bool                   `json:"prepared"`
	EnforcementConfirmed bool                   `json:"enforcementConfirmed"`
	ProviderID           string                 `json:"providerID"`
	AgentRef             string                 `json:"agentRef"`
	Generation           uint64                 `json:"generation"`
	PolicyHash           string                 `json:"policyHash"`
	AcceptedAt           time.Time              `json:"acceptedAt"`
	At                   time.Time              `json:"at"`
	LastAttemptAt        time.Time              `json:"lastAttemptAt"`
	LastError            string                 `json:"lastError,omitempty"`
	Restored             bool                   `json:"restored"`
	SameBoot             bool                   `json:"sameBoot"`
	CacheFault           bool                   `json:"cacheFault"`
	Projection           networkplan.Projection `json:"projection"`
}

// New locks and restores before accepting any replacement. Corrupt or foreign
// caches fail startup and cannot be overwritten through a bootstrap path.
func New(c Config) (*Store, error) {
	if c.Now == nil {
		c.Now = time.Now
	}
	if c.Path == "" || c.ProviderID == "" || c.AgentRef == "" || c.RuntimeKey == "" || c.BootID == "" || c.Now().IsZero() {
		return nil, fmt.Errorf("static state requires explicit identity, path, runtime scope and clock")
	}
	if c.Facts.AuthorityID != c.ProviderID || len(c.Facts.Agents) != 1 || c.Facts.Agents[0].Name != c.AgentRef {
		return nil, fmt.Errorf("facts do not match this static authority/Agent")
	}
	data, e := json.Marshal(c.Facts)
	if e != nil {
		return nil, e
	}
	var owned configuration.Facts
	if e = json.Unmarshal(data, &owned); e != nil {
		return nil, e
	}
	c.Facts = owned
	lock, e := lockJournal(c.Path)
	if e != nil {
		return nil, e
	}
	s := &Store{cfg: c, lock: lock, wake: make(chan struct{}, 1), sameBoot: true}
	record, exists, e := readJournal(c.Path)
	if e == nil && exists {
		if record.Version != 1 || record.ProviderType != "static" || record.ProviderID != c.ProviderID || record.AgentRef != c.AgentRef || record.RuntimeKey != c.RuntimeKey || record.BootID == "" || record.Generation == 0 || record.AcceptedAt.IsZero() {
			e = fmt.Errorf("cache identity, scope or version mismatch")
		}
		if e == nil {
			var bundle *configuration.Bundle
			bundle, e = configuration.Decode(record.Policy)
			if e == nil {
				s.current, e = configuration.Compile(bundle, c.Facts)
			}
			if e == nil && s.current.Hash() != record.PolicyHash {
				e = fmt.Errorf("cache policy hash mismatch")
			}
		}
		if e == nil {
			s.record = record
			s.restored = true
			s.sameBoot = record.BootID == c.BootID
		}
	}
	if e != nil {
		s.Close()
		return nil, fmt.Errorf("restore static cache: %w", e)
	}
	return s, nil
}
func (s *Store) Accept(ctx context.Context, policy []byte) error {
	return s.accept(ctx, policy, writeJournal)
}

// accept separates publication from state advancement so filesystem durability
// failures retain their explicit pre/post-publication outcomes.
func (s *Store) accept(ctx context.Context, policy []byte, save func(string, cacheRecord) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.check(ctx); e != nil {
		return e
	}
	s.lastAttempt = s.cfg.Now().Round(0).UTC()
	reject := func(e error) error { s.lastError = e.Error(); return e }
	b, e := configuration.Decode(policy)
	if e != nil {
		return reject(e)
	}
	var next *configuration.Candidate
	if s.current == nil {
		next, e = configuration.Compile(b, s.cfg.Facts)
	} else {
		next, _, e = configuration.Replace(s.current, b, s.cfg.Facts)
	}
	if e != nil {
		return reject(e)
	}
	if s.current != nil && next.Hash() == s.current.Hash() {
		s.lastError = ""
		s.signal()
		return nil
	}
	if s.record.Generation == ^uint64(0) {
		return reject(fmt.Errorf("accepted generation exhausted"))
	}
	r := cacheRecord{Version: 1, ProviderType: "static", ProviderID: s.cfg.ProviderID, AgentRef: s.cfg.AgentRef, RuntimeKey: s.cfg.RuntimeKey, BootID: s.cfg.BootID, Generation: s.record.Generation + 1, AcceptedAt: s.lastAttempt, PolicyHash: next.Hash(), Policy: append([]byte(nil), policy...)}
	if e = ctx.Err(); e != nil {
		return reject(e)
	}
	if e = save(s.cfg.Path, r); e != nil {
		var ce *commitError
		if errors.As(e, &ce) && ce.Published {
			s.faulted = true
		}
		return reject(e)
	}
	s.current = next
	s.record = r
	s.restored = false
	s.sameBoot = true
	s.lastError = ""
	s.signal()
	return nil
}
func (s *Store) check(ctx context.Context) error {
	if e := ctx.Err(); e != nil {
		return e
	}
	if s.closed {
		return fmt.Errorf("static state is closed")
	}
	if s.faulted {
		return fmt.Errorf("cache durability is uncertain; reopen and recover")
	}
	return nil
}
func (s *Store) signal() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}
func (s *Store) Current() *configuration.Candidate {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.current
}
func (s *Store) Status(at time.Time) (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := Status{State: "unprepared", ProviderID: s.cfg.ProviderID, AgentRef: s.cfg.AgentRef, At: at.Round(0).UTC(), Generation: s.record.Generation, PolicyHash: s.record.PolicyHash, AcceptedAt: s.record.AcceptedAt, LastAttemptAt: s.lastAttempt, LastError: s.lastError, Restored: s.restored, SameBoot: s.sameBoot, CacheFault: s.faulted}
	if at.IsZero() {
		return st, fmt.Errorf("status requires explicit time")
	}
	if s.closed {
		return st, fmt.Errorf("static state is closed")
	}
	if s.current != nil {
		p, e := s.current.Plan().Projection(s.cfg.AgentRef, at)
		if e != nil {
			return st, e
		}
		st.Prepared = true
		st.State = "prepared"
		st.Projection = p
	}
	return st, nil
}
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	s.signal()
	return unlockJournal(s.lock)
}
