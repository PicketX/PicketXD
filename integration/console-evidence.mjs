// SPDX-License-Identifier: GPL-3.0-or-later
import assert from 'node:assert/strict'
import { validateManagedHumans } from './managed-humans.mjs'
import { coreCommit, validateInstalledEvidence } from './controller-install.mjs'
export { coreCommit, verifyControllerBundle } from './controller-install.mjs'

const both = outcome => [[4, outcome], [6, outcome]]
export const requiredCases = {
  baseline: both('transport-deny'),
  grant4: [[4, 'allow'], [6, 'transport-deny']],
  grant6: both('allow'),
  sibling: both('transport-deny'),
  path: both('http-deny'), method: both('http-deny'), host: both('http-deny'),
  forged: both('http-deny'), overlap: both('allow'),
  browser: [[4, 'allow']],
  revoke4: [[4, 'transport-deny'], [6, 'allow']],
  establishedRevoke4: [[4, 'http-deny']],
  revoke6: both('transport-deny'),
  expiring: both('allow'), expired4: [[4, 'transport-deny'], [6, 'allow']], expired: both('transport-deny'),
  establishedExpired: both('http-deny'), unmanaged: both('allow'),
  humanPending4: both('transport-deny'),
  humanApproved4: [[4, 'allow'], [6, 'transport-deny']],
  humanPending6: [[4, 'allow'], [6, 'transport-deny']],
  humanApproved6: both('allow'), humanSibling: both('transport-deny'),
  humanBrowser: [[4, 'allow']],
  humanAccountDisabled: both('allow'), humanAccountReenabled: both('allow'),
  humanRevoke4: [[4, 'transport-deny'], [6, 'allow']],
  humanEstablishedRevoke4: [[4, 'http-deny']],
  humanExpired: both('transport-deny'),
  humanEstablishedExpired6: [[6, 'http-deny']],
  humanRejected: both('transport-deny'),
}
export function validateHumanApproval(r, identities) {
  const source = r.family === 4 ? '127.0.0.1/32' : '::1/128'
  assert([4, 6].includes(r.family))
  assert.notEqual(identities.requester, identities.approver)
  assert.equal(r.requester, identities.requester); assert.equal(r.approver, identities.approver)
  assert.equal(r.source, source); assert.equal(r.grantSource, source)
  assert.deepEqual(r.resource, { name: 'demo', uid: 'resource-demo-v1' })
  assert.deepEqual([...r.requestedEndpoints].sort(), ['sibling', 'web'])
  assert.deepEqual(r.approvedEndpoints, ['web']); assert.deepEqual(r.grantEndpoints, ['web'])
  const requested = Date.parse(r.requestedExpiry), approved = Date.parse(r.approvedExpiry)
  assert(Number.isFinite(requested) && Number.isFinite(approved) && approved < requested, 'shortened fixed absolute deadline')
  assert.equal(r.grantExpiry, r.approvedExpiry)
  // Retain and check the entire public Grant view, not just its first scope.
  assert.deepEqual(r.grant, { name: r.uid, uid: r.grantUID, source,
    scopes: [{ resource_name: 'demo', resource_uid: 'resource-demo-v1', selection: 'named', endpoint_names: ['web'] }],
    mode: 'bounded', expires_at: r.approvedExpiry, revoked: false })
  assert.equal(r.decisionReason, `Native narrowed IPv${r.family}`)
  assert.equal(r.auditActor, r.approver)
  assert.equal(r.auditRevision, r.revision)
  assert.equal(r.auditReason, `approve ${r.uid}: ${r.decisionReason}`)
}
export function validateEvidence(report) {
  validateInstalledEvidence(report.deployment, report)
  assert.equal(report.coreCommit, coreCommit, 'pinned Core source')
  const identities = validateManagedHumans(report)
  for (const name of ['agentCommit', 'agentTree']) assert.match(report[name] ?? '', /^[a-f0-9]{40}$/, name)
  for (const name of ['agentBinarySHA256', 'controllerBinarySHA256']) assert.match(report[name] ?? '', /^[a-f0-9]{64}$/, name)
  assert.equal(typeof report.kernel, 'string'); assert(report.kernel.length > 0)
  assert(report.confirmations.length > 0, 'actual enforcement confirmations required')
  for (const c of report.confirmations) {
    assert(Number.isSafeInteger(c.revision) && c.revision > 0)
    assert.equal(c.generation, c.revision); assert.equal(c.appliedGeneration, c.revision)
    assert.equal(c.enforcementConfirmed, true)
    assert.match(c.policyHash ?? '', /^[a-f0-9]{64}$/)
    assert.equal(c.appliedPolicyHash, c.policyHash)
  }
  assert(Array.isArray(report.humanRequests), 'actual human workflow proof required')
  assert.equal(report.humanRequests.length, 2, 'both human source families required')
  const timeline = report.humanTimeline
  for (const key of ['baseline', 'approved4', 'approved6', 'revoked4', 'expired', 'rejected']) {
    assert(Number.isSafeInteger(timeline?.[key]) && timeline[key] > 0, 'complete human revision timeline')
    assert(report.confirmations.some(c => c.revision === timeline[key]), 'each transition requires actual confirmation')
  }
  assert.equal(timeline.approved4, timeline.baseline + 1)
  assert.equal(timeline.approved6, timeline.approved4 + 1)
  assert.equal(timeline.revoked4, timeline.approved6 + 1)
  assert.equal(timeline.expired, timeline.revoked4); assert.equal(timeline.rejected, timeline.revoked4)
  const caseRevisions = { humanPending4: timeline.baseline, humanApproved4: timeline.approved4, humanPending6: timeline.approved4,
    humanApproved6: timeline.approved6, humanSibling: timeline.approved6, humanBrowser: timeline.approved6,
    humanAccountDisabled: timeline.approved6, humanAccountReenabled: timeline.approved6,
    humanRevoke4: timeline.revoked4, humanEstablishedRevoke4: timeline.revoked4, humanExpired: timeline.expired,
    humanEstablishedExpired6: timeline.expired, humanRejected: timeline.rejected }
  const uids = new Set(), grants = new Set()
  report.humanRequests.forEach((r, i) => {
    const family = [4, 6][i]
    assert.equal(r.family, family)
    for (const field of ['uid', 'grantUID']) assert.equal(typeof r[field], 'string')
    assert(r.uid.length > 0 && !uids.has(r.uid)); uids.add(r.uid)
    assert(r.grantUID.length > 0 && !grants.has(r.grantUID)); grants.add(r.grantUID)
    validateHumanApproval(r, identities)
    assert.equal(r.revision, i === 0 ? timeline.approved4 : timeline.approved6, 'approval/case revision correlation')
    assert(report.confirmations.some(c => c.revision === r.revision), 'approved revision actually applied')
    assert.equal(r.finalGrantState, i === 0 ? 'revoked' : 'expired')
  })
  assert.equal(report.cases.length, Object.keys(requiredCases).length, 'exact required cases')
  const names = new Set()
  for (const c of report.cases) {
    assert.equal(typeof c.name, 'string', 'case name must be a string')
    assert(!names.has(c.name), 'duplicate case'); names.add(c.name)
    assert(Object.hasOwn(requiredCases, c.name), 'unknown case')
    const expected = requiredCases[c.name]
    assert.equal(c.status, 'pass', c.name)
    assert(report.confirmations.some(x => x.revision === c.revision), 'case requires matching actual enforcement confirmation')
    if (Object.hasOwn(caseRevisions, c.name)) assert.equal(c.revision, caseRevisions[c.name], 'human case must use its exact transition revision')
    assert.equal(c.observations.length, expected.length, c.name)
    c.observations.forEach((o, i) => {
      const [family, outcome] = expected[i]
      assert.equal(o.family, family, c.name); assert.equal(o.outcome, outcome, c.name)
      assert.equal(o.status, outcome === 'allow' ? 200 : outcome === 'http-deny' ? 403 : null, c.name)
      assert.equal(o.appDelta, outcome === 'allow' ? 1 : 0, c.name)
    })
  }
}
