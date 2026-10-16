// SPDX-License-Identifier: GPL-3.0-or-later
// Package hostagent composes static desired state and namespace-owned admission.
package hostagent

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/PicketX/PicketX/pkg/configuration"
	"github.com/PicketX/PicketX/pkg/networkplan"
	"github.com/PicketX/PicketXD/internal/hostowner"
	"github.com/PicketX/PicketXD/internal/hostpolicy"
	"github.com/PicketX/PicketXD/internal/markstate"
	"net/netip"
	"path/filepath"
	"time"
)

type Config struct {
	PolicyPath, StateDir, ProviderID, AgentRef, Profile string
	Destinations                                        []netip.Prefix
	Marks                                               markstate.Range
	Poll                                                time.Duration
	Once                                                bool
}

func (c Config) normalized() (Config, error) {
	if c.PolicyPath == "" || !filepath.IsAbs(c.StateDir) || filepath.Clean(c.StateDir) == "/" || c.Poll <= 0 {
		return c, fmt.Errorf("policy, absolute non-root state-dir and positive poll required")
	}
	if e := c.Marks.Validate(); e != nil {
		return c, e
	}
	var raw []string
	for _, p := range c.Destinations {
		raw = append(raw, p.String())
	}
	bounds, e := networkplan.ParseDestinations(raw)
	if e != nil || len(bounds) == 0 {
		return c, fmt.Errorf("explicit valid destination bounds required")
	}
	c.Destinations = bounds
	c.StateDir = filepath.Clean(c.StateDir)
	_, e = hostpolicy.NewScope(c.ProviderID, networkplan.ExecutionContext{AgentRef: c.AgentRef, ProfileRef: c.Profile, ID: "pending", NetworkSpaceID: "pending", Class: networkplan.HostLocal, Stage: networkplan.OriginalDestination, Handling: networkplan.Graceful, Destinations: bounds})
	return c, e
}
func (c Config) runtimeKey() string {
	data, _ := json.Marshal(struct {
		Agent, Profile string
		Destinations   []netip.Prefix
		Marks          markstate.Range
	}{c.AgentRef, c.Profile, c.Destinations, c.Marks})
	return fmt.Sprintf("%x", sha256.Sum256(data))
}
func (c Config) facts(scope hostowner.RuntimeScope) (configuration.Facts, *hostpolicy.Scope, error) {
	if scope.BootID == "" || scope.NetworkSpaceID == "" {
		return configuration.Facts{}, nil, fmt.Errorf("held namespace identity required")
	}
	context := networkplan.ExecutionContext{AgentRef: c.AgentRef, ProfileRef: c.Profile, ID: scope.NetworkSpaceID + "/" + c.runtimeKey(), NetworkSpaceID: scope.NetworkSpaceID, Class: networkplan.HostLocal, Stage: networkplan.OriginalDestination, Handling: networkplan.Graceful, Destinations: c.Destinations}
	sc, e := hostpolicy.NewScope(c.ProviderID, context)
	if e != nil {
		return configuration.Facts{}, nil, e
	}
	f := configuration.Facts{AuthorityID: c.ProviderID, Agents: []networkplan.Agent{{Name: c.AgentRef, Capabilities: &networkplan.CapabilityProfile{Classes: []networkplan.TrafficClass{networkplan.HostLocal}, Protocols: []networkplan.Protocol{networkplan.TCP, networkplan.UDP}, Families: []networkplan.Family{networkplan.IPv4, networkplan.IPv6}, Handling: []networkplan.HandlingMode{networkplan.Graceful}, OriginalDestination: true}}}, Contexts: []networkplan.ExecutionContext{context}}
	return f, sc, nil
}
