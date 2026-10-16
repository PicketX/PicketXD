// SPDX-License-Identifier: GPL-3.0-or-later
package hostowner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/PicketX/PicketX/pkg/networkplan"
	"github.com/PicketX/PicketXD/internal/markstate"
)

type markKernel struct {
	*memoryKernel
	occupied []uint32
	scanErr  error
	scans    int
	onScan   func()
}

func (k *markKernel) OccupiedMarks(context.Context, markstate.Range) ([]uint32, error) {
	k.scans++
	if k.onScan != nil {
		k.onScan()
	}
	return k.occupied, k.scanErr
}
func markSetup(t *testing.T) (Config, environment, *markKernel) {
	c, e, k := setup(t)
	b := &markKernel{memoryKernel: k}
	e.backend = b
	return c, e, b
}
func markTarget(id string) markstate.Target {
	return markstate.Target{Key: networkplan.TargetKey{AgentRef: "node", ContextID: "ctx", Entry: networkplan.EntryRef{ResourceName: id, EndpointName: "https"}}, Shape: strings.Repeat("a", 64)}
}
func markRange() markstate.Range { return markstate.Range{First: 100, Last: 107} }
func reserve(t *testing.T, s *Session, targets ...markstate.Target) MarkStatus {
	t.Helper()
	st, e := s.ReconcileMarks(context.Background(), markRange(), targets)
	if e != nil || st.ObservedAt.IsZero() || st.State.Range != markRange() {
		t.Fatalf("reservation missing %v %v", st, e)
	}
	return st
}
func TestDurableMarks(t *testing.T) {
	c, e, b := markSetup(t)
	writes := 0
	e.save = func(p string, r record) error { writes++; return writeRecord(p, r) }
	s := mustOpen(t, c, e)
	before, _ := readRecordOnly(c.StatePath)
	if before.Version != 1 {
		t.Fatal("initial record changed")
	}
	first := reserve(t, s, markTarget("a"))
	if len(first.State.Entries) != 1 || first.State.Entries[0].Mark != 100 {
		t.Fatal(first)
	}
	r, _, err := readRecord(c.StatePath)
	if err != nil || r.Version != 2 || r.Marks == nil || r.Marks.Entries[0].Mark != 100 {
		t.Fatal("not durably upgraded", r, err)
	}
	count := writes
	same := reserve(t, s, markTarget("a"))
	if writes != count || b.scans != 2 {
		t.Fatal("no-op did not rescan or rewrote", writes, count, b.scans)
	}
	same.State.Entries[0].Retired = true
	s.Close()
	s = mustOpen(t, c, e)
	defer s.Close()
	restored := reserve(t, s, markTarget("a"))
	if restored.State.Entries[0].Retired || restored.State.Entries[0].Mark != 100 {
		t.Fatal("return aliases or restart lost ledger", restored)
	}
	reserve(t, s)
	again := reserve(t, s, markTarget("a"))
	if len(again.State.Entries) != 2 || again.State.Entries[1].Mark != 101 || !again.State.Entries[0].Retired {
		t.Fatal("restart allowed reuse", again)
	}
	if s.Status().EnforcementConfirmed {
		t.Fatal("reservation claims enforcement")
	}
}
func readRecordOnly(path string) (record, error) { r, _, e := readRecord(path); return r, e }
func TestMarkPublicationFailures(t *testing.T) {
	for _, published := range []bool{false, true} {
		t.Run(fmt.Sprint(published), func(t *testing.T) {
			c, e, _ := markSetup(t)
			boom := errors.New("mark publication fault")
			e.save = func(p string, r record) error {
				if r.Marks != nil {
					if published {
						return writeRecordWithSync(p, r, func(string) error { return boom })
					}
					return boom
				}
				return writeRecord(p, r)
			}
			s := mustOpen(t, c, e)
			st, err := s.ReconcileMarks(context.Background(), markRange(), []markstate.Target{markTarget("a")})
			if !errors.Is(err, boom) || len(st.State.Entries) != 0 {
				t.Fatal("publish error returned assignments", st, err)
			}
			if _, err = s.ReconcileMarks(context.Background(), markRange(), nil); err == nil {
				t.Fatal("faulted session still operates")
			}
			if _, err = s.Verify(context.Background()); err == nil {
				t.Fatal("faulted session verifies")
			}
			s.Close()
			e.save = nil
			s = mustOpen(t, c, e)
			defer s.Close()
			st = reserve(t, s, markTarget("a"))
			if st.State.Entries[0].Mark != 100 {
				t.Fatal("uncertain publication reused/advanced wrong mark", st)
			}
		})
	}
}
func TestMarkScopeAndScanFailures(t *testing.T) {
	for _, kind := range []string{"range", "agent", "collision", "scan", "drift", "after-scan-drift", "cancel", "closed"} {
		t.Run(kind, func(t *testing.T) {
			c, e, b := markSetup(t)
			s := mustOpen(t, c, e)
			defer s.Close()
			before, _ := os.ReadFile(c.StatePath)
			ctx := context.Background()
			r := markRange()
			targets := []markstate.Target{markTarget("a")}
			switch kind {
			case "range":
				r.First = 0
			case "agent":
				targets[0].Key.AgentRef = "other"
			case "collision":
				b.occupied = []uint32{107}
			case "scan":
				b.scanErr = errors.New("incomplete second family")
			case "drift":
				b.observation.Marker = "foreign"
			case "after-scan-drift":
				b.onScan = func() { b.observation.Marker = "foreign" }
			case "cancel":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "closed":
				s.Close()
			}
			if st, err := s.ReconcileMarks(ctx, r, targets); err == nil || len(st.State.Entries) != 0 {
				t.Fatal("invalid attempt yielded allocation", st, err)
			}
			after, _ := os.ReadFile(c.StatePath)
			if !bytes.Equal(before, after) {
				t.Fatal("failed validation published state")
			}
		})
	}
}
func TestMarkHistoryAcrossBoot(t *testing.T) {
	c, e, b := markSetup(t)
	s := mustOpen(t, c, e)
	reserve(t, s, markTarget("a"))
	reserve(t, s)
	s.Close()
	e.namespace = namespace{BootID: "boot-2", Device: 9, Inode: 99}
	b.observation = observation{}
	s = mustOpen(t, c, e)
	defer s.Close()
	st := reserve(t, s, markTarget("a"))
	if len(st.State.Entries) != 2 || !st.State.Entries[0].Retired || st.State.Entries[1].Mark != 101 {
		t.Fatal("new boot discarded mark history", st)
	}
}
func TestMarkJournalValidation(t *testing.T) {
	c, e, _ := markSetup(t)
	s := mustOpen(t, c, e)
	reserve(t, s, markTarget("a"))
	s.Close()
	r, err := readRecordOnly(c.StatePath)
	if err != nil || r.Marks == nil {
		t.Fatal("no v2 record", r, err)
	}
	for _, change := range []func(*record){func(r *record) { r.Version = 1 }, func(r *record) { r.Marks = nil }, func(r *record) { r.Marks.Entries[0].Mark = 999 }, func(r *record) { r.Marks.Entries[0].Target.Key.AgentRef = "other" }} {
		bad := r
		copy := r.Marks.Clone()
		bad.Marks = &copy
		change(&bad)
		if validRecord(bad) == nil {
			t.Fatal("invalid journal accepted", bad)
		}
	}
	// A maximal legal ledger exceeds the old 32KiB limit but remains bounded.
	many := r
	state := markstate.State{Range: markstate.Range{First: 1, Last: 4096}}
	for i := range 4096 {
		target := markTarget(fmt.Sprint(i))
		target.Key.Entry.ResourceName = strings.Repeat("x", 240) + fmt.Sprint(i)
		state.Entries = append(state.Entries, markstate.Assignment{Target: target, Mark: uint32(i + 1), Retired: true})
	}
	many.Marks = &state
	path := filepath.Join(t.TempDir(), "large", "owner.json")
	if err = writeRecord(path, many); err != nil {
		t.Fatal("legal bounded history refused", err)
	}
	got, err := readRecordOnly(path)
	if err != nil || !reflect.DeepEqual(got, many) {
		t.Fatal("large history roundtrip failed", err)
	}
	data, _ := json.Marshal(many)
	if len(data) <= 32768 || len(data) > maxRecordBytes {
		t.Fatal("wrong journal bound coverage", len(data), maxRecordBytes)
	}
}
func TestMarkJournalPinnedDirectory(t *testing.T) {
	c, e, _ := markSetup(t)
	s := mustOpen(t, c, e)
	defer s.Close()
	dir := filepath.Dir(c.StatePath)
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.Mkdir(outside, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(dir, dir+".old"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, dir); err != nil {
		t.Fatal(err)
	}
	reserve(t, s, markTarget("a"))
	data, err := os.ReadFile(filepath.Join(dir+".old", filepath.Base(c.StatePath)))
	if err != nil {
		t.Fatal(err)
	}
	var r record
	if json.Unmarshal(data, &r) != nil || r.Version != 2 {
		t.Fatal("save left pinned directory", string(data))
	}
	if _, err = os.Stat(filepath.Join(outside, filepath.Base(c.StatePath))); !os.IsNotExist(err) {
		t.Fatal("mark publication followed renamed path")
	}
}

// Accepted identities may expand sixfold through encoding/json HTML/control
// escapes. Full capacity must still survive the real publication/read path.
func TestMarkJournalEscapedCapacity(t *testing.T) {
	c, e, _ := markSetup(t)
	s := mustOpen(t, c, e)
	s.Close()
	r, err := readRecordOnly(c.StatePath)
	if err != nil {
		t.Fatal(err)
	}
	identity := strings.Repeat("<", 256)
	r.Version = 2
	r.AgentRef = identity
	state := markstate.State{Range: markstate.Range{First: 1, Last: 4096}}
	target := markTarget(identity)
	target.Key.AgentRef = identity
	target.Key.ContextID = identity
	target.Key.Entry.EndpointName = identity
	for i := range 4096 {
		state.Entries = append(state.Entries, markstate.Assignment{Target: target, Mark: uint32(i + 1), Retired: true})
	}
	r.Marks = &state
	if err = validRecord(r); err != nil {
		t.Fatal("maximum accepted identities rejected", err)
	}
	path := filepath.Join(t.TempDir(), "escaped", "owner.json")
	if err = writeRecord(path, r); err != nil {
		t.Fatal("maximum escaped ledger publication failed", err)
	}
	got, err := readRecordOnly(path)
	if err != nil || !reflect.DeepEqual(got, r) {
		t.Fatal("maximum escaped ledger roundtrip failed", err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() <= 8<<20 || info.Size() > maxRecordBytes {
		t.Fatal("worst-case capacity regression did not cover escaped expansion", info, err)
	}
}
