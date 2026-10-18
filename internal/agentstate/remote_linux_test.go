// SPDX-License-Identifier: GPL-3.0-or-later
package agentstate

import (
	"context"
	"crypto/sha256"
	"github.com/PicketX/PicketX/pkg/agentsnapshot"
	"github.com/PicketX/PicketX/pkg/configuration"
	agentv1 "github.com/PicketX/PicketX/pkg/protocol/agent/v1alpha1"
	"google.golang.org/protobuf/proto"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func remoteFixture(t *testing.T) (Config, *agentv1.Snapshot) {
	t.Helper()
	c := testConfig(t)
	c.Remote = &RemoteConfig{Address: "127.0.0.1:4444", Plaintext: true, TokenFile: "unused"}
	raw, e := os.ReadFile("../../examples/prepare/policy.yaml")
	if e != nil {
		t.Fatal(e)
	}
	b, e := configuration.Decode(raw)
	if e != nil {
		t.Fatal(e)
	}
	p, e := configuration.Compile(b, c.Facts)
	if e != nil {
		t.Fatal(e)
	}
	s, e := agentsnapshot.Build(p, c.AgentRef, 1)
	if e != nil {
		t.Fatal(e)
	}
	return c, s
}
func resign(s *agentv1.Snapshot) {
	s.Sha256 = nil
	b, _ := proto.MarshalOptions{Deterministic: true}.Marshal(s)
	h := sha256.Sum256(b)
	s.Sha256 = h[:]
}
func TestRemoteRevisionCacheAndProviderBinding(t *testing.T) {
	c, snap := remoteFixture(t)
	s, e := New(c)
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	if e = s.AcceptSnapshot(ctx, snap); e != nil {
		t.Fatal(e)
	}
	st, _, _ := s.Snapshot(time.Now())
	if st.Generation != 1 || !st.Prepared {
		t.Fatal(st)
	}
	snap.Revision = 2
	resign(snap)
	if e = s.AcceptSnapshot(ctx, snap); e != nil {
		t.Fatal(e)
	}
	if e = s.AcceptSnapshot(ctx, snap); e != nil {
		t.Fatal("retransmission", e)
	}
	different := proto.Clone(snap).(*agentv1.Snapshot)
	different.Targets = nil
	resign(different)
	if e = s.AcceptSnapshot(ctx, different); e == nil {
		t.Fatal("same-version equivocation accepted")
	}
	different.Revision = 1
	resign(different)
	if e = s.AcceptSnapshot(ctx, different); e == nil {
		t.Fatal("stale revision accepted")
	}
	_ = s.Close()
	s, e = New(c)
	if e != nil {
		t.Fatal(e)
	}
	st, _, _ = s.Snapshot(time.Date(2026, 10, 15, 14, 0, 0, 0, time.UTC))
	if st.Generation != 2 || !st.Restored || len(st.Projection.Allow) != 0 || len(st.Projection.Protected) != 2 {
		t.Fatal(st)
	}
	_ = s.Close()
	changed := c
	copyRemote := *c.Remote
	copyRemote.Address = "127.0.0.1:5555"
	changed.Remote = &copyRemote
	if x, e := New(changed); e == nil {
		x.Close()
		t.Fatal("foreign endpoint adopted cache")
	}
	changed = c
	changed.Remote = nil
	if x, e := New(changed); e == nil {
		x.Close()
		t.Fatal("static adopted remote cache")
	}
	// Canonical journal remains mandatory; corrupt raw envelope cannot be adopted.
	r, _, e := readJournal(c.Path)
	if e != nil {
		t.Fatal(e)
	}
	r.Remote[0] ^= 1
	if e = writeJournal(c.Path, r); e != nil {
		t.Fatal(e)
	}
	if x, e := New(c); e == nil {
		x.Close()
		t.Fatal("corrupt remote envelope restored")
	}
}
func TestRemoteCannotUseStaticReload(t *testing.T) {
	c, snap := remoteFixture(t)
	s, e := New(c)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	raw, _ := os.ReadFile("../../examples/prepare/policy.yaml")
	if e = s.Accept(context.Background(), raw); e == nil {
		t.Fatal("remote mixed static publication")
	}
	if e = s.AcceptSnapshot(context.Background(), snap); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "policy")
	os.WriteFile(path, raw, 0600)
	if e = s.Reload(context.Background(), path); e == nil {
		t.Fatal("remote loaded file")
	}
}
