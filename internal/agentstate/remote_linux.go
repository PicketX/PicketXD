// SPDX-License-Identifier: GPL-3.0-or-later
package agentstate

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	agentv1 "github.com/PicketX/PicketX/pkg/protocol/agent/v1alpha1"
	"github.com/PicketX/PicketXD/internal/remotepolicy"
	"google.golang.org/protobuf/proto"
)

type RemoteConfig struct {
	Address, TokenFile, CAFile, ServerName string
	Plaintext                              bool
}
type remoteEnvelope struct {
	revision uint64
	raw      []byte
}

func (s *Store) providerType() string {
	if s.cfg.Remote != nil {
		return "controller"
	}
	return "static"
}
func (s *Store) cacheVersion() int {
	if s.cfg.Remote != nil {
		return 2
	}
	return 1
}
func (s *Store) providerKey() string {
	if s.cfg.Remote == nil {
		return ""
	}
	data, _ := json.Marshal(s.cfg.Remote)
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}
func (s *Store) verifyRemoteRecord(r cacheRecord) error {
	var snap agentv1.Snapshot
	if e := proto.Unmarshal(r.Remote, &snap); e != nil {
		return e
	}
	if snap.Revision != r.Generation {
		return fmt.Errorf("remote cache revision mismatch")
	}
	policy, e := remotepolicy.Convert(&snap, s.cfg.Facts)
	if e != nil {
		return e
	}
	if !bytes.Equal(policy, r.Policy) {
		return fmt.Errorf("remote cache policy mismatch")
	}
	return nil
}
func (s *Store) AcceptSnapshot(ctx context.Context, snap *agentv1.Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.check(ctx); e != nil {
		return e
	}
	if s.cfg.Remote == nil {
		return fmt.Errorf("Controller publication disabled for static provider")
	}
	reject := func(e error) error {
		s.lastError = "remote snapshot rejected"
		s.lastAttempt = s.cfg.Now().Round(0).UTC()
		return e
	}
	policy, e := remotepolicy.Convert(snap, s.cfg.Facts)
	if e != nil {
		return reject(e)
	}
	raw, e := (proto.MarshalOptions{Deterministic: true}).Marshal(snap)
	if e != nil {
		return reject(e)
	}
	if snap.Revision < s.record.Generation {
		return reject(fmt.Errorf("remote revision regressed"))
	}
	if snap.Revision == s.record.Generation && !bytes.Equal(raw, s.record.Remote) {
		return reject(fmt.Errorf("remote revision equivocation"))
	}
	if e = s.acceptLocked(ctx, policy, writeJournal, &remoteEnvelope{snap.Revision, raw}); e != nil {
		return e
	}
	s.remoteState = "current"
	return nil
}
