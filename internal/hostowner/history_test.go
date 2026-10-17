// SPDX-License-Identifier: GPL-3.0-or-later
package hostowner

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/PicketX/PicketX/pkg/networkplan"
	"github.com/PicketX/PicketXD/internal/hostpolicy"
	"github.com/PicketX/PicketXD/internal/hostrevoke"
	"net/netip"
	"os"
	"reflect"
	"testing"
)

func historySetup(t *testing.T) (*Session, Config, environment, *cleanupTestKernel) {
	t.Helper()
	c, e, b := applicationSetup(t)
	k := &cleanupTestKernel{applicationKernel: b}
	e.backend = k
	return mustOpen(t, c, e), c, e, k
}
func historyUnitPolicy(t *testing.T, id, source string) *hostpolicy.Policy {
	t.Helper()
	return admissionPolicy(t, true, false, func(in *networkplan.Input) {
		in.Logical.Grants[0].ID = id
		in.Logical.Grants[0].Source = netip.MustParsePrefix(source)
	})
}
func applyHistoryUnit(t *testing.T, s *Session, id, source string) {
	t.Helper()
	if _, e := s.ApplyPolicy(context.Background(), historyUnitPolicy(t, id, source), markRange()); e != nil {
		t.Fatal(e)
	}
}
func TestHistoryFreshRestartRecordedCleanup(t *testing.T) {
	s, c, e, k := historySetup(t)
	if s.HistoryStatus().Tracked || s.HistoryStatus().Complete {
		t.Fatal("marker has history proof")
	}
	applyHistoryUnit(t, s, "a", "127.0.0.1/32")
	st := s.HistoryStatus()
	if !st.Tracked || !st.Complete || st.Members != 1 || st.Targets != 1 || st.Bytes <= 0 || st.Digest == "" {
		t.Fatal(st)
	}
	old := *s.record.Application.Confirmed
	f1 := ownedFlow(old)
	f1.Original.Source = netip.MustParseAddr("127.0.0.1")
	f1.Reply.Destination = f1.Original.Source
	f2 := ownedFlow(old)
	k.flows = []hostrevoke.Flow{f1, f2}
	applyHistoryUnit(t, s, "b", "127.0.0.2/32")
	if _, e := s.ApplyPolicy(context.Background(), admissionPolicy(t, false, false), markRange()); e != nil {
		t.Fatal(e)
	}
	if s.HistoryStatus().Members != 2 {
		t.Fatal("forgot earlier source")
	}
	s.Close()
	if s.HistoryStatus().Complete {
		t.Fatal("closed history complete")
	}
	s = mustOpen(t, c, e)
	defer s.Close()
	if !s.HistoryStatus().Complete || s.HistoryStatus().Members != 2 {
		t.Fatal(s.HistoryStatus())
	}
	before, _ := os.ReadFile(c.StatePath)
	r, err := s.RevokeRecorded(context.Background())
	if err != nil || r.Selected != 2 || r.Deleted != 2 || r.Remaining != 0 {
		t.Fatal(r, err)
	}
	after, _ := os.ReadFile(c.StatePath)
	if !reflect.DeepEqual(before, after) || s.HistoryStatus().Members != 2 {
		t.Fatal("cleanup forgot history or wrote journal")
	}
}
func TestHistoryLegacyNoOpRemainsIncomplete(t *testing.T) {
	s, c, e, k := historySetup(t)
	p := historyUnitPolicy(t, "a", "127.0.0.1/32")
	if _, err := s.ApplyPolicy(context.Background(), p, markRange()); err != nil {
		t.Fatal(err)
	}
	attempt := s.ApplicationStatus().Attempt
	digest := s.ApplicationStatus().ImageDigest
	s.Close()
	r, _, err := readRecord(c.StatePath)
	if err != nil {
		t.Fatal(err)
	}
	r.Version = 3
	r.Application.History = nil
	if err = writeRecord(c.StatePath, r); err != nil {
		t.Fatal(err)
	}
	s = mustOpen(t, c, e)
	if s.HistoryStatus().Tracked || s.HistoryStatus().Complete {
		t.Fatal("legacy silently complete")
	}
	if _, err = s.RevokeRecorded(context.Background()); err == nil || k.deletes != 0 {
		t.Fatal("legacy allowed recorded cleanup", err)
	}
	applies := k.applies
	if _, err = s.ApplyPolicy(context.Background(), p, markRange()); err != nil {
		t.Fatal(err)
	}
	if !s.HistoryStatus().Tracked || s.HistoryStatus().Complete || s.HistoryStatus().Members != 1 || k.applies != applies || s.ApplicationStatus().Attempt != attempt || s.ApplicationStatus().ImageDigest != digest {
		t.Fatal("no-op migration changed application/coverage", s.HistoryStatus())
	}
	s.Close()
	s = mustOpen(t, c, e)
	defer s.Close()
	if s.HistoryStatus().Complete {
		t.Fatal("legacy coverage promoted after restart")
	}
}
func TestHistoryPendingRecoveryRemembersOnlyLiveCandidate(t *testing.T) {
	for _, accepted := range []bool{false, true} {
		t.Run(map[bool]string{false: "rejected", true: "installed"}[accepted], func(t *testing.T) {
			s, c, e, k := historySetup(t)
			applyHistoryUnit(t, s, "a", "127.0.0.1/32")
			boom := errors.New("uncertain")
			if accepted {
				s.persist = func(r record) error {
					if r.Application != nil && r.Application.Pending == nil && r.Application.Confirmed != nil {
						return boom
					}
					return writeRecord(c.StatePath, r)
				}
			} else {
				k.applyErr = boom
			}
			if _, err := s.ApplyPolicy(context.Background(), historyUnitPolicy(t, "b", "127.0.0.2/32"), markRange()); err == nil {
				t.Fatal("injected ambiguity succeeded")
			}
			if s.HistoryStatus().Complete {
				t.Fatal("pending history claimed complete")
			}
			if _, err := s.RevokeRecorded(context.Background()); err == nil || k.deletes != 0 {
				t.Fatal("uncertain cleanup")
			}
			r, _, err := readRecord(c.StatePath)
			if err != nil || r.Application.Pending == nil || len(r.Application.History.State.Targets[0].Members) != 1 {
				t.Fatal("pending candidate remembered before live recovery", r, err)
			}
			s.Close()
			k.applyErr = nil
			s = mustOpen(t, c, e)
			defer s.Close()
			want := 1
			if accepted {
				want = 2
			}
			if !s.HistoryStatus().Complete || s.HistoryStatus().Members != want || s.record.Application.Pending != nil {
				t.Fatal(s.HistoryStatus(), want)
			}
		})
	}
}
func TestHistoryNewBootResetsOnlyCoverage(t *testing.T) {
	s, c, e, k := historySetup(t)
	applyHistoryUnit(t, s, "a", "127.0.0.1/32")
	applyHistoryUnit(t, s, "b", "127.0.0.2/32")
	marks := s.record.Marks.Clone()
	baseline := s.record.Application.Baseline
	s.Close()
	k.observation = observation{}
	e.namespace.BootID = "new-boot"
	e.namespace.Inode++
	s = mustOpen(t, c, e)
	defer s.Close()
	if s.HistoryStatus().Complete || s.HistoryStatus().Members != 0 || !reflect.DeepEqual(s.record.Marks, &marks) || !reflect.DeepEqual(s.record.Application.Baseline, baseline) {
		t.Fatal("new boot kept old scopes or lost retirement/baseline")
	}
	applyHistoryUnit(t, s, "c", "127.0.0.3/32")
	if !s.HistoryStatus().Complete || s.HistoryStatus().Members != 1 {
		t.Fatal(s.HistoryStatus())
	}
}
func TestHistoryCorruptionAndScopeRejectBeforeEffects(t *testing.T) {
	s, c, e, k := historySetup(t)
	applyHistoryUnit(t, s, "a", "127.0.0.1/32")
	before, _ := os.ReadFile(c.StatePath)
	applies := k.applies
	state := s.record.Marks.Clone()
	p := historyUnitPolicy(t, "b", "127.0.0.2/32")
	in, _ := p.Input()
	in.Contexts[0].ID = "other-context"
	scope, err := hostpolicy.NewScope("static", in.Contexts[0])
	if err != nil {
		t.Fatal(err)
	}
	p, err = hostpolicy.CompileInput(in, scope)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ApplyPolicy(context.Background(), p, markRange()); err == nil || k.applies != applies || !reflect.DeepEqual(s.record.Marks, &state) {
		t.Fatal("history scope changed before rejection", err)
	}
	after, _ := os.ReadFile(c.StatePath)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("scope rejection wrote journal")
	}
	s.Close()
	for _, mode := range []string{"missing", "lost source", "shape"} {
		t.Run(mode, func(t *testing.T) {
			var r record
			if err := json.Unmarshal(before, &r); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "missing":
				r.Application.History = nil
			case "lost source":
				r.Application.History.State.Targets[0].Members = nil
			case "shape":
				r.Application.History.State.Targets[0].Identity.Shape = "invalid"
			}
			data, _ := json.Marshal(r)
			if err := os.WriteFile(c.StatePath, data, 0600); err != nil {
				t.Fatal(err)
			}
			if next, err := openWith(context.Background(), c, e); err == nil {
				next.Close()
				t.Fatal("corrupt coverage accepted")
			}
			if k.applies != applies {
				t.Fatal("corruption changed kernel")
			}
			after, _ := os.ReadFile(c.StatePath)
			if !reflect.DeepEqual(data, after) {
				t.Fatal("corruption overwritten")
			}
		})
	}
	if err := os.WriteFile(c.StatePath, before, 0600); err != nil {
		t.Fatal(err)
	}
}
