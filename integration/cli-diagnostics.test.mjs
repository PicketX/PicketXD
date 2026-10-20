// SPDX-License-Identifier: GPL-3.0-or-later
import test from 'node:test'
import assert from 'node:assert/strict'
import { mkdtemp, writeFile, rm } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { inspectInstalledCLI, validateCLIDiagnostics } from './cli-diagnostics.mjs'

// Unit-only inputs. Actual acceptance executes the shipped CLI in native CI.
function diagnosticsFixture(d, report) {
  const cases = ['before-agent', 'applied', 'grants-applied', 'agent-stopped', 'controller-restored', 'agent-restarted'].map((phase, i) => {
    const healthy = [1, 2, 5].includes(i), revision = i < 2 ? report.cases.find(c => c.name === 'baseline').revision : d.restart.revision
    const result = doctor => ({ scope: 'controller-observations', authority: d.authority, revision: String(revision),
      observed_at: '2026-10-20T10:00:00Z', ...(doctor ? { ready: true } : {}),
      nodes: [{ agent_ref: 'node', desired_revision: String(revision), ...(healthy ? { reported_revision: String(revision) } : {}),
        state: healthy ? 'reported-applied' : 'disconnected', fresh_for_millis: healthy ? 59000 : 0 }], enforcement_confirmed: false })
    return { phase, revision, status: result(false), doctor: result(true), statusExit: 0, doctorExit: healthy ? 0 : 1,
      ...(healthy ? { rawStatusIndex: i === 5 ? d.restart.freshStatusIndex : 0, controllerReceipt: { ...d.restart.controllerReceipt, desiredRevision: revision, reportRevision: revision } } : {}) }
  })
  return { source: structuredClone(d.source), cliSHA256: d.cliSHA256, runtimePATH: '', cases }
}
const unit = () => {
  const d = { source: { commit: 'a'.repeat(40), tree: 'b'.repeat(40), platform: 'linux-amd64' }, cliSHA256: 'c'.repeat(64), runtimePATH: '', authority: 'controller:' + '1'.repeat(32),
    restart: { revision: 2, statusOffset: 1, freshStatusIndex: 1, controllerReceipt: { authority: 'controller:' + '1'.repeat(32), agentRef: 'node', desiredRevision: 2, desiredSHA256: 'd'.repeat(64), connection: 'connected', freshness: 'fresh', matchesDesired: true, reportRevision: 2, reportSHA256: 'd'.repeat(64), phase: 'APPLIED' } } }
  const report = { cases: [{ name: 'baseline', revision: 2 }, { name: 'grant6', revision: 2 }], confirmations: [{ revision: 2, generation: 2, appliedGeneration: 2, policyHash: 'e'.repeat(64), appliedPolicyHash: 'e'.repeat(64), enforcementConfirmed: true }] }
  const evidence = { statuses: [0, 1].map(() => ({ enforcementConfirmed: true, desired: { generation: 2, policyHash: 'e'.repeat(64) }, appliedGeneration: 2, appliedPolicyHash: 'e'.repeat(64) })) }
  return { d, report, evidence, proof: diagnosticsFixture(d, report) }
}
test('six complete unit phases correlate observations, not independent enforcement', () => { const u = unit(); validateCLIDiagnostics(u.proof, u.d, u.report, u.evidence) })
for (const [name, mutate] of [
  ['missing proof', u => { u.proof = undefined }], ['missing phase', u => u.proof.cases.pop()],
  ['wrong order', u => u.proof.cases.reverse()], ['different source', u => { u.proof.source.commit = 'f'.repeat(40) }],
  ['different binary', u => { u.proof.cliSHA256 = 'f'.repeat(64) }], ['inherited PATH', u => { u.proof.runtimePATH = '/usr/bin' }],
  ['secret extra', u => { u.proof.key = 'private-marker' }], ['wrong exit', u => { u.proof.cases[0].doctorExit = 0 }],
  ['null exit', u => { u.proof.cases[1].statusExit = null }], ['fake enforcement', u => { u.proof.cases[1].status.enforcement_confirmed = true }],
  ['wrong scope', u => { u.proof.cases[1].doctor.scope = 'kernel' }], ['unready', u => { u.proof.cases[1].doctor.ready = false }],
  ['wrong authority', u => { u.proof.cases[1].status.authority = 'other' }], ['old revision', u => { u.proof.cases[1].status.revision = '1' }],
  ['bad clock', u => { u.proof.cases[1].doctor.observed_at = 'never' }], ['extra node', u => u.proof.cases[1].status.nodes.push({})],
  ['wrong node', u => { u.proof.cases[1].status.nodes[0].agent_ref = 'other' }], ['wrong desired', u => { u.proof.cases[1].status.nodes[0].desired_revision = '1' }],
  ['stale report', u => { u.proof.cases[1].status.nodes[0].reported_revision = '1' }], ['zero healthy TTL', u => { u.proof.cases[1].doctor.nodes[0].fresh_for_millis = 0 }],
  ['excess TTL', u => { u.proof.cases[1].doctor.nodes[0].fresh_for_millis = 60001 }], ['disconnected TTL', u => { u.proof.cases[0].doctor.nodes[0].fresh_for_millis = 1 }],
  ['private nested data', u => { u.proof.cases[1].status.nodes[0].token = 'private-marker' }], ['missing raw proof', u => { delete u.evidence.statuses }],
  ['bad raw index', u => { u.proof.cases[1].rawStatusIndex = 99 }], ['old restart index', u => { u.proof.cases[5].rawStatusIndex = 0 }],
  ['unapplied raw', u => { u.evidence.statuses[0].enforcementConfirmed = false }], ['wrong raw policy', u => { u.evidence.statuses[0].appliedPolicyHash = 'f'.repeat(64) }],
  ['no confirmation', u => { u.report.confirmations = [] }], ['missing receipt', u => { delete u.proof.cases[1].controllerReceipt }],
  ['wrong typed snapshot', u => { u.proof.cases[1].controllerReceipt.reportSHA256 = 'f'.repeat(64) }],
  ['disconnected receipt', u => { u.proof.cases[1].controllerReceipt.connection = 'disconnected' }],
  ['fabricated disconnected correlation', u => { u.proof.cases[0].rawStatusIndex = 0 }],
]) test(`CLI gate rejects ${name} without disclosing input`, () => {
  const u = unit(); mutate(u)
  assert.throws(() => validateCLIDiagnostics(u.proof, u.d, u.report, u.evidence), e => e.message === 'invalid installed CLI evidence')
})

