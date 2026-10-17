// SPDX-License-Identifier: GPL-3.0-or-later
package hostagent

import (
	"github.com/PicketX/PicketXD/internal/hostrevoke"
	"time"
)

// StrictStatus is an observation of this reconcile, never a durable receipt.
// Consumers must also match current desired/image identity and validUntil.
type StrictStatus struct {
	Enabled          bool              `json:"enabled"`
	CleanupConfirmed bool              `json:"cleanupConfirmed"`
	ObservedAt       time.Time         `json:"observedAt"`
	ValidUntil       time.Time         `json:"validUntil"`
	ImageDigest      string            `json:"imageDigest,omitempty"`
	Result           hostrevoke.Result `json:"result"`
}
