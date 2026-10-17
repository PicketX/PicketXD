// SPDX-License-Identifier: GPL-3.0-or-later
package hostagent

import (
	"github.com/PicketX/PicketXD/internal/hostrevoke"
	"time"
)

// StrictStatus is an observation of this reconcile, never a durable receipt.
// Consumers must also match current desired/image identity and validUntil.
type StrictStatus struct {
	PreviousCleanup  *CleanupObservation `json:"previousCleanup,omitempty"`
	Enabled          bool                `json:"enabled"`
	CleanupConfirmed bool                `json:"cleanupConfirmed"`
	ObservedAt       time.Time           `json:"observedAt"`
	ValidUntil       time.Time           `json:"validUntil"`
	ImageDigest      string              `json:"imageDigest,omitempty"`
	Result           hostrevoke.Result   `json:"result"`
}

// CleanupObservation identifies cleanup of the retained image before a possible
// replacement. It is separate from the current desired-state proof.
type CleanupObservation struct {
	Confirmed   bool              `json:"confirmed"`
	ImageDigest string            `json:"imageDigest"`
	ObservedAt  time.Time         `json:"observedAt"`
	Result      hostrevoke.Result `json:"result"`
}
