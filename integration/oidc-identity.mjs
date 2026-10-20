// SPDX-License-Identifier: GPL-3.0-or-later
// Native acceptance adapter only: explicit local bindings, no identity simulation.
import assert from 'node:assert/strict'
import { lstat, readFile, writeFile, rename, mkdtemp, chmod, rm, mkdir } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join, resolve } from 'node:path'
import { randomBytes } from 'node:crypto'

export const oidcCoreCommit = '81ec5bc8a483010f99672f7ff5d982ad4ee6698f'
export const oidcCoreTree = '1b04ceb9d86f4d72c30e12c540097aafd915b2d2'
export const keycloakSHA256 = '4e2740b181faa2ea623a1c2ecf00855935311e546046e2f857615a47ed24cbe8'
const only = (v, names) => { assert(v && typeof v === 'object' && !Array.isArray(v)); assert.deepEqual(Object.keys(v).sort(), [...names].sort(), 'public OIDC fields only') }
const role = user => user === 'requester' ? 'role:requester' : 'role:admin'

export async function installedSessionCookie(context, origin) {
  return (await context.cookies(origin + '/console/')).find(c => c.name === 'picketx_session')
}

export function validateOidcEvidence(r, report) {
  only(r, ['result', 'provider', 'source', 'tls', 'identities', 'logins', 'approvals', 'giteaActors', 'cleanup'])
  assert.equal(r.result, 'pass'); assert.equal(report.result, 'pass')
  assert.deepEqual(r.provider, { kind: 'real-keycloak', version: '26.8.1', archiveSHA256: keycloakSHA256, disposable: true })
  assert.deepEqual(r.source, { coreCommit: oidcCoreCommit, coreTree: oidcCoreTree, agentCommit: report.agentCommit, agentTree: report.agentTree })
  assert.equal(report.coreCommit, oidcCoreCommit)
  for (const value of [report.agentCommit, report.agentTree]) assert.match(value, /^[a-f0-9]{40}$/)
  assert.deepEqual(r.tls, { verified: true, unknownCARejected: true, wrongSANRejected: true })
  assert.deepEqual(r.cleanup, { trustRemoved: true, providerStateRemoved: true })
  only(r.identities, ['requester', 'approver']); assert.notEqual(r.identities.requester, r.identities.approver)
  for (const username of ['requester', 'approver']) {
    const account = report.managedAccounts.directory.accounts.find(a => a.username === username)
    assert(account); assert.match(account.id, /^principal:account-[a-f0-9]{32}$/)
    assert.equal(r.identities[username], account.id); assert.deepEqual(account.roles, [role(username)])
  }
  assert.deepEqual(r.logins, report.oidc.logins)
  assert(r.logins.length >= 5, 'fresh independent OIDC and recovery/Gitea logins required')
  for (const username of ['requester', 'approver']) assert(r.logins.filter(l => l.username === username).length >= (username === 'requester' ? 3 : 2))
  for (const l of r.logins) {
    only(l, ['username', 'id', 'roles', 'method', 's256', 'nonce', 'secureSession', 'noBrowserTokens'])
    assert(['requester', 'approver'].includes(l.username)); assert.equal(l.id, r.identities[l.username])
    assert.deepEqual(l.roles, [role(l.username)]); assert.equal(l.method, 'oidc')
    for (const name of ['s256', 'nonce', 'secureSession', 'noBrowserTokens']) assert.equal(l[name], true)
  }
  const applied = revision => assert(report.confirmations.some(c => c.revision === revision && c.enforcementConfirmed === true && c.appliedGeneration === revision), 'OIDC revision must actually be applied')
  assert.equal(r.approvals.length, 2)
  r.approvals.forEach((a, i) => {
    only(a, ['family', 'revision', 'requester', 'approver'])
    assert.equal(a.family, [4, 6][i]); assert.equal(a.requester, r.identities.requester); assert.equal(a.approver, r.identities.approver)
    const actual = report.humanRequests[i]
    assert.deepEqual(a, { family: actual.family, revision: actual.revision, requester: actual.requester, approver: actual.approver }); applied(a.revision)
  })
  assert.equal(r.giteaActors.length, 4); assert.deepEqual(r.giteaActors, report.oidc.giteaAudit)
  r.giteaActors.forEach((a, i) => {
    only(a, ['revision', 'actor']); assert.equal(a.actor, r.identities.approver)
    assert.equal(a.revision, report.gitea.grants[i].revision); applied(a.revision)
  })
}

