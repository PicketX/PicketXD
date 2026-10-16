// SPDX-License-Identifier: GPL-3.0-or-later
// Package hostowner owns namespace-bound host admission and its durable recovery journal.
package hostowner

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/PicketX/PicketXD/internal/markstate"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type Config struct{ StatePath, ProviderID, AgentRef, RuntimeKey string }
type namespace struct {
	BootID        string
	Device, Inode uint64
}
type observation struct {
	Exists bool
	Marker string
	Image  string
}
type kernel interface {
	Observe(context.Context) (observation, error)
	Create(context.Context, string) error
	Close() error
}
type environment struct {
	namespace  namespace
	lockRoot   string
	backend    kernel
	save       func(string, record) error
	revalidate func() error
	release    func() error
}
type record struct {
	Application *applicationRecord `json:"application,omitempty"`
	Marks       *markstate.State   `json:"marks,omitempty"`
	Version     int                `json:"version"`
	Phase       string             `json:"phase"`
	ProviderID  string             `json:"providerID"`
	AgentRef    string             `json:"agentRef"`
	RuntimeKey  string             `json:"runtimeKey"`
	OwnerID     string             `json:"ownerID"`
	Epoch       string             `json:"epoch"`
	BootID      string             `json:"bootID"`
	Device      uint64             `json:"device"`
	Inode       uint64             `json:"inode"`
}
type Status struct {
	OwnershipConfirmed   bool      `json:"ownershipConfirmed"`
	EnforcementConfirmed bool      `json:"enforcementConfirmed"`
	OwnerID              string    `json:"ownerID"`
	Epoch                string    `json:"epoch"`
	ObservedAt           time.Time `json:"observedAt"`
}
type Session struct {
	mu                  sync.Mutex
	record              record
	backend             kernel
	revalidate          func() error
	releases            []func() error
	closed, verified    bool
	faulted             bool
	applicationVerified bool
	persist             func(record) error
	observedAt          time.Time
}

