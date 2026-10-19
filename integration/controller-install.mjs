// SPDX-License-Identifier: GPL-3.0-or-later
// Native acceptance adapter, not a product installer or credential API.
import assert from 'node:assert/strict'
import { cp, lstat, readFile, readdir, realpath, writeFile } from 'node:fs/promises'
import { join, resolve } from 'node:path'
import { createHash } from 'node:crypto'
import { execFileSync } from 'node:child_process'
import { validateInstalledIngress } from './installed-ingress.mjs'

export const coreCommit = 'adfa1b111e719b606222f6643c7343afa98d3f03'
export const coreTree = '7a5cbb6c764c5e2b4a98e956a3e23fb1b3169d71'
export const digest = value => createHash('sha256').update(value).digest('hex')
// Read-observation clocks are not persisted business state. All other fields,
// including audit event timestamps and independent absolute Grant deadlines, stay.
export function businessViewDigest(name, view) {
  assert(['publication', 'accounts', 'grants', 'audit'].includes(name))
  const document = structuredClone(view)
  const clock = { accounts: 'observedAt', grants: 'observed_at' }[name]
  if (clock) {
    assert(typeof document[clock] === 'string' && Number.isFinite(Date.parse(document[clock])), 'actual read observation clock')
    delete document[clock]
  }
  return digest(JSON.stringify(document))
}

export function parseEnforceStatusLog(log) {
  return log.split('\n').filter(line => line.trim()).map(line => JSON.parse(line))
    .filter(event => event && typeof event === 'object' && Object.hasOwn(event, 'desired'))
}
const hashPattern = /^[a-f0-9]{64}$/
function keys(value, expected) {
  assert(value && typeof value === 'object' && !Array.isArray(value), 'required public object')
  assert.deepEqual(Object.keys(value).sort(), [...expected].sort(), 'only public evidence fields')
}

export async function verifyControllerBundle(directory) {
  directory = resolve(directory)
  assert.equal(await realpath(directory), directory, 'no symlinked bundle root')
  const manifest = await readFile(join(directory, 'SHA256SUMS'))
  const covered = new Set()
  for (const line of manifest.toString().trimEnd().split('\n')) {
    const match = /^([a-f0-9]{64})  (.+)$/.exec(line)
    assert(match, 'valid SHA256 manifest line required')
    const [, expected, path] = match
    assert(!path.includes('\\') && path.split('/').every(p => p && p !== '.' && p !== '..'), 'safe relative manifest path')
    assert(!covered.has(path), 'duplicate manifest path'); covered.add(path)
    let current = directory
    for (const component of path.split('/')) {
      current = join(current, component)
      assert(!(await lstat(current)).isSymbolicLink(), 'symlinked bundle entry')
    }
    assert((await lstat(current)).isFile(), 'regular manifest file required')
    assert.equal(digest(await readFile(current)), expected, 'bundle content checksum')
  }
  async function walk(dir, prefix = '') {
    for (const name of await readdir(dir)) {
      const path = prefix + name, full = join(dir, name), info = await lstat(full)
      assert(!info.isSymbolicLink(), 'no symlink in bundle')
      if (info.isDirectory()) await walk(full, path + '/')
      else { assert(info.isFile(), 'regular bundle file'); assert(path === 'SHA256SUMS' || covered.has(path), 'unlisted bundle file') }
    }
  }
  await walk(directory)
  for (const path of ['SOURCE.json', 'bin/picketx', 'bin/picketx-controller', 'web/index.html']) assert(covered.has(path), 'required installed content')
  for (const path of ['bin/picketx', 'bin/picketx-controller']) assert((await lstat(join(directory, path))).mode & 0o111, 'executable mode retained')
  const source = JSON.parse(await readFile(join(directory, 'SOURCE.json'), 'utf8'))
  assert.deepEqual(source, { commit: coreCommit, tree: coreTree, platform: 'linux-amd64' }, 'pinned installed Core source')
  return { source, manifestSHA256: digest(manifest), cliSHA256: digest(await readFile(join(directory, 'bin/picketx'))),
    controllerSHA256: digest(await readFile(join(directory, 'bin/picketx-controller'))) }
}

