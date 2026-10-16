// SPDX-License-Identifier: GPL-3.0-or-later
package agentstate

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestSnapshotAssociatesImmutableCandidate(t *testing.T) {
	c := testConfig(t)
	s := open(t, c)
	st, p, e := s.Snapshot(c.Now())
	if e != nil || p != nil || st.Prepared {
		t.Fatal(st, p, e)
	}
	if e = s.Accept(context.Background(), policy(t)); e != nil {
		t.Fatal(e)
	}
	st, p, e = s.Snapshot(c.Now())
	if e != nil || p.Hash() != st.PolicyHash || st.Generation != 1 {
		t.Fatal(st, e)
	}
	original := p.Hash()
	policyBytes := policy(t)
	done := make(chan error, 1)
	go func() {
		for i := 0; i < 12; i++ {
			raw := strings.ReplaceAll(string(policyBytes), "grant-browser-v1", fmt.Sprintf("grant-browser-%d", i))
			if e := s.Accept(context.Background(), []byte(raw)); e != nil {
				done <- e
				return
			}
		}
		done <- nil
	}()
	for i := 0; i < 200; i++ {
		snap, current, e := s.Snapshot(c.Now())
		if e != nil || current.Hash() != snap.PolicyHash {
			t.Fatal("torn snapshot", snap, e)
		}
	}
	if e := <-done; e != nil {
		t.Fatal(e)
	}
	if p.Hash() != original || st.Generation != 1 {
		t.Fatal("previous snapshot mutated")
	}
	if _, _, e = s.Snapshot(time.Time{}); e == nil {
		t.Fatal("zero time accepted")
	}
	s.Close()
	if _, _, e = s.Snapshot(c.Now()); e == nil {
		t.Fatal("closed snapshot")
	}
}
