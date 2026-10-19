// SPDX-License-Identifier: GPL-3.0-or-later
import test from 'node:test'
import assert from 'node:assert/strict'
import { spawn } from 'node:child_process'
import { once } from 'node:events'
import { assertServicesAlive } from './process-health.mjs'

test('signal-terminated actual service cannot establish policy denial', async () => {
  const child = spawn(process.execPath, ['-e', 'setInterval(() => {}, 1000)'], { stdio: 'ignore' })
  await once(child, 'spawn')
  try {
    assert.doesNotThrow(() => assertServicesAlive([child]))
    const ended = once(child, 'exit'); child.kill('SIGTERM'); await ended
    assert.equal(child.exitCode, null); assert.equal(child.signalCode, 'SIGTERM')
    assert.throws(() => assertServicesAlive([child]))
  } finally { if (child.exitCode === null && child.signalCode === null) child.kill('SIGKILL') }
})