export async function initializeInstalledController(bundle, temp) {
  temp = await realpath(temp)
  const deployed = join(temp, 'installed-controller'), instance = join(temp, 'controller')
  await cp(bundle, deployed, { recursive: true, errorOnExist: true, force: false })
  const proof = await verifyControllerBundle(deployed)
  const runtimeOptions = { cwd: temp, env: { PATH: '' } }
  execFileSync(join(deployed, 'bin/picketx'), ['controller-init', '--directory', instance,
    '--assets', join(deployed, 'web'), '--origin', 'http://127.0.0.1:18080', '--listen', '127.0.0.1:18080',
    '--agent-listen', '127.0.0.1:19090', '--authz-listen', '127.0.0.1:19091', '--agent-ref', 'node', '--route-id', 'route-v1'],
  { ...runtimeOptions, encoding: 'utf8', timeout: 10000 })
  const configPath = join(instance, 'controller.yaml'), configBytes = await readFile(configPath)
  const config = JSON.parse(configBytes)
  assert.equal(config.source.mode, 'database'); assert.equal(config.management.users_mode, 'database')
  assert.equal(config.browser.assets, join(deployed, 'web'))
  assert.match(config.agent.authority, /^controller:[a-f0-9]{32}$/)
  assert.equal((await lstat(instance)).mode & 0o777, 0o700)
  for (const name of ['controller.yaml', 'bootstrap-key', 'agent-token', 'ingress-token', 'principals.json', 'agents.json', 'callers.json']) {
    const info = await lstat(join(instance, name))
    assert(info.isFile() && (info.mode & 0o777) === 0o600, 'private generated regular file')
  }
  const secrets = {}
  for (const [key, name] of [['adminToken', 'bootstrap-key'], ['agentToken', 'agent-token'], ['ingressToken', 'ingress-token']]) {
    const secret = (await readFile(join(instance, name), 'utf8')).trim()
    assert(/^[a-f0-9]{64}$/.test(secret), 'valid generated private key'); secrets[key] = secret
  }
  assert.equal(new Set(Object.values(secrets)).size, 3, 'independent generated credentials')
  const principals = JSON.parse(await readFile(join(instance, 'principals.json'), 'utf8'))
  assert.deepEqual(principals, { principals: [{ id: 'principal:bootstrap', sha256: digest(secrets.adminToken), roles: ['role:admin'] }] })
  const agents = JSON.parse(await readFile(join(instance, 'agents.json'), 'utf8'))
  assert.deepEqual(agents, { credentials: [{ id: 'initial-agent', agent_ref: 'node', sha256: digest(secrets.agentToken) }] })
  const callerPath = join(instance, 'callers.json'), callers = JSON.parse(await readFile(callerPath, 'utf8'))
  assert.deepEqual(callers, { callers: [{ id: 'initial-ingress', sha256: digest(secrets.ingressToken), routes: ['route-v1'] }] })
  // Explicit operator-reviewed enrollment of the second existing ingress route.
  callers.callers[0].routes.push('route-gitea-v1')
  await writeFile(callerPath, JSON.stringify(callers), { mode: 0o600 })
  return { proof, ...secrets, runtimeOptions, configPath, configBytes, instance,
    authority: config.agent.authority, agentTokenFile: join(instance, 'agent-token'),
    cliBinary: join(deployed, 'bin/picketx'), ingressTokenFile: join(instance, 'ingress-token'),
    controllerBinary: join(deployed, 'bin/picketx-controller') }
}

