// SPDX-License-Identifier: GPL-3.0-or-later
package hostagent

import (
	"context"
	"errors"
	"github.com/PicketX/PicketX/pkg/networkplan"
	"github.com/PicketX/PicketXD/internal/agentstate"
	"github.com/PicketX/PicketXD/internal/hostowner"
	"github.com/PicketX/PicketXD/internal/hostpolicy"
	"github.com/PicketX/PicketXD/internal/hostrevoke"
	"github.com/PicketX/PicketXD/internal/markstate"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

type strictOwner struct {
	*fakeOwner
	calls      []string
	result     hostrevoke.Result
	cleanupErr error
	onCleanup  func()
}

func (o *strictOwner) ApplyPolicy(ctx context.Context, p *hostpolicy.Policy, m markstate.Range) (hostowner.ApplicationStatus, error) {
	o.calls = append(o.calls, "apply")
	st, e := o.fakeOwner.ApplyPolicy(ctx, p, m)
	if e == nil {
		in, _ := p.Input()
		o.app.Handling = in.Contexts[0].Handling
		st = o.app
	}
	return st, e
}
func (o *strictOwner) Verify(ctx context.Context) (hostowner.Status, error) {
	o.calls = append(o.calls, "verify")
	return o.fakeOwner.Verify(ctx)
}
func (o *strictOwner) RevokeRecorded(context.Context) (hostrevoke.Result, error) {
	o.calls = append(o.calls, "cleanup")
	if !o.app.EnforcementConfirmed {
		panic("cleanup before confirmed application")
	}
	if o.onCleanup != nil {
		o.onCleanup()
	}
	return o.result, o.cleanupErr
}
func strictRunSetup(t *testing.T) (Config, Dependencies, *strictOwner, *[]Status) {
	t.Helper()
	c, d, o, reports := runSetup(t)
	c.Handling = networkplan.Strict
	raw, _ := os.ReadFile(c.PolicyPath)
	if e := os.WriteFile(c.PolicyPath, []byte(strings.ReplaceAll(string(raw), "handling: graceful", "handling: strict")), 0600); e != nil {
		t.Fatal(e)
	}
	s := &strictOwner{fakeOwner: o}
	d.OpenOwner = func(context.Context, hostowner.Config) (Owner, error) { return s, nil }
	return c, d, s, reports
}
func TestStrictDriverOrdersAndSeparatesCleanup(t *testing.T) {
	c, d, o, reports := strictRunSetup(t)
	if e := Run(context.Background(), c, d); e != nil {
		t.Fatal(e)
	}
	st := (*reports)[0]
	if !reflect.DeepEqual(o.calls, []string{"apply", "cleanup"}) || st.Strict == nil || !st.Strict.Enabled || !st.Strict.CleanupConfirmed || !st.EnforcementConfirmed || st.Strict.ImageDigest != st.Application.ImageDigest {
		t.Fatal(st, o.calls)
	}
	c, d, o, reports = strictRunSetup(t)
	c.Once = false
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	d.Emit = func(st Status) error {
		*reports = append(*reports, st)
		if len(*reports) == 2 {
			cancel()
		}
		return nil
	}
	if e := Run(ctx, c, d); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(o.calls, []string{"apply", "cleanup", "verify", "cleanup"}) {
		t.Fatal(o.calls)
	}
	c, d, base, reports := runSetup(t)
	g := &strictOwner{fakeOwner: base}
	d.OpenOwner = func(context.Context, hostowner.Config) (Owner, error) { return g, nil }
	if e := Run(context.Background(), c, d); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(g.calls, []string{"apply"}) || (*reports)[0].Strict != nil {
		t.Fatal("graceful invokes strict cleanup", g.calls)
	}
}
func TestStrictDriverFailuresCannotConfirm(t *testing.T) {
	for _, mode := range []string{"cleanup error", "remaining", "application error", "boundary", "generation"} {
		t.Run(mode, func(t *testing.T) {
			c, d, o, reports := strictRunSetup(t)
			switch mode {
			case "cleanup error":
				o.cleanupErr = errors.New("partial delete")
			case "remaining":
				o.result.Remaining = 1
			case "application error":
				o.applyErr = errors.New("uncertain apply")
			case "boundary":
				now := time.Date(2026, 10, 15, 9, 59, 59, 0, time.UTC)
				d.Now = func() time.Time { return now }
				o.onCleanup = func() { now = now.Add(time.Second) }
			case "generation":
				var store *agentstate.Store
				d.NewStore = func(sc agentstate.Config) (Store, error) { var e error; store, e = agentstate.New(sc); return store, e }
				o.onCleanup = func() {
					raw, _ := os.ReadFile(c.PolicyPath)
					if e := store.Accept(context.Background(), []byte(strings.ReplaceAll(string(raw), "grant-browser-v1", "grant-browser-v2"))); e != nil {
						t.Fatal(e)
					}
				}
			}
			if e := Run(context.Background(), c, d); e == nil {
				t.Fatal("strict failure succeeded", mode)
			}
			st := (*reports)[len(*reports)-1]
			if st.EnforcementConfirmed || st.Strict == nil || st.Strict.CleanupConfirmed {
				t.Fatal(st)
			}
			if mode == "application error" && len(o.calls) != 1 {
				t.Fatal("cleanup after failed image", o.calls)
			}
		})
	}
}
func TestStrictModeMismatchBeforeDesiredStore(t *testing.T) {
	c, d, o, _ := strictRunSetup(t)
	o.app.Handling = networkplan.Graceful
	opened := false
	d.NewStore = func(agentstate.Config) (Store, error) { opened = true; return nil, errors.New("store must not open") }
	if e := Run(context.Background(), c, d); e == nil || opened || o.applies != 0 || o.closes != 1 {
		t.Fatal(e, opened, o)
	}
}
func TestHandlingDefaultsAndIdentity(t *testing.T) {
	c, _, _, _ := runSetup(t)
	a, e := c.normalized()
	if e != nil || a.Handling != networkplan.Graceful {
		t.Fatal(a, e)
	}
	key := a.runtimeKey()
	c.Handling = networkplan.Strict
	b, e := c.normalized()
	if e != nil || b.runtimeKey() != key {
		t.Fatal("handling changed durable scope identity", e)
	}
	f, _, e := b.facts(hostowner.RuntimeScope{BootID: "boot", NetworkSpaceID: "net"})
	if e != nil || f.Contexts[0].Handling != networkplan.Strict || !reflect.DeepEqual(f.Agents[0].Capabilities.Handling, []networkplan.HandlingMode{networkplan.Strict}) {
		t.Fatal(f, e)
	}
	c.Handling = "invalid"
	if _, e = c.normalized(); e == nil {
		t.Fatal("unknown handling accepted")
	}
}

func TestStrictVerifyFailureSkipsCleanup(t *testing.T) {
	c, d, o, reports := strictRunSetup(t)
	c.Once = false
	boom := errors.New("live drift")
	d.Emit = func(st Status) error { *reports = append(*reports, st); o.verifyErr = boom; return nil }
	if e := Run(context.Background(), c, d); !errors.Is(e, boom) {
		t.Fatal(e)
	}
	st := (*reports)[len(*reports)-1]
	if !reflect.DeepEqual(o.calls, []string{"apply", "cleanup", "verify"}) || st.EnforcementConfirmed || st.Strict.CleanupConfirmed {
		t.Fatal(o.calls, st)
	}
}

func TestStrictFinalSnapshotBoundaryCannotConfirm(t *testing.T) {
	c, d, o, reports := strictRunSetup(t)
	now := time.Date(2026, 10, 15, 9, 59, 59, 0, time.UTC)
	cleaned, reads := false, 0
	o.onCleanup = func() { cleaned = true }
	d.Now = func() time.Time {
		if cleaned {
			reads++
			if reads > 1 {
				return now.Add(time.Second)
			}
		}
		return now
	}
	e := Run(context.Background(), c, d)
	st := (*reports)[len(*reports)-1]
	if e == nil || st.EnforcementConfirmed || st.Strict.CleanupConfirmed {
		t.Fatal("final snapshot crossed membership boundary but retained strict confirmation", e, st)
	}
}

func TestStrictExistingImageCleanedBeforeReplacement(t *testing.T) {
	c, d, o, _ := strictRunSetup(t)
	o.app = hostowner.ApplicationStatus{Handling: networkplan.Strict, Phase: "confirmed", ImageDigest: "previous-image", EnforcementConfirmed: true}
	staleTCP := true
	o.onCleanup = func() {
		if o.app.ImageDigest == "previous-image" {
			staleTCP = false
		}
	}
	o.onApply = func() {
		if staleTCP {
			t.Fatal("new permission installed while expired old TCP still retained")
		}
	}
	if err := Run(context.Background(), c, d); err != nil {
		t.Fatal(err)
	}
	if staleTCP || !reflect.DeepEqual(o.calls, []string{"cleanup", "apply", "cleanup"}) {
		t.Fatal(staleTCP, o.calls)
	}
}

func TestStrictPreviousCleanupFailurePreventsReplacement(t *testing.T) {
	for _, mode := range []string{"error", "remaining"} {
		t.Run(mode, func(t *testing.T) {
			c, d, o, reports := strictRunSetup(t)
			o.app = hostowner.ApplicationStatus{Handling: networkplan.Strict, Phase: "confirmed", ImageDigest: "previous-image", EnforcementConfirmed: true}
			if mode == "error" {
				o.cleanupErr = errors.New("old deletion failed")
			} else {
				o.result.Remaining = 1
			}
			if err := Run(context.Background(), c, d); err == nil {
				t.Fatal("previous cleanup failure accepted")
			}
			st := (*reports)[len(*reports)-1]
			if o.applies != 0 || !reflect.DeepEqual(o.calls, []string{"cleanup"}) || st.EnforcementConfirmed || st.Strict.CleanupConfirmed {
				t.Fatal("replacement installed after old cleanup failure", o.calls, st)
			}
		})
	}
}
