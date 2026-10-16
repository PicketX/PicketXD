// SPDX-License-Identifier: GPL-3.0-or-later
package hostagent

import (
	"context"
	"errors"
	"github.com/PicketX/PicketX/pkg/configuration"
	"github.com/PicketX/PicketXD/internal/agentstate"
	"github.com/PicketX/PicketXD/internal/hostowner"
	"github.com/PicketX/PicketXD/internal/hostpolicy"
	"github.com/PicketX/PicketXD/internal/markstate"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeOwner struct {
	applies, verifies, closes int
	app                       hostowner.ApplicationStatus
	applyErr, verifyErr       error
	onApply                   func()
	scope                     hostowner.RuntimeScope
}

func (o *fakeOwner) RuntimeScope(context.Context) (hostowner.RuntimeScope, error) {
	return o.scope, nil
}
func (o *fakeOwner) ApplyPolicy(_ context.Context, p *hostpolicy.Policy, _ markstate.Range) (hostowner.ApplicationStatus, error) {
	o.applies++
	if _, e := p.Input(); e != nil {
		return o.app, e
	}
	if o.onApply != nil {
		o.onApply()
	}
	if o.applyErr != nil {
		return o.app, o.applyErr
	}
	o.app = hostowner.ApplicationStatus{Phase: "confirmed", Attempt: uint64(o.applies), ImageDigest: "image", EnforcementConfirmed: true, ObservedAt: time.Now()}
	return o.app, nil
}
func (o *fakeOwner) Verify(context.Context) (hostowner.Status, error) {
	o.verifies++
	if o.verifyErr != nil {
		o.app.EnforcementConfirmed = false
	}
	return o.Status(), o.verifyErr
}
func (o *fakeOwner) Status() hostowner.Status {
	return hostowner.Status{OwnershipConfirmed: o.verifyErr == nil, EnforcementConfirmed: o.app.EnforcementConfirmed}
}
func (o *fakeOwner) ApplicationStatus() hostowner.ApplicationStatus { return o.app }
func (o *fakeOwner) Close() error                                   { o.closes++; return nil }
func runSetup(t *testing.T) (Config, Dependencies, *fakeOwner, *[]Status) {
	t.Helper()
	root := t.TempDir()
	raw, e := os.ReadFile("../../examples/prepare/policy.yaml")
	if e != nil {
		t.Fatal(e)
	}
	policy := filepath.Join(root, "policy.yaml")
	if e = os.WriteFile(policy, raw, 0600); e != nil {
		t.Fatal(e)
	}
	c := Config{PolicyPath: policy, StateDir: filepath.Join(root, "state"), ProviderID: "static-test", AgentRef: "agent-a", Profile: "host", Destinations: []netip.Prefix{netip.MustParsePrefix("192.0.2.100/32"), netip.MustParsePrefix("2001:db8::100/128")}, Marks: markstate.Range{First: 100, Last: 107}, Poll: 10 * time.Millisecond, Once: true}
	owner := &fakeOwner{scope: hostowner.RuntimeScope{BootID: "held-boot", NetworkSpaceID: "held-netns"}}
	reports := &[]Status{}
	d := Dependencies{OpenOwner: func(_ context.Context, oc hostowner.Config) (Owner, error) {
		if filepath.Dir(oc.StatePath) != c.StateDir {
			t.Fatal("owner path outside private state-dir")
		}
		return owner, nil
	}, Emit: func(st Status) error { *reports = append(*reports, st); return nil }}
	return c, d, owner, reports
}
func TestOnceAppliesThenRetainsCacheOnInvalidPublication(t *testing.T) {
	c, d, o, reports := runSetup(t)
	if e := Run(context.Background(), c, d); e != nil {
		t.Fatal(e)
	}
	st := (*reports)[0]
	if !st.EnforcementConfirmed || st.Desired.Generation != 1 || st.AppliedGeneration != 1 || st.AppliedPolicyHash != st.Desired.PolicyHash || o.applies != 1 || o.closes != 1 {
		t.Fatal(st, o)
	}
	os.WriteFile(c.PolicyPath, []byte("broken"), 0600)
	if e := Run(context.Background(), c, d); e == nil {
		t.Fatal("invalid once reload exited success")
	}
	st = (*reports)[len(*reports)-1]
	if !st.EnforcementConfirmed || !st.Desired.Restored || st.Desired.LastError == "" || st.Desired.Generation != 1 {
		t.Fatal(st)
	}
}
func TestLoopInvalidReloadVerifiesWithoutReapplying(t *testing.T) {
	c, d, o, reports := runSetup(t)
	c.Once = false
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	d.Emit = func(st Status) error {
		*reports = append(*reports, st)
		if len(*reports) == 1 {
			os.WriteFile(c.PolicyPath, []byte("broken"), 0600)
		}
		if st.Desired.LastError != "" {
			cancel()
		}
		return nil
	}
	if e := Run(ctx, c, d); e != nil {
		t.Fatal(e)
	}
	last := (*reports)[len(*reports)-1]
	if !last.EnforcementConfirmed || last.Desired.LastError == "" || o.applies != 1 || o.verifies == 0 || o.closes != 1 {
		t.Fatal(last, o)
	}
}
func TestNewDesiredFailureDoesNotClaimOldReceipt(t *testing.T) {
	c, d, o, reports := runSetup(t)
	c.Once = false
	boom := errors.New("native apply uncertain")
	d.Emit = func(st Status) error {
		*reports = append(*reports, st)
		if len(*reports) == 1 {
			raw, _ := os.ReadFile(c.PolicyPath)
			os.WriteFile(c.PolicyPath, []byte(strings.ReplaceAll(string(raw), "grant-browser-v1", "grant-browser-v2")), 0600)
			o.applyErr = boom
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	e := Run(ctx, c, d)
	if !errors.Is(e, boom) {
		t.Fatal(e)
	}
	last := (*reports)[len(*reports)-1]
	if last.EnforcementConfirmed || last.Desired.Generation != 2 || last.AppliedGeneration != 1 || last.LastApplyError == "" || !last.Application.EnforcementConfirmed {
		t.Fatal("old proof mislabeled current", last)
	}
}
func TestCoherentCandidateChangesDuringApply(t *testing.T) {
	c, d, o, reports := runSetup(t)
	var store *agentstate.Store
	d.NewStore = func(sc agentstate.Config) (Store, error) { var e error; store, e = agentstate.New(sc); return store, e }
	o.onApply = func() {
		raw, _ := os.ReadFile(c.PolicyPath)
		if e := store.Accept(context.Background(), []byte(strings.ReplaceAll(string(raw), "grant-browser-v1", "grant-browser-v2"))); e != nil {
			t.Fatal(e)
		}
	}
	if e := Run(context.Background(), c, d); e == nil {
		t.Fatal("once reported stable success after newer acceptance")
	}
	last := (*reports)[len(*reports)-1]
	if last.EnforcementConfirmed || last.Desired.Generation != 2 || last.AppliedGeneration != 1 {
		t.Fatal(last)
	}
}
func TestConfigRejectsBeforeEffects(t *testing.T) {
	for _, which := range []string{"relative", "root", "marks", "wide", "poll", "agent", "destinations", "policy", "emit"} {
		t.Run(which, func(t *testing.T) {
			c, d, o, _ := runSetup(t)
			switch which {
			case "relative":
				c.StateDir = "relative"
			case "root":
				c.StateDir = "/"
			case "marks":
				c.Marks.First = 0
			case "wide":
				c.Marks.Last = 10000
			case "poll":
				c.Poll = 0
			case "agent":
				c.AgentRef = ""
			case "destinations":
				c.Destinations = nil
			case "policy":
				c.PolicyPath = ""
			case "emit":
				d.Emit = nil
			}
			opened := 0
			d.OpenOwner = func(context.Context, hostowner.Config) (Owner, error) { opened++; return o, nil }
			if e := Run(context.Background(), c, d); e == nil || opened != 0 {
				t.Fatal("bad config caused effects", e, opened)
			}
		})
	}
}
func TestVerifyAndStartupFailuresCloseResources(t *testing.T) {
	c, d, o, reports := runSetup(t)
	c.Once = false
	boom := errors.New("drift")
	d.Emit = func(st Status) error { *reports = append(*reports, st); o.verifyErr = boom; return nil }
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if e := Run(ctx, c, d); !errors.Is(e, boom) {
		t.Fatal(e)
	}
	last := (*reports)[len(*reports)-1]
	if last.EnforcementConfirmed || last.LastApplyError == "" || o.closes != 1 {
		t.Fatal(last, o)
	}
	c, d, o, _ = runSetup(t)
	d.NewStore = func(agentstate.Config) (Store, error) { return nil, boom }
	if e := Run(context.Background(), c, d); !errors.Is(e, boom) || o.closes != 1 {
		t.Fatal(e, o)
	}
}

type faultStore struct{ *agentstate.Store }

func (s faultStore) Snapshot(at time.Time) (agentstate.Status, *configuration.Candidate, error) {
	st, p, e := s.Store.Snapshot(at)
	st.CacheFault = true
	return st, p, e
}
func TestCacheFaultAndInitialInvalidDoNotApply(t *testing.T) {
	c, d, o, reports := runSetup(t)
	d.NewStore = func(sc agentstate.Config) (Store, error) { s, e := agentstate.New(sc); return faultStore{s}, e }
	if e := Run(context.Background(), c, d); e == nil || o.applies != 0 || len(*reports) != 1 || (*reports)[0].EnforcementConfirmed {
		t.Fatal(e, o, *reports)
	}
	c, d, o, reports = runSetup(t)
	os.WriteFile(c.PolicyPath, []byte("broken"), 0600)
	if e := Run(context.Background(), c, d); e == nil || o.applies != 0 || len(*reports) != 1 || (*reports)[0].EnforcementConfirmed {
		t.Fatal(e, o, *reports)
	}
}
func TestFactsAndRuntimeKeyAreCanonical(t *testing.T) {
	c, _, _, _ := runSetup(t)
	a, e := c.normalized()
	if e != nil {
		t.Fatal(e)
	}
	c.Destinations = []netip.Prefix{c.Destinations[1], c.Destinations[0], c.Destinations[0]}
	b, e := c.normalized()
	if e != nil || a.runtimeKey() != b.runtimeKey() {
		t.Fatal("flag order changed runtime identity", e)
	}
	facts, _, e := a.facts(hostowner.RuntimeScope{BootID: "pin-boot", NetworkSpaceID: "pin-net"})
	if e != nil || facts.Contexts[0].NetworkSpaceID != "pin-net" || !strings.HasPrefix(facts.Contexts[0].ID, "pin-net/") {
		t.Fatal(facts, e)
	}
	c.Marks.First++
	b, e = c.normalized()
	if e != nil || a.runtimeKey() == b.runtimeKey() {
		t.Fatal("mark range not bound to runtime identity", e)
	}
	raw, e := os.ReadFile("../../examples/enforce/policy.yaml")
	if e != nil {
		t.Fatal(e)
	}
	bundle, e := configuration.Decode(raw)
	if e != nil {
		t.Fatal(e)
	}
	a.AgentRef = "node"
	a.Destinations = []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32"), netip.MustParsePrefix("::1/128")}
	facts, _, e = a.facts(hostowner.RuntimeScope{BootID: "pin-boot", NetworkSpaceID: "pin-net"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = configuration.Compile(bundle, facts); e != nil {
		t.Fatal("runnable example invalid", e)
	}
}
