// SPDX-License-Identifier: GPL-3.0-or-later
// Must run inside the disposable namespace created by test-console-kernel.sh.
import assert from 'node:assert/strict'
import { mkdtemp, mkdir, readFile, readlink, writeFile, rm, copyFile, chmod } from 'node:fs/promises'
import { tmpdir, release } from 'node:os'
import { resolve, join } from 'node:path'
import { pathToFileURL } from 'node:url'
import { randomBytes, createHash } from 'node:crypto'
import { spawn, execFileSync } from 'node:child_process'
import { once } from 'node:events'
import http from 'node:http'
import { coreCommit, requiredCases, validateEvidence } from './console-evidence.mjs'
import { runHumanRequests } from './human-requests.mjs'
import { provisionManagedHumans } from './managed-humans.mjs'
import { assertServicesAlive } from './process-health.mjs'
import { runGiteaApplication, validateGiteaEvidence, redactFixtureFailure } from './gitea-application.mjs'
import { prepareInstalledIngress, installedNginxConfiguration } from './installed-ingress.mjs'
import { initializeInstalledController, businessViewDigest, parseEnforceStatusLog } from './controller-install.mjs'
import { validateAgentDistributionEvidence } from '../scripts/agent-bundle.mjs'

assert.equal(process.env.PICKETX_CONSOLE_KERNEL_TESTS, '1', 'explicit native opt-in required')
assert.notEqual(await readlink('/proc/self/ns/net'), await readlink('/proc/1/ns/net'), 'disposable network namespace required')
const root = resolve(import.meta.dirname, '..'), core = resolve(process.argv[2])
const evidence = join(root, 'artifacts/console-kernel')
await mkdir(evidence, { recursive: true })
const temp = await mkdtemp(join(tmpdir(), 'picketx-console-kernel-'))
const hash = x => createHash('sha256').update(x).digest('hex')
let adminToken = '', agentToken = '', ingressToken = ''
const password = randomBytes(32).toString('hex')
const applicationSecrets = []
const runtimeEnv = Object.fromEntries(Object.entries(process.env).filter(([k]) => !k.startsWith('PICKETX_')))
const report = { coreCommit, agentCommit: execFileSync('git', ['rev-parse', 'HEAD'], { cwd: root, encoding: 'utf8' }).trim(), agentTree: execFileSync('git', ['rev-parse', 'HEAD^{tree}'], { cwd: root, encoding: 'utf8' }).trim(), kernel: release(), confirmations: [], cases: [] }
const children = [], servers = [], keep = new Map(), status = [], logs = new Map()
let browser, appHits = 0, healthy = false, latestControllerReceipt
const origin = 'http://127.0.0.1:18080', host = 'protected.example:18445'
const pause = ms => new Promise(r => setTimeout(r, ms))
const privateFile = (name, body) => writeFile(join(temp, name), body, { mode: 0o600 })
function start(name, binary, args, options = {}) {
  const child = spawn(binary, args, { env: runtimeEnv, stdio: ['ignore', 'pipe', 'pipe'], ...options })
  children.push(child); logs.set(name, '')
  child.stdout.on('data', x => logs.set(name, logs.get(name) + x))
  child.stderr.on('data', x => logs.set(name, logs.get(name) + x))
  child.on('error', e => logs.set(name, logs.get(name) + e.message))
  return child
}
async function until(check, description, limit = 15000) {
  const end = Date.now() + limit
  while (Date.now() < end) { if (await check()) return; await pause(100) }
  throw new Error(`timed out: ${description}`)
}
async function api(path) {
  const r = await fetch(origin + '/api/v1/' + path, { headers: { Authorization: `Bearer ${adminToken}` }, signal: AbortSignal.timeout(3000) })
  assert.equal(r.status, 200, path); return r.json()
}
async function confirmed(statusOffset = 0) {
  const revision = (await api('publication')).revision
  await until(() => status.slice(statusOffset).some(s => s.enforcementConfirmed && s.desired.generation === revision && s.appliedGeneration === revision && s.appliedPolicyHash === s.desired.policyHash), `Agent applying revision ${revision}`)
  const s = status.slice(statusOffset).findLast(s => s.enforcementConfirmed && s.appliedGeneration === revision)
  report.confirmations.push({ revision, generation: s.desired.generation, appliedGeneration: s.appliedGeneration, policyHash: s.desired.policyHash, appliedPolicyHash: s.appliedPolicyHash, enforcementConfirmed: s.enforcementConfirmed })
  // Also check the actual gRPC report received by the management API.
  await until(async () => {
    const observations = await api('agent-observations')
    const node = observations.nodes.find(n => n.agent_ref === 'node')
    if (node?.report?.phase !== 'APPLIED' || Number(node.report.revision) !== revision || node.report.sha256 !== node.desired_sha256 || node.connection !== 'connected' || node.freshness !== 'fresh' || !node.matches_desired) return false
    latestControllerReceipt = { authority: observations.authority, agentRef: node.agent_ref,
      desiredRevision: Number(node.desired_revision), desiredSHA256: node.desired_sha256,
      connection: node.connection, freshness: node.freshness, matchesDesired: node.matches_desired,
      reportRevision: Number(node.report.revision), reportSHA256: node.report.sha256, phase: node.report.phase }
    return true
  }, `Controller receiving actual Agent report ${revision}`)
  return revision
}
async function request(family, options = {}) {
  const { port = 18445, path = '/allowed', method = 'GET', headers = {}, persistent = false } = options
  const before = appHits
  let socket
  const result = await new Promise((resolve, reject) => {
    const req = http.request({ hostname: family === 4 ? '127.0.0.1' : '::1', port, path, method, headers: { Host: host, ...headers }, agent: persistent ? keep.get(family).agent : false }, res => {
      res.resume(); res.on('end', () => resolve({ status: res.statusCode, outcome: res.statusCode === 200 ? 'allow' : 'http-deny' }))
    })
    req.on('socket', s => { socket = s })
    req.on('error', e => {
      if (['ETIMEDOUT', 'ECONNREFUSED', 'ECONNRESET'].includes(e.code)) resolve({ status: null, outcome: 'transport-deny', error: e.code })
      else reject(e)
    })
    req.setTimeout(700, () => req.destroy(Object.assign(new Error('probe timeout'), { code: 'ETIMEDOUT' })))
    // setTimeout alone does not cover every connect phase.
    const timer = setTimeout(() => req.destroy(Object.assign(new Error('probe deadline'), { code: 'ETIMEDOUT' })), 1200)
    req.once('close', () => clearTimeout(timer)); req.end()
  })
  if (persistent && result.status !== null) {
    const entry = keep.get(family)
    if (entry.socket) assert.equal(socket, entry.socket, 'must reuse the established TCP connection')
    entry.socket = socket
  }
  return { family, ...result, appDelta: appHits - before }
}
async function scenario(name, options = {}) {
  assert(healthy, 'positive controls must establish healthy services first')
  async function health() {
    assertServicesAlive(children)
    // An unprotected Nginx-owned listener proves contemporaneous ingress health;
    // unlike the protected probe it cannot mistake policy denial for health.
    for (const family of [4, 6]) {
      const control = await request(family, { port: 18447, path: '/health' })
      assert.equal(control.status, 200, 'live dual-stack ingress positive control')
      assert.equal(control.appDelta, 0, 'health listener must not forward to application')
    }
    assertServicesAlive(children)
  }
  await health()
  const observations = []
  for (const [family, outcome] of requiredCases[name]) {
    const o = await request(family, options); observations.push(o)
    assert.equal(o.outcome, outcome, `${name} IPv${family}: ${JSON.stringify(o)}`)
    assert.equal(o.status, outcome === 'allow' ? 200 : outcome === 'http-deny' ? 403 : null, name)
    assert.equal(o.appDelta, outcome === 'allow' ? 1 : 0, name)
  }
  await health()
  report.cases.push({ name, revision: report.confirmations.at(-1).revision, status: 'pass', observations }); console.log(`PASS ${name}`)
}
const basePolicy = `apiVersion: picketx.io/v1alpha1
kind: Resource
metadata: {name: demo, uid: resource-demo-v1}
spec: {endpoints: [{name: web, network: {protocol: TCP, port: 18445}}, {name: sibling, network: {protocol: TCP, port: 18446}}]}
---
apiVersion: picketx.io/v1alpha1
kind: EnforcementBinding
metadata: {name: demo-host, uid: binding-demo-v1}
spec:
  resources: [{resourceRef: {name: demo, uid: resource-demo-v1}}]
  agents: [{agentRef: {name: node}}]
  enforcement: {profileRef: host, class: host-local, stage: original-destination, allowedDestinations: [127.0.0.1, '::1'], handling: graceful}
---
apiVersion: picketx.io/v1alpha1
kind: HTTPRoute
metadata: {name: demo-http, uid: route-v1}
spec: {resourceRef: {name: demo, uid: resource-demo-v1}, endpointName: web, host: 'protected.example:18445', methods: [GET], paths: [/allowed]}
`
const facts = { authorityID: 'static-test', agents: [{ Name: 'node', Labels: {}, Capabilities: { Classes: ['host-local'], Protocols: ['tcp', 'udp'], Families: [4, 6], Handling: ['graceful'], OriginalDestination: true } }], contexts: [{ AgentRef: 'node', ProfileRef: 'host', ID: 'controller-context', NetworkSpaceID: 'declarative-not-local', Class: 'host-local', Stage: 'original-destination', Handling: 'graceful', Destinations: ['127.0.0.1/32', '::1/128'] }] }
try {
  assert.equal(execFileSync('git', ['rev-parse', 'HEAD'], { cwd: core, encoding: 'utf8' }).trim(), coreCommit)
  const installed = await initializeInstalledController(join(root, 'artifacts/controller-bundle'), temp)
  ;({ adminToken, agentToken, ingressToken } = installed)
  facts.authorityID = installed.authority
  const agentBinary = join(temp, 'installed-picketxd')
  await copyFile(join(root, 'artifacts/picketxd-console'), agentBinary); await chmod(agentBinary, 0o755)
  report.agentBinarySHA256 = hash(await readFile(agentBinary))
  report.agentDistribution = JSON.parse(await readFile(join(root, 'artifacts/agent-distribution/proof.json'), 'utf8'))
  validateAgentDistributionEvidence(report.agentDistribution, report)
  assert.equal(report.agentBinarySHA256, hash(await readFile(join(root, 'artifacts/picketxd-console'))))
  report.controllerBinarySHA256 = installed.proof.controllerSHA256
  report.deployment = { ...installed.proof, agentSHA256: report.agentBinarySHA256,
    runtimePATH: installed.runtimeOptions.env.PATH, relocated: true, initialized: true,
    configPreserved: false, credentialsPrivate: true, authority: installed.authority,
    sourceMode: 'database', usersMode: 'database', ingressRoutes: ['route-v1', 'route-gitea-v1'] }
  execFileSync('ip', ['link', 'set', 'lo', 'up'])
  let controller = start('controller', installed.controllerBinary, ['-config', installed.configPath], installed.runtimeOptions)
  const ready = () => until(async () => { try { return (await fetch(origin + '/readyz', { signal: AbortSignal.timeout(500) })).ok } catch { return false } }, 'installed Controller startup')
  await ready()
  for (const port of [18443, 18446, 18448]) {
    const server = http.createServer((req, res) => { appHits++; res.end('protected real HTTP application') })
    server.listen({ host: '::', port, ipv6Only: false }); await once(server, 'listening'); servers.push(server)
  }
  const ingress = await prepareInstalledIngress(installed)
  report.deployment.ingress = ingress.proof
  const nginx = installedNginxConfiguration(temp, ingress.primary, ingress.privateRoute)
  await privateFile('nginx.conf', nginx)
  execFileSync('nginx', ['-t', '-p', temp + '/', '-c', join(temp, 'nginx.conf')])
  ingress.proof.servers.demo = hash(await readFile(join(temp, 'nginx.conf')))
  start('nginx', 'nginx', ['-p', temp + '/', '-c', join(temp, 'nginx.conf'), '-g', 'daemon off;'])
  await until(async () => (await request(4)).status === 403, 'Nginx authorization startup')
  // Direct positive controls before kernel ownership: both listeners exist, not fake denial.
  for (const family of [4, 6]) {
    assert.equal((await request(family)).status, 403)
    for (const port of [18443, 18446, 18448]) assert.equal((await request(family, { port })).status, 200)
    keep.set(family, { agent: new http.Agent({ keepAlive: true, maxSockets: 1 }) })
  }
  healthy = true
  const { chromium } = await import(pathToFileURL(join(core, 'apps/web/node_modules/playwright/index.mjs')))
  browser = await chromium.launch({ headless: true, args: ['--host-resolver-rules=MAP protected.example 127.0.0.1', '--no-proxy-server'] })
  const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } })
  const pageErrors = []; page.on('pageerror', e => pageErrors.push(e.message))
  await page.goto(origin + '/console/')
  await page.getByLabel('Access key', { exact: true }).fill(adminToken)
  await page.getByRole('button', { name: 'Sign in', exact: true }).click()
  await page.getByLabel('Policy YAML').waitFor()
  await provisionManagedHumans({ page, password, evidence, report })
  async function publish(policy, reason) {
    await page.getByRole('button', { name: 'Desired configuration', exact: true }).click()
    const reload = page.getByRole('button', { name: 'Reload latest into editor', exact: true })
    page.once('dialog', d => d.accept()); await reload.click()
    await page.getByLabel('Policy YAML').fill(policy)
    await page.getByLabel('Facts JSON').fill(JSON.stringify(facts))
    await page.getByLabel('Change reason', { exact: true }).fill(reason)
    const response = page.waitForResponse(r => r.url().endsWith('/publication') && r.request().method() === 'PUT')
    await page.getByRole('button', { name: 'Save desired configuration', exact: true }).click()
    assert.equal((await response).status(), 200)
    await page.getByText('Desired configuration saved. Node application is not confirmed.', { exact: true }).waitFor()
  }
  await publish(basePolicy, 'Combined native baseline')
  const agentArgs = ['enforce', '--controller', '127.0.0.1:19090', '--agent-token-file', installed.agentTokenFile, '--controller-plaintext', '--state-dir', join(temp, 'agent-state'), '--provider', installed.authority, '--agent', 'node', '--destination', '127.0.0.1', '--destination', '::1', '--mark-first', '0x504a0000', '--mark-last', '0x504a003f', '--poll', '30ms']
  function startAgent(name) {
    const child = start(name, agentBinary, agentArgs, installed.runtimeOptions)
    let buffer = ''
    child.stdout.on('data', chunk => { buffer += chunk; let i; while ((i = buffer.indexOf('\n')) >= 0) { const line = buffer.slice(0, i); buffer = buffer.slice(i + 1); if (line) status.push(JSON.parse(line)) } })
    return child
  }
  let agent = startAgent('agent')
  await confirmed(); await scenario('baseline')
  await page.getByRole('button', { name: 'Grants', exact: true }).click()
  // Grants remains mounted across tabs and deliberately does not silently rebase
  // its draft after another editor publishes a different Resource catalog.
  await page.getByRole('button', { name: 'Use latest Grant revision', exact: true }).click()
  async function issue(name, source) {
    await page.getByLabel('Grant name', { exact: true }).fill(name)
    await page.getByLabel('Source IP or CIDR', { exact: true }).fill(source)
    await page.getByLabel('Resource', { exact: true }).selectOption('resource-demo-v1')
    await page.getByRole('checkbox', { name: 'web TCP/18445', exact: true }).check()
    await page.getByLabel('Grant issue reason', { exact: true }).fill(`Combined native ${name}`)
    const response = page.waitForResponse(r => r.url().endsWith('/grants') && r.request().method() === 'POST')
    await page.getByRole('button', { name: 'Issue Grant', exact: true }).click()
    const r = await response; assert.equal(r.status(), 201); const grant = await r.json()
    await page.locator(`[data-grant-id="${grant.uid}"]`).waitFor(); await confirmed(); return grant
  }
  async function revoke(grant) {
    await page.locator(`[data-grant-id="${grant.uid}"]`).getByRole('button', { name: 'Revoke Grant', exact: true }).click()
    await page.getByLabel('Revocation reason', { exact: true }).fill(`Combined revoke ${grant.uid}`)
    const response = page.waitForResponse(r => r.url().endsWith(`/grants/${grant.uid}/revoke`))
    await page.getByRole('button', { name: 'Confirm revocation', exact: true }).click()
    assert.equal((await response).status(), 200)
    await page.locator(`[data-grant-id="${grant.uid}"]`).getByText('Revoked', { exact: true }).waitFor(); await confirmed()
  }
  const four = await issue('browser-four', '127.0.0.1'); await scenario('grant4')
  const six = await issue('browser-six', '::1'); await scenario('grant6', { persistent: true })
  // Restart both real deployed processes with active grants, never reinitialize.
  const paths = { publication: 'publication', accounts: 'accounts', grants: 'grants?limit=100', audit: 'audit?limit=100' }
  const views = async () => Object.fromEntries(await Promise.all(Object.entries(paths).map(async ([k, path]) => [k, businessViewDigest(k, await api(path))])))
  const beforeViews = await views(), revision = (await api('publication')).revision
  const cookie = (await page.context().cookies()).map(c => `${c.name}=${c.value}`).join('; ')
  async function stopDeployed(child) {
    const exited = once(child, 'close'); child.kill('SIGTERM')
    const timer = setTimeout(() => child.kill('SIGKILL'), 5000)
    const [code, signal] = await exited; clearTimeout(timer)
    assert.equal(code, 0, 'clean deployed process exit'); assert.equal(signal, null)
    children.splice(children.indexOf(child), 1)
    return code
  }
  const agentExit = await stopDeployed(agent), controllerExit = await stopDeployed(controller)
  // Retain complete raw history for independent correlation; fence old receipts.
  const statusOffset = status.length
  controller = start('controller-restarted', installed.controllerBinary, ['-config', installed.configPath], installed.runtimeOptions)
  await ready()
  const oldSessionStatus = (await fetch(origin + '/console/api/v1/publication', { headers: { Cookie: cookie } })).status
  assert.equal(oldSessionStatus, 401)
  const afterViews = await views(); assert.deepEqual(afterViews, beforeViews)
  agent = startAgent('agent-restarted')
  assert.equal(await confirmed(statusOffset), revision)
  const freshStatusIndex = status.findLastIndex((s, i) => i >= statusOffset && s.enforcementConfirmed && s.appliedGeneration === revision)
  const fresh = status[freshStatusIndex]
  const observations = []
  for (const family of [4, 6]) {
    const result = await request(family); observations.push(result)
    assert.deepEqual(result, { family, outcome: 'allow', status: 200, appDelta: 1 })
  }
  assert.deepEqual(await readFile(installed.configPath), installed.configBytes)
  report.deployment.configPreserved = true
  report.deployment.restart = { revision, controllerExit, agentExit, oldSessionStatus,
    freshStatus: Boolean(fresh), statusOffset, freshStatusIndex, freshGeneration: fresh.appliedGeneration, freshPolicyHash: fresh.appliedPolicyHash,
    views: Object.fromEntries(Object.keys(paths).map(k => [k, { before: beforeViews[k], after: afterViews[k] }])),
    observations, controllerReceipt: latestControllerReceipt }
  await page.context().clearCookies(); await page.goto(origin + '/console/')
  await page.getByLabel('Access key', { exact: true }).fill(adminToken)
  await page.getByRole('button', { name: 'Sign in', exact: true }).click()
  await page.getByLabel('Policy YAML').waitFor()
  await page.getByRole('button', { name: 'Grants', exact: true }).click()
  console.log('PASS installed empty-PATH Controller/Agent: clean restart, identical DB views, fresh kernel/report confirmation and dual-stack traffic')
  for (const family of [4, 6]) {
    assertServicesAlive(children)
    const admittedStatus = (await request(family)).status
    assert.equal(admittedStatus, 200, 'private test requires admitted kernel and primary route')
    const denied = await request(family, { path: '/private' })
    assert.equal(denied.status, 403); assert.equal(denied.appDelta, 0)
    const healthStatus = (await request(family, { port: 18447, path: '/health' })).status
    assert.equal(healthStatus, 200); assertServicesAlive(children)
    ingress.proof.privateDenials.push({ server: 'demo', family, revision, status: denied.status,
      appDelta: denied.appDelta, admittedStatus, healthStatus })
  }
  console.log('PASS generated demo private route: dual-stack unauthorized caller, zero application forwarding')
  await scenario('sibling', { port: 18446 })
  await scenario('path', { path: '/denied', persistent: true })
  await scenario('method', { method: 'POST', persistent: true })
  await scenario('host', { headers: { Host: 'untrusted.example' }, persistent: true })
  await scenario('forged', { path: '/denied', headers: { 'X-PicketX-URI': '/allowed', 'X-PicketX-Host': host, 'X-PicketX-Source': '127.0.0.1', 'X-Forwarded-For': '127.0.0.1' }, persistent: true })
  // Actual browser navigation reaches the application through both enforcement layers.
  const appPage = await browser.newPage()
  const beforeBrowser = appHits
  const reached = await appPage.goto('http://protected.example:18445/allowed', { timeout: 3000 })
  assert.equal(reached.status(), 200)
  assert.equal(await appPage.locator('body').textContent(), 'protected real HTTP application')
  await appPage.screenshot({ path: join(evidence, 'application.png'), fullPage: true })
  assert.equal(appHits - beforeBrowser, 1)
  report.cases.push({ name: 'browser', revision: report.confirmations.at(-1).revision, status: 'pass', observations: [{ family: 4, outcome: 'allow', status: reached.status(), appDelta: appHits - beforeBrowser }] })
  await appPage.close()
  const overlap = await issue('browser-overlap', '127.0.0.1')
  await revoke(four); await scenario('overlap')
  await revoke(overlap); await scenario('revoke4'); await scenario('establishedRevoke4', { persistent: true })
  await revoke(six); await scenario('revoke6')
  await page.screenshot({ path: join(evidence, 'grants-desktop.png'), fullPage: true })
  const expiry = Date.now() + 12000
  let bounded = basePolicy
  for (const [family, source] of [[4, '127.0.0.1/32'], [6, '::1/128']]) bounded += `---\napiVersion: picketx.io/v1alpha1\nkind: Grant\nmetadata: {name: deadline-${family}, uid: deadline-${family}-v1}\nspec:\n  subject: {type: source, cidr: '${source}'}\n  scopes: [{resourceRef: {name: demo, uid: resource-demo-v1}, endpointNames: [web]}]\n  validity: {mode: bounded, expiresAt: '${new Date(expiry + (family === 6 ? 8000 : 0)).toISOString()}'}\n`
  await publish(bounded, 'Combined absolute deadline'); await confirmed()
  for (const entry of keep.values()) { entry.agent.destroy(); entry.agent = new http.Agent({ keepAlive: true, maxSockets: 1 }); entry.socket = null }
  await scenario('expiring', { persistent: true })
  await pause(Math.max(0, expiry + 250 - Date.now()))
  await scenario('expired4')
  await pause(Math.max(0, expiry + 8250 - Date.now()))
  await scenario('expired'); await scenario('establishedExpired', { persistent: true })
  await scenario('unmanaged', { port: 18448 })
  // The previous expiry case intentionally retained established connections.
  // The human sequence must establish its own sockets after pending denial.
  for (const entry of keep.values()) { entry.agent.destroy(); entry.agent = new http.Agent({ keepAlive: true, maxSockets: 1 }); entry.socket = null }
  await runHumanRequests({ browser, origin, host, password, evidence, report, api, confirmed, scenario, adminPage: page, appHits: () => appHits })
  report.gitea = await runGiteaApplication({ root, ingress, temp, evidence, chromium, page, publish, confirmed, api, start, children, secrets: applicationSecrets })
  validateGiteaEvidence(report.gitea, report.confirmations)
  await page.getByRole('button', { name: 'Nodes', exact: true }).click()
  await page.getByRole('button', { name: 'Refresh nodes', exact: true }).click()
  await page.getByRole('heading', { name: 'node', exact: true }).waitFor()
  await page.screenshot({ path: join(evidence, 'nodes-desktop.png'), fullPage: true })
  await page.setViewportSize({ width: 375, height: 812 })
  assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
  await page.screenshot({ path: join(evidence, 'nodes-mobile.png'), fullPage: true })
  const audit = await api('audit?limit=100')
  for (const reason of ['Combined native baseline', 'Combined native browser-four', 'Combined native browser-six', 'Combined absolute deadline']) assert(audit.events.some(e => e.reason === reason && e.actor === 'principal:bootstrap'))
  assert.deepEqual(pageErrors, [])
  validateEvidence(report, { statuses: status, restartedStatuses: parseEnforceStatusLog(logs.get('agent-restarted')) })
  report.result = 'pass'
  console.log(`PASS combined native evidence: ${report.cases.length} cases, ${report.confirmations.length} actual Agent confirmations`)
} catch (error) {
  throw redactFixtureFailure(error, [adminToken, agentToken, ingressToken, password, ...applicationSecrets])
} finally {
  await browser?.close()
  for (const entry of keep.values()) entry.agent.destroy()
  for (const child of children.reverse()) {
    if (child.exitCode === null && child.signalCode === null) {
      const exited = once(child, 'close'); child.kill('SIGTERM')
      const timer = setTimeout(() => child.kill('SIGKILL'), 3000)
      await exited; clearTimeout(timer)
    }
  }
  for (const server of servers) { server.closeAllConnections(); await new Promise(r => server.close(r)) }
  const redact = x => [adminToken, agentToken, ingressToken, password, ...applicationSecrets].filter(Boolean).reduce((s, secret) => s.replaceAll(secret, '[REDACTED]'), x)
  for (const [name, log] of logs) await writeFile(join(evidence, name + '.log'), redact(log))
  await writeFile(join(evidence, 'report.json'), JSON.stringify(report, null, 2))
  await writeFile(join(evidence, 'agent-status.json'), JSON.stringify(status, null, 2))
  // Only the known mkdtemp directory belongs to this fixture; never the worktree.
  await rm(temp, { recursive: true })
}
