// SPDX-License-Identifier: GPL-3.0-or-later
// Controlled public unit data/filesystem only; never native identity/kernel proof.
import assert from 'node:assert/strict'
import test from 'node:test'
import { mkdtemp, mkdir, writeFile, readFile, lstat, rm, symlink } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
let helper
try { helper = await import('./oidc-identity.mjs') } catch (e) { if (e.code !== 'ERR_MODULE_NOT_FOUND') throw e; helper = {} }
const ids = { requester: 'principal:account-' + '1'.repeat(32), approver: 'principal:account-' + '2'.repeat(32) }
function fixture() {
  const login = username => ({ username, id: ids[username], roles: [username === 'requester' ? 'role:requester' : 'role:admin'], method: 'oidc', s256: true, nonce: true, secureSession: true, noBrowserTokens: true })
  const logins = ['requester', 'approver', 'requester', 'requester', 'approver'].map(login)
  const giteaAudit = [10, 11, 12, 13].map(revision => ({ revision, actor: ids.approver }))
  const report = { result: 'pass', coreCommit: '64c8cb8e6c92669873031ca45cff47dbb38d0224', agentCommit: 'a'.repeat(40), agentTree: 'b'.repeat(40),
    managedAccounts: { directory: { accounts: Object.entries(ids).map(([username, id]) => ({ username, id, roles: [username === 'requester' ? 'role:requester' : 'role:admin'] })) } },
    humanRequests: [4, 6].map((family, i) => ({ family, revision: 3 + i, requester: ids.requester, approver: ids.approver })),
    confirmations: [3, 4, 10, 11, 12, 13].map(revision => ({ revision, enforcementConfirmed: true, appliedGeneration: revision })),
    gitea: { grants: [10, 11, 12, 13].map(revision => ({ revision })) }, oidc: { logins, giteaAudit } }
  const receipt = { result: 'pass', provider: { kind: 'real-keycloak', version: '26.8.1', archiveSHA256: '4e2740b181faa2ea623a1c2ecf00855935311e546046e2f857615a47ed24cbe8', disposable: true },
    source: { coreCommit: report.coreCommit, coreTree: '4637c029a34c71ec04b66fcc205c288f1280dfb9', agentCommit: report.agentCommit, agentTree: report.agentTree },
    tls: { verified: true, unknownCARejected: true, wrongSANRejected: true }, identities: { ...ids },
    logins: structuredClone(logins), approvals: structuredClone(report.humanRequests), giteaActors: structuredClone(giteaAudit),
    cleanup: { trustRemoved: true, providerStateRemoved: true } }
  return { receipt, report }
}
test('OIDC gate accepts correlated independent humans and actual applied approval/Gitea revisions', () => {
  assert.equal(typeof helper.validateOidcEvidence, 'function', 'required OIDC conjunction gate')
  const f = fixture(); helper.validateOidcEvidence(f.receipt, f.report)
})
for (const [name, change] of [
  ['missing receipt', f => { f.receipt = undefined }],
  ['signed fixture substituted for actual provider', f => { f.receipt.provider.kind = 'signed-protocol-fixture' }],
  ['unverified source', f => { f.receipt.source.coreCommit = '0'.repeat(40) }],
  ['wrong source tree', f => { f.receipt.source.coreTree = '0'.repeat(40) }],
  ['stale Agent source', f => { f.receipt.source.agentCommit = '0'.repeat(40) }],
  ['unknown CA accepted', f => { f.receipt.tls.unknownCARejected = false }],
  ['hostname bypass', f => { f.receipt.tls.wrongSANRejected = false }],
  ['trust residue', f => { f.receipt.cleanup.trustRemoved = false }],
  ['provider state residue', f => { f.receipt.cleanup.providerStateRemoved = false }],
  ['same requester and approver', f => { f.receipt.identities.approver = ids.requester }],
  ['no fresh OIDC recovery', f => { f.receipt.logins = f.report.oidc.logins = f.receipt.logins.slice(0, 2) }],
  ['password fallback', f => { f.receipt.logins[0].method = f.report.oidc.logins[0].method = 'password' }],
  ['missing PKCE', f => { f.receipt.logins[0].s256 = f.report.oidc.logins[0].s256 = false }],
  ['missing nonce', f => { f.receipt.logins[0].nonce = f.report.oidc.logins[0].nonce = false }],
  ['insecure session', f => { f.receipt.logins[0].secureSession = f.report.oidc.logins[0].secureSession = false }],
  ['browser token retained', f => { f.receipt.logins[0].noBrowserTokens = f.report.oidc.logins[0].noBrowserTokens = false }],
  ['requester privilege widening', f => { f.receipt.logins[0].roles = f.report.oidc.logins[0].roles = ['role:admin'] }],
  ['partial source families', f => { f.receipt.approvals.pop() }],
  ['approval not actually applied', f => { f.report.confirmations[0].enforcementConfirmed = false }],
  ['Gitea bootstrap actor masquerades as OIDC', f => { f.receipt.giteaActors[0].actor = f.report.oidc.giteaAudit[0].actor = 'principal:bootstrap' }],
  ['private receipt field', f => { f.receipt.clientSecret = 'PRIVATE-UNIT-MARKER' }],
  ['private nested login field', f => { f.receipt.logins[0].token = f.report.oidc.logins[0].token = 'PRIVATE-UNIT-MARKER' }],
]) test(`OIDC gate rejects ${name}`, () => {
  assert.equal(typeof helper.validateOidcEvidence, 'function', 'required OIDC conjunction gate')
  const f = fixture(); change(f); assert.throws(() => helper.validateOidcEvidence(f.receipt, f.report))
})

