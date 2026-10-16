// SPDX-License-Identifier: GPL-3.0-or-later
package hostowner

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"os"
	"reflect"
	"testing"

	"github.com/PicketX/PicketX/pkg/core"
	"github.com/PicketX/PicketX/pkg/networkplan"
	"github.com/PicketX/PicketXD/internal/hostpolicy"
	"github.com/PicketX/PicketXD/internal/hostrules"
)

func admissionPolicy(t *testing.T, allow, empty bool, edit ...func(*networkplan.Input)) *hostpolicy.Policy {
	t.Helper()
	dest := []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32"), netip.MustParsePrefix("::1/128")}
	c := networkplan.ExecutionContext{AgentRef: "node", ProfileRef: "host", ID: "ctx", NetworkSpaceID: "space", Class: networkplan.HostLocal, Stage: networkplan.OriginalDestination, Handling: networkplan.Graceful, Destinations: dest}
	in := networkplan.Input{AuthorityID: "static", Logical: core.Snapshot{Resources: []core.Resource{{Name: "uid", Endpoints: []core.Endpoint{{Name: "web"}}}}}, Networks: []networkplan.NetworkDefinition{{Entry: networkplan.EntryRef{ResourceName: "uid", EndpointName: "web"}, Protocol: networkplan.TCP, Port: 18443}}, Agents: []networkplan.Agent{{Name: "node", Capabilities: &networkplan.CapabilityProfile{Classes: []networkplan.TrafficClass{networkplan.HostLocal}, Protocols: []networkplan.Protocol{networkplan.TCP, networkplan.UDP}, Families: []networkplan.Family{networkplan.IPv4, networkplan.IPv6}, Handling: []networkplan.HandlingMode{networkplan.Graceful}, OriginalDestination: true}}}, Contexts: []networkplan.ExecutionContext{c}}
	if !empty {
		in.Bindings = []networkplan.Binding{{ID: "binding", Resources: []networkplan.ResourceSelection{{ResourceName: "uid"}}, Agents: []networkplan.AgentSelection{{AgentRef: "node"}}, Context: networkplan.ContextRequest{ProfileRef: "host", Class: c.Class, Stage: c.Stage, Handling: c.Handling, AllowedDestinations: dest}}}
	}
	if allow {
		in.Logical.Grants = []core.Grant{{ID: "allow", Source: netip.MustParsePrefix("127.0.0.0/8"), Scopes: []core.Scope{{ResourceName: "uid"}}, Validity: core.Validity{Mode: core.LifetimePermanent}}}
	}
	for _, fn := range edit {
		fn(&in)
	}
	plan, e := networkplan.Compile(in)
	if e != nil {
		t.Fatal(e)
	}
	scope, e := hostpolicy.NewScope("static", c)
	if e != nil {
		t.Fatal(e)
	}
	p, e := hostpolicy.Compile(plan, scope)
	if e != nil {
		t.Fatal(e)
	}
	return p
}

type applicationKernel struct {
	*markKernel
	images      []hostrules.Image
	applies     int
	applyErr    error
	afterApply  func()
	beforeApply func()
}

