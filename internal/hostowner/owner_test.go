// SPDX-License-Identifier: GPL-3.0-or-later
package hostowner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type memoryKernel struct {
	observation           observation
	creates               int
	observeErr, createErr error
	closed                bool
	afterCreate           func()
}

func TestOwnershipObservationPrecedesPublication(t *testing.T) {
	c, e, _ := setup(t)
	var publishing time.Time
	e.save = func(path string, r record) error {
		if r.Phase == "confirmed" {
			publishing = time.Now()
		}
		return writeRecord(path, r)
	}
	s := mustOpen(t, c, e)
	defer s.Close()
	if publishing.IsZero() || s.Status().ObservedAt.After(publishing) {
		t.Fatal("journal publication time falsely reported as live observation")
	}
}

func (b *memoryKernel) Observe(context.Context) (observation, error) {
	return b.observation, b.observeErr
}
func (b *memoryKernel) Create(_ context.Context, m string) error {
	b.creates++
	if b.createErr != nil {
		return b.createErr
	}
	b.observation = observation{Exists: true, Marker: m}
	if b.afterCreate != nil {
		b.afterCreate()
	}
	return nil
}
func (b *memoryKernel) Close() error { b.closed = true; return nil }
func setup(t *testing.T) (Config, environment, *memoryKernel) {
	t.Helper()
	d := t.TempDir()
	b := &memoryKernel{}
	return Config{StatePath: filepath.Join(d, "state", "owner.json"), ProviderID: "static", AgentRef: "node", RuntimeKey: "scope"}, environment{namespace: namespace{BootID: "boot-1", Device: 4, Inode: 42}, lockRoot: filepath.Join(d, "run"), backend: b}, b
}
func mustOpen(t *testing.T, c Config, e environment) *Session {
	t.Helper()
	s, err := openWith(context.Background(), c, e)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func TestFreshClaimAndReopen(t *testing.T) {
	c, e, b := setup(t)
	s := mustOpen(t, c, e)
	st := s.Status()
	if !st.OwnershipConfirmed || st.EnforcementConfirmed || st.OwnerID == "" || b.creates != 1 {
		t.Fatalf("unexpected claim: %+v creates=%d", st, b.creates)
	}
	r, exists, err := readRecord(c.StatePath)
	if err != nil || !exists || r.Phase != "confirmed" {
		t.Fatalf("journal %+v %v %v", r, exists, err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	if !b.observation.Exists {
		t.Fatal("close removed kernel ownership")
	}
	b.closed = false
	s = mustOpen(t, c, e)
	defer s.Close()
	if s.Status().OwnerID != st.OwnerID || b.creates != 1 {
		t.Fatal("restart rewrote ownership")
	}
	b.observation.Marker = "foreign"
	if _, err = s.Verify(context.Background()); err == nil || s.Status().OwnershipConfirmed {
		t.Fatal("drift still reported owned")
	}
}
func TestForeignAndDriftRefused(t *testing.T) {
	for _, which := range []string{"no-journal", "foreign-marker", "missing-table", "wrong-provider", "wrong-runtime"} {
		t.Run(which, func(t *testing.T) {
			c, e, b := setup(t)
			if which != "no-journal" {
				s := mustOpen(t, c, e)
				s.Close()
			}
			before := b.creates
			switch which {
			case "no-journal":
				b.observation = observation{Exists: true, Marker: "foreign"}
			case "foreign-marker":
				b.observation.Marker = "foreign"
			case "missing-table":
				b.observation = observation{}
			case "wrong-provider":
				c.ProviderID = "other"
			case "wrong-runtime":
				c.RuntimeKey = "other"
			}
			s, err := openWith(context.Background(), c, e)
			if s != nil {
				s.Close()
			}
			if err == nil || b.creates != before {
				t.Fatalf("unsafe acceptance: %v creates=%d", err, b.creates)
			}
		})
	}
}
func TestCrashBoundaries(t *testing.T) {
	for _, point := range []string{"pending-save", "uncertain-pending", "dispatch-before", "dispatch-after", "readback", "confirmed-save", "uncertain-confirmed"} {
		t.Run(point, func(t *testing.T) {
			c, e, b := setup(t)
			boom := errors.New(point)
			calls := 0
			e.save = func(path string, r record) error {
				calls++
				if calls == 1 && (point == "pending-save" || point == "uncertain-pending") {
					if point == "uncertain-pending" {
						if err := writeRecord(path, r); err != nil {
							return err
						}
					}
					return boom
				}
				if r.Phase == "confirmed" && (point == "confirmed-save" || point == "uncertain-confirmed") {
					if point == "uncertain-confirmed" {
						if err := writeRecord(path, r); err != nil {
							return err
						}
					}
					return boom
				}
				return writeRecord(path, r)
			}
			switch point {
			case "dispatch-before":
				b.createErr = boom
			case "dispatch-after":
				b.afterCreate = func() { b.observeErr = boom }
			case "readback":
				b.afterCreate = func() { b.observation.Marker = "foreign" }
			}
			s, err := openWith(context.Background(), c, e)
			if s != nil {
				s.Close()
			}
			if err == nil {
				t.Fatal("fault reported success")
			}
			if (point == "pending-save" || point == "uncertain-pending") && b.creates != 0 {
				t.Fatal("kernel before durable pending")
			}
			if point == "readback" {
				return
			} // Foreign structure must remain refused, never overwritten.
			b.createErr = nil
			b.observeErr = nil
			b.afterCreate = nil
			e.save = nil
			b.closed = false
			s = mustOpen(t, c, e)
			defer s.Close()
			if !s.Status().OwnershipConfirmed {
				t.Fatal("valid pending not recovered")
			}
		})
	}
}
func TestNewBootAndNamespaceIdentity(t *testing.T) {
	for _, which := range []string{"same-boot-new-namespace", "new-boot-absent", "new-boot-present"} {
		t.Run(which, func(t *testing.T) {
			c, e, b := setup(t)
			s := mustOpen(t, c, e)
			old := s.Status()
			s.Close()
			e.namespace.Inode++
			if which != "same-boot-new-namespace" {
				e.namespace.BootID = "boot-2"
			}
			if which == "new-boot-absent" {
				b.observation = observation{}
			}
			s, err := openWith(context.Background(), c, e)
			if which != "new-boot-absent" {
				if s != nil {
					s.Close()
				}
				if err == nil {
					t.Fatal("adopted foreign epoch")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			st := s.Status()
			if st.OwnerID != old.OwnerID || st.Epoch == old.Epoch || st.EnforcementConfirmed {
				t.Fatalf("invalid reboot claim: %+v", st)
			}
		})
	}
}
func TestAlternateStatePathExclusion(t *testing.T) {
	c, e, _ := setup(t)
	s := mustOpen(t, c, e)
	defer s.Close()
	other := c
	other.StatePath = filepath.Join(filepath.Dir(c.StatePath), "other.json")
	if s2, err := openWith(context.Background(), other, e); err == nil {
		s2.Close()
		t.Fatal("alternate state path evaded namespace lock")
	}
	otherEnv := e
	otherEnv.namespace.Inode++
	if s2, err := openWith(context.Background(), c, otherEnv); err == nil {
		s2.Close()
		t.Fatal("another namespace raced same journal")
	}
}
func TestJournalPrivateCanonicalBounded(t *testing.T) {
	for _, which := range []string{"duplicate", "unknown", "null", "oversize", "symlink", "fifo", "public", "hardlink", "parent-symlink"} {
		t.Run(which, func(t *testing.T) {
			c, e, _ := setup(t)
			s := mustOpen(t, c, e)
			s.Close()
			data, _ := os.ReadFile(c.StatePath)
			switch which {
			case "duplicate":
				data = append([]byte(`{"version":1,`), data[1:]...)
				os.WriteFile(c.StatePath, data, 0600)
			case "unknown":
				data = append([]byte(`{"extra":1,`), data[1:]...)
				os.WriteFile(c.StatePath, data, 0600)
			case "null":
				os.WriteFile(c.StatePath, []byte(`null`), 0600)
			case "oversize":
				os.WriteFile(c.StatePath, make([]byte, maxRecordBytes+1), 0600)
			case "symlink":
				os.Rename(c.StatePath, c.StatePath+".real")
				os.Symlink(c.StatePath+".real", c.StatePath)
			case "fifo":
				os.Remove(c.StatePath)
				makeFIFO(t, c.StatePath)
			case "public":
				os.Chmod(c.StatePath, 0644)
			case "hardlink":
				os.Link(c.StatePath, c.StatePath+".link")
			case "parent-symlink":
				dir := filepath.Dir(c.StatePath)
				os.Rename(dir, dir+".real")
				os.Symlink(dir+".real", dir)
			}
			s, err := openWith(context.Background(), c, e)
			if s != nil {
				s.Close()
			}
			if err == nil {
				t.Fatal("unsafe journal accepted")
			}
		})
	}
}
func TestCanceledAndClosed(t *testing.T) {
	c, e, b := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if s, err := openWith(ctx, c, e); err == nil {
		s.Close()
		t.Fatal("canceled open accepted")
	}
	if b.creates != 0 {
		t.Fatal("canceled create")
	}
	s := mustOpen(t, c, e)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Verify(context.Background()); err == nil || s.Status().OwnershipConfirmed {
		t.Fatal("closed session verified")
	}
}
