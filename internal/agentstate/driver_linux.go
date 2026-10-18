// SPDX-License-Identifier: GPL-3.0-or-later
package agentstate

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/PicketX/PicketX/pkg/configuration"
	"golang.org/x/sys/unix"
)

func ReadPolicy(path string) ([]byte, error) {
	f, e := os.OpenFile(path, os.O_RDONLY|unix.O_NONBLOCK, 0)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil {
		return nil, e
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("policy publication must be a regular file")
	}
	raw, e := io.ReadAll(io.LimitReader(f, configuration.MaxBytes+1))
	if e != nil {
		return nil, e
	}
	if len(raw) > configuration.MaxBytes {
		return nil, fmt.Errorf("policy exceeds %d bytes", configuration.MaxBytes)
	}
	return raw, nil
}
func (s *Store) Reload(ctx context.Context, path string) error {
	if s.cfg.Remote != nil {
		return fmt.Errorf("static reload disabled for Controller provider")
	}
	raw, e := ReadPolicy(path)
	if e == nil {
		return s.Accept(ctx, raw)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if check := s.check(ctx); check != nil {
		return check
	}
	s.lastAttempt = s.cfg.Now().Round(0).UTC()
	s.lastError = e.Error()
	return e
}

// Run emits current prepared intent. A boundary never captures a candidate;
// every wake re-reads latest state, independent of publication failures.
func (s *Store) Run(ctx context.Context, path string, poll time.Duration, emit func(Status) error) error {
	if s.cfg.Remote != nil {
		return s.runRemote(ctx, path, poll, emit)
	}
	if poll <= 0 || emit == nil {
		return fmt.Errorf("positive poll and status consumer required")
	}
	s.mu.Lock()
	if e := s.check(ctx); e != nil {
		s.mu.Unlock()
		return e
	}
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("preparation loop already running")
	}
	s.running = true
	s.mu.Unlock()
	defer func() { s.mu.Lock(); s.running = false; s.mu.Unlock() }()
	loadErr := s.Reload(ctx, path)
	if loadErr != nil && s.Current() == nil {
		return loadErr
	}
	nextPoll := s.cfg.Now().Add(poll)
	for {
		if e := ctx.Err(); e != nil {
			return e
		}
		now := s.cfg.Now()
		st, e := s.Status(now)
		if e != nil {
			return e
		}
		if e = emit(st); e != nil {
			return e
		}
		if st.CacheFault {
			return fmt.Errorf("cache durability is uncertain; reopen and recover")
		}
		// Emission can synchronously accept a newer candidate. The wake channel
		// ensures it is observed before a timer based on this projection fires.
		wait := max(time.Duration(0), nextPoll.Sub(s.cfg.Now()))
		if next := st.Projection.NextChange; !next.IsZero() {
			wait = min(wait, max(time.Duration(0), next.Sub(s.cfg.Now())))
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-s.wake:
			timer.Stop()
			continue
		case <-timer.C:
			if !s.cfg.Now().Before(nextPoll) {
				loadErr = s.Reload(ctx, path)
				if loadErr != nil && s.Current() == nil {
					return loadErr
				}
				nextPoll = s.cfg.Now().Add(poll)
			}
		}
	}
}
