// SPDX-License-Identifier: GPL-3.0-or-later
package agentstate

import (
	"context"
	"errors"
	"golang.org/x/sys/unix"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PicketX/PicketX/pkg/configuration"
	"github.com/PicketX/PicketX/pkg/networkplan"
)

func testConfig(t *testing.T) Config {
	t.Helper()
	f := configuration.Facts{AuthorityID: "static-test", Agents: []networkplan.Agent{{Name: "agent-a", Capabilities: &networkplan.CapabilityProfile{Classes: []networkplan.TrafficClass{networkplan.HostLocal}, Protocols: []networkplan.Protocol{networkplan.TCP, networkplan.UDP}, Families: []networkplan.Family{networkplan.IPv4, networkplan.IPv6}, Handling: []networkplan.HandlingMode{networkplan.Graceful}, OriginalDestination: true}}}, Contexts: []networkplan.ExecutionContext{{AgentRef: "agent-a", ProfileRef: "host", ID: "host-v1", NetworkSpaceID: "trusted-test-space", Class: networkplan.HostLocal, Stage: networkplan.OriginalDestination, Destinations: []netip.Prefix{netip.MustParsePrefix("192.0.2.100/32"), netip.MustParsePrefix("2001:db8::100/128")}, Handling: networkplan.Graceful}}}
	return Config{Path: filepath.Join(t.TempDir(), "state", "accepted.json"), ProviderID: "static-test", AgentRef: "agent-a", RuntimeKey: "host-scope-v1", BootID: "boot-1", Facts: f, Now: func() time.Time { return time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC) }}
}
func policy(t *testing.T) []byte {
	t.Helper()
	b, e := os.ReadFile("../../examples/prepare/policy.yaml")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func open(t *testing.T, c Config) *Store {
	t.Helper()
	s, e := New(c)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}
func TestAcceptedCacheReplayPreservesAbsoluteDeadlines(t *testing.T) {
	c := testConfig(t)
	s := open(t, c)
	raw := policy(t)
	if e := s.Accept(context.Background(), raw); e != nil {
		t.Fatal(e)
	}
	for i := range raw {
		raw[i] = 'x'
	}
	st, e := s.Status(c.Now())
	if e != nil {
		t.Fatal(e)
	}
	if !st.Prepared || st.EnforcementConfirmed || st.Generation != 1 || len(st.Projection.Allow) != 2 || st.Restored {
		t.Fatalf("bad status %#v", st)
	}
	if e = s.Accept(context.Background(), []byte("invalid")); e == nil {
		t.Fatal("invalid reload accepted")
	}
	at := c.Now().Add(time.Hour)
	st, _ = s.Status(at)
	if st.LastError == "" || len(st.Projection.Allow) != 1 || st.Generation != 1 {
		t.Fatal("failed reload renewed permission or replaced accepted state")
	}
	if e = s.Accept(context.Background(), policy(t)); e != nil {
		t.Fatal(e)
	}
	st, _ = s.Status(at)
	if st.Generation != 1 || st.LastError != "" {
		t.Fatal("equivalent candidate advanced generation or retained error")
	}
	if e = s.Close(); e != nil {
		t.Fatal(e)
	}
	c.BootID = "boot-2"
	restored := open(t, c)
	st, _ = restored.Status(c.Now().Add(3 * time.Hour))
	if !st.Restored || st.SameBoot || st.EnforcementConfirmed || len(st.Projection.Allow) != 0 || len(st.Projection.Protected) != 2 {
		t.Fatalf("cache invented fresh access/apply: %#v", st)
	}
	info, e := os.Stat(c.Path)
	if e != nil {
		t.Fatal(e)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatal("cache not private")
	}
}
func TestCacheIdentityCorruptionAndLocking(t *testing.T) {
	c := testConfig(t)
	s := open(t, c)
	if e := s.Accept(context.Background(), policy(t)); e != nil {
		t.Fatal(e)
	}
	if other, e := New(c); e == nil {
		other.Close()
		t.Fatal("concurrent writer accepted")
	}
	if e := s.Close(); e != nil {
		t.Fatal(e)
	}
	for _, change := range []func(*Config){func(v *Config) { v.ProviderID = "other"; v.Facts.AuthorityID = "other" }, func(v *Config) { v.AgentRef = "other" }, func(v *Config) { v.RuntimeKey = "other" }} {
		alt := c
		change(&alt)
		if other, e := New(alt); e == nil {
			other.Close()
			t.Fatal("mismatched cache restored")
		}
	}
	data, e := os.ReadFile(c.Path)
	if e != nil {
		t.Fatal(e)
	}
	for name, b := range map[string][]byte{"garbage": []byte("["), "duplicate": []byte(strings.Replace(string(data), `"version":1`, `"version":2,"version":1`, 1)), "unknown": []byte(strings.Replace(string(data), `"version":1`, `"version":1,"applied":true`, 1)), "hash": []byte(strings.Replace(string(data), `"policyHash":"`, `"policyHash":"bad`, 1))} {
		t.Run(name, func(t *testing.T) {
			if e := os.WriteFile(c.Path, b, 0600); e != nil {
				t.Fatal(e)
			}
			if other, e := New(c); e == nil {
				other.Close()
				t.Fatal("corrupt cache restored")
			}
		})
	}
}
func TestPersistenceFailureKeepsPreviousCandidate(t *testing.T) {
	c := testConfig(t)
	s := open(t, c)
	if e := s.Accept(context.Background(), policy(t)); e != nil {
		t.Fatal(e)
	}
	before := s.Current().Hash()
	// Replacing a regular cache with a directory forces rename to fail before
	// publication even when tests run as root.
	if e := os.Remove(c.Path); e != nil {
		t.Fatal(e)
	}
	if e := os.Mkdir(c.Path, 0700); e != nil {
		t.Fatal(e)
	}
	next := strings.Replace(string(policy(t)), "10:00:00Z", "09:30:00Z", 1)
	if e := s.Accept(context.Background(), []byte(next)); e == nil {
		t.Fatal("failed publication accepted")
	}
	st, _ := s.Status(c.Now())
	if st.Generation != 1 || s.Current().Hash() != before || st.LastError == "" {
		t.Fatal("accepted state advanced despite failed persistence")
	}
}
func TestPrivateFilesAndClosedStore(t *testing.T) {
	c := testConfig(t)
	target := filepath.Join(t.TempDir(), "target")
	if e := os.WriteFile(target, []byte("x"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.MkdirAll(filepath.Dir(c.Path), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink(target, c.Path); e != nil {
		t.Fatal(e)
	}
	if s, e := New(c); e == nil {
		s.Close()
		t.Fatal("symlink cache accepted")
	}
	if e := os.Remove(c.Path); e != nil {
		t.Fatal(e)
	}
	s := open(t, c)
	s.Close()
	if e := s.Accept(context.Background(), policy(t)); e == nil {
		t.Fatal("closed store accepted")
	}
	c = testConfig(t)
	c.Facts.AuthorityID = "wrong"
	if s, e := New(c); e == nil {
		s.Close()
		t.Fatal("mixed facts authority")
	}
	c = testConfig(t)
	s = open(t, c)
	c.Facts.Contexts[0].Destinations[0] = netip.MustParsePrefix("0.0.0.0/0")
	if e := s.Accept(context.Background(), policy(t)); e != nil {
		t.Fatal("caller changed accepted facts", e)
	}
}

func TestCacheRejectsFIFOWithoutBlockingStartup(t *testing.T) {
	c := testConfig(t)
	if e := os.MkdirAll(filepath.Dir(c.Path), 0700); e != nil {
		t.Fatal(e)
	}
	if e := unix.Mkfifo(c.Path, 0600); e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() {
		_, _, e := readJournal(c.Path)
		done <- e
	}()
	select {
	case e := <-done:
		if e == nil {
			t.Fatal("FIFO cache accepted")
		}
	case <-time.After(time.Second):
		// Release a regressed blocking reader before failing the test.
		f, e := os.OpenFile(c.Path, os.O_WRONLY|unix.O_NONBLOCK, 0)
		if e == nil {
			f.Close()
			<-done
		}
		t.Fatal("FIFO blocked journal read before regular-file validation")
	}
	// Startup also rejects it, but durable ancestor sync has no 150ms SLA.
	if s, e := New(c); e == nil {
		s.Close()
		t.Fatal("FIFO cache accepted at startup")
	}
}

func TestNewStateHierarchySyncsEveryCreatedParent(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "a", "b", "c")
	var synced []string
	e := prepareDirectory(dir, func(path string) error {
		info, e := os.Stat(path)
		if e != nil || !info.IsDir() {
			t.Fatalf("sync before directory creation: %s %v", path, e)
		}
		synced = append(synced, filepath.Clean(path))
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	// Each newly created directory and its parent must be durably published.
	for _, wanted := range []string{base, filepath.Join(base, "a"), filepath.Join(base, "a", "b"), dir} {
		found := false
		for _, got := range synced {
			found = found || got == wanted
		}
		if !found {
			t.Fatalf("created component not synced: %s; got %v", wanted, synced)
		}
	}
	failed := filepath.Join(t.TempDir(), "private", "deep")
	if e := prepareDirectory(failed, func(string) error { return errors.New("directory fsync failed") }); e == nil {
		t.Fatal("directory creation sync failure ignored")
	}
}
func TestPostRenameUncertaintyFaultsUntilRecovery(t *testing.T) {
	c := testConfig(t)
	s := open(t, c)
	if e := s.Accept(context.Background(), policy(t)); e != nil {
		t.Fatal(e)
	}
	next := []byte(strings.Replace(string(policy(t)), "10:00:00Z", "09:30:00Z", 1))
	e := s.accept(context.Background(), next, func(path string, r cacheRecord) error {
		return writeJournalWithSync(path, r, func(string) error { return errors.New("injected directory fsync failure") })
	})
	var commit *commitError
	if !errors.As(e, &commit) || !commit.Published {
		t.Fatalf("post-publication uncertainty not classified: %v", e)
	}
	st, _ := s.Status(c.Now())
	if !st.CacheFault || st.Generation != 1 || st.LastError == "" {
		t.Fatalf("uncertain disk completion advanced acceptance: %#v", st)
	}
	if e := s.Accept(context.Background(), policy(t)); e == nil {
		t.Fatal("faulted store accepted new candidate")
	}
	s.Close()
	restored := open(t, c)
	st, _ = restored.Status(c.Now())
	if !st.Restored || st.Generation != 2 || st.CacheFault || st.EnforcementConfirmed {
		t.Fatalf("reopen did not validate actually published state: %#v", st)
	}
}
