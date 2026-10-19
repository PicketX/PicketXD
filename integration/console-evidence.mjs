// SPDX-License-Identifier: GPL-3.0-or-later
import assert from 'node:assert/strict'

export const coreCommit = 'a99d9fc479bdf1490f78e669291ec67ca84876e3'
const both = outcome => [[4, outcome], [6, outcome]]
export const requiredCases = {
  baseline: both('transport-deny'),
  grant4: [[4, 'allow'], [6, 'transport-deny']],
  grant6: both('allow'),
  sibling: both('transport-deny'),
  path: both('http-deny'), method: both('http-deny'), host: both('http-deny'),
  forged: both('http-deny'), overlap: both('allow'),
  revoke4: [[4, 'transport-deny'], [6, 'allow']],
  establishedRevoke4: [[4, 'http-deny']],
  revoke6: both('transport-deny'),
  expiring: both('allow'), expired: both('transport-deny'),
  establishedExpired: both('http-deny'), unmanaged: both('allow'),
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
  assert.equal(report.cases.length, Object.keys(requiredCases).length, 'exact required cases')
  const names = new Set()
  for (const c of report.cases) {
    assert(!names.has(c.name), 'duplicate case'); names.add(c.name)
    const expected = requiredCases[c.name]; assert(expected, 'unknown case')
    assert.equal(c.status, 'pass', c.name)
    assert.equal(c.observations.length, expected.length, c.name)
    c.observations.forEach((o, i) => {
      const [family, outcome] = expected[i]
      assert.equal(o.family, family, c.name); assert.equal(o.outcome, outcome, c.name)
      assert.equal(o.status, outcome === 'allow' ? 200 : outcome === 'http-deny' ? 403 : null, c.name)
      assert.equal(o.appDelta, outcome === 'allow' ? 1 : 0, c.name)
    })
  }
}
