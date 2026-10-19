// SPDX-License-Identifier: GPL-3.0-or-later
import assert from 'node:assert/strict'
import test from 'node:test'
import { mkdtemp, mkdir, writeFile, readFile, rm, symlink, copyFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join, resolve } from 'node:path'
import { createHash } from 'node:crypto'
import { spawnSync, execFileSync } from 'node:child_process'

const root = resolve(import.meta.dirname, '..')
const hash = s => createHash('sha256').update(s).digest('hex')
const source = { commit: 'a'.repeat(40), tree: 'b'.repeat(40), platform: 'linux-amd64', go: 'go1.27.0' }
async function fixture(t) {
  const dir = await mkdtemp(join(tmpdir(), 'agent-bundle-test-'))
  t.after(() => rm(dir, { recursive: true, force: true }))
  const files = {
    'bin/picketxd': 'unit fixture; never executed', 'SOURCE.json': JSON.stringify(source),
    LICENSE: 'GPL-3.0-or-later fixture', 'THIRD_PARTY_NOTICES.md': 'notices fixture',
    'licenses/index.json': JSON.stringify([{ dependency: 'fixture', version: 'v1', files: ['licenses/fixture/LICENSE'] }]),
    'licenses/fixture/LICENSE': 'upstream notice fixture',
    'source/PicketXD/go.mod': 'module github.com/PicketX/PicketXD\ngo 1.27.0\n',
    'source/PicketXD/go.sum': 'checksums fixture', 'source/PicketXD/cmd/picketxd/main.go': 'package main',
    'source/PicketXD/vendor/modules.txt': '# fixture v1\n',
    'source/PicketXD/LICENSE': 'GPL-3.0-or-later fixture',
    'deploy/README.md': 'English', 'deploy/README.zh-CN.md': '中文',
  }
  for (const [path, bytes] of Object.entries(files)) {
    await mkdir(join(dir, path, '..'), { recursive: true })
    await writeFile(join(dir, path), bytes, { mode: path === 'bin/picketxd' ? 0o755 : 0o644 })
  }
  await writeFile(join(dir, 'SHA256SUMS'), Object.keys(files).sort().map(path => `${hash(files[path])}  ${path}\n`).join(''))
  return dir
}
// These tests catch a verifier that accepts extra/tampered content or unsafe paths;
// the literal fixture is never represented as an executable or native proof.
test('bundle verifier returns verified source and actual binary digest', async t => {
  const dir = await fixture(t), { verifyAgentBundle } = await import('../scripts/agent-bundle.mjs')
  const proof = await verifyAgentBundle(dir)
  assert.deepEqual(proof.source, source)
  assert.equal(proof.binarySHA256, hash('unit fixture; never executed'))
  assert.equal(proof.files, 13)
})
for (const [name, change] of [
  ['changed binary', d => writeFile(join(d, 'bin/picketxd'), 'tampered')],
  ['unlisted file', d => writeFile(join(d, 'unlisted'), 'not covered')],
  ['missing source', d => rm(join(d, 'source/PicketXD/go.mod'))],
  ['symlinked binary', async d => { await rm(join(d, 'bin/picketxd')); await symlink('/bin/true', join(d, 'bin/picketxd')) }],
  ['duplicate manifest entry', async d => { const p = join(d, 'SHA256SUMS'), s = await readFile(p, 'utf8'); await writeFile(p, s + s.split('\n')[0] + '\n') }],
  ['traversal manifest path', d => writeFile(join(d, 'SHA256SUMS'), `${'0'.repeat(64)}  ../outside\n`)],
]) test('bundle rejects ' + name, async t => {
  const dir = await fixture(t), { verifyAgentBundle } = await import('../scripts/agent-bundle.mjs')
  await change(dir); await assert.rejects(verifyAgentBundle(dir))
})
for (const kind of ['file', 'directory', 'dangling-symlink', 'noncanonical', 'dirty-source']) {
  test('builder preserves operator paths and rejects ' + kind + ' before build', async t => {
    const dir = await mkdtemp(join(tmpdir(), 'agent-build-guard-'))
    t.after(() => rm(dir, { recursive: true, force: true }))
    await mkdir(join(dir, 'repo/scripts'), { recursive: true })
    await copyFile(join(root, 'scripts/build-agent-bundle.sh'), join(dir, 'repo/scripts/build-agent-bundle.sh'))
    const cwd = join(dir, 'repo'), git = args => execFileSync('git', args, { cwd, stdio: 'ignore' })
    git(['init', '-q']); git(['add', '.']); git(['-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.invalid', 'commit', '-qm', 'fixture'])
    let target = join(dir, 'bundle')
    if (kind === 'file') await writeFile(target, 'operator data')
    if (kind === 'directory') { await mkdir(target); await writeFile(join(target, 'keep'), 'operator data') }
    if (kind === 'dangling-symlink') await symlink(join(dir, 'missing'), target)
    if (kind === 'noncanonical') target = dir + '/repo/../bundle'
    if (kind === 'dirty-source') await writeFile(join(cwd, 'unknown.go'), 'uncommitted')
    const p = spawnSync('bash', ['scripts/build-agent-bundle.sh', target], { cwd, encoding: 'utf8' })
    assert.equal(p.status, 2, p.stderr)
    if (kind === 'file') assert.equal(await readFile(target, 'utf8'), 'operator data')
    if (kind === 'directory') assert.equal(await readFile(join(target, 'keep'), 'utf8'), 'operator data')
    if (kind === 'dirty-source' || kind === 'noncanonical') await assert.rejects(readFile(join(dir, 'bundle/bin/picketxd')))
  })
}

