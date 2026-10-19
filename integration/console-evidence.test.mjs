// SPDX-License-Identifier: GPL-3.0-or-later
import test from 'node:test'
import assert from 'node:assert/strict'
import { validateEvidence, requiredCases, coreCommit } from './console-evidence.mjs'

// Unit data only; never emitted as native acceptance evidence.
const fixture = () => ({
  coreCommit, agentCommit: 'a'.repeat(40), agentTree: 'b'.repeat(40),
  kernel: 'unit-fixture', agentBinarySHA256: 'c'.repeat(64), controllerBinarySHA256: 'd'.repeat(64),
  confirmations: [{ revision: 2, generation: 2, appliedGeneration: 2, policyHash: 'e'.repeat(64), appliedPolicyHash: 'e'.repeat(64), enforcementConfirmed: true }],
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
]) test(`reject ${name}`, () => { const r = fixture(); mutate(r); assert.throws(() => validateEvidence(r)) })
test('require independently expiring IPv4 while IPv6 remains allowed', () => assert.deepEqual(requiredCases.expired4, [[4, 'transport-deny'], [6, 'allow']]))
test('require actual browser application acceptance', () => assert.deepEqual(requiredCases.browser, [[4, 'allow']]))
