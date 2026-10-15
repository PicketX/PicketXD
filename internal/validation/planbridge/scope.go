// SPDX-License-Identifier: GPL-3.0-or-later
package planbridge

import (
	"fmt"
	"net/netip"
	"slices"
	"strings"

	"github.com/PicketX/PicketX/pkg/networkplan"
)

type Identity struct{ AuthorityID, Epoch, AgentRef, NamespaceID string }
type Catalog uint8

const (
	Ordinary Catalog = 1
	NAT      Catalog = 2
)

type Scope struct {
	identity Identity
	context  networkplan.ExecutionContext
	catalog  Catalog
	valid    bool
}

// NewScope validates declarative test facts. A later topology guard must prove
// these facts against its live namespace before any backend action.
func NewScope(id Identity, context networkplan.ExecutionContext, catalog Catalog) (*Scope, error) {
	for _, value := range []string{id.AuthorityID, id.Epoch, id.AgentRef, id.NamespaceID} {
		if strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("fixture identity fields must not be empty")
		}
	}
	if context.AgentRef != id.AgentRef || context.NetworkSpaceID != id.NamespaceID {
		return nil, fmt.Errorf("fixture Agent/namespace identity mismatch")
	}
	if context.Class != networkplan.Transit || context.Stage != networkplan.OriginalDestination {
		return nil, fmt.Errorf("fixture requires transit at original destination")
	}
	if context.Handling != networkplan.Graceful && context.Handling != networkplan.Strict {
		return nil, fmt.Errorf("unsupported fixture handling %q", context.Handling)
	}
	var destinations []netip.Prefix
	switch catalog {
	case Ordinary:
		destinations = []netip.Prefix{netip.MustParsePrefix("192.0.2.100/32"), netip.MustParsePrefix("2001:db8:1:0:2::100/128")}
	case NAT:
		destinations = []netip.Prefix{netip.MustParsePrefix("192.0.2.101/32"), netip.MustParsePrefix("2001:db8:1:0:2::101/128")}
	default:
		return nil, fmt.Errorf("unsupported fixture catalog %d", catalog)
	}
	for _, destination := range context.Destinations {
		if !slices.Contains(destinations, destination) {
			return nil, fmt.Errorf("destination %s is outside fixture catalog", destination)
		}
	}
	normalized, err := networkplan.NormalizeExecutionContext(context)
	if err != nil {
		return nil, err
	}
	return &Scope{identity: id, context: normalized, catalog: catalog, valid: true}, nil
}

func (s *Scope) supports(m networkplan.Match) bool {
	if !slices.Contains(s.context.Destinations, m.Destination) {
		return false
	}
	if s.catalog == NAT {
		return m.Protocol == networkplan.TCP && (m.Port == 8000 || m.Port == 8001)
	}
	return (m.Protocol == networkplan.TCP && (m.Port == 443 || m.Port == 22)) || (m.Protocol == networkplan.UDP && m.Port == 8002)
}
