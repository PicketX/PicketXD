// SPDX-License-Identifier: GPL-3.0-or-later
// Real process/filesystem failure boundary tests, not successful Core/native proof.
import test from 'node:test'
import assert from 'node:assert/strict'
import { mkdtemp, readFile, rm } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { createHash } from 'node:crypto'
import { spawnSync } from 'node:child_process'

const adapter = new URL('./installed-ingress.mjs', import.meta.url).href
const cliSHA256 = createHash('sha256').update(await readFile(process.execPath)).digest('hex')
for (const failure of ['exit', 'cwd', 'read', 'identity']) {
  test(`preparation ${failure} failure publishes no private paths or raw error fields`, async () => {
    const directory = await mkdtemp(join(tmpdir(), 'px-ingress-private-unit-'))
    try {
      const installed = { cliBinary: failure === 'read' ? join(directory, 'missing-cli') : process.execPath,
        proof: { cliSHA256: failure === 'identity' ? '0'.repeat(64) : cliSHA256 },
        instance: join(directory, 'instance'), ingressTokenFile: join(directory, 'ingress-token'),
        runtimeOptions: { cwd: failure === 'cwd' ? join(directory, 'missing-cwd') : directory, env: { PATH: '' } } }
      const source = `import {prepareInstalledIngress} from ${JSON.stringify(adapter)};
        import {inspect} from 'node:util';
        try { await prepareInstalledIngress(JSON.parse(process.argv[1])); process.exitCode=3; }
        catch(error) { console.log(JSON.stringify({name:error.name, keys:Reflect.ownKeys(error),
          diagnostic:inspect(error,{showHidden:true,depth:5})})); process.exitCode=1; }`
      const child = spawnSync(process.execPath, ['--input-type=module', '-e', source, JSON.stringify(installed)],
        { encoding: 'utf8', timeout: 15000, env: { PATH: '' } })
      assert.equal(child.status, 1, 'actual adapter failure must not become success')
      assert.equal(child.signal, null)
      const captured = child.stdout + child.stderr
      assert(!captured.includes(directory), 'neither caller stream nor Error may disclose private directory')
      assert(!captured.includes('--token-file'), 'command arguments must not escape')
      const error = JSON.parse(child.stdout)
      assert.equal(error.name, 'Error')
      assert.deepEqual(error.keys.sort(), ['message', 'stack'], 'no cause/raw subprocess fields, including hidden fields')
      assert.equal(child.stderr, '', 'no subprocess stderr forwarded outside safe failure boundary')
    } finally { await rm(directory, { recursive: true, force: true }) }
  })
}