func scopeRecord(c Config, n namespace) record {
	return record{Version: 1, ProviderID: c.ProviderID, AgentRef: c.AgentRef, RuntimeKey: c.RuntimeKey, BootID: n.BootID, Device: n.Device, Inode: n.Inode, Phase: "pending"}
}
func randomID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(id[:]), nil
}
func bounded(s string) bool {
	return s != "" && len(s) <= 256 && utf8.ValidString(s) && !strings.ContainsRune(s, 0)
}
func hexID(s string) bool {
	b, err := hex.DecodeString(s)
	return err == nil && len(b) == 16 && hex.EncodeToString(b) == s
}
func validRecord(r record) error {
	if (r.Version < 1 || r.Version > 3) || (r.Version >= 2) != (r.Marks != nil) || (r.Version == 3) != (r.Application != nil) || (r.Phase != "pending" && r.Phase != "confirmed") || !bounded(r.ProviderID) || !bounded(r.AgentRef) || !bounded(r.RuntimeKey) || !bounded(r.BootID) || r.Device == 0 || r.Inode == 0 || !hexID(r.OwnerID) || !hexID(r.Epoch) {
		return fmt.Errorf("invalid ownership journal identity/version/phase")
	}
	if r.Marks != nil {
		if err := r.Marks.Validate(); err != nil {
			return fmt.Errorf("invalid ownership mark ledger: %w", err)
		}
		for _, assignment := range r.Marks.Entries {
			if assignment.Target.Key.AgentRef != r.AgentRef {
				return fmt.Errorf("mark ledger Agent differs from owner")
			}
		}
	}
	return validateApplication(r)
}
func markerFor(r record) string {
	scope := struct {
		Provider, Agent, Runtime, Boot string
		Device, Inode                  uint64
	}{r.ProviderID, r.AgentRef, r.RuntimeKey, r.BootID, r.Device, r.Inode}
	data, _ := json.Marshal(scope)
	hash := sha256.Sum256(data)
	return fmt.Sprintf("picketx-owner-v1:%s:%s:%x", r.OwnerID, r.Epoch, hash)
}
func openWith(ctx context.Context, c Config, e environment) (out *Session, err error) {
	s := &Session{backend: e.backend, revalidate: e.revalidate}
	if e.release != nil {
		s.releases = append(s.releases, e.release)
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, s.Close())
		}
	}()
	if e.backend == nil {
		return nil, fmt.Errorf("kernel backend required")
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if !filepath.IsAbs(c.StatePath) || !bounded(c.ProviderID) || !bounded(c.AgentRef) || !bounded(c.RuntimeKey) || !bounded(e.namespace.BootID) || e.namespace.Device == 0 || e.namespace.Inode == 0 {
		return nil, fmt.Errorf("absolute state path and explicit ownership scope required")
	}
	c.StatePath = filepath.Clean(c.StatePath)
	if filepath.Base(c.StatePath) == "/" {
		return nil, fmt.Errorf("state path must name a regular file")
	}
	releases, journal, err := acquireLocks(c.StatePath, e.lockRoot, e.namespace)
	if err != nil {
		return nil, err
	}
	s.releases = append(s.releases, releases...)
	r, exists, err := journal.read()
	if err != nil {
		return nil, err
	}
	newBoot := false
	if exists {
		if err = validRecord(r); err != nil {
			return nil, err
		}
		if r.ProviderID != c.ProviderID || r.AgentRef != c.AgentRef || r.RuntimeKey != c.RuntimeKey {
			return nil, fmt.Errorf("ownership scope mismatch")
		}
		newBoot = r.BootID != e.namespace.BootID
		if !newBoot && (r.Device != e.namespace.Device || r.Inode != e.namespace.Inode) {
			return nil, fmt.Errorf("same-boot namespace identity mismatch")
		}
	}
	if err = s.check(ctx); err != nil {
		return nil, err
	}
	if err = configureImages(e.backend, r, newBoot); err != nil {
		return nil, err
	}
	live, err := e.backend.Observe(ctx)
	observedAt := time.Now().Round(0).UTC()
	if err != nil {
		return nil, fmt.Errorf("observe ownership: %w", err)
	}
	if live.Exists && (!exists || newBoot || live.Marker != markerFor(r)) {
		return nil, fmt.Errorf("foreign or unrecognized inet picketx ownership")
	}
	if exists && !newBoot && r.Phase == "confirmed" && !live.Exists {
		return nil, fmt.Errorf("confirmed ownership drift: table absent")
	}
	save := e.save
	if save == nil {
		save = func(_ string, r record) error { return journal.write(r, nil) }
	}
	s.persist = func(r record) error { return save(c.StatePath, r) }
	if !exists || newBoot {
		owner := r.OwnerID
		marks := r.Marks
		application := r.Application
		r = scopeRecord(c, e.namespace)
		if marks != nil {
			r.Version = 2
			r.Marks = marks
		}
		if owner == "" {
			owner, err = randomID()
			if err != nil {
				return nil, err
			}
		}
		if application != nil {
			r.Version = 3
			r.Application = &applicationRecord{Attempt: application.Attempt, Baseline: application.Baseline}
		}
		r.OwnerID = owner
		r.Epoch, err = randomID()
		if err != nil {
			return nil, err
		}
	}
	if !live.Exists {
		r.Phase = "pending"
		if err = s.check(ctx); err != nil {
			return nil, err
		}
		// Rewrite recovered pending too: readable bytes alone do not prove a
		// previous attempt's uncertain fsync completed durably.
		if err = save(c.StatePath, r); err != nil {
			return nil, fmt.Errorf("persist pending ownership: %w", err)
		}
		if err = s.check(ctx); err != nil {
			return nil, err
		}
		if err = e.backend.Create(ctx, markerFor(r)); err != nil {
			return nil, fmt.Errorf("exclusive ownership create uncertain; reopen to inspect: %w", err)
		}
		if err = s.check(ctx); err != nil {
			return nil, err
		}
		live, err = e.backend.Observe(ctx)
		observedAt = time.Now().Round(0).UTC()
		if err != nil {
			return nil, fmt.Errorf("ownership readback: %w", err)
		}
		if !live.Exists || live.Marker != markerFor(r) {
			return nil, fmt.Errorf("ownership readback mismatch")
		}
	}
	if err = s.check(ctx); err != nil {
		return nil, err
	}
	if err = recoverApplication(&r, live, newBoot, save, c.StatePath); err != nil {
		return nil, err
	}
	if r.Phase != "confirmed" {
		r.Phase = "confirmed"
		if err = save(c.StatePath, r); err != nil {
			return nil, fmt.Errorf("persist confirmed ownership: %w", err)
		}
	}
	if err = s.check(ctx); err != nil {
		return nil, err
	}
	s.record = r
	s.verified = true
	s.applicationVerified = r.Application != nil && r.Application.Confirmed != nil && live.Image == r.Application.Confirmed.Digest()
	configureImages(s.backend, r, false)
	s.observedAt = observedAt
	return s, nil
}
func (s *Session) check(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.faulted {
		return fmt.Errorf("ownership journal durability is uncertain; close and reopen")
	}
	if s.closed {
		return fmt.Errorf("ownership session closed")
	}
	if s.revalidate != nil {
		return s.revalidate()
	}
	return nil
}
func (s *Session) status() Status {
	return Status{OwnershipConfirmed: s.verified && !s.closed && !s.faulted, EnforcementConfirmed: s.verified && s.applicationVerified && !s.closed && !s.faulted, OwnerID: s.record.OwnerID, Epoch: s.record.Epoch, ObservedAt: s.observedAt}
}
func (s *Session) Status() Status { s.mu.Lock(); defer s.mu.Unlock(); return s.status() }
func (s *Session) Verify(ctx context.Context) (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.verifyLocked(ctx)
}
func (s *Session) verifyLocked(ctx context.Context) (Status, error) {
	s.verified = false
	s.applicationVerified = false
	if err := s.check(ctx); err != nil {
		return s.status(), err
	}
	live, err := s.backend.Observe(ctx)
	if err != nil {
		return s.status(), err
	}
	if !live.Exists || live.Marker != markerFor(s.record) || live.Image != confirmedDigest(s.record) {
		return s.status(), fmt.Errorf("ownership drift detected")
	}
	if err = s.check(ctx); err != nil {
		return s.status(), err
	}
	s.verified = true
	s.applicationVerified = s.record.Application != nil && s.record.Application.Confirmed != nil
	s.observedAt = time.Now().Round(0).UTC()
	return s.status(), nil
}
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	s.verified = false
	var errs []error
	if s.backend != nil {
		errs = append(errs, s.backend.Close())
	}
	for i := len(s.releases) - 1; i >= 0; i-- {
		errs = append(errs, s.releases[i]())
	}
	return errors.Join(errs...)
}
