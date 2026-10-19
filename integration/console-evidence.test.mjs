// SPDX-License-Identifier: GPL-3.0-or-later
import test from 'node:test'
import assert from 'node:assert/strict'
import { validateEvidence, requiredCases, coreCommit } from './console-evidence.mjs'

// Unit data only; never emitted as native acceptance evidence.
const fixture = () => ({
  coreCommit, agentCommit: 'a'.repeat(40), agentTree: 'b'.repeat(40),
  kernel: 'unit-fixture', agentBinarySHA256: 'c'.repeat(64), controllerBinarySHA256: 'd'.repeat(64),
  confirmations: [{ revision: 2, generation: 2, appliedGeneration: 2, policyHash: 'e'.repeat(64), appliedPolicyHash: 'e'.repeat(64), enforcementConfirmed: true }],
  humanRequests: [4, 6].map(family => ({
    family, uid: `request-${family}`, requester: 'principal:requester', approver: 'principal:approver',
    source: family === 4 ? '127.0.0.1/32' : '::1/128', requestedEndpoints: ['web', 'sibling'],
    approvedEndpoints: ['web'], requestedExpiry: '2026-10-19T09:01:00Z', approvedExpiry: '2026-10-19T09:00:30Z',
    grantUID: `grant-${family}`, grantSource: family === 4 ? '127.0.0.1/32' : '::1/128',
    grantEndpoints: ['web'], grantExpiry: '2026-10-19T09:00:30Z', revision: 2,
    auditActor: 'principal:approver', auditReason: `approve request-${family}`,
    finalGrantState: family === 4 ? 'revoked' : 'expired',
  })),
  cases: Object.entries(requiredCases).map(([name, specs]) => ({ name, revision: 2, status: 'pass', observations: specs.map(([family, outcome]) => ({ family, outcome, status: outcome === 'allow' ? 200 : outcome === 'http-deny' ? 403 : null, appDelta: outcome === 'allow' ? 1 : 0 })) })),
})
test('complete unit report satisfies the evidence schema', () => assert.doesNotThrow(() => validateEvidence(fixture())))
for (const [name, mutate] of [
  ['missing case', r => r.cases.pop()],
  ['duplicate case', r => r.cases.push(r.cases[0])],
  ...['toString', 'valueOf', 'toLocaleString', 'constructor', '__proto__'].map(name => [`inherited case name ${name}`, r => { r.cases[0].name = name; r.cases[0].observations = [] }]),
  ['same-length missing required case', r => { r.cases[0].name = 'unexpected'; r.cases[0].observations = [] }],
  ['array name replacing a distinct required case', r => { r.cases[1].name = ['baseline']; r.cases[1].observations = structuredClone(r.cases[0].observations) }],
  ['nested array name replacing a distinct required case', r => { r.cases[1].name = [['baseline']]; r.cases[1].observations = structuredClone(r.cases[0].observations) }],
  ['failed case', r => { r.cases[0].status = 'fail' }],
  ['skipped case', r => { r.cases[0].status = 'skip' }],
  ['missing IPv6', r => r.cases[0].observations.pop()],
  ['transport failure presented as HTTP denial', r => { r.cases.find(x => x.name === 'path').observations[0].outcome = 'transport-deny' }],
  ['denied request reaching application', r => { r.cases[0].observations[0].appDelta = 1 }],
  ['allowed request without application hit', r => { r.cases.find(x => x.name === 'grant4').observations[0].appDelta = 0 }],
  ['unconfirmed enforcement', r => { r.confirmations[0].enforcementConfirmed = false }],
  ['stale generation', r => { r.confirmations[0].appliedGeneration = 1 }],
  ['mismatched policy hash', r => { r.confirmations[0].appliedPolicyHash = 'f'.repeat(64) }],
  ['no confirmation', r => { r.confirmations = [] }],
  ['case with no corresponding confirmation', r => { r.cases[0].revision = 3 }],
  ['wrong Core source', r => { r.coreCommit = 'f'.repeat(40) }],
  ['missing binary identity', r => { delete r.agentBinarySHA256 }],
  ['missing human proof', r => { delete r.humanRequests }],
  ['one-family human proof', r => { r.humanRequests.pop() }],
  ['duplicate human family', r => { r.humanRequests[1] = structuredClone(r.humanRequests[0]) }],
  ['self-approved human request', r => { r.humanRequests[0].approver = r.humanRequests[0].requester }],
  ['source replaced by requester identity', r => { r.humanRequests[0].source = 'principal:requester' }],
  ['approved sibling access', r => { r.humanRequests[0].approvedEndpoints.push('sibling') }],
  ['extended approved deadline', r => { r.humanRequests[0].approvedExpiry = '2026-10-19T09:02:00Z' }],
  ['invalid human deadline', r => { r.humanRequests[0].approvedExpiry = 'invalid' }],
  ['Grant source retarget', r => { r.humanRequests[0].grantSource = '127.0.0.2/32' }],
  ['Grant scope broadened', r => { r.humanRequests[0].grantEndpoints.push('sibling') }],
  ['Grant lifetime extended', r => { r.humanRequests[0].grantExpiry = '2026-10-19T09:02:00Z' }],
  ['unconfirmed human revision', r => { r.humanRequests[0].revision = 99 }],
  ['wrong human audit actor', r => { r.humanRequests[0].auditActor = 'principal:admin' }],
  ['uncorrelated human audit', r => { r.humanRequests[0].auditReason = 'unrelated approval' }],
  ['unrevoked human Grant', r => { r.humanRequests[0].finalGrantState = 'valid-desired' }],
]) test(`reject ${name}`, () => { const r = fixture(); mutate(r); assert.throws(() => validateEvidence(r)) })
test('require independently expiring IPv4 while IPv6 remains allowed', () => assert.deepEqual(requiredCases.expired4, [[4, 'transport-deny'], [6, 'allow']]))
test('require actual browser application acceptance', () => assert.deepEqual(requiredCases.browser, [[4, 'allow']]))
