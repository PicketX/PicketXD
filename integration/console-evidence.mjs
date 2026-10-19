// SPDX-License-Identifier: GPL-3.0-or-later
import assert from 'node:assert/strict'

export const coreCommit = 'b79e242926c67d931537c160333008df699fa598'
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
  humanRevoke4: [[4, 'transport-deny'], [6, 'allow']],
  humanEstablishedRevoke4: [[4, 'http-deny']],
  humanExpired: both('transport-deny'),
  humanEstablishedExpired6: [[6, 'http-deny']],
  humanRejected: both('transport-deny'),
}
export function validateEvidence(report) {
  assert.equal(report.coreCommit, coreCommit, 'pinned Core source')
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
  const uids = new Set(), grants = new Set()
  report.humanRequests.forEach((r, i) => {
    const family = [4, 6][i], source = ['127.0.0.1/32', '::1/128'][i]
    assert.equal(r.family, family)
    for (const field of ['uid', 'grantUID']) assert.equal(typeof r[field], 'string')
    assert(r.uid.length > 0 && !uids.has(r.uid)); uids.add(r.uid)
    assert(r.grantUID.length > 0 && !grants.has(r.grantUID)); grants.add(r.grantUID)
    assert.equal(r.requester, 'principal:requester'); assert.equal(r.approver, 'principal:approver')
    assert.equal(r.source, source); assert.equal(r.grantSource, source)
    assert.deepEqual([...r.requestedEndpoints].sort(), ['sibling', 'web'])
    assert.deepEqual(r.approvedEndpoints, ['web']); assert.deepEqual(r.grantEndpoints, ['web'])
    const requested = Date.parse(r.requestedExpiry), approved = Date.parse(r.approvedExpiry)
    assert(Number.isFinite(requested) && Number.isFinite(approved) && approved < requested, 'shortened fixed absolute deadline')
    assert.equal(r.grantExpiry, r.approvedExpiry)
    assert(report.confirmations.some(c => c.revision === r.revision), 'approved revision actually applied')
    assert.equal(r.auditActor, r.approver); assert.equal(typeof r.auditReason, 'string'); assert(r.auditReason.includes(r.uid))
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
    assert.equal(c.observations.length, expected.length, c.name)
    c.observations.forEach((o, i) => {
      const [family, outcome] = expected[i]
      assert.equal(o.family, family, c.name); assert.equal(o.outcome, outcome, c.name)
      assert.equal(o.status, outcome === 'allow' ? 200 : outcome === 'http-deny' ? 403 : null, c.name)
      assert.equal(o.appDelta, outcome === 'allow' ? 1 : 0, c.name)
    })
  }
}
