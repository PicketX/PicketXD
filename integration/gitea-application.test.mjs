// SPDX-License-Identifier: GPL-3.0-or-later
// Literal gate regression fixtures only, never native execution evidence.
import test from 'node:test'
import assert from 'node:assert/strict'

export function giteaFixture() {
  const grants = [4, 6].flatMap(family => ['https', 'ssh'].map(endpoint => ({ family, endpoint,
    revision: 20 + (family === 4 ? 0 : 1) + (endpoint === 'ssh' ? 2 : 0),
    grant: { name: `gitea-${endpoint}-${family}`, uid: `grant-${endpoint}-${family}`, source: family === 4 ? '127.0.0.1/32' : '::1/128',
      scopes: [{ resource_name: 'gitea', resource_uid: 'resource-gitea-v1', selection: 'named', endpoint_names: [endpoint] }],
      mode: 'bounded', expires_at: '2026-10-19T15:00:00Z', revoked: false } })))
  const cases = ['baseline', 'httpsOnly', 'both', 'revokeHTTPS4', 'revokeSSH4', 'revokeHTTPS6', 'revokeSSH6'].map((name, index) => ({ name, revision: index === 0 ? 19 : index === 1 ? 21 : index === 2 ? 23 : index + 21,
    observations: [4, 6].flatMap(family => ['https', 'ssh'].map(endpoint => ({ family, endpoint,
      outcome: index === 0 || (index === 1 && endpoint === 'ssh') || (index >= 3 && family === 4 && endpoint === 'https') || (index >= 4 && family === 4) || (index >= 5 && endpoint === 'https') || index === 6 ? 'transport-deny' : 'allow' }))) }))
  return { version: '28.2.0', binarySHA256: '15681fc615d56611c274f52803fef4d6d5316b620f93b677dd4b831a74e50b9f', oid: 'a'.repeat(40), grants, cases,
    browser: { login: true, repository: true }, applicationAuth: [4, 6].map(family => ({ family, anonymousHTTPS: 'denied', outsiderHTTPS: 'denied', outsiderSSH: 'denied', ownerHTTPS: 'allow', ownerSSH: 'allow', clonedOID: 'a'.repeat(40) })),
    establishedHTTPS4: { revision: 24, sameSocket: true, before: 200, after: 403, upstreamAfter: 0 },
    tls: { untrustedRejected: true }, ssh: { wrongHostKeyRejected: true }, healthyControls: true }
}
const module = await import('./gitea-application.mjs').catch(e => { if (e.code === 'ERR_MODULE_NOT_FOUND') return {}; throw e })
const validate = (...args) => { assert.equal(typeof module.validateGiteaEvidence, 'function', 'required real Gitea evidence validator missing'); return module.validateGiteaEvidence(...args) }
const confirmations = Array.from({ length: 28 }, (_, i) => ({ revision: i + 1 }))
test('SSH host-key rejection is not misclassified as application permission denial', () => {
  assert.equal(module.classifyGitFailure('Host key verification failed.\nfatal: Could not read from remote repository.'), 'host-key-deny')
})
test('fixture failures redact credentials even when the process exception repeats its arguments', () => {
  assert.equal(typeof module.redactFixtureFailure, 'function')
  const e = module.redactFixtureFailure(new Error('Command failed: app --password native-secret\nPRIVATE KEY'), ['native-secret', 'PRIVATE KEY'])
  assert(!e.message.includes('native-secret')); assert(!e.message.includes('PRIVATE KEY'))
  assert.match(e.message, /REDACTED/)
})
for (const message of [
  'kex_exchange_identification: read: Connection reset by peer\nfatal: Could not read from remote repository.',
  'ssh: connect to host ::1 port 19222: Network is unreachable\nfatal: Could not read from remote repository.',
  'Load key "/private/outsider": invalid format\nnobody@127.0.0.1: Permission denied (publickey).\nfatal: Could not read from remote repository.',
]) test(`unrecognized SSH failure is not app isolation: ${message.split('\n')[0]}`, () => {
  assert.equal(module.classifyGitFailure(message, 'ssh'), null)
})
test('specific pinned Gitea read-permission rejection identifies application isolation', () => {
  assert.equal(module.classifyGitFailure('error: User key 2:native has no "read" permission for owner/private\nfatal: Could not read from remote repository.', 'ssh'), 'app-deny')
  assert.equal(module.classifyGitFailure('error: Cannot find repository owner/private\nfatal: Could not read from remote repository.', 'ssh'), 'app-deny')
  assert.equal(module.classifyGitFailure('Authentication failed.\nfatal: Could not read from remote repository.', 'ssh'), null)
})
test('required complete Gitea proof is accepted', () => validate(giteaFixture(), confirmations))
for (const [name, change] of [
  ['missing workflow', p => delete p.cases], ['missing family', p => p.applicationAuth.pop()],
  ['HTTPS-only accidentally permits SSH', p => p.cases[1].observations[1].outcome = 'allow'],
  ['revocation disables independent SSH', p => p.cases[3].observations[1].outcome = 'transport-deny'],
  ['outsider private repo readable', p => p.applicationAuth[0].outsiderSSH = 'allow'],
  ['wrong OID', p => p.applicationAuth[1].clonedOID = 'b'.repeat(40)],
  ['unconfirmed revision', p => p.cases[0].revision = 999],
  ['same connection unproven', p => p.establishedHTTPS4.sameSocket = false],
  ['denied request forwarded', p => p.establishedHTTPS4.upstreamAfter = 1],
  ['Grant widens endpoint', p => p.grants[0].grant.scopes[0].endpoint_names.push('ssh')],
  ['Grant wrong source', p => p.grants[0].grant.source = '0.0.0.0/0'],
  ['missing positive controls', p => p.healthyControls = false],
  ['TLS bypass', p => p.tls.untrustedRejected = false],
  ['SSH key bypass', p => p.ssh.wrongHostKeyRejected = false],
  ['non-string case name', p => p.cases[0].name = ['baseline']],
  ['secret extra field', p => p.password = 'unit-secret'],
  ['secret Grant field', p => p.grants[0].grant.token = 'unit-secret'],
  ['secret observation field', p => p.cases[0].observations[0].authorization = 'unit-secret'],
]) test(`gate rejects ${name}`, () => { const p = giteaFixture(); change(p); assert.throws(() => validate(p, confirmations)) })
