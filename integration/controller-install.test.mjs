// SPDX-License-Identifier: GPL-3.0-or-later
import assert from 'node:assert/strict'
import test from 'node:test'
import { mkdtemp, mkdir, writeFile, readFile, rm, symlink } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { createHash } from 'node:crypto'
import { execFileSync, spawnSync } from 'node:child_process'
import * as installed from './controller-install.mjs'
import * as gate from './console-evidence.mjs'

// Filesystem unit data only, never executed or emitted as native artifact proof.
async function fixture(run) {
  const dir = await mkdtemp(join(tmpdir(), 'px-installed-unit-'))
  try {
    await mkdir(join(dir, 'bin')); await mkdir(join(dir, 'web'))
    const files = { 'bin/picketx': 'unit cli', 'bin/picketx-controller': 'unit Controller', 'web/index.html': 'unit Web',
      'SOURCE.json': JSON.stringify({ commit: gate.coreCommit, tree: '64ed0dcc4b6a3232e093150d7fc100cc9a852e43', platform: 'linux-amd64' }) }
    let manifest = ''
    for (const [path, body] of Object.entries(files)) {
      await writeFile(join(dir, path), body, { mode: path.startsWith('bin/') ? 0o755 : 0o644 })
      manifest += createHash('sha256').update(body).digest('hex') + '  ' + path + '\n'
    }
    await writeFile(join(dir, 'SHA256SUMS'), manifest)
    await run(dir, manifest)
  } finally { await rm(dir, { recursive: true, force: true }) }
}
test('actual manifest-covered regular files are verified before execution', async () => {
  assert.equal(typeof gate.verifyControllerBundle, 'function', 'installed artifact verifier required')
  await fixture(async dir => {
    const verified = await gate.verifyControllerBundle(dir)
    assert.equal(verified.source.commit, gate.coreCommit)
    assert.equal(verified.controllerSHA256, createHash('sha256').update('unit Controller').digest('hex'))
  })
})
for (const [name, change] of [
  ['tampered Web', async dir => writeFile(join(dir, 'web/index.html'), 'changed')],
  ['unlisted extra file', async dir => writeFile(join(dir, 'web/extra.js'), 'not covered')],
  ['traversal', async (dir, m) => writeFile(join(dir, 'SHA256SUMS'), m + 'a'.repeat(64) + '  ../outside\n')],
  ['duplicate manifest entry', async (dir, m) => writeFile(join(dir, 'SHA256SUMS'), m + m.split('\n')[0] + '\n')],
  ['missing mandatory executable', async (dir, m) => writeFile(join(dir, 'SHA256SUMS'), m.split('\n').filter(l => !l.endsWith('bin/picketx')).join('\n'))],
  ['symlink asset', async dir => { await rm(join(dir, 'web/index.html')); await symlink('../SOURCE.json', join(dir, 'web/index.html')) }],
]) test(`installed artifact refuses ${name}`, async () => {
  assert.equal(typeof gate.verifyControllerBundle, 'function')
  await fixture(async (dir, m) => { await change(dir, m); await assert.rejects(() => gate.verifyControllerBundle(dir)) })
})

test('restart comparison ignores only fresh read timestamps, preserving full business state', () => {
  assert.equal(typeof installed.businessViewDigest, 'function', 'business-view comparison required')
  const grants = { authority: 'controller:unit', revision: 4, observed_at: '2026-10-19T14:00:00Z',
    grants: [{ uid: 'grant4', expires_at: '2026-10-19T15:00:00Z', revoked: false, scopes: [{ endpoint_names: ['web'] }] }],
    resources: [{ uid: 'resource1', endpoints: [{ name: 'web', port: 18445 }] }], has_more: false, next_after: 'grant4' }
  const accounts = { mode: 'database', revision: 2, observedAt: '2026-10-19T14:00:00Z',
    accounts: [{ id: 'person', roles: ['role:admin'], version: 1, disabled: false }],
    events: [{ at: '2026-10-19T13:00:00Z', actor: 'principal:bootstrap', operation: 'create' }] }
  for (const [name, view, field] of [['grants', grants, 'observed_at'], ['accounts', accounts, 'observedAt']]) {
    const original = structuredClone(view), later = { ...view, [field]: '2026-10-19T14:01:00Z' }
    assert.equal(installed.businessViewDigest(name, later), installed.businessViewDigest(name, view))
    assert.deepEqual(view, original, 'comparison must not mutate actual API evidence')
  }
  for (const change of [g => { g.grants[0].expires_at = '2026-10-19T15:01:00Z' },
    g => { g.grants[0].revoked = true }, g => { g.grants[0].scopes[0].endpoint_names.push('ssh') },
    g => { g.resources[0].endpoints[0].port = 22 }, g => { g.revision++ }]) {
    const changed = structuredClone(grants); change(changed)
    assert.notEqual(installed.businessViewDigest('grants', changed), installed.businessViewDigest('grants', grants))
  }
  for (const change of [a => { a.accounts[0].disabled = true }, a => { a.accounts[0].roles = [] },
    a => { a.events[0].at = '2026-10-19T13:01:00Z' }, a => { a.revision++ }]) {
    const changed = structuredClone(accounts); change(changed)
    assert.notEqual(installed.businessViewDigest('accounts', changed), installed.businessViewDigest('accounts', accounts))
  }
  for (const name of ['publication', 'audit']) {
    assert.notEqual(installed.businessViewDigest(name, { observed_at: 'before' }), installed.businessViewDigest(name, { observed_at: 'after' }))
  }
})

test('pinned secondary CI checkout is excluded without hiding unreviewed Agent source', async () => {
  const dir = await mkdtemp(join(tmpdir(), 'px-ci-source-unit-'))
  try {
    execFileSync('git', ['init', '--quiet', dir])
    await writeFile(join(dir, '.gitignore'), await readFile(new URL('../.gitignore', import.meta.url)))
    await mkdir(join(dir, '.ci-core')); await writeFile(join(dir, '.ci-core/go.mod'), 'unit pinned secondary checkout')
    await writeFile(join(dir, 'unreviewed-agent.go'), 'package unit')
    assert.equal(spawnSync('git', ['-C', dir, 'check-ignore', '--', '.ci-core/go.mod']).status, 0)
    assert.equal(spawnSync('git', ['-C', dir, 'check-ignore', '--', 'unreviewed-agent.go']).status, 1)
  } finally { await rm(dir, { recursive: true, force: true }) }
})
