// SPDX-License-Identifier: GPL-3.0-or-later
// Package remotepolicy translates authenticated intent, never remote kernel facts.
package remotepolicy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/netip"
	"sort"
	"strings"
	"time"

	"github.com/PicketX/PicketX/pkg/agentsnapshot"
	"github.com/PicketX/PicketX/pkg/configuration"
	"github.com/PicketX/PicketX/pkg/core"
	"github.com/PicketX/PicketX/pkg/networkplan"
	agentv1 "github.com/PicketX/PicketX/pkg/protocol/agent/v1alpha1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type object map[string]any

func document(kind, id string, spec any) object {
	return object{"apiVersion": configuration.APIVersion, "kind": kind, "metadata": object{"name": id, "uid": id}, "spec": spec}
}
func ref(resource, endpoint string) object {
	return object{"resourceRef": object{"name": resource, "uid": resource}, "endpointNames": []string{endpoint}}
}
func within(p netip.Prefix, bounds []netip.Prefix) bool {
	for _, b := range bounds {
		if p.Addr().BitLen() == b.Addr().BitLen() && p.Bits() >= b.Bits() && b.Contains(p.Addr()) {
			return true
		}
	}
	return false
}
func unknown(m protoreflect.Message) bool {
	if len(m.GetUnknown()) > 0 {
		return true
	}
	bad := false
	m.Range(func(f protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		if f.IsList() && f.Message() != nil {
			l := v.List()
			for i := 0; i < l.Len(); i++ {
				if unknown(l.Get(i).Message()) {
					bad = true
					break
				}
			}
		} else if f.Message() != nil && !f.IsMap() {
			bad = unknown(v.Message())
		}
		return !bad
	})
	return bad
}

