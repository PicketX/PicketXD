// SPDX-License-Identifier: GPL-3.0-or-later
// Controlled public unit data/filesystem only; never native identity/kernel proof.
import assert from 'node:assert/strict'
import test from 'node:test'
import { mkdtemp, mkdir, writeFile, readFile, lstat, rm, symlink } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { spawnSync } from 'node:child_process'
import { join } from 'node:path'
let helper
try { helper = await import('./oidc-identity.mjs') } catch (e) { if (e.code !== 'ERR_MODULE_NOT_FOUND') throw e; helper = {} }
const ids = { requester: 'principal:account-' + '1'.repeat(32), approver: 'principal:account-' + '2'.repeat(32) }
test('session lookup includes the installed console Cookie path, never the site root', async () => {
  const cookie = { name: 'picketx_session', value: 'UNIT-PRIVATE', path: '/console/', secure: true, httpOnly: true, sameSite: 'Strict' }
  // Controlled API contract only, not a real browser/login proof. Like Playwright,
  // a URL-filtered cookie query excludes cookies not applicable to its path.
  const context = { cookies: async url => {
    const target = new URL(url); assert.equal(target.origin, 'https://127.0.0.1:18081')
    return target.pathname.startsWith(cookie.path) ? [cookie] : []
  } }
  assert.equal(await helper.installedSessionCookie(context, 'https://127.0.0.1:18081'), cookie)
})
function fixture() {
  const login = username => ({ username, id: ids[username], roles: [username === 'requester' ? 'role:requester' : 'role:admin'], method: 'oidc', s256: true, nonce: true, secureSession: true, noBrowserTokens: true })
  const logins = ['requester', 'approver', 'requester', 'requester', 'approver'].map(login)
  const giteaAudit = [10, 11, 12, 13].map(revision => ({ revision, actor: ids.approver }))
  const report = { result: 'pass', coreCommit: 'c67da10767d5cb92012366b6328baa31be8afa61', agentCommit: 'a'.repeat(40), agentTree: 'b'.repeat(40),
    managedAccounts: { directory: { accounts: Object.entries(ids).map(([username, id]) => ({ username, id, roles: [username === 'requester' ? 'role:requester' : 'role:admin'] })) } },
    humanRequests: [4, 6].map((family, i) => ({ family, revision: 3 + i, requester: ids.requester, approver: ids.approver })),
    confirmations: [3, 4, 10, 11, 12, 13].map(revision => ({ revision, enforcementConfirmed: true, appliedGeneration: revision })),
    gitea: { grants: [10, 11, 12, 13].map(revision => ({ revision })) }, oidc: { logins, giteaAudit } }
  const receipt = { result: 'pass', provider: { kind: 'real-keycloak', version: '26.8.1', archiveSHA256: '4e2740b181faa2ea623a1c2ecf00855935311e546046e2f857615a47ed24cbe8', disposable: true },
    source: { coreCommit: report.coreCommit, coreTree: 'b2d6855d8406d70f98cc0d4c17fae9e31803a249', agentCommit: report.agentCommit, agentTree: report.agentTree },
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

// Abrupt termination skips the native child's finally: the parent must own its
// whole separate tree, including Gitea traversal paths and private credentials.
for (const signal of ['SIGTERM', 'SIGKILL']) test(`parent removes native private state after actual child ${signal}`, async () => {
  assert.equal(typeof helper.createOidcNativeWorkspace, 'function', 'parent-owned native workspace required')
  const workspace = await helper.createOidcNativeWorkspace()
  try {
    assert.equal((await lstat(workspace.directory)).mode & 0o777, 0o711)
    const child = spawnSync(process.execPath, ['--input-type=module', '-e', `
      import {mkdtemp,mkdir,writeFile,chmod} from 'node:fs/promises';
      import {tmpdir} from 'node:os'; import {join} from 'node:path';
      const dir=await mkdtemp(join(tmpdir(),'picketx-console-kernel-'));
      await chmod(dir,0o711); await mkdir(join(dir,'instance'),{mode:0o700});
      await writeFile(join(dir,'instance','oidc.json'),'PRIVATE-TEST-CREDENTIAL',{mode:0o600});
      await writeFile(join(dir,'gitea-public'),'public',{mode:0o644});
      await writeFile(join(tmpdir(),'child-path'),dir,{mode:0o600});
      const nss=join(process.env.HOME,'.pki/nssdb'); await mkdir(nss,{recursive:true,mode:0o700});
      await writeFile(join(nss,'cert9.db'),'UNIT-PROVIDER-AND-GITEA-TRUST',{mode:0o600});
      process.kill(process.pid,process.argv[1]);
    `, signal], { env: {...process.env, ...workspace.env}, encoding: 'utf8' })
    assert.equal(child.signal, signal)
    const path = await readFile(join(workspace.directory,'child-path'),'utf8')
    assert(path.startsWith(workspace.directory + '/picketx-console-kernel-'))
    assert.equal(await readFile(join(path,'instance','oidc.json'),'utf8'),'PRIVATE-TEST-CREDENTIAL')
    assert.equal((await lstat(path)).mode & 0o777,0o711,'Gitea public ancestors remain traversable')
    assert.equal((await lstat(join(path,'instance'))).mode & 0o777,0o700,'private Controller directory')
    assert.equal((await lstat(join(path,'instance','oidc.json'))).mode & 0o777,0o600,'private Controller credentials')
    assert(workspace.env.HOME.startsWith(workspace.directory + '/'))
    assert.equal((await lstat(workspace.env.HOME)).mode & 0o777,0o700)
    const nss = join(workspace.env.HOME,'.pki/nssdb')
    assert.equal(await readFile(join(nss,'cert9.db'),'utf8'),'UNIT-PROVIDER-AND-GITEA-TRUST')
    await workspace.cleanup()
    await assert.rejects(lstat(workspace.directory),{code:'ENOENT'})
  } finally { await workspace.cleanup() }
})
