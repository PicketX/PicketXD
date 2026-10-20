// SPDX-License-Identifier: GPL-3.0-or-later
import test from 'node:test'
import assert from 'node:assert/strict'
import { validateHTTPAuditEvidence } from './http-audit.mjs'
import { auditFixture, issuanceFixture } from './fixtures/http-audit.mjs'
const report = () => ({ deployment: { restart: { revision: 2 } },
  managedAccounts: { directory: { accounts: [{ id: 'principal:account-' + '2'.repeat(32), username: 'approver', roles: ['role:admin'] }] } },
  grantIssuances: issuanceFixture(), cases: [{ name: 'grant4', revision: 1 }, { name: 'grant6', revision: 2 }],
  confirmations: [{ revision: 2, generation: 2, appliedGeneration: 2, enforcementConfirmed: true }] })
test('actual-shaped ordered four-probe DOM correlation is accepted', () => assert.doesNotThrow(() => validateHTTPAuditEvidence(auditFixture(), report())))
test('OIDC mode requires actual OIDC context and provisioned approver', () => {
  const r = report(), proof = auditFixture(); r.oidc = { logins: [] }; proof.mode = 'oidc'
  assert.doesNotThrow(() => validateHTTPAuditEvidence(proof, r))
})
for (const [name, change] of [
  ['missing proof', () => null], ['missing probe', r => r.probes.pop()], ['duplicate probe', r => { r.probes[1] = structuredClone(r.probes[0]) }],
  ['old ID', r => { r.probes[0].event.id = r.probes[0].before }], ['duplicate event', r => { r.probes[1].event.id = r.probes[0].event.id }],
  ['unsafe ID', r => { r.probes[0].event.id = Number.MAX_SAFE_INTEGER + 1 }], ['negative before', r => { r.probes[0].before = -1 }],
  ['unrelated revision', r => { r.revision = 3 }], ['wrong event revision', r => { r.probes[0].event.revision++ }],
  ['wrong family', r => { r.probes[0].family = 6 }], ['wrong source', r => { r.probes[0].event.source = '::1' }],
  ['wrong route', r => { r.probes[0].event.route = 'route-other' }], ['wrong caller', r => { r.probes[0].event.caller = 'other-ingress' }],
  ['wrong reason', r => { r.probes[1].event.reason = 'unavailable' }], ['fake boolean', r => { r.probes[0].event.allowed = 'true' }],
  ['denial reaches app', r => { r.probes[1].appDelta = 1 }], ['allow without app hit', r => { r.probes[0].appDelta = 0 }],
  ['transport failure as403', r => { r.probes[1].status = null }], ['allow wrong status', r => { r.probes[0].status = 202 }],
  ['wrong Grant family', r => { r.probes[0].event.grant_ids = ['grant-six'] }], ['missing Grant', r => { r.probes[0].event.grant_ids = [] }],
  ['duplicate Grants', r => { r.grants[1] = r.grants[0] }], ['bootstrap view', r => { r.principal = 'principal:bootstrap' }],
  ['wrong login mode', r => { r.mode = 'oidc' }], ['missing DOM row', r => r.rows.pop()],
  ['different DOM decision', r => { r.rows[1].allowed = true }], ['DOM Grant substitution', r => { r.rows[0].grant_ids = ['other'] }],
  ['missing mobile', r => { r.viewports = [1440] }], ['private proof field', r => { r.token = 'private-unit-marker' }],
  ['private event field', r => { r.probes[0].event.uri = '/private-unit-marker' }], ['private DOM field', r => { r.rows[0].cookie = 'private-unit-marker' }],
]) test(`rejects ${name} with fixed nonprivate error`, () => {
  let proof = auditFixture(); const out = change(proof); if (name === 'missing proof') proof = out
  assert.throws(() => validateHTTPAuditEvidence(proof, report()), e => e.message === 'Invalid installed HTTP audit evidence' && !e.stack.includes('private-unit-marker'))
})
test('matching UI/API revision without actual native confirmation is rejected', () => {
  const r = report(); r.confirmations[0].enforcementConfirmed = false
  assert.throws(() => validateHTTPAuditEvidence(auditFixture(), r))
})
for (const [name, change] of [
  ['coherent family swap', proof => {
    proof.grants.reverse()
    for (const i of [0, 2]) { proof.probes[i].event.grant_ids = [proof.grants[i / 2]]; proof.rows[i].grant_ids = [...proof.probes[i].event.grant_ids] }
  }],
  ['coherent unrelated UID', proof => {
    proof.grants[0] = 'grant-unrelated'; proof.probes[0].event.grant_ids = ['grant-unrelated']; proof.rows[0].grant_ids = ['grant-unrelated']
  }],
  ['missing independent issuance', (_, r) => { delete r.grantIssuances }],
  ['wrong issuance revision', (_, r) => { r.grantIssuances[0].revision = 2 }],
  ['wrong issuance source', (_, r) => { r.grantIssuances[0].source = '::1' }],
  ['wrong issuance case', (_, r) => { r.cases[0].revision = 2 }],
  ['private issuance field', (_, r) => { r.grantIssuances[0].token = 'private-unit-marker' }],
]) test(`independent issuance rejects ${name}`, () => {
  const proof = auditFixture(), r = report(); change(proof, r)
  assert.throws(() => validateHTTPAuditEvidence(proof, r), e => e.message === 'Invalid installed HTTP audit evidence')
})
