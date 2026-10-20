// SPDX-License-Identifier: GPL-3.0-or-later
// Disposable native real-provider wrapper; never a production IdP installer.
import assert from 'node:assert/strict'
import { mkdtemp, mkdir, readFile, readlink, writeFile, stat, rm } from 'node:fs/promises'
import { join, resolve } from 'node:path'
import { tmpdir } from 'node:os'
import { pathToFileURL } from 'node:url'
import { randomBytes } from 'node:crypto'
import { spawn, spawnSync } from 'node:child_process'
import { once } from 'node:events'
import { coreCommit, coreTree, parseEnforceStatusLog } from './controller-install.mjs'
import { validateEvidence } from './console-evidence.mjs'
import { validateGiteaEvidence } from './gitea-application.mjs'
import { keycloakSHA256, validateOidcEvidence, createOidcNativeWorkspace } from './oidc-identity.mjs'

const root = resolve(import.meta.dirname, '..'), core = resolve(process.argv[2] ?? '/missing-core'), input = resolve(process.argv[3] ?? '/missing-provider')
const evidence = join(root, 'artifacts/console-kernel/oidc')
let phase = 'guard', directory, provider, child, trustLabel, nativeWorkspace, nss
const pause = ms => new Promise(r => setTimeout(r, ms))
function command(bin, args) {
  const r = spawnSync(bin, args, { encoding: 'utf8', timeout: 15000, stdio: ['ignore', 'pipe', 'pipe'] })
  assert.equal(r.status, 0, 'owned real-provider deployment command failed'); return r.stdout
}
async function stopProvider() {
  if (!provider || provider.exitCode !== null || provider.signalCode !== null) return
  const exited = once(provider, 'close'); provider.kill('SIGTERM')
  const timer = setTimeout(() => provider.kill('SIGKILL'), 10000)
  try { await exited } finally { clearTimeout(timer) }
}
async function stopOwnedGroup() {
  if (!child?.pid) return
  const exists = () => { try { process.kill(-child.pid, 0); return true } catch (e) { if (e.code === 'ESRCH') return false; throw e } }
  if (!exists()) return
  try { process.kill(-child.pid, 'SIGTERM') } catch (e) { if (e.code !== 'ESRCH') throw e }
  const end = Date.now() + 5000
  while (exists() && Date.now() < end) await pause(100)
  if (exists()) { try { process.kill(-child.pid, 'SIGKILL') } catch (e) { if (e.code !== 'ESRCH') throw e } }
  const killed = Date.now() + 5000
  while (exists() && Date.now() < killed) await pause(100)
  assert(!exists(), 'owned native process group remains')
}
async function run() {
  assert(process.env.GITHUB_ACTIONS === 'true' && process.env.PICKETX_CONSOLE_KERNEL_TESTS === '1', 'disposable CI opt-in required')
  assert.notEqual(await readlink('/proc/self/ns/net'), await readlink('/proc/1/ns/net'), 'disposable network namespace required')
  assert.equal(command('git', ['-C', core, 'rev-parse', 'HEAD']).trim(), coreCommit)
  assert.equal(command('git', ['-C', core, 'rev-parse', 'HEAD^{tree}']).trim(), coreTree)
  const { createTestCertificate, requestHTTPS, stagePrivateProvider } = await import(pathToFileURL(join(core, 'scripts/real-idp-https.mjs')))
  directory = await mkdtemp(join(tmpdir(), 'picketx-oidc-conjunction-'))
  let receipt, report
  try {
    phase = 'provider-setup'
    command('ip', ['link', 'set', 'lo', 'up'])
    const certificate = await createTestCertificate(directory), ca = await readFile(certificate.ca)
    const home = await stagePrivateProvider(input, directory)
    assert(command(join(home, 'bin/kc.sh'), ['--version']).includes('26.8.1'), 'pinned real provider version')
    const password = randomBytes(32).toString('hex'), clientSecret = randomBytes(32).toString('hex')
    const subjects = { requester: '11111111-0400-4000-8000-111111111111', approver: '22222222-0400-4000-8000-222222222222' }
    const issuer = 'https://127.0.0.2:19460/realms/picketx-joint'
    await mkdir(join(home, 'data/import'), { recursive: true, mode: 0o700 })
    await writeFile(join(home, 'data/import/picketx-joint-realm.json'), JSON.stringify({ realm: 'picketx-joint', enabled: true, sslRequired: 'all', registrationAllowed: false,
      users: Object.entries(subjects).map(([username, id]) => ({ id, username, enabled: true, emailVerified: true, firstName: 'Native', lastName: username, email: username + '@example.test', credentials: [{ type: 'password', value: password, temporary: false }] })),
      clients: [{ clientId: 'picketx-installed-joint', enabled: true, protocol: 'openid-connect', publicClient: false, secret: clientSecret, standardFlowEnabled: true, directAccessGrantsEnabled: false, redirectUris: ['https://127.0.0.1:18081/console/api/v1/oidc/callback'], webOrigins: ['https://127.0.0.1:18081'], attributes: { 'pkce.code.challenge.method': 'S256' } }],
    }), { mode: 0o600, flag: 'wx' })
    const config = join(directory, 'private-native.json')
    await writeFile(config, JSON.stringify({ issuer, clientSecret, password, subjects, ...certificate }), { mode: 0o600, flag: 'wx' })
    phase = 'provider-start'
    provider = spawn(join(home, 'bin/kc.sh'), ['start', '--db=dev-file', '--http-enabled=false', '--http-host=127.0.0.2', '--https-port=19460', '--hostname=https://127.0.0.2:19460', `--https-certificate-file=${certificate.cert}`, `--https-certificate-key-file=${certificate.key}`, '--import-realm'], { env: { ...process.env, JAVA_OPTS_APPEND: '-Xms128m -Xmx512m' }, stdio: 'ignore' })
    provider.on('error', () => {})
    let ready = false; const deadline = Date.now() + 120000
    while (Date.now() < deadline) {
      assert(provider.pid && provider.exitCode === null && provider.signalCode === null, 'owned provider exited')
      try { ready = (await requestHTTPS(issuer + '/.well-known/openid-configuration', { ca })).status === 200 } catch { /* bounded real startup */ }
      if (ready) break; await pause(200)
    }
    assert(ready, 'real provider readiness deadline')
    await assert.rejects(requestHTTPS(issuer + '/.well-known/openid-configuration'), { message: 'HTTPS observation failed' })
    await assert.rejects(requestHTTPS(issuer + '/.well-known/openid-configuration', { ca, servername: 'wrong.example.test' }), { message: 'HTTPS observation failed' })
    phase = 'browser-trust'
    nativeWorkspace = await createOidcNativeWorkspace()
    nss = join(nativeWorkspace.env.HOME, '.pki/nssdb')
    await mkdir(nss, { recursive: true })
    try { await readFile(join(nss, 'cert9.db')) } catch (e) { if (e.code !== 'ENOENT') throw e; command('certutil', ['-N', '--empty-password', '-d', `sql:${nss}`]) }
    trustLabel = 'picketx-oidc-joint-' + randomBytes(8).toString('hex')
    command('certutil', ['-A', '-d', `sql:${nss}`, '-n', trustLabel, '-t', 'C,,', '-i', certificate.ca])
    phase = 'native-conjunction'
    child = spawn(process.execPath, [join(root, 'integration/console-kernel.mjs'), core], { cwd: root, detached: true,
      env: { ...process.env, ...nativeWorkspace.env, NODE_EXTRA_CA_CERTS: certificate.ca, PICKETX_REAL_OIDC_CONFIG: config }, stdio: ['ignore', 'pipe', 'pipe'] })
    // No provider/child exception/query/body/private path goes into public logs.
    child.stdout.resume()
    let diagnostic = ''
    child.stderr.on('data', bytes => {
      diagnostic = (diagnostic + bytes.toString()).slice(-4096)
      const match = /(?:^|\n)OIDC native failed phase ([a-z-]+)\r?\n/.exec(diagnostic)
      if (match) phase = 'native-' + match[1]
    })
    let timedOut = false, forceTimer
    const timer = setTimeout(() => {
      timedOut = true
      try { process.kill(-child.pid, 'SIGTERM') } catch { /* final checked group cleanup */ }
      forceTimer = setTimeout(() => { try { process.kill(-child.pid, 'SIGKILL') } catch { /* final checked cleanup */ } }, 5000)
    }, 600000)
    let result
    try { result = await once(child, 'close') } finally { clearTimeout(timer); clearTimeout(forceTimer) }
    assert(!timedOut && result[0] === 0 && result[1] === null, 'actual native OIDC child must complete')
    report = JSON.parse(await readFile(join(evidence, 'report.json'), 'utf8'))
    validateEvidence(report, { statuses: JSON.parse(await readFile(join(evidence, 'agent-status.json'), 'utf8')), restartedStatuses: parseEnforceStatusLog(await readFile(join(evidence, 'agent-restarted.log'), 'utf8')) })
    validateGiteaEvidence(report.gitea, report.confirmations)
    receipt = { result: 'pass', provider: { kind: 'real-keycloak', version: '26.8.1', archiveSHA256: keycloakSHA256, disposable: true },
      source: { coreCommit, coreTree, agentCommit: report.agentCommit, agentTree: report.agentTree },
      tls: { verified: true, unknownCARejected: true, wrongSANRejected: true },
      identities: Object.fromEntries(report.managedAccounts.directory.accounts.map(a => [a.username, a.id])),
      logins: report.oidc.logins, approvals: report.humanRequests.map(({ family, revision, requester, approver }) => ({ family, revision, requester, approver })),
      giteaActors: report.oidc.giteaAudit, cleanup: { trustRemoved: false, providerStateRemoved: false } }
  } finally {
    let failed = false
    try { await stopOwnedGroup() } catch { failed = true }
    try { await stopProvider() } catch { failed = true }
    if (trustLabel) { try { command('certutil', ['-D', '-d', `sql:${nss}`, '-n', trustLabel]); trustLabel = undefined } catch { failed = true } }
    // Removes both native credentials and both fixture CA databases after group exit.
    if (nativeWorkspace) { try { await nativeWorkspace.cleanup() } catch { failed = true } }
    if (directory) {
      try { await rm(directory, { recursive: true, force: true }); await assert.rejects(stat(directory), { code: 'ENOENT' }); directory = undefined } catch { failed = true }
    }
    if (failed) { phase = 'cleanup'; throw new Error('owned real-provider cleanup failed') }
  }
  phase = 'public-receipt'
  receipt.cleanup = { trustRemoved: true, providerStateRemoved: true }
  validateOidcEvidence(receipt, report)
  await writeFile(join(evidence, 'oidc-report.json'), JSON.stringify(receipt, null, 2) + '\n')
  console.log('PASS actual real Keycloak26.8.1 verified HTTPS → existing OIDC humans/independent approval → installed Controller/Agent/kernel/Nginx/application/Gitea IPv4+IPv6; exact applied revisions, revoke/expiry, live-SSO local revocation, private provider/trust cleanup')
  console.log(JSON.stringify(receipt))
}
try { await run() } catch { console.error('Real OIDC conjunction failed phase ' + phase); process.exitCode = 1 }