export function validateInstalledEvidence(d, report, evidence) {
  keys(d, ['source', 'manifestSHA256', 'cliSHA256', 'controllerSHA256', 'agentSHA256', 'runtimePATH', 'relocated',
    'initialized', 'configPreserved', 'credentialsPrivate', 'authority', 'sourceMode', 'usersMode', 'ingressRoutes', 'ingress', 'restart'])
  assert.deepEqual(d.source, { commit: coreCommit, tree: coreTree, platform: 'linux-amd64' })
  for (const field of ['manifestSHA256', 'cliSHA256', 'controllerSHA256', 'agentSHA256']) assert.match(d[field], hashPattern)
  assert.equal(d.controllerSHA256, report.controllerBinarySHA256); assert.equal(d.agentSHA256, report.agentBinarySHA256)
  assert.equal(d.runtimePATH, '')
  for (const field of ['relocated', 'initialized', 'configPreserved', 'credentialsPrivate']) assert.equal(d[field], true)
  assert.match(d.authority, /^controller:[a-f0-9]{32}$/)
  assert.equal(d.sourceMode, 'database'); assert.equal(d.usersMode, 'database')
  assert.deepEqual(d.ingressRoutes, ['route-v1', 'route-gitea-v1'])
  validateInstalledIngress(d.ingress, d.cliSHA256, report.confirmations)
  const r = d.restart
  keys(r, ['revision', 'controllerExit', 'agentExit', 'oldSessionStatus', 'freshStatus', 'statusOffset', 'freshStatusIndex', 'freshGeneration', 'freshPolicyHash', 'views', 'observations', 'controllerReceipt'])
  assert.equal(r.controllerExit, 0); assert.equal(r.agentExit, 0); assert.equal(r.oldSessionStatus, 401)
  assert.equal(r.freshStatus, true); assert.equal(r.freshGeneration, r.revision)
  assert(Number.isSafeInteger(r.statusOffset) && r.statusOffset > 0)
  assert(Number.isSafeInteger(r.freshStatusIndex) && r.freshStatusIndex >= r.statusOffset, 'fresh post-restart status required')
  keys(evidence, ['statuses', 'restartedStatuses'])
  assert(Array.isArray(evidence.statuses) && Array.isArray(evidence.restartedStatuses) && evidence.restartedStatuses.length > 0, 'actual raw status history and restarted log required')
  assert(r.freshStatusIndex < evidence.statuses.length && r.statusOffset < evidence.statuses.length, 'raw status index in bounds')
  assert.deepEqual(evidence.statuses.slice(r.statusOffset), evidence.restartedStatuses, 'raw restarted process boundary')
  const selected = evidence.statuses[r.freshStatusIndex]
  assert.equal(selected.enforcementConfirmed, true)
  assert.equal(selected.desired?.generation, r.revision); assert.equal(selected.appliedGeneration, r.revision)
  assert.equal(selected.desired?.policyHash, r.freshPolicyHash); assert.equal(selected.appliedPolicyHash, r.freshPolicyHash)
  assert.equal(r.revision, report.cases.find(c => c.name === 'grant6')?.revision)
  assert(report.confirmations.some(c => c.revision === r.revision && c.enforcementConfirmed && c.appliedGeneration === r.revision && c.appliedPolicyHash === r.freshPolicyHash))
  const receipt = r.controllerReceipt
  keys(receipt, ['authority', 'agentRef', 'desiredRevision', 'desiredSHA256', 'connection', 'freshness', 'matchesDesired', 'reportRevision', 'reportSHA256', 'phase'])
  assert.equal(receipt.authority, d.authority); assert.equal(receipt.agentRef, 'node')
  assert.equal(receipt.desiredRevision, r.revision); assert.equal(receipt.reportRevision, r.revision)
  assert.equal(receipt.connection, 'connected'); assert.equal(receipt.freshness, 'fresh'); assert.equal(receipt.matchesDesired, true)
  assert.equal(receipt.phase, 'APPLIED'); assert.match(receipt.desiredSHA256, hashPattern)
  // Typed snapshot SHA and projected Agent policy hash are different domains.
  assert.equal(receipt.reportSHA256, receipt.desiredSHA256)
  keys(r.views, ['publication', 'accounts', 'grants', 'audit'])
  for (const value of Object.values(r.views)) { keys(value, ['before', 'after']); assert.match(value.before, hashPattern); assert.equal(value.after, value.before) }
  assert.deepEqual(r.observations, [4, 6].map(family => ({ family, outcome: 'allow', status: 200, appDelta: 1 })))
}