func (k *applicationKernel) ConfigureImages(_ string, images []hostrules.Image) { k.images = images }
func (k *applicationKernel) ApplyImage(_ context.Context, _ string, im hostrules.Image) error {
	k.applies++
	if k.beforeApply != nil {
		k.beforeApply()
	}
	if k.applyErr != nil {
		return k.applyErr
	}
	k.observation.Image = im.Digest()
	if k.afterApply != nil {
		k.afterApply()
	}
	return nil
}
func applicationSetup(t *testing.T) (Config, environment, *applicationKernel) {
	c, e, b := markSetup(t)
	k := &applicationKernel{markKernel: b}
	e.backend = k
	return c, e, k
}
func TestApplicationDurableAndRecovery(t *testing.T) {
	c, e, k := applicationSetup(t)
	s := mustOpen(t, c, e)
	p := admissionPolicy(t, true, false)
	k.beforeApply = func() {
		r, _, err := readRecord(c.StatePath)
		if err != nil || r.Application == nil || r.Application.Pending == nil || r.Application.Confirmed != nil {
			t.Fatal("pending not durable before dispatch", r, err)
		}
	}
	st, err := s.ApplyPolicy(context.Background(), p, markRange())
	if err != nil || !st.EnforcementConfirmed || st.Phase != "confirmed" || st.Attempt != 1 || st.ImageDigest == "" {
		t.Fatal(st, err)
	}
	if !s.Status().EnforcementConfirmed {
		t.Fatal("verified application missing from status")
	}
	k.beforeApply = nil
	before, _ := os.ReadFile(c.StatePath)
	if _, err = s.ApplyPolicy(context.Background(), p, markRange()); err != nil || k.applies != 1 {
		t.Fatal("no-op reapplied", err, k.applies)
	}
	after, _ := os.ReadFile(c.StatePath)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("no-op rewrote journal")
	}
	s.Close()
	if s.ApplicationStatus().EnforcementConfirmed {
		t.Fatal("closed session confirmed")
	}
	s = mustOpen(t, c, e)
	defer s.Close()
	if !s.ApplicationStatus().EnforcementConfirmed || k.applies != 1 {
		t.Fatal("live recovery re-dispatched or unconfirmed")
	}
	if _, err = s.ReconcileMarks(context.Background(), markRange(), nil); err == nil {
		t.Fatal("retired still-applied mark outside transaction")
	}
	if _, err = s.ApplyPolicy(context.Background(), admissionPolicy(t, false, true), markRange()); err != nil {
		t.Fatal(err)
	}
	if s.ApplicationStatus().Attempt != 2 {
		t.Fatal("withdrawal not applied")
	}
}
func TestApplicationUncertainPhases(t *testing.T) {
	for _, phase := range []string{"pending-save", "dispatch", "readback", "confirmed-save"} {
		t.Run(phase, func(t *testing.T) {
			c, e, k := applicationSetup(t)
			s := mustOpen(t, c, e)
			boom := errors.New(phase)
			switch phase {
			case "pending-save":
				s.persist = func(r record) error {
					if r.Application != nil && r.Application.Pending != nil {
						return boom
					}
					return writeRecord(c.StatePath, r)
				}
			case "dispatch":
				k.applyErr = boom
			case "readback":
				k.afterApply = func() { k.observeErr = boom }
			case "confirmed-save":
				s.persist = func(r record) error {
					if r.Application != nil && r.Application.Confirmed != nil && r.Application.Pending == nil {
						return boom
					}
					return writeRecord(c.StatePath, r)
				}
			}
			st, err := s.ApplyPolicy(context.Background(), admissionPolicy(t, true, false), markRange())
			if !errors.Is(err, boom) || st.EnforcementConfirmed || s.Status().EnforcementConfirmed {
				t.Fatal("ambiguous application reported success", st, err)
			}
			if _, err = s.ApplyPolicy(context.Background(), admissionPolicy(t, true, false), markRange()); err == nil {
				t.Fatal("faulted session reused")
			}
			s.Close()
			k.applyErr = nil
			k.observeErr = nil
			k.afterApply = nil
			s = mustOpen(t, c, e)
			defer s.Close()
			if s.ApplicationStatus().EnforcementConfirmed != (phase == "readback" || phase == "confirmed-save") {
				t.Fatal("wrong old/new recovery", s.ApplicationStatus())
			}
			if phase == "pending-save" && k.applies != 0 {
				t.Fatal("dispatched without pending durability")
			}
		})
	}
}
func TestApplicationDriftAndNewBoot(t *testing.T) {
	c, e, k := applicationSetup(t)
	s := mustOpen(t, c, e)
	if _, err := s.ApplyPolicy(context.Background(), admissionPolicy(t, true, false), markRange()); err != nil {
		t.Fatal(err)
	}
	s.Close()
	k.observation.Image = "unknown"
	if next, err := openWith(context.Background(), c, e); err == nil {
		next.Close()
		t.Fatal("unknown image adopted")
	}
	k.observation = observation{}
	e.namespace.BootID = "boot-2"
	e.namespace.Inode++
	s = mustOpen(t, c, e)
	defer s.Close()
	if s.ApplicationStatus().EnforcementConfirmed {
		t.Fatal("new boot restored application")
	}
	r, _, err := readRecord(c.StatePath)
	if err != nil || r.Marks == nil || r.Application.Confirmed != nil {
		t.Fatal(r, err)
	}
}
func TestImageIdentityInKernelLayout(t *testing.T) {
	p := admissionPolicy(t, true, false)
	targets, _ := p.MarkTargets()
	c, e, _ := applicationSetup(t)
	s := mustOpen(t, c, e)
	defer s.Close()
	marks := reserve(t, s, targets...)
	im, err := hostrules.Build(p, marks.State)
	if err != nil {
		t.Fatal(err)
	}
	layout, err := im.Layout(markerFor(s.record))
	if err != nil {
		t.Fatal(err)
	}
	if len(layout.Rules) < 2 || string(layout.Rules[1].UserData) != "picketx-image-v1:"+im.Digest() {
		t.Fatal("kernel cannot distinguish semantically different images")
	}
}
func TestApplicationValidationBeforeEffects(t *testing.T) {
	c, e, k := applicationSetup(t)
	s := mustOpen(t, c, e)
	defer s.Close()
	before, _ := os.ReadFile(c.StatePath)
	s.record.ProviderID = "other"
	if _, err := s.ApplyPolicy(context.Background(), admissionPolicy(t, true, false), markRange()); err == nil || k.applies != 0 || k.scans != 0 {
		t.Fatal("scope mismatch had effects")
	}
	s.record.ProviderID = "static"
	after, _ := os.ReadFile(c.StatePath)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("scope mismatch published")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.ApplyPolicy(ctx, admissionPolicy(t, true, false), markRange()); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	s.Close()
	if _, err := s.ApplyPolicy(context.Background(), admissionPolicy(t, true, false), markRange()); err == nil {
		t.Fatal("closed apply")
	}
}
func TestApplicationJournalTampering(t *testing.T) {
	c, e, _ := applicationSetup(t)
	s := mustOpen(t, c, e)
	defer s.Close()
	if _, err := s.ApplyPolicy(context.Background(), admissionPolicy(t, true, false), markRange()); err != nil {
		t.Fatal(err)
	}
	original, _ := os.ReadFile(c.StatePath)
	for _, which := range []string{"mark", "authority", "attempt", "context", "version"} {
		t.Run(which, func(t *testing.T) {
			var r record
			if err := json.Unmarshal(original, &r); err != nil {
				t.Fatal(err)
			}
			switch which {
			case "mark":
				r.Application.Confirmed.Targets[0].Mark++
			case "authority":
				r.Application.Confirmed.AuthorityID = "other"
			case "attempt":
				r.Application.Attempt = 0
			case "context":
				r.Application.Confirmed.Context.Handling = networkplan.Strict
			case "version":
				r.Version = 2
			}
			if validRecord(r) == nil {
				t.Fatal("tampered application journal accepted")
			}
		})
	}
}
