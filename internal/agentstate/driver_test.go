// SPDX-License-Identifier: GPL-3.0-or-later
package agentstate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunExpiresCurrentStateDespiteInvalidReload(t *testing.T) {
	c := testConfig(t)
	c.Now = time.Now
	s := open(t, c)
	deadline := time.Now().Add(220 * time.Millisecond).UTC()
	raw := strings.Replace(string(policy(t)), "2026-10-15T10:00:00Z", deadline.Format(time.RFC3339Nano), 1)
	path := filepath.Join(t.TempDir(), "policy.yaml")
	if e := os.WriteFile(path, []byte(raw), 0600); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	sawLive, sawExpired := false, false
	e := s.Run(ctx, path, 20*time.Millisecond, func(st Status) error {
		if len(st.Projection.Allow) > 0 {
			sawLive = true
			if e := os.WriteFile(path, []byte("invalid"), 0600); e != nil {
				return e
			}
		}
		if sawLive && st.LastError != "" && !time.Now().Before(deadline) && len(st.Projection.Allow) == 0 {
			sawExpired = true
			cancel()
		}
		return nil
	})
	if !errors.Is(e, context.Canceled) || !sawLive || !sawExpired {
		t.Fatalf("loop froze/renewed expiry: live=%v expired=%v err=%v", sawLive, sawExpired, e)
	}
}
func TestDriverUsesLatestGenerationAndCancellation(t *testing.T) {
	c := testConfig(t)
	c.Now = time.Now
	s := open(t, c)
	deadline := time.Now().Add(160 * time.Millisecond).UTC()
	initial := strings.Replace(string(policy(t)), "2026-10-15T10:00:00Z", deadline.Format(time.RFC3339Nano), 1)
	path := filepath.Join(t.TempDir(), "policy.yaml")
	os.WriteFile(path, []byte(initial), 0600)
	next := strings.Replace(initial, "subject: {type: source, cidr: 198.51.100.9}", "revoked: true\n  subject: {type: source, cidr: 198.51.100.9}", 1)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	updated := false
	sawLatest := false
	e := s.Run(ctx, path, 25*time.Millisecond, func(st Status) error {
		if !updated {
			updated = true
			if e := os.WriteFile(path, []byte(next), 0600); e != nil {
				return e
			}
			return s.Accept(ctx, []byte(next))
		}
		if st.Generation == 2 && len(st.Projection.Allow) == 0 {
			sawLatest = true
			if time.Now().After(deadline) {
				cancel()
			}
		}
		if st.Generation != 2 || len(st.Projection.Allow) != 0 {
			return errors.New("old timer revived previous candidate")
		}
		return nil
	})
	if !sawLatest || !errors.Is(e, context.Canceled) {
		t.Fatalf("latest/cancel %v %v", sawLatest, e)
	}
	// Missing publication can fall back only when a valid cache already exists.
	s.Close()
	restored := open(t, c)
	ctx2, stop := context.WithCancel(context.Background())
	e = restored.Run(ctx2, path+"-missing", time.Second, func(st Status) error {
		if !st.Restored || st.LastError == "" {
			t.Fatal("fallback hid reload error")
		}
		stop()
		return nil
	})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	fresh := open(t, testConfig(t))
	if e = fresh.Run(context.Background(), path+"-missing", time.Second, func(Status) error { return nil }); e == nil {
		t.Fatal("bootstrap accepted missing publication")
	}
}
