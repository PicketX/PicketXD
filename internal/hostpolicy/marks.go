// SPDX-License-Identifier: GPL-3.0-or-later
package hostpolicy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/PicketX/PicketX/pkg/networkplan"
	"github.com/PicketX/PicketXD/internal/markstate"
)

// MarkTargets describes complete physical target incarnations. Permission
// membership, deadlines and Binding provenance do not rotate a graceful mark.
// This produces requests only; callers still need durable ownership reservation.
func (p *Policy) MarkTargets() ([]markstate.Target, error) {
	inventory, e := p.Inventory()
	if e != nil {
		return nil, e
	}
	var targets []markstate.Target
	for _, item := range inventory {
		protection := item.Protection
		data, e := json.Marshal(struct {
			Context networkplan.ExecutionContext
			Matches []networkplan.Match
		}{protection.Context, protection.Matches})
		if e != nil {
			return nil, e
		}
		hash := sha256.Sum256(data)
		target := markstate.Target{Key: protection.Key, Shape: hex.EncodeToString(hash[:])}
		if e = target.Validate(); e != nil {
			return nil, e
		}
		targets = append(targets, target)
	}
	return targets, nil
}