// Separate from the provider's 0700 root: Gitea needs public ancestor traversal.
// The wrapper owns this entire root even when the native child's finally is lost.
export async function createOidcNativeWorkspace() {
  const directory = await mkdtemp(join(tmpdir(), 'picketx-oidc-native-'))
  try {
    await chmod(directory, 0o711)
    const home = join(directory, 'home')
    await mkdir(home, { mode: 0o700 })
    return { directory, env: { TMPDIR: directory, HOME: home },
      async cleanup() {
        await rm(directory, { recursive: true, force: true })
        await assert.rejects(lstat(directory), { code: 'ENOENT' })
      } }
  } catch (error) { await rm(directory, { recursive: true, force: true }); throw error }
}

export async function configureOidcIdentity(installed, config) {
  const info = await lstat(installed.instance), file = await lstat(installed.configPath)
  assert(info.isDirectory() && !info.isSymbolicLink() && (info.mode & 0o777) === 0o700, 'private installed instance required')
  assert(file.isFile() && !file.isSymbolicLink() && (file.mode & 0o777) === 0o600, 'private regular installed config required')
  assert.equal(resolve(installed.configPath), join(resolve(installed.instance), 'controller.yaml'))
  assert.equal(config.issuer, 'https://127.0.0.2:19460/realms/picketx-joint')
  assert(typeof config.clientSecret === 'string' && /^[a-f0-9]{64}$/.test(config.clientSecret), 'private client credential required')
  const runtime = JSON.parse(await readFile(installed.configPath, 'utf8'))
  assert.equal(runtime.browser.origin, 'https://127.0.0.1:18081')
  const provider = join(installed.instance, 'oidc.json'), bindings = join(installed.instance, 'oidc-bindings.json')
  await writeFile(bindings, '{"bindings":[]}', { mode: 0o600, flag: 'wx' })
  await writeFile(provider, JSON.stringify({ issuer: config.issuer, client_id: 'picketx-installed-joint', client_secret: config.clientSecret, bindings }), { mode: 0o600, flag: 'wx' })
  runtime.browser.oidc_config = provider
  await writeFile(installed.configPath, JSON.stringify(runtime), { mode: 0o600 })
  installed.configBytes = await readFile(installed.configPath)
  installed.runtimeOptions.env.SSL_CERT_FILE = config.ca
  const accounts = report => ['requester', 'approver'].map(username => {
    const account = report.managedAccounts.directory.accounts.find(a => a.username === username)
    assert(account); assert.deepEqual(account.roles, [role(username)]); assert.match(account.id, /^principal:account-[a-f0-9]{32}$/)
    return { username, id: account.id, roles: account.roles }
  })
  return {
    async bindAccounts(report) {
      const rows = accounts(report).map(account => {
        assert(typeof config.subjects[account.username] === 'string' && config.subjects[account.username].length > 0)
        return { subject: config.subjects[account.username], principal_id: account.id, generation: randomBytes(16).toString('hex') }
      })
      assert.notEqual(rows[0].subject, rows[1].subject)
      const pending = bindings + '.pending'
      await writeFile(pending, JSON.stringify({ bindings: rows }), { mode: 0o600, flag: 'wx' }); await rename(pending, bindings)
      report.oidc = { logins: [], giteaAudit: [] }
    },
    async signIn(page, username, report) {
      const account = accounts(report).find(a => a.username === username); assert(account, 'explicit existing human required')
      const origin = runtime.browser.origin, providerOrigin = new URL(config.issuer).origin
      const observed = { s256: false, nonce: false }
      const observe = request => {
        const url = new URL(request.url())
        if (url.origin === providerOrigin && url.pathname.endsWith('/protocol/openid-connect/auth')) {
          observed.s256 = url.searchParams.get('code_challenge_method') === 'S256' && Boolean(url.searchParams.get('code_challenge'))
          observed.nonce = Boolean(url.searchParams.get('nonce'))
        }
      }
      page.on('request', observe)
      let loginPhase = 'oidc-console-open'
      try {
        await page.goto(origin + '/console/')
        const previous = (await installedSessionCookie(page.context(), origin))?.value
        const start = page.getByRole('button', { name: 'Continue with organization', exact: true })
        const signOut = page.getByRole('button', { name: 'Sign out', exact: true })
        const loginDeadline = Date.now() + 15000
        while (!await start.isVisible() && Date.now() < loginDeadline) {
          // The Gitea publisher previously used the independent recovery key.
          // End that local session explicitly, never relabel it as an OIDC login.
          if (await signOut.isVisible()) { await signOut.click(); break }
          await new Promise(r => setTimeout(r, 100))
        }
        loginPhase = 'oidc-start-control'
        await start.waitFor({ timeout: 15000 })
        await page.getByRole('button', { name: 'Continue with organization', exact: true }).click()
        loginPhase = 'oidc-provider-arrival'
        let me, submitted = false; const deadline = Date.now() + 20000
        while (Date.now() < deadline) {
          const site = new URL(page.url()).origin
          if (site === providerOrigin && !submitted && await page.locator('#username').isVisible()) {
            loginPhase = 'oidc-provider-credentials'
            await page.locator('#username').fill(username); await page.locator('#password').fill(config.password)
            await page.locator('#kc-login').click(); submitted = true
            loginPhase = 'oidc-provider-submitted'
          } else if (site === origin) {
            loginPhase = 'oidc-console-return'
            try { me = await page.evaluate(async () => { const r = await fetch('/console/api/v1/me'); return r.ok ? r.json() : undefined }) } catch { /* bounded navigation transition */ }
            if (me) break
          }
          await new Promise(r => setTimeout(r, 100))
        }
        assert(me, 'actual OIDC login deadline'); loginPhase = 'oidc-local-identity'; assert.equal(me.id, account.id); assert.deepEqual(me.roles, account.roles)
        assert(observed.s256 && observed.nonce, 'actual provider authorization request required, no password fallback')
        loginPhase = 'oidc-secure-session'
        const cookie = await installedSessionCookie(page.context(), origin)
        assert(cookie?.secure && cookie.httpOnly && cookie.sameSite === 'Strict', 'normal HTTPS secure local session required')
        if (previous) assert.notEqual(cookie.value, previous, 'OIDC must establish a new local session')
        assert.equal(await page.evaluate(() => localStorage.length + sessionStorage.length), 0, 'no browser token storage')
        report.oidc.logins.push({ username, id: me.id, roles: me.roles, method: 'oidc', ...observed, secureSession: true, noBrowserTokens: true })
      } catch {
        // Fixed classification only: never publish the URL, query, form, or raw error.
        if (await page.locator('#input-error-username').count().catch(() => 0)) loginPhase = 'oidc-provider-input-rejected'
        else if (await page.locator('#kc-error-message').count().catch(() => 0)) loginPhase = 'oidc-provider-error'
        else if (new URL(page.url()).origin === origin && await page.getByRole('alert').count().catch(() => 0)) loginPhase = 'oidc-console-alert'
        const failure = new Error('OIDC native login failed'); failure.oidcPhase = loginPhase; throw failure
      } finally { page.off('request', observe) }
    },
  }
}