const distribution = () => ({ source, manifestSHA256: 'c'.repeat(64), binarySHA256: 'd'.repeat(64), files: 13,
  offlineBuild: true, offlineTests: true, relocated: true, runtimePATH: '', preparationOnly: true,
  originalPrepared: true, rebuiltPrepared: true, enforcementConfirmed: false,
  originalPolicyHash: 'e'.repeat(64), rebuiltPolicyHash: 'e'.repeat(64) })
const nativeReport = () => ({ agentCommit: source.commit, agentTree: source.tree, agentBinarySHA256: 'd'.repeat(64) })
for (const [name, mutate] of [
  ['missing proof', () => undefined],
  ['unverified offline build', p => ({ ...p, offlineBuild: false })],
  ['untested offline source', p => ({ ...p, offlineTests: false })],
  ['source revision mismatch', p => ({ ...p, source: { ...source, commit: 'f'.repeat(40) } })],
  ['runtime binary substitution', p => ({ ...p, binarySHA256: 'f'.repeat(64) })],
  ['inherited runtime path', p => ({ ...p, runtimePATH: '/usr/bin' })],
  ['preparation called enforcement', p => ({ ...p, enforcementConfirmed: true })],
  ['different rebuilt policy', p => ({ ...p, rebuiltPolicyHash: 'f'.repeat(64) })],
  ['private extra fields', p => ({ ...p, token: 'private' })],
]) test('native distribution gate rejects ' + name, async () => {
  const { validateAgentDistributionEvidence } = await import('../scripts/agent-bundle.mjs')
  assert.throws(() => validateAgentDistributionEvidence(mutate(distribution()), nativeReport()))
})
test('native distribution gate accepts bound build/source/preparation proof', async () => {
  const { validateAgentDistributionEvidence } = await import('../scripts/agent-bundle.mjs')
  validateAgentDistributionEvidence(distribution(), nativeReport())
})

// Real small Go repositories, not fake native-Agent acceptance. Each mutation
// changes observable compiled code or ISA while the claimed Git commit stays clean.
for (const mutation of ['ignored source', 'external overlay', 'ambient ISA']) {
  test('builder compiles only committed source with baseline ISA despite ' + mutation, async t => {
    const dir = await mkdtemp(join(tmpdir(), 'agent-source-identity-'))
    t.after(() => rm(dir, { recursive: true, force: true }))
    const cwd = join(dir, 'repo'), output = join(dir, 'bundle')
    for (const file of ['scripts/build-agent-bundle.sh', 'scripts/agent-bundle.mjs', 'LICENSE', 'THIRD_PARTY_NOTICES.md', 'go.sum',
      'deploy/agent/README.md', 'deploy/agent/README.zh-CN.md', 'deploy/systemd/picketxd.service', 'deploy/systemd/README.md', 'deploy/systemd/README.zh-CN.md']) {
      await mkdir(join(cwd, file, '..'), { recursive: true }); await copyFile(join(root, file), join(cwd, file))
    }
    await mkdir(join(cwd, 'cmd/picketxd'), { recursive: true })
    await writeFile(join(cwd, 'go.mod'), 'module github.com/PicketX/PicketXD\n\ngo 1.27.0\n\nrequire github.com/spf13/pflag v1.0.9\n')
    const main = join(cwd, 'cmd/picketxd/main.go')
    await writeFile(main, 'package main\nimport("fmt";_ "github.com/spf13/pflag")\nfunc main(){fmt.Println("COMMITTED")}\n')
    const git = args => execFileSync('git', args, { cwd, encoding: 'utf8' }).trim()
    git(['init', '-q']); git(['add', '.']); git(['-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.invalid', 'commit', '-qm', 'committed fixture'])
    const env = { ...process.env, GOENV: 'off', GOWORK: 'off', GOFLAGS: '', GOAMD64: 'v1' }
    if (mutation === 'ignored source') {
      await writeFile(join(cwd, '.git/info/exclude'), 'cmd/picketxd/ignored.go\n')
      await writeFile(join(cwd, 'cmd/picketxd/ignored.go'), 'package main\nimport "fmt"\nfunc init(){fmt.Println("IGNORED_INPUT")}\n')
    } else if (mutation === 'external overlay') {
      const replacement = join(dir, 'replacement.go'), overlay = join(dir, 'overlay.json')
      await writeFile(replacement, 'package main\nimport("fmt";_ "github.com/spf13/pflag")\nfunc main(){fmt.Println("OVERLAY_INPUT")}\n')
      await writeFile(overlay, JSON.stringify({ Replace: { [main]: replacement } }))
      env.GOFLAGS = '-overlay=' + overlay
    } else env.GOAMD64 = 'v3'
    assert.equal(git(['status', '--porcelain', '--untracked-files=all']), '', 'contamination invisible to ordinary Git status')
    const p = spawnSync('bash', ['scripts/build-agent-bundle.sh', output], { cwd, env, encoding: 'utf8', timeout: 120000 })
    assert.equal(p.status, 0, p.stderr)
    const binary = join(output, 'bin/picketxd'), build = execFileSync('go', ['version', '-m', binary], { encoding: 'utf8' })
    assert(build.includes('GOAMD64=v1'), 'portable baseline ISA, not ambient host selection')
    assert(build.includes('vcs.revision=' + git(['rev-parse', 'HEAD'])) && build.includes('vcs.modified=false'))
    assert.equal(execFileSync(binary, [], { cwd: dir, env: { PATH: '' }, encoding: 'utf8' }), 'COMMITTED\n')
  })
}
