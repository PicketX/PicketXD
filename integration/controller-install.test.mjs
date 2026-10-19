// SPDX-License-Identifier: GPL-3.0-or-later
import assert from 'node:assert/strict'
import test from 'node:test'
import { mkdtemp, mkdir, writeFile, readFile, rm, symlink } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { createHash } from 'node:crypto'
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
