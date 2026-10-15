// SPDX-License-Identifier: GPL-3.0-or-later

// Package planbridge compiles shared network plans into private M0 fixtures.
// It is isolated validation infrastructure, not a production policy provider.
//
// A Scope fixes declarative identity, context and the supported fixture tuple
// catalog. NewScope does not inspect a live namespace or prove a traffic path;
// a later topology guard must bind and recheck those facts before any side effect.
//
// Prepare validates the complete authoritative candidate and selected fixture
// targets; a predecessor means the coordinator's accepted desired plan, never
// proof of installed permissions. Materialize uses only an explicit evaluation
// time. Returned snapshots, provenance and inventory are owned by the caller.
// Grant and Deny membership remains separate; the M0 backend must apply deny
// precedence. Only an explicitly permanent contribution has a zero expiry.
//
// An empty snapshot is explicit empty-owned intent. Existing M0 Apply/Revoke
// require nonempty protection; callers must not synthesize wildcard protection
// or treat this result as permission for unscoped cleanup. The fixture Coordinator adds live guards, ACK/readback confirmation, actual
// emitted receipts/history, timers and exact empty-table cleanup. A successful
// conversion alone is not applied access.
package planbridge