async function privateFixture(run) {
  const dir = await mkdtemp(join(tmpdir(), 'picketx-oidc-installed-unit-'))
  try {
    const instance = join(dir, 'instance'); await mkdir(instance, { mode: 0o700 })
    const configPath = join(instance, 'controller.yaml')
    await writeFile(configPath, JSON.stringify({ source: { mode: 'database' }, browser: { origin: 'https://127.0.0.1:18081' } }), { mode: 0o600 })
    const installed = { instance, configPath, runtimeOptions: { env: { PATH: '' } } }
    const config = { issuer: 'https://127.0.0.2:19460/realms/picketx-joint', clientSecret: 'f'.repeat(64), password: 'e'.repeat(64), ca: join(dir, 'ca.pem'), subjects: { requester: '11111111-0400-4000-8000-111111111111', approver: '22222222-0400-4000-8000-222222222222' } }
    await run({ dir, installed, config })
  } finally { await rm(dir, { recursive: true, force: true }) }
}
test('installed OIDC writes private explicit bindings and retains DB authority and empty PATH', async () => {
  assert.equal(typeof helper.configureOidcIdentity, 'function')
  await privateFixture(async ({ installed, config }) => {
    const identity = await helper.configureOidcIdentity(installed, config)
    const f = fixture(); await identity.bindAccounts(f.report)
    const runtime = JSON.parse(await readFile(installed.configPath, 'utf8'))
    assert.deepEqual(runtime.source, { mode: 'database' }); assert.equal(installed.runtimeOptions.env.PATH, '')
    assert.equal(installed.runtimeOptions.env.SSL_CERT_FILE, config.ca)
    assert.equal(Buffer.compare(await readFile(installed.configPath), installed.configBytes), 0)
    const provider = JSON.parse(await readFile(runtime.browser.oidc_config, 'utf8'))
    assert(provider.client_secret === config.clientSecret); assert.equal(provider.client_id, 'picketx-installed-joint')
    const bindings = JSON.parse(await readFile(provider.bindings, 'utf8')).bindings
    assert.equal(bindings.length, 2)
    for (const [i, username] of ['requester', 'approver'].entries()) {
      assert.equal(bindings[i].subject, config.subjects[username]); assert.equal(bindings[i].principal_id, ids[username])
      assert.match(bindings[i].generation, /^[a-f0-9]{32}$/)
    }
    assert.notEqual(bindings[0].generation, bindings[1].generation)
    for (const path of [installed.configPath, runtime.browser.oidc_config, provider.bindings]) assert.equal((await lstat(path)).mode & 0o777, 0o600)
    assert.equal((await lstat(installed.instance)).mode & 0o777, 0o700)
  })
})
test('OIDC setup refuses a symlinked Controller config before writing provider state', async () => {
  assert.equal(typeof helper.configureOidcIdentity, 'function')
  await privateFixture(async ({ dir, installed, config }) => {
    const outside = join(dir, 'outside'); await writeFile(outside, 'retained caller bytes')
    await rm(installed.configPath); await symlink(outside, installed.configPath)
    await assert.rejects(helper.configureOidcIdentity(installed, config))
    assert.equal(await readFile(outside, 'utf8'), 'retained caller bytes')
  })
})