// Convert accepts only the currently implemented single host profile. Namespace
// IDs are checked for internal consistency and replaced with held local facts.
func Convert(s *agentv1.Snapshot, f configuration.Facts) ([]byte, error) {
	if e := agentsnapshot.Verify(s); e != nil {
		return nil, e
	}
	if unknown(s.ProtoReflect()) || proto.Size(s) > 4<<20 || len(s.Targets) > 1024 {
		return nil, fmt.Errorf("unsupported or oversized remote snapshot")
	}
	if s.AuthorityId != f.AuthorityID || len(f.Agents) != 1 || s.AgentRef != f.Agents[0].Name || len(f.Contexts) != 1 {
		return nil, fmt.Errorf("remote identity differs from local scope")
	}
	local := f.Contexts[0]
	resources := map[string][]object{}
	bindings := map[string][]object{}
	contribs := map[string]*agentv1.Contribution{}
	scopes := map[string][]object{}
	seen := map[string]bool{}
	var remote *agentv1.Context
	for _, t := range s.Targets {
		if t == nil || t.Key == nil || t.Context == nil || t.Required == nil || len(t.Matches) == 0 || len(t.BindingIds) == 0 {
			return nil, fmt.Errorf("incomplete remote target")
		}
		k, c := t.Key, t.Context
		key := k.ResourceId + "\x00" + k.EndpointName
		if seen[key] || k.ResourceId == "" || k.EndpointName == "" || k.AgentRef != s.AgentRef || c.AgentRef != s.AgentRef || k.ContextId != c.Id || c.Id == "" || c.NetworkSpaceId == "" {
			return nil, fmt.Errorf("ambiguous target identity")
		}
		seen[key] = true
		if remote == nil {
			remote = c
		} else if !proto.Equal(remote, c) {
			return nil, fmt.Errorf("multiple remote contexts are unsupported")
		}
		if c.ProfileRef != local.ProfileRef || c.TrafficClass != string(networkplan.HostLocal) || c.ObservationStage != string(networkplan.OriginalDestination) || c.Handling != string(local.Handling) {
			return nil, fmt.Errorf("unsupported remote host profile")
		}
		bounds, e := networkplan.ParseDestinations(c.Destinations)
		if e != nil || len(bounds) == 0 {
			return nil, fmt.Errorf("invalid remote destination bounds")
		}
		for _, p := range bounds {
			if !within(p, local.Destinations) {
				return nil, fmt.Errorf("remote context exceeds local bounds")
			}
		}
		var addresses []string
		protocol := ""
		port := uint32(0)
		matches := map[string]bool{}
		for _, m := range t.Matches {
			if m == nil || m.Port == 0 || m.Port > 65535 || (m.Protocol != "tcp" && m.Protocol != "udp") {
				return nil, fmt.Errorf("unsupported remote match")
			}
			p, e := netip.ParsePrefix(m.Destination)
			if e != nil || p != p.Masked() || !within(p, bounds) || !within(p, local.Destinations) || matches[p.String()] {
				return nil, fmt.Errorf("match outside local scope or duplicated")
			}
			matches[p.String()] = true
			if protocol == "" {
				protocol = m.Protocol
				port = m.Port
			}
			if protocol != m.Protocol || port != m.Port {
				return nil, fmt.Errorf("mixed network entry")
			}
			addresses = append(addresses, p.String())
		}
		resources[k.ResourceId] = append(resources[k.ResourceId], object{"name": k.EndpointName, "network": object{"protocol": strings.ToUpper(protocol), "port": port, "addresses": addresses}})
		ids := map[string]bool{}
		for _, id := range t.BindingIds {
			if id == "" || ids[id] {
				return nil, fmt.Errorf("invalid binding IDs")
			}
			ids[id] = true
			bindings[id] = append(bindings[id], ref(k.ResourceId, k.EndpointName))
		}
		for _, domain := range []struct {
			kind   string
			values []*agentv1.Contribution
		}{{"Grant", t.Grants}, {"NetworkDeny", t.Denies}} {
			unique := map[string]bool{}
			for _, g := range domain.values {
				if g == nil || g.Id == "" || unique[g.Id] {
					return nil, fmt.Errorf("invalid contribution")
				}
				unique[g.Id] = true
				if _, e := core.ParseSource(g.Source); e != nil {
					return nil, e
				}
				if g.LifetimeMode != 1 && g.LifetimeMode != 2 {
					return nil, fmt.Errorf("unsupported lifetime")
				}
				v := core.Validity{Mode: core.LifetimeMode(g.LifetimeMode)}
				for _, item := range []struct {
					s string
					p *time.Time
				}{{g.NotBefore, &v.NotBefore}, {g.ExpiresAt, &v.ExpiresAt}} {
					if item.s != "" {
						at, e := time.Parse(time.RFC3339Nano, item.s)
						if e != nil || at.IsZero() {
							return nil, fmt.Errorf("invalid absolute deadline")
						}
						*item.p = at
					}
				}
				if _, e := core.NormalizeValidity(v); e != nil {
					return nil, e
				}
				id := domain.kind + "/" + g.Id
				if old := contribs[id]; old != nil && !proto.Equal(old, g) {
					return nil, fmt.Errorf("contribution identity changed within snapshot")
				}
				contribs[id] = g
				sc := ref(k.ResourceId, k.EndpointName)
				sc["actions"] = []string{"connect"}
				scopes[id] = append(scopes[id], sc)
			}
		}
	}
	var docs []object
	for _, id := range sortedKeys(resources) {
		docs = append(docs, document("Resource", id, object{"endpoints": resources[id]}))
	}
	for _, id := range sortedKeys(contribs) {
		g := contribs[id]
		kind, _, _ := strings.Cut(id, "/")
		mode := "bounded"
		if g.LifetimeMode == 2 {
			mode = "permanent"
		}
		validity := object{"mode": mode}
		if g.NotBefore != "" {
			validity["notBefore"] = g.NotBefore
		}
		if g.ExpiresAt != "" {
			validity["expiresAt"] = g.ExpiresAt
		}
		docs = append(docs, document(kind, g.Id, object{"subject": object{"type": "source", "cidr": g.Source}, "scopes": scopes[id], "validity": validity, "revoked": g.Revoked}))
	}
	for _, id := range sortedKeys(bindings) {
		docs = append(docs, document("EnforcementBinding", id, object{"resources": bindings[id], "agents": []object{{"agentRef": object{"name": s.AgentRef}}}, "enforcement": object{"profileRef": local.ProfileRef, "class": string(local.Class), "stage": string(local.Stage), "handling": string(local.Handling), "allowedDestinations": prefixStrings(local.Destinations)}}))
	}
	if len(docs) == 0 {
		docs = append(docs, document("Configuration", "remote-empty", object{"empty": true}))
	}
	var out bytes.Buffer
	for i, d := range docs {
		if i > 0 {
			out.WriteString("\n---\n")
		}
		b, e := json.Marshal(d)
		if e != nil {
			return nil, e
		}
		out.Write(b)
	}
	b, e := configuration.Decode(out.Bytes())
	if e != nil {
		return nil, e
	}
	candidate, e := configuration.Compile(b, f)
	if e != nil {
		return nil, e
	}
	// Recompute capability requirements and full scoped inventory; a field not
	// represented by the supported adapter must not be silently dropped.
	round, e := agentsnapshot.Build(candidate, s.AgentRef, s.Revision)
	if e != nil {
		return nil, e
	}
	want := map[string]*agentv1.Target{}
	for _, t := range s.Targets {
		want[t.Key.ResourceId+"\x00"+t.Key.EndpointName] = t
	}
	if len(round.Targets) != len(want) {
		return nil, fmt.Errorf("remote inventory not preserved")
	}
	for _, t := range round.Targets {
		w := want[t.Key.ResourceId+"\x00"+t.Key.EndpointName]
		if w == nil {
			return nil, fmt.Errorf("remote target not preserved")
		}
		if !proto.Equal(w.Required, t.Required) {
			return nil, fmt.Errorf("unsupported capability requirements")
		}
	}
	return out.Bytes(), nil
}
func sortedKeys[V any](m map[string]V) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
func prefixStrings(ps []netip.Prefix) []string {
	var out []string
	for _, p := range ps {
		out = append(out, p.String())
	}
	return out
}
