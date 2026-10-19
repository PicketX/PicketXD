// SPDX-License-Identifier: GPL-3.0-or-later
// Build-only tooling; retains verbatim notices, not automated legal clearance.
import assert from 'node:assert/strict'
import { cp, mkdir, readFile, readdir, lstat, realpath, writeFile } from 'node:fs/promises'
import { execFileSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const digest = bytes => createHash('sha256').update(bytes).digest('hex')
export function validateAgentDistributionEvidence(proof, report) {
  assert(proof && typeof proof === 'object' && !Array.isArray(proof), 'actual Agent distribution proof required')
  assert.deepEqual(Object.keys(proof).sort(), ['source', 'manifestSHA256', 'binarySHA256', 'files', 'offlineBuild', 'offlineTests',
    'relocated', 'runtimePATH', 'preparationOnly', 'originalPrepared', 'rebuiltPrepared', 'enforcementConfirmed',
    'originalPolicyHash', 'rebuiltPolicyHash'].sort(), 'public distribution fields only')
  assert.deepEqual(Object.keys(proof.source).sort(), ['commit', 'go', 'platform', 'tree'])
  assert.equal(proof.source.commit, report.agentCommit); assert.equal(proof.source.tree, report.agentTree)
  assert.equal(proof.source.platform, 'linux-amd64'); assert.match(proof.source.go, /^go1\.27\.\d+$/)
  for (const field of ['manifestSHA256', 'binarySHA256', 'originalPolicyHash', 'rebuiltPolicyHash']) assert.match(proof[field], /^[a-f0-9]{64}$/)
  assert.equal(proof.binarySHA256, report.agentBinarySHA256, 'shipped binary must be the running Agent')
  assert(Number.isSafeInteger(proof.files) && proof.files > 0)
  for (const field of ['offlineBuild', 'offlineTests', 'relocated', 'preparationOnly', 'originalPrepared', 'rebuiltPrepared']) assert.equal(proof[field], true, field)
  assert.equal(proof.runtimePATH, ''); assert.equal(proof.enforcementConfirmed, false, 'preparation is not enforcement')
  assert.equal(proof.originalPolicyHash, proof.rebuiltPolicyHash)
}
const safe = path => !path.includes('\\') && path.split('/').every(p => p && p !== '.' && p !== '..')
async function walk(root, prefix = '') {
  const files = []
  for (const name of (await readdir(join(root, prefix))).sort()) {
    assert(name !== '.git' && name !== 'node_modules', 'no checkout/build credentials in bundle')
    const path = prefix + name, info = await lstat(join(root, path))
    assert(!info.isSymbolicLink(), 'no symlink in bundle')
    if (info.isDirectory()) files.push(...await walk(root, path + '/'))
    else { assert(info.isFile(), 'regular bundle files only'); files.push(path) }
  }
  return files
}
export async function verifyAgentBundle(directory) {
  directory = resolve(directory)
  assert.equal(await realpath(directory), directory, 'canonical nonsymlink bundle root')
  const files = await walk(directory), covered = new Set(), manifest = await readFile(join(directory, 'SHA256SUMS'))
  for (const line of manifest.toString().trimEnd().split('\n')) {
    const m = /^([a-f0-9]{64})  (.+)$/.exec(line)
    assert(m && safe(m[2]) && m[2] !== 'SHA256SUMS', 'safe manifest entry')
    const [, hash, path] = m
    assert(!covered.has(path), 'duplicate manifest entry'); covered.add(path)
    assert(files.includes(path), 'listed regular file exists')
    assert.equal(digest(await readFile(join(directory, path))), hash, 'bundle checksum')
  }
  assert.deepEqual([...covered].sort(), files.filter(f => f !== 'SHA256SUMS').sort(), 'every bundle file covered')
  for (const file of ['bin/picketxd', 'SOURCE.json', 'LICENSE', 'THIRD_PARTY_NOTICES.md', 'licenses/index.json',
    'source/PicketXD/go.mod', 'source/PicketXD/go.sum', 'source/PicketXD/LICENSE',
    'source/PicketXD/cmd/picketxd/main.go', 'source/PicketXD/vendor/modules.txt',
    'deploy/README.md', 'deploy/README.zh-CN.md']) assert(covered.has(file), 'required bundle content: ' + file)
  assert((await lstat(join(directory, 'bin/picketxd'))).mode & 0o111, 'binary executable mode')
  const source = JSON.parse(await readFile(join(directory, 'SOURCE.json'), 'utf8'))
  assert.deepEqual(Object.keys(source).sort(), ['commit', 'go', 'platform', 'tree'])
  assert.match(source.commit, /^[a-f0-9]{40}$/); assert.match(source.tree, /^[a-f0-9]{40}$/)
  assert.equal(source.platform, 'linux-amd64'); assert.match(source.go, /^go1\.27\.\d+$/)
  const index = JSON.parse(await readFile(join(directory, 'licenses/index.json'), 'utf8'))
  assert(Array.isArray(index) && index.length > 0, 'upstream notice index')
  const dependencies = new Set()
  for (const item of index) {
    assert(typeof item.dependency === 'string' && item.dependency && !dependencies.has(item.dependency), 'distinct notice dependency')
    dependencies.add(item.dependency)
    assert(typeof item.version === 'string' && item.version && Array.isArray(item.files) && item.files.length, 'versioned notice source')
    for (const path of item.files) assert(typeof path === 'string' && path.startsWith('licenses/') && safe(path) && covered.has(path), 'retained upstream text')
  }
  return { source, manifestSHA256: digest(manifest), binarySHA256: digest(await readFile(join(directory, 'bin/picketxd'))), files: covered.size }
}
const run = args => execFileSync('go', args, { encoding: 'utf8', maxBuffer: 16 << 20 }).trim()
const notice = /^(license|licence|copying|copyright|notice|patents)(?:[._-].*)?$|^third[-_ ]party.*(?:licen[sc]e|notice)/i
async function notices(root, prefix = '', all = false) {
  const files = []
  for (const entry of await readdir(join(root, prefix), { withFileTypes: true })) {
    if (entry.isSymbolicLink() || entry.name === '.git' || entry.name === 'node_modules') continue
    const path = prefix + entry.name
    if (entry.isDirectory()) files.push(...await notices(root, path + '/', all || /^licen[sc]es?$/i.test(entry.name)))
    else if (entry.isFile() && (all || notice.test(entry.name))) files.push(path)
  }
  return files.sort()
}
async function collect(directory) {
  await mkdir(join(directory, 'licenses')) // Exclusive: never replace a prior collection.
  const index = [], modules = await readFile(join(directory, 'source/PicketXD/vendor/modules.txt'), 'utf8')
  for (const match of modules.matchAll(/^# (\S+) (v\S+)$/gm)) {
    const [, dependency, version] = match, root = run(['list', '-m', '-f', '{{.Dir}}', dependency])
    assert(root, 'actual module source directory: ' + dependency)
    const files = await notices(root); assert(files.length, 'actual upstream notices: ' + dependency)
    const retained = []
    for (const file of files) {
      const path = join('licenses/go', dependency + '@' + version, file)
      await mkdir(join(directory, path, '..'), { recursive: true })
      await cp(join(root, file), join(directory, path), { errorOnExist: true, force: false }); retained.push(path)
    }
    index.push({ dependency, version, files: retained })
  }
  assert(index.length > 0, 'vendored dependencies required')
  await mkdir(join(directory, 'licenses/go-toolchain'))
  const goroot = run(['env', 'GOROOT'])
  for (const name of ['LICENSE', 'PATENTS']) await cp(join(goroot, name), join(directory, 'licenses/go-toolchain', name))
  index.push({ dependency: 'Go toolchain/runtime', version: run(['env', 'GOVERSION']), files: ['licenses/go-toolchain/LICENSE', 'licenses/go-toolchain/PATENTS'] })
  await writeFile(join(directory, 'licenses/index.json'), JSON.stringify(index, null, 2) + '\n')
}
async function seal(directory) {
  const files = await walk(directory)
  assert(!files.includes('SHA256SUMS'), 'never overwrite manifest')
  const lines = []
  for (const path of files.sort()) { assert(safe(path), 'safe bundle filename'); lines.push(`${digest(await readFile(join(directory, path)))}  ${path}\n`) }
  await writeFile(join(directory, 'SHA256SUMS'), lines.join(''), { flag: 'wx' })
  console.log(JSON.stringify(await verifyAgentBundle(directory)))
}
if (resolve(process.argv[1] ?? '') === fileURLToPath(import.meta.url)) {
  const [command, directory] = process.argv.slice(2)
  assert(directory && process.argv.length === 4, 'usage: agent-bundle.mjs collect|seal|verify DIRECTORY')
  if (command === 'collect') await collect(resolve(directory))
  else if (command === 'seal') await seal(resolve(directory))
  else { assert.equal(command, 'verify'); console.log(JSON.stringify(await verifyAgentBundle(directory))) }
}
