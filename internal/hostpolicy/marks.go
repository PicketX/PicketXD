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
		target, e := TargetIdentity(protection.Key, protection.Context, protection.Matches)
		if e != nil {
			return nil, e
		}

		targets = append(targets, target)
	}
	return targets, nil
}

// TargetIdentity describes physical shape only; it is not an ownership receipt.
func TargetIdentity(key networkplan.TargetKey, context networkplan.ExecutionContext, matches []networkplan.Match) (markstate.Target, error) {
	data, e := json.Marshal(struct {
		Context networkplan.ExecutionContext
		Matches []networkplan.Match
	}{context, matches})
	if e != nil {
		return markstate.Target{}, e
	}
	hash := sha256.Sum256(data)
	target := markstate.Target{Key: key, Shape: hex.EncodeToString(hash[:])}
	if e = target.Validate(); e != nil {
		return markstate.Target{}, e
	}
	return target, nil
}
