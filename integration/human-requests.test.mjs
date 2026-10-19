// SPDX-License-Identifier: GPL-3.0-or-later
import test from 'node:test'
import assert from 'node:assert/strict'
import { assertDeadline } from './human-requests.mjs'

test('deadline proof accepts equivalent Go RFC3339Nano and browser ISO precision', () => {
  assert.doesNotThrow(() => assertDeadline('2026-10-19T09:00:30.01Z', '2026-10-19T09:00:30.010Z'))
  assert.doesNotThrow(() => assertDeadline('2026-10-19T09:00:30Z', '2026-10-19T09:00:30.000Z'))
})
for (const value of ['2026-10-19T09:00:29Z', '2026-10-19T09:00:31Z', '', 'invalid']) {
  test(`deadline proof rejects changed/invalid instant ${JSON.stringify(value)}`, () => {
    assert.throws(() => assertDeadline(value, '2026-10-19T09:00:30.000Z'))
  })
}
