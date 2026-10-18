// SPDX-License-Identifier: GPL-3.0-or-later
package remotepolicy

import (
	"crypto/sha256"
	"github.com/PicketX/PicketX/pkg/agentsnapshot"
	"github.com/PicketX/PicketX/pkg/configuration"
	"github.com/PicketX/PicketX/pkg/networkplan"
	agentv1 "github.com/PicketX/PicketX/pkg/protocol/agent/v1alpha1"
	"google.golang.org/protobuf/proto"
	"net/netip"
	"os"
	"testing"
	"time"
)

func fixture(t *testing.T) (*agentv1.Snapshot, configuration.Facts) {
	t.Helper()
	raw, e := os.ReadFile("../../examples/prepare/policy.yaml")
	if e != nil {
		t.Fatal(e)
	}
	f := configuration.Facts{AuthorityID: "controller:test", Agents: []networkplan.Agent{{Name: "agent-a", Capabilities: &networkplan.CapabilityProfile{Classes: []networkplan.TrafficClass{networkplan.HostLocal}, Protocols: []networkplan.Protocol{networkplan.TCP, networkplan.UDP}, Families: []networkplan.Family{4, 6}, Handling: []networkplan.HandlingMode{networkplan.Graceful}, OriginalDestination: true}}}, Contexts: []networkplan.ExecutionContext{{AgentRef: "agent-a", ProfileRef: "host", ID: "controller-context", NetworkSpaceID: "remote-space", Class: networkplan.HostLocal, Stage: networkplan.OriginalDestination, Handling: networkplan.Graceful, Destinations: []netip.Prefix{netip.MustParsePrefix("192.0.2.100/32"), netip.MustParsePrefix("2001:db8::100/128")}}}}
	b, e := configuration.Decode(raw)
	if e != nil {
		t.Fatal(e)
	}
	c, e := configuration.Compile(b, f)
	if e != nil {
		t.Fatal(e)
	}
	s, e := agentsnapshot.Build(c, "agent-a", 1)
	if e != nil {
		t.Fatal(e)
	}
	f.Contexts[0].ID = "local-held"
	f.Contexts[0].NetworkSpaceID = "local-namespace"
	return s, f
}
func sign(s *agentv1.Snapshot) {
	s.Sha256 = nil
	b, _ := proto.MarshalOptions{Deterministic: true}.Marshal(s)
	h := sha256.Sum256(b)
	s.Sha256 = h[:]
}
func compile(t *testing.T, s *agentv1.Snapshot, f configuration.Facts) *configuration.Candidate {
	t.Helper()
	raw, e := Convert(s, f)
	if e != nil {
		t.Fatal(e)
	}
	b, e := configuration.Decode(raw)
	if e != nil {
		t.Fatal(e)
	}
	c, e := configuration.Compile(b, f)
	if e != nil {
		t.Fatal(e)
	}
	return c
}
func TestLocalMappingAndAbsoluteDeadlines(t *testing.T) {
	s, f := fixture(t)
	c := compile(t, s, f)
	inv, e := c.Plan().Inventory("agent-a")
	if e != nil {
		t.Fatal(e)
	}
	if len(inv) != 2 || inv[0].Protection.Context.ID != "local-held" || inv[0].Protection.Context.NetworkSpaceID != "local-namespace" {
		t.Fatal(inv)
	}
	at := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
	before, _ := c.Plan().Projection("agent-a", at)
	after, _ := c.Plan().Projection("agent-a", at.Add(4*time.Hour))
	if len(before.Allow) != 2 || len(after.Allow) != 0 || len(after.Protected) != 2 {
		t.Fatal(before, after)
	}
	s.Targets = nil
	sign(s)
	empty := compile(t, s, f)
	v, _ := empty.Plan().Inventory("agent-a")
	if len(v) != 0 {
		t.Fatal(v)
	}
}
func TestRejectEntireUnsupportedSnapshot(t *testing.T) {
	for _, name := range []string{"digest", "authority", "agent", "nil-target", "context", "stage", "handling", "destination", "match", "port", "capability", "unknown", "duplicate", "lifetime", "deadline", "source", "inconsistent-context"} {
		t.Run(name, func(t *testing.T) {
			s, f := fixture(t)
			switch name {
			case "digest":
				s.Sha256[0] ^= 1
			case "authority":
				s.AuthorityId = "other"
			case "agent":
				s.AgentRef = "other"
			case "nil-target":
				s.Targets = append(s.Targets, nil)
			case "context":
				s.Targets[0].Context.ProfileRef = "other"
			case "stage":
				s.Targets[0].Context.ObservationStage = "post-nat"
			case "handling":
				s.Targets[0].Context.Handling = "strict"
			case "destination":
				s.Targets[0].Context.Destinations = []string{"0.0.0.0/0"}
			case "match":
				s.Targets[0].Matches[0].Destination = "192.0.2.101/32"
			case "port":
				s.Targets[0].Matches[0].Port = 65536
			case "capability":
				s.Targets[0].Required.Families = []uint32{260}
			case "unknown":
				s.ProtoReflect().SetUnknown([]byte{0x30, 1})
			case "duplicate":
				s.Targets = append(s.Targets, proto.Clone(s.Targets[0]).(*agentv1.Target))
			case "lifetime":
				s.Targets[0].Grants[0].LifetimeMode = 257
			case "deadline":
				s.Targets[0].Grants[0].ExpiresAt = "invalid"
			case "source":
				s.Targets[0].Grants[0].Source = "bad"
			case "inconsistent-context":
				s.Targets[0].Context.NetworkSpaceId = "other"
			}
			if name != "digest" {
				sign(s)
			}
			if _, e := Convert(s, f); e == nil {
				t.Fatal("unsupported snapshot accepted")
			}
		})
	}
}
func TestDenyAndIndependentUnion(t *testing.T) {
	s, f := fixture(t)
	g := proto.Clone(s.Targets[0].Grants[0]).(*agentv1.Contribution)
	g.Id = "independent"
	g.ExpiresAt = "2026-10-15T13:00:00Z"
	s.Targets[0].Grants = append(s.Targets[0].Grants, g)
	deny := proto.Clone(g).(*agentv1.Contribution)
	deny.Id = "restriction"
	s.Targets[0].Denies = []*agentv1.Contribution{deny}
	sign(s)
	c := compile(t, s, f)
	p, e := c.Plan().Projection("agent-a", time.Date(2026, 10, 15, 11, 0, 0, 0, time.UTC))
	if e != nil {
		t.Fatal(e)
	}
	if len(p.Deny) == 0 || len(p.Allow) == 0 {
		t.Fatal(p)
	}
}