async function processFixture(t, mode = 'valid') {
  const directory = await mkdtemp(join(tmpdir(), 'picketx-cli-unit-'))
  t.after(() => rm(directory, { recursive: true, force: true }))
  const binary = join(directory, 'unit-cli'), u = unit(), row = u.proof.cases[mode === 'applied' ? 1 : 0]
  await writeFile(binary, `#!${process.execPath}\nconst mode=process.env.UNIT_MODE;const doctor=process.argv[2]==='doctor';
if(mode==='timeout')setTimeout(()=>{},20000);
else if(mode==='signal')process.kill(process.pid,'SIGTERM');
else if(mode==='overflow')process.stdout.write('private-marker'.repeat(10000));
else if(mode==='malformed'){process.stdout.write('private-marker');process.exitCode=1;}
else {const args=process.argv.slice(3);if(JSON.stringify(args)!==JSON.stringify(['--controller','http://127.0.0.1:18080','--key-file',${JSON.stringify(join(directory, 'bootstrap-key'))},'--agent','node','--format','json','--timeout','5s']))throw Error('private-marker');process.stdout.write(JSON.stringify(doctor?${JSON.stringify(row.doctor)}:${JSON.stringify(row.status)}));process.exitCode=mode==='exit2'?2:(doctor?${row.doctorExit}:0);}
`, { mode: 0o700 })
  return { cliBinary: mode === 'launch' ? join(directory, 'missing-private-marker') : binary, instance: directory,
    authority: u.d.authority, runtimeOptions: { env: { PATH: '', UNIT_MODE: mode } } }
}
test('actual unit child boundary preserves observed degraded exit and public report', async t => {
  const installed = await processFixture(t)
  const result = await inspectInstalledCLI(installed, 'http://127.0.0.1:18080', { phase: 'before-agent', revision: 2, state: 'disconnected' })
  assert.equal(result.statusExit, 0); assert.equal(result.doctorExit, 1); assert.equal(result.doctor.ready, true)
})
test('actual unit child boundary preserves healthy doctor exit without enforcement assertion', async t => {
  const installed = await processFixture(t, 'applied')
  const result = await inspectInstalledCLI(installed, 'http://127.0.0.1:18080', { phase: 'applied', revision: 2, state: 'reported-applied' })
  assert.equal(result.doctorExit, 0); assert.equal(result.doctor.enforcement_confirmed, false)
})
for (const mode of ['launch', 'signal', 'overflow', 'malformed', 'exit2', 'timeout']) test(`actual unit child ${mode} is not a degraded doctor result`, async t => {
  const installed = await processFixture(t, mode)
  await assert.rejects(inspectInstalledCLI(installed, 'http://127.0.0.1:18080', { phase: 'before-agent', revision: 2, state: 'disconnected' }), e => e.message === 'installed CLI inspection failed')
})
