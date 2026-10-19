// SPDX-License-Identifier: GPL-3.0-or-later
import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import test from 'node:test'
test('Controller service public evidence rejects forged process, cache, expiry, receipt and packet proof', () => {
 assert.doesNotThrow(() => execFileSync('python3', ['scripts/test-controller-service.py', '--self-test'], { cwd: new URL('..', import.meta.url), timeout: 10000, stdio: 'pipe' }))
})
