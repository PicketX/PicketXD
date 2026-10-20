// SPDX-License-Identifier: GPL-3.0-or-later
// Hand-derived UNIT data only. Never written as native acceptance evidence.
export function auditFixture(principal = 'principal:account-' + '2'.repeat(32), revision = 2) {
  const events = [
    { id: 11, caller: 'initial-ingress', route: 'route-v1', source: '127.0.0.1', allowed: true, reason: 'allowed', revision, grant_ids: ['grant-four'], deny_ids: [] },
    { id: 12, caller: 'initial-ingress', route: 'route-v1', source: '127.0.0.1', allowed: false, reason: 'route-mismatch', revision, grant_ids: [], deny_ids: [] },
    { id: 13, caller: 'initial-ingress', route: 'route-v1', source: '::1', allowed: true, reason: 'allowed', revision, grant_ids: ['grant-six'], deny_ids: [] },
    { id: 14, caller: 'initial-ingress', route: 'route-v1', source: '::1', allowed: false, reason: 'route-mismatch', revision, grant_ids: [], deny_ids: [] },
  ]
  return { mode: 'password', revision, principal, grants: ['grant-four', 'grant-six'],
    probes: events.map((event, i) => ({ name: ['allow4', 'deny4', 'allow6', 'deny6'][i], family: i < 2 ? 4 : 6,
      before: 10 + i, event, status: event.allowed ? 200 : 403, appDelta: event.allowed ? 1 : 0 })),
    rows: structuredClone(events), viewports: [1440, 375] }
}
