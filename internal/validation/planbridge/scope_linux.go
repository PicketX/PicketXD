package planbridge

import (
	"github.com/PicketX/PicketX/pkg/networkplan"
	"slices"
)

// Facts are the declarative contract revalidated by the trusted live topology
// guard before each backend action. They do not themselves prove isolation.
type Facts struct {
	AuthorityID, Epoch, NamespaceID string
	Context                         networkplan.ExecutionContext
	NAT                             bool
}

func (s *Scope) Facts() Facts {
	c := s.context
	c.Destinations = slices.Clone(c.Destinations)
	return Facts{AuthorityID: s.identity.AuthorityID, Epoch: s.identity.Epoch, NamespaceID: s.identity.NamespaceID, Context: c, NAT: s.catalog == NAT}
}
