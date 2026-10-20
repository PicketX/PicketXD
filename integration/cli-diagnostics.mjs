// SPDX-License-Identifier: GPL-3.0-or-later
// Native acceptance adapter. CLI observations never establish kernel enforcement.
import assert from 'node:assert/strict'
import { execFile } from 'node:child_process'
import { join } from 'node:path'

const phases = ['before-agent', 'applied', 'grants-applied', 'agent-stopped', 'controller-restored', 'agent-restarted']
const healthyPhase = phase => ['applied', 'grants-applied', 'agent-restarted'].includes(phase)
const hash = /^[a-f0-9]{64}$/
const decimal = /^[1-9][0-9]*$/
function keys(value, names) {
  assert(value && typeof value === 'object' && !Array.isArray(value))
  assert.deepEqual(Object.keys(value).sort(), [...names].sort())
}
function publicReport(value, authority, revision, state, doctor) {
  keys(value, ['scope', 'authority', 'revision', 'observed_at', 'nodes', 'enforcement_confirmed', ...(doctor ? ['ready'] : [])])
  assert.equal(value.scope, 'controller-observations'); assert.equal(value.authority, authority)
  assert.equal(value.revision, String(revision)); assert.equal(value.enforcement_confirmed, false)
  assert(typeof value.observed_at === 'string' && /^\d{4}-\d\d-\d\dT.*Z$/.test(value.observed_at) && Number.isFinite(Date.parse(value.observed_at)))
  if (doctor) assert.equal(value.ready, true)
  assert(Array.isArray(value.nodes) && value.nodes.length === 1)
  const n = value.nodes[0]
  keys(n, ['agent_ref', 'desired_revision', 'state', 'fresh_for_millis', ...(Object.hasOwn(n, 'reported_revision') ? ['reported_revision'] : [])])
  assert.equal(n.agent_ref, 'node'); assert.equal(n.desired_revision, String(revision)); assert.equal(n.state, state)
  assert(Number.isSafeInteger(n.fresh_for_millis) && n.fresh_for_millis <= 60000)
  if (state === 'reported-applied') {
    assert.equal(n.reported_revision, String(revision)); assert(n.fresh_for_millis > 0)
  } else {
    assert.equal(n.fresh_for_millis, 0)
    if (Object.hasOwn(n, 'reported_revision')) assert(typeof n.reported_revision === 'string' && decimal.test(n.reported_revision) && BigInt(n.reported_revision) <= BigInt(revision))
  }
}
function validateCase(c, authority, revision, phase) {
  assert(Number.isSafeInteger(revision) && revision > 0)
  assert.equal(c.phase, phase); assert.equal(c.revision, revision)
  const healthy = healthyPhase(phase), state = healthy ? 'reported-applied' : 'disconnected'
  assert.equal(c.statusExit, 0); assert.equal(c.doctorExit, healthy ? 0 : 1)
  publicReport(c.status, authority, revision, state, false)
  publicReport(c.doctor, authority, revision, state, true)
}
function inspectCommand(installed, origin, command) {
  return new Promise((resolve, reject) => {
    execFile(installed.cliBinary, [command, '--controller', origin, '--key-file', join(installed.instance, 'bootstrap-key'),
      '--agent', 'node', '--format', 'json', '--timeout', '5s'],
    { ...installed.runtimeOptions, shell: false, encoding: 'utf8', timeout: 10000, maxBuffer: 64 * 1024 }, (error, stdout) => {
      // Node launch/timeout/buffer failures are NOT ordinary numeric exit 1.
      if (error && (error.killed || error.signal || ![0, 1].includes(error.code))) return reject(new Error('installed CLI inspection failed'))
      try { resolve({ report: JSON.parse(stdout), exit: error ? error.code : 0 }) }
      catch { reject(new Error('installed CLI inspection failed')) }
    })
  })
}
export async function inspectInstalledCLI(installed, origin, expected) {
  try {
    assert.equal(installed.runtimeOptions.env.PATH, '')
    assert(phases.includes(expected.phase))
    assert.equal(expected.state, healthyPhase(expected.phase) ? 'reported-applied' : 'disconnected')
    const status = await inspectCommand(installed, origin, 'status'), doctor = await inspectCommand(installed, origin, 'doctor')
    const result = { phase: expected.phase, revision: expected.revision, status: status.report, doctor: doctor.report, statusExit: status.exit, doctorExit: doctor.exit }
    validateCase(result, installed.authority, expected.revision, expected.phase)
    return result
  } catch { throw new Error('installed CLI inspection failed') }
}
export function validateCLIDiagnostics(proof, d, report, evidence) {
  try {
    keys(proof, ['source', 'cliSHA256', 'runtimePATH', 'cases'])
    assert.deepEqual(proof.source, d.source); assert.equal(proof.cliSHA256, d.cliSHA256)
    assert.equal(proof.runtimePATH, ''); assert.equal(proof.runtimePATH, d.runtimePATH)
    assert(Array.isArray(proof.cases) && proof.cases.length === phases.length)
    const baseline = report.cases.find(c => c.name === 'baseline')?.revision
    const grants = report.cases.find(c => c.name === 'grant6')?.revision
    for (const [i, c] of proof.cases.entries()) {
      const phase = phases[i], healthy = healthyPhase(phase), revision = i < 2 ? baseline : grants
      keys(c, ['phase', 'revision', 'status', 'doctor', 'statusExit', 'doctorExit', ...(healthy ? ['rawStatusIndex', 'controllerReceipt'] : [])])
      validateCase(c, d.authority, revision, phase)
      if (!healthy) continue
      assert(Array.isArray(evidence.statuses) && Number.isSafeInteger(c.rawStatusIndex) && c.rawStatusIndex >= 0 && c.rawStatusIndex < evidence.statuses.length)
      if (phase === 'agent-restarted') assert(c.rawStatusIndex >= d.restart.statusOffset)
      else assert(c.rawStatusIndex < d.restart.statusOffset)
      const s = evidence.statuses[c.rawStatusIndex]
      assert.equal(s.enforcementConfirmed, true); assert.equal(s.desired?.generation, revision); assert.equal(s.appliedGeneration, revision)
      assert.match(s.desired?.policyHash, hash); assert.equal(s.appliedPolicyHash, s.desired.policyHash)
      assert(report.confirmations.some(r => r.revision === revision && r.generation === revision && r.appliedGeneration === revision && r.enforcementConfirmed === true && r.policyHash === s.desired.policyHash && r.appliedPolicyHash === s.appliedPolicyHash))
      const receipt = c.controllerReceipt
      keys(receipt, ['authority', 'agentRef', 'desiredRevision', 'desiredSHA256', 'connection', 'freshness', 'matchesDesired', 'reportRevision', 'reportSHA256', 'phase'])
      assert.equal(receipt.authority, d.authority); assert.equal(receipt.agentRef, 'node')
      assert.equal(receipt.desiredRevision, revision); assert.equal(receipt.reportRevision, revision)
      assert.equal(receipt.connection, 'connected'); assert.equal(receipt.freshness, 'fresh'); assert.equal(receipt.matchesDesired, true)
      assert.equal(receipt.phase, 'APPLIED'); assert.match(receipt.desiredSHA256, hash); assert.equal(receipt.reportSHA256, receipt.desiredSHA256)
      // Typed snapshot hashes and projected Agent policy hashes have distinct domains.
      if (phase === 'agent-restarted') assert.deepEqual(receipt, d.restart.controllerReceipt)
    }
  } catch { throw new Error('invalid installed CLI evidence') }
}
